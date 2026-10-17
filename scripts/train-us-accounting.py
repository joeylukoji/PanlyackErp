"""Train a bounded synthetic US purchase/sale/cash/bank role LoRA with local retrieval."""
import argparse
import hashlib
import importlib.util
import json
import os
import random
import sys
from pathlib import Path

spec = importlib.util.spec_from_file_location("vision_training", Path(__file__).with_name("train-light-invoices.py"))
training = importlib.util.module_from_spec(spec)
spec.loader.exec_module(training)
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "ai-service"))
from app.accounting_advisor import AccountingFacts, AccountRoles, prompt


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--dataset", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--epochs", type=int, default=6)
    args = parser.parse_args()
    if not 1 <= args.epochs <= 12:
        raise ValueError("Epoch count must be between 1 and 12")
    args.output.mkdir(parents=True, exist_ok=False)
    data = args.dataset.read_bytes()
    rows = json.loads(data)
    groups = {split: [r for r in rows if r["split"] == split] for split in ("train", "validation", "test")}
    if not all(groups.values()) or sum(map(len, groups.values())) != len(rows):
        raise ValueError("Expected nonempty train, validation and test partitions")
    products = {split: {p for r in items for p in r["facts"]["products"]} for split, items in groups.items()}
    if any(products[a] & products[b] for a, b in (("train", "validation"), ("train", "test"), ("validation", "test"))):
        raise ValueError("Product reference leakage")
    for row in rows:
        AccountingFacts.model_validate(row["facts"])
        AccountRoles.model_validate(row["target"])
    os.environ["ACCOUNTING_RAG_EXAMPLES_PATH"] = str(args.dataset.resolve())
    import torch
    from huggingface_hub import hf_hub_download
    from peft import LoraConfig, get_peft_model
    from transformers import AutoModelForVision2Seq, AutoProcessor
    assert torch.cuda.is_available()
    torch.set_num_threads(2)
    torch.manual_seed(42)
    weights = Path(hf_hub_download(training.MODEL, "model.safetensors", revision=training.REVISION, local_files_only=True))
    with weights.open("rb") as stream:
        assert hashlib.file_digest(stream, "sha256").hexdigest() == "74dea5904032e5ae99a2e0eef5179e6ac0f1dedc3ab0c7c2a5d4d387c843203e"
    processor = AutoProcessor.from_pretrained(training.MODEL, revision=training.REVISION, local_files_only=True)
    model = AutoModelForVision2Seq.from_pretrained(training.MODEL, revision=training.REVISION, local_files_only=True,
                                                  torch_dtype=torch.bfloat16, attn_implementation="sdpa").to("cuda")
    model = get_peft_model(model, LoraConfig(r=8, lora_alpha=16, lora_dropout=.05,
                           target_modules=r".*text_model.*\.(q_proj|v_proj)$", task_type="CAUSAL_LM"))
    model.config.use_cache = False
    model.print_trainable_parameters()

    def encoded(row, with_target=True):
        messages = [{"role": "user", "content": [{"type": "text", "text": prompt(AccountingFacts.model_validate(row["facts"]))}]}]
        prefix = processor.apply_chat_template(messages, add_generation_prompt=True)
        if not with_target:
            return processor.tokenizer(prefix, return_tensors="pt").to("cuda")
        messages.append({"role": "assistant", "content": [{"type": "text", "text": json.dumps(row["target"])}]})
        full = processor.apply_chat_template(messages, add_generation_prompt=False)
        inputs = processor.tokenizer(full, return_tensors="pt")
        assert inputs.input_ids.shape[1] <= 1536
        labels = inputs.input_ids.clone()
        labels[:, :processor.tokenizer(prefix, return_tensors="pt").input_ids.shape[1]] = -100
        assert (labels != -100).any()
        inputs["labels"] = labels
        return inputs.to("cuda")

    def validation_loss():
        model.eval()
        with torch.no_grad():
            value = sum(model(**encoded(row)).loss.item() for row in groups["validation"]) / len(groups["validation"])
        model.train()
        return value

    report = {"model": training.MODEL, "revision": training.REVISION, "dataset_sha256": hashlib.sha256(data).hexdigest(),
              "train": len(groups["train"]), "validation": len(groups["validation"]), "reserved_test": len(groups["test"]),
              "source": "synthetic Go US purchase/sales/settlement and balanced cash-bank cases; explicit tax treatment; not accountant-reviewed", "production_ready": False,
              "prompt_version": 2, "rag_examples": "train partition only; structural examples repeat across product holdouts",
              "baseline_validation_loss": validation_loss(), "epochs": [], "best_validation_loss": None}
    context_path = os.getenv("ACCOUNTING_PUBLIC_CONTEXT_PATH", "")
    report["public_context_sha256"] = hashlib.sha256(Path(context_path).read_bytes()).hexdigest() if context_path else None
    (args.output / "dataset.json").write_bytes(data)
    optimizer = torch.optim.AdamW((p for p in model.parameters() if p.requires_grad), lr=2e-4)
    for epoch in range(args.epochs):
        epoch_rows = list(groups["train"])
        random.Random(42 + epoch).shuffle(epoch_rows)
        optimizer.zero_grad()
        total = 0
        for step, row in enumerate(epoch_rows):
            loss = model(**encoded(row)).loss
            assert torch.isfinite(loss)
            (loss / 4).backward()
            total += loss.item()
            if (step + 1) % 4 == 0:
                torch.nn.utils.clip_grad_norm_(model.parameters(), 1)
                assert any(p.grad is not None and p.grad.abs().max() > 0 for p in model.parameters() if p.requires_grad)
                optimizer.step()
                optimizer.zero_grad()
            if step % 24 == 0:
                print(json.dumps({"epoch": epoch+1, "example": step+1, "loss": loss.item()}), flush=True)
        measured = validation_loss()
        report["epochs"].append({"epoch": epoch+1, "training_loss": total/len(groups["train"]), "validation_loss": measured})
        if report["best_validation_loss"] is None or measured < report["best_validation_loss"]:
            report["best_validation_loss"] = measured
            report["best_epoch"] = epoch+1
            model.save_pretrained(args.output / "adapter")
            processor.save_pretrained(args.output / "adapter")
        print(json.dumps(report["epochs"][-1]), flush=True)
    from peft import PeftModel
    import gc
    del model, optimizer
    gc.collect()
    torch.cuda.empty_cache()
    base = AutoModelForVision2Seq.from_pretrained(training.MODEL, revision=training.REVISION, local_files_only=True,
                                                  torch_dtype=torch.bfloat16, attn_implementation="sdpa").to("cuda")
    model = PeftModel.from_pretrained(base, args.output / "adapter").eval()
    model.config.use_cache = True
    predictions = []
    for row in groups["validation"]:
        inputs = encoded(row, with_target=False)
        with torch.no_grad():
            output = model.generate(**inputs, max_new_tokens=160, do_sample=False)
        generated = processor.tokenizer.decode(output[0, inputs.input_ids.shape[1]:], skip_special_tokens=True)
        try:
            roles = AccountRoles.model_validate_json(generated)
            match = all(sorted(getattr(roles, key)) == sorted(row["target"][key]) for key in ("debit_roles", "credit_roles"))
        except ValueError:
            roles, match = None, False
        predictions.append({"id": row["id"], "facts": row["facts"], "expected": row["target"],
                            "generated": generated, "valid_roles": roles is not None, "exact_roles": match})
        print(json.dumps({"validation": row["id"], "valid_roles": roles is not None, "exact_roles": match}), flush=True)
    report.update(completed=True, valid_roles=sum(p["valid_roles"] for p in predictions),
                  exact_roles=sum(p["exact_roles"] for p in predictions),
                  peak_gpu_gib=torch.cuda.max_memory_allocated()/1024**3, test_evaluated=False)
    (args.output / "validation-predictions.json").write_text(json.dumps(predictions, indent=2), encoding="utf-8")
    (args.output / "report.json").write_text(json.dumps(report, indent=2), encoding="utf-8")
    print(json.dumps(report), flush=True)


if __name__ == "__main__":
    main()

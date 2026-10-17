"""Small, isolated image-to-JSON LoRA pilot using supplied synthetic labels."""
import os
os.environ.setdefault("HF_HOME", "D:/PanlyackERP-training/huggingface")
os.environ.setdefault("HF_HUB_DISABLE_SYMLINKS_WARNING", "1")
os.environ["USE_TF"] = "0"
os.environ["USE_FLAX"] = "0"
os.environ["HF_HUB_DISABLE_XET"] = "1"
import hashlib
import json
import random
from decimal import Decimal
from pathlib import Path

from PIL import Image

ROOT = Path("D:/PanlyackERP-training")
OUT = ROOT / "invoice-smolvlm-lora"
PACK = Path("C:/Users/Joy/Downloads/PanlyackERP_training_review_100")
MODEL = "HuggingFaceTB/SmolVLM-256M-Instruct"
REVISION = "7e3e67edbbed1bf9888184d9df282b700a323964"
PROMPT = 'Read the invoice. Return only JSON with number, partner, issue_date, total. Copy the date as printed. Use decimal strings for total; null if missing.'


def records():
    result = []
    seen = set()
    for lot in ("local_50", "public_50"):
        rows = json.loads((PACK / lot / "annotations.json").read_text(encoding="utf-8"), parse_float=Decimal)
        for row in rows:
            if row["source_kind"] == "public_archival_scan":
                continue
            raw = row["source_labels"]
            gt = raw.get("ground_truth", raw)
            target = {
                "number": gt.get("invoice_number", gt.get("invoice_no")),
                "partner": gt.get("supplier_name", gt.get("store")),
                "issue_date": gt.get("invoice_date", gt.get("date")),
                "total": gt.get("total_ttc", gt.get("total_amount", gt.get("total"))),
            }
            if target["number"] is None or target["total"] is None:
                raise ValueError(f"Missing supplied target: {row['id']}")
            target["total"] = str(target["total"])
            image = PACK / lot / row["image"]
            digest = hashlib.sha256(image.read_bytes()).hexdigest()
            if digest in seen:
                continue
            seen.add(digest)
            with Image.open(image) as im:
                im.verify()
            result.append({"id": f"{lot}/{row['id']}", "image": str(image), "sha256": digest, "target": target})
    random.Random(42).shuffle(result)
    return result


def main():
    import torch
    from peft import LoraConfig, get_peft_model
    from transformers import AutoModelForVision2Seq, AutoProcessor
    from huggingface_hub import hf_hub_download
    assert torch.cuda.is_available(), "CUDA required for this bounded pilot"
    assert not OUT.exists() or {p.name for p in OUT.iterdir()} <= {"split.json"}, "Refusing to overwrite an existing training run"
    torch.set_num_threads(2)
    torch.manual_seed(42)
    rows = records()
    assert len(rows) >= 20
    validation, train = rows[:max(5, len(rows)//10)], rows[max(5, len(rows)//10):]
    OUT.mkdir(parents=True, exist_ok=True)
    (OUT / "split.json").write_text(json.dumps({"train": train, "validation": validation, "label_status": "supplied_synthetic_labels_not_human_confirmed", "scope": "four header fields only; no line-item training", "layout_independence": False}, indent=2, ensure_ascii=False), encoding="utf-8")
    processor = AutoProcessor.from_pretrained(MODEL, revision=REVISION)
    weights = Path(hf_hub_download(MODEL, "model.safetensors", revision=REVISION))
    expected_hash = "74dea5904032e5ae99a2e0eef5179e6ac0f1dedc3ab0c7c2a5d4d387c843203e"
    def weight_hash():
        with weights.open("rb") as stream:
            return hashlib.file_digest(stream, "sha256").hexdigest()
    if weight_hash() != expected_hash:
        print("Corrupt cached model detected; downloading complete verified weights over HTTP", flush=True)
        import requests
        import time
        temporary = weights.resolve().with_suffix(".verified-download")
        url = f"https://huggingface.co/{MODEL}/resolve/{REVISION}/model.safetensors?download=true&verified_full=1"
        for attempt in range(12):
            received = temporary.stat().st_size if temporary.exists() else 0
            if received == 513028808:
                break
            try:
                with requests.get(url + f"&offset={received}", headers={"Range": f"bytes={received}-"}, stream=True, timeout=(15, 30)) as response:
                    response.raise_for_status()
                    assert response.status_code == 206 and response.headers.get("Content-Range", "").startswith(f"bytes {received}-"), "Unexpected download range"
                    with temporary.open("ab") as stream:
                        for chunk in response.iter_content(1024 * 1024):
                            stream.write(chunk)
                            received += len(chunk)
                            if received // (32*1024*1024) != (received-len(chunk)) // (32*1024*1024):
                                print(f"Model download: {received / 1024**2:.0f} / 489 MiB", flush=True)
            except requests.RequestException as error:
                print(f"Download attempt {attempt+1} interrupted: {type(error).__name__}; retained downloaded bytes", flush=True)
                time.sleep(2)
        with temporary.open("rb") as stream:
            assert hashlib.file_digest(stream, "sha256").hexdigest() == expected_hash, "Downloaded model checksum mismatch"
        temporary.replace(weights.resolve())
    assert weight_hash() == expected_hash, "Model checksum mismatch; training refused"
    model = AutoModelForVision2Seq.from_pretrained(MODEL, revision=REVISION, torch_dtype=torch.bfloat16, attn_implementation="sdpa").to("cuda")
    model = get_peft_model(model, LoraConfig(r=8, lora_alpha=16, lora_dropout=0.05, target_modules=r".*text_model.*\.(q_proj|v_proj)$", task_type="CAUSAL_LM"))
    model.config.use_cache = False
    model.print_trainable_parameters()

    def batch(row):
        with Image.open(row["image"]) as raw:
            image = raw.convert("RGB")
            image.thumbnail((1024, 1024))
        messages = [{"role": "user", "content": [{"type": "image"}, {"type": "text", "text": PROMPT}]}]
        prefix = processor.apply_chat_template(messages, add_generation_prompt=True)
        messages.append({"role": "assistant", "content": [{"type": "text", "text": json.dumps(row["target"], ensure_ascii=False)}]})
        full = processor.apply_chat_template(messages, add_generation_prompt=False)
        inputs = processor(text=full, images=[image], return_tensors="pt")
        prompt_length = processor(text=prefix, images=[image], return_tensors="pt")["input_ids"].shape[1]
        labels = inputs["input_ids"].clone()
        labels[:, :prompt_length] = -100
        assert (labels != -100).any()
        inputs["labels"] = labels
        return inputs.to("cuda", dtype=torch.bfloat16)

    def evaluate():
        model.eval()
        with torch.no_grad():
            loss = sum(model(**batch(row)).loss.item() for row in validation) / len(validation)
        model.train()
        return loss

    report = {"model": MODEL, "revision": REVISION, "gpu": torch.cuda.get_device_name(), "train_images": len(train), "validation_images": len(validation), "baseline_validation_loss": evaluate(), "epochs": [], "production_ready": False}
    optimizer = torch.optim.AdamW((p for p in model.parameters() if p.requires_grad), lr=1e-4)
    for epoch in range(3):
        optimizer.zero_grad()
        total = 0
        for step, row in enumerate(train):
            loss = model(**batch(row)).loss
            assert torch.isfinite(loss)
            (loss / min(4, len(train) - (step // 4) * 4)).backward()
            total += loss.item()
            if (step+1)%4 == 0 or step+1 == len(train):
                torch.nn.utils.clip_grad_norm_(model.parameters(), 1.0)
                assert any(p.grad is not None and p.grad.abs().max() > 0 for p in model.parameters() if p.requires_grad), "No nonzero adapter gradients"
                optimizer.step()
                optimizer.zero_grad()
            if step%10 == 0:
                print(json.dumps({"epoch": epoch+1, "image": step+1, "loss": loss.item()}), flush=True)
        report["epochs"].append({"epoch": epoch+1, "train_loss": total/len(train), "validation_loss": evaluate()})
        print(json.dumps(report["epochs"][-1]), flush=True)
    model.save_pretrained(OUT / "adapter")
    processor.save_pretrained(OUT / "adapter")
    report["peak_gpu_gib"] = torch.cuda.max_memory_allocated()/1024**3
    report["completed"] = True
    (OUT / "report.json").write_text(json.dumps(report, indent=2), encoding="utf-8")
    print(json.dumps(report), flush=True)


if __name__ == "__main__":
    main()

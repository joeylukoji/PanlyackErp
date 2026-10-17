"""Record generated header predictions on the pilot's held-out images."""
import importlib.util
import json
import argparse
from pathlib import Path

spec = importlib.util.spec_from_file_location("invoice_training", Path(__file__).with_name("train-light-invoices.py"))
training = importlib.util.module_from_spec(spec)
spec.loader.exec_module(training)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--run", type=Path, default=training.OUT)
    parser.add_argument("--dataset", type=Path)
    args = parser.parse_args()
    import torch
    torch.set_num_threads(2)
    from peft import PeftModel
    from PIL import Image
    from transformers import AutoModelForVision2Seq, AutoProcessor

    root = args.run
    assert (root / "report.json").is_file(), "Training must finish first"
    processor = AutoProcessor.from_pretrained(root / "adapter")
    base = AutoModelForVision2Seq.from_pretrained(training.MODEL, revision=training.REVISION, local_files_only=True, torch_dtype=torch.bfloat16, attn_implementation="sdpa").to("cuda")
    model = PeftModel.from_pretrained(base, root / "adapter").eval()
    model.config.use_cache = True
    rows = training.dataset_split(args.dataset, check_training_ocr=False)[1] if args.dataset else json.loads((root / "split.json").read_text(encoding="utf-8"))["validation"]
    results = []
    for row in rows:
        with Image.open(row["image"]) as raw:
            image = raw.convert("RGB")
            image.thumbnail((1024, 1024))
        messages = [{"role": "user", "content": [{"type": "image"}, {"type": "text", "text": training.prompt_for(row)}]}]
        prompt = processor.apply_chat_template(messages, add_generation_prompt=True)
        inputs = processor(text=prompt, images=[image], return_tensors="pt").to("cuda", dtype=torch.bfloat16)
        with torch.no_grad():
            output = model.generate(**inputs, max_new_tokens=160, do_sample=False)
        text = processor.decode(output[0, inputs["input_ids"].shape[1]:], skip_special_tokens=True)
        try:
            parsed = json.loads(text)
            assert isinstance(parsed, dict)
        except (ValueError, AssertionError):
            parsed = None
        results.append({"id": row["id"], "expected": row["target"], "generated": text, "parsed": parsed,
                        "exact_fields": {key: bool(parsed is not None and key in parsed and parsed[key] == value) for key, value in row["target"].items()}})
        print(json.dumps(results[-1], ensure_ascii=False), flush=True)
    report = {"documents": len(rows), "valid_json": sum(r["parsed"] is not None for r in results),
              "exact_fields": {key: sum(r["exact_fields"][key] for r in results) for key in rows[0]["target"]},
              "scope": "synthetic held-out images; layouts may overlap training; not real-document accuracy", "predictions": results}
    output_name = "ocr-generation-evaluation.json" if args.dataset else "generation-evaluation.json"
    (root / output_name).write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({k: v for k, v in report.items() if k != "predictions"}), flush=True)


if __name__ == "__main__":
    main()

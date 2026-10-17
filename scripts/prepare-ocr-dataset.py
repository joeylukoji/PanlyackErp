"""Prepare photometric variants without changing labels or leaking validation parents."""
import argparse
import hashlib
import html
import importlib.util
import json
import os
import shutil
import sys
from pathlib import Path

from PIL import Image, ImageEnhance, ImageFilter

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "ai-service"))
spec = importlib.util.spec_from_file_location("invoice_training", Path(__file__).with_name("train-light-invoices.py"))
training = importlib.util.module_from_spec(spec)
spec.loader.exec_module(training)


def write(path: Path, value: object) -> None:
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def prepare(rows: list[dict], output: Path, count: int, validation_ids: set[str]) -> list[dict]:
    if output.exists():
        raise ValueError("Output already exists; use --ocr-only to resume OCR")
    train = [row for row in rows if row["id"] not in validation_ids]
    if count < len(rows) or not train or not validation_ids or not validation_ids <= {r["id"] for r in rows}:
        raise ValueError("Invalid count or source split")
    if len({row["sha256"] for row in rows}) != len(rows):
        raise ValueError("Duplicate source images")
    output.mkdir(parents=True)
    for folder in ("images", "labels", "ocr", "source_annotations"):
        (output / folder).mkdir()
    result = []
    for index in range(count):
        original = index < len(rows)
        parent = rows[index] if original else train[(index - len(rows)) % len(train)]
        number = index + 1
        identifier = f"invoice-{number:03d}"
        suffix = Path(parent["image"]).suffix if original else ".jpg"
        path = output / "images" / (identifier + suffix)
        transform = {"kind": "original"}
        if original:
            shutil.copyfile(parent["image"], path)
        else:
            radius = [.35, .45, .55][(index - len(rows)) % 3]
            brightness = [.96, 1.02, .99][(index - len(rows)) % 3]
            with Image.open(parent["image"]) as image:
                image = ImageEnhance.Brightness(image.convert("RGB")).enhance(brightness)
                image.filter(ImageFilter.GaussianBlur(radius)).save(path, quality=94)
            transform = {"kind": "mild_blur_brightness_jpeg", "blur_radius": radius, "brightness": brightness, "jpeg_quality": 94, "geometry_changed": False}
        sha = hashlib.sha256(path.read_bytes()).hexdigest()
        with Image.open(path) as image:
            image.verify()
        row = {"id": identifier, "image": str(path.resolve()), "sha256": sha,
               "source_id": parent["id"], "origin_sha256": parent["sha256"],
               "split": "validation" if parent["id"] in validation_ids else "train",
               "target": parent["target"], "transform": transform,
               "label_status": "supplied_synthetic_labels_not_human_confirmed",
               "ocr_file": str((output / "ocr" / (identifier + ".json")).resolve())}
        result.append(row)
        write(output / "labels" / (identifier + ".json"), row)
    assert len({r["sha256"] for r in result}) == count
    assert not ({r["origin_sha256"] for r in result if r["split"] == "train"} & {r["origin_sha256"] for r in result if r["split"] == "validation"})
    write(output / "annotations.json", result)
    write(output / "manifest.json", {"images": count, "original_documents": len(rows), "augmented_images": count - len(rows),
          "train": sum(r["split"] == "train" for r in result), "validation": sum(r["split"] == "validation" for r in result),
          "parent_split_overlap": 0, "validation_augmented": 0, "ocr_completed": 0,
          "layout_independence": False, "human_field_confirmation": False, "training_started": False})
    cards = []
    for row in result:
        image_name = "images/" + Path(row["image"]).name
        cards.append(f'<article><h2>{row["id"]}</h2><p>{html.escape(row["source_id"])} · {row["split"]} · {row["transform"]["kind"]}</p><a href="{image_name}"><img loading="lazy" src="{image_name}" alt="{row["id"]}"></a><p><a href="labels/{row["id"]}.json">Labels</a> · <a href="ocr/{row["id"]}.json">OCR (après traitement)</a></p><pre>{html.escape(json.dumps(row["target"], ensure_ascii=False, indent=2))}</pre></article>')
    (output / "index.html").write_text('<!doctype html><html lang="fr"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Factures : OCR et variantes</title><style>body{font:16px system-ui;margin:24px;background:#f3f5f7;color:#16212f}main{display:grid;grid-template-columns:repeat(auto-fit,minmax(280px,1fr));gap:20px}article{background:white;padding:18px;border:1px solid #ddd}img{width:100%;height:350px;object-fit:contain}pre{white-space:pre-wrap;overflow-wrap:anywhere}</style><h1>150 images pour préparer l’entraînement</h1><p>85 factures originales annotées + 65 variantes légères. Les annotations fournies restent à confirmer. Les 8 images de validation restent originales ; aucune de leurs variantes ne rejoint l’entraînement.</p><main>' + ''.join(cards) + '</main></html>', encoding="utf-8")
    return result


def run_ocr(output: Path) -> None:
    from app.ocr import read_document

    rows = json.loads((output / "annotations.json").read_text(encoding="utf-8"))
    manifest = json.loads((output / "manifest.json").read_text(encoding="utf-8"))
    for index, row in enumerate(rows):
        image = Path(row["image"])
        if hashlib.sha256(image.read_bytes()).hexdigest() != row["sha256"]:
            raise ValueError("Dataset image changed after preparation")
        path = Path(row["ocr_file"])
        if path.exists():
            observation = json.loads(path.read_text(encoding="utf-8"))
            if observation.get("image_sha256") != row["sha256"]:
                raise ValueError("Cached OCR does not match image")
        else:
            observation = read_document(image.read_bytes())
            observation["image_sha256"] = row["sha256"]
            write(path, observation)
        manifest["ocr_completed"] = index + 1
        write(output / "manifest.json", manifest)
        print(json.dumps({"ocr_completed": index + 1, "images": len(rows), "lines": len(observation["lines"]), "id": row["id"]}), flush=True)
    verify(output)


def verify(output: Path) -> dict:
    rows = json.loads((output / "annotations.json").read_text(encoding="utf-8"))
    parents = {r["sha256"]: r for r in rows if r["transform"]["kind"] == "original"}
    assert len({r["sha256"] for r in rows}) == len(rows)
    empty = []
    for row in rows:
        image = Path(row["image"])
        assert hashlib.sha256(image.read_bytes()).hexdigest() == row["sha256"]
        parent = parents[row["origin_sha256"]]
        assert row["target"] == parent["target"] and row["split"] == parent["split"]
        with Image.open(image) as variant, Image.open(parent["image"]) as original:
            assert variant.size == original.size
            variant.verify()
        observation = json.loads(Path(row["ocr_file"]).read_text(encoding="utf-8"))
        assert observation["image_sha256"] == row["sha256"]
        assert observation["human_confirmation_required"] is True
        if not observation["text"].strip():
            empty.append(row["id"])
    assert not ({r["origin_sha256"] for r in rows if r["split"] == "train"} & {r["origin_sha256"] for r in rows if r["split"] == "validation"})
    report = {"images_decoded_and_hashed": len(rows), "ocr_caches_verified": len(rows),
              "original_documents": len(parents), "augmented_images": len(rows) - len(parents),
              "unchanged_parent_labels_and_dimensions": True, "parent_split_overlap": 0,
              "empty_ocr_documents": empty, "field_accuracy_measured": False}
    write(output / "verification.json", report)
    return report


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, default=Path("D:/PanlyackERP-training/dataset-150"))
    parser.add_argument("--with-ocr", action="store_true")
    parser.add_argument("--ocr-only", action="store_true")
    parser.add_argument("--gpu", action="store_true")
    args = parser.parse_args()
    if args.gpu:
        os.environ["OCR_GPU"] = "1"
    if not args.ocr_only:
        rows = training.records()
        previous = json.loads((training.OUT / "split.json").read_text(encoding="utf-8"))
        validation_ids = {r["id"] for r in previous["validation"]}
        prepare(rows, args.output, 150, validation_ids)
        for lot in ("local_50", "public_50"):
            shutil.copyfile(training.PACK / lot / "annotations.json", args.output / "source_annotations" / (lot + ".json"))
        print("Prepared 150 images from 85 original documents; 142 train and 8 original validation images", flush=True)
    if args.with_ocr or args.ocr_only:
        run_ocr(args.output)


if __name__ == "__main__":
    main()

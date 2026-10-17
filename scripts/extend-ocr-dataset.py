"""Freeze newly supplied invoices as a separate test partition."""
import argparse
import csv
import hashlib
import html
import importlib.util
import json
import os
import shutil
from decimal import Decimal
from pathlib import Path

spec = importlib.util.spec_from_file_location("ocr_pack", Path(__file__).with_name("prepare-ocr-dataset.py"))
pack = importlib.util.module_from_spec(spec)
spec.loader.exec_module(pack)


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def contained(root, relative):
    path = (root / relative).resolve()
    if not path.is_relative_to(root.resolve()) or not path.is_file():
        raise ValueError("Missing or unsafe source file: " + str(relative))
    return path


def sources(root):
    candidates, snapshots = [], []
    for tag, folder, labels, csv_name in (
        ("receipts", "outputs/photos_factures", "labels_photos.json", "labels_photos.csv"),
        ("suppliers", "facture_fournisseurs", "annotations.json", "ground_truth.csv"),
    ):
        directory = root / folder
        metadata_hashes = {name: digest(contained(directory, name)) for name in (labels, csv_name, "crops_index.json")}
        raw = json.loads((directory / labels).read_text(encoding="utf-8"), parse_float=Decimal)
        rows = raw["images"] if isinstance(raw, dict) else raw
        with (directory / csv_name).open(encoding="utf-8-sig", newline="") as stream:
            csv_rows = list(csv.DictReader(stream))
        csv_map = {Path(r["image"]).name: r for r in csv_rows}
        if len(csv_map) != len(rows) or len(csv_rows) != len(rows):
            raise ValueError("CSV/JSON record count or image-key mismatch")
        for row in rows:
            gt = row.get("ground_truth", row)
            target = {"number": gt.get("invoice_number", gt.get("invoice_no")),
                      "partner": gt.get("supplier_name", gt.get("store")),
                      "issue_date": gt.get("invoice_date", gt.get("date")),
                      "total": str(gt.get("total_ttc", gt.get("total_amount", gt.get("total"))))}
            if not all(target.values()) or target["total"] == "None":
                raise ValueError("Required source header missing")
            csv_row = csv_map[Path(row["image"]).name]
            if (csv_row.get("invoice_number", csv_row.get("invoice_no")) != target["number"]
                or csv_row.get("supplier", csv_row.get("store")) != target["partner"]
                or csv_row.get("invoice_date", csv_row.get("date")) != target["issue_date"]
                or Decimal(csv_row.get("total_ttc_or_total", csv_row.get("total"))) != Decimal(target["total"])):
                raise ValueError("CSV/JSON header disagreement: " + row["image"])
            image = contained(directory, row["image"])
            crops = {field: contained(directory, filename) for field, filename in row.get("crops", {}).items()}
            candidates.append({"source_id": tag + "/" + row["image"], "image": str(image), "sha256": digest(image),
                               "target": target, "supplied_annotation": json.loads(json.dumps(row, default=str)),
                               "crops": crops})
        for filename in (labels, csv_name, "crops_index.json"):
            path = contained(directory, filename)
            snapshots.append((tag, path, metadata_hashes[filename]))
    return candidates, snapshots


def select_unseen(candidates, baseline):
    hashes = {r["sha256"] for r in baseline}
    identities = {pack.training.invoice_identity(r["target"]) for r in baseline}
    selected, excluded = [], []
    new_hashes, new_ids = set(), set()
    for row in candidates:
        identity = pack.training.invoice_identity(row["target"])
        if row["sha256"] in hashes:
            reason = "already_present_image"
        elif identity in identities:
            reason = "known_invoice_rendered_again"
        elif row["sha256"] in new_hashes or identity in new_ids:
            reason = "duplicate_new_invoice"
        else:
            selected.append(row)
            new_hashes.add(row["sha256"])
            new_ids.add(identity)
            continue
        excluded.append({"source_id": row["source_id"], "reason": reason, "sha256": row["sha256"]})
    return selected, excluded


def extend(source, baseline, output):
    if output.exists():
        raise ValueError("Refusing to overwrite an existing dataset")
    previous = json.loads((baseline / "annotations.json").read_text(encoding="utf-8"))
    candidates, snapshots = sources(source)
    new, excluded = select_unseen(candidates, previous)
    if not new:
        raise ValueError("No unseen invoices found")
    shutil.copytree(baseline, output)
    for directory in ("test/images", "test/labels", "test/ocr", "test/crops", "source_updates"):
        (output / directory).mkdir(parents=True, exist_ok=True)
    rows = []
    for old in previous:
        row = dict(old)
        row["image"] = str((output / Path(old["image"]).relative_to(baseline)).resolve())
        row["ocr_file"] = str((output / Path(old["ocr_file"]).relative_to(baseline)).resolve())
        rows.append(row)
        pack.write(output / "labels" / (row["id"] + ".json"), row)
    for index, source_row in enumerate(new, 1):
        identifier = f"test-{index:03d}"
        destination = output / "test/images" / (identifier + Path(source_row["image"]).suffix)
        shutil.copyfile(source_row["image"], destination)
        crops = {}
        for field, path in source_row["crops"].items():
            if not field.replace("_", "").isalnum():
                raise ValueError("Unsafe crop field")
            crop = output / "test/crops" / identifier / (field + path.suffix)
            crop.parent.mkdir(exist_ok=True)
            shutil.copyfile(path, crop)
            crops[field] = {"file": str(crop.relative_to(output)).replace("\\", "/"), "sha256": digest(crop)}
        row = {"id": identifier, "image": str(destination.resolve()), "sha256": source_row["sha256"],
               "origin_sha256": source_row["sha256"], "source_id": source_row["source_id"], "split": "test",
               "target": source_row["target"], "transform": {"kind": "original"},
               "label_status": "supplied_synthetic_labels_not_human_confirmed",
               "ocr_file": str((output / "test/ocr" / (identifier + ".json")).resolve()),
               "supplied_annotation": source_row["supplied_annotation"], "labelled_crops": crops}
        rows.append(row)
        pack.write(output / "test/labels" / (identifier + ".json"), row)
    for tag, path, expected in snapshots:
        destination = output / "source_updates" / tag / path.name
        destination.parent.mkdir(exist_ok=True)
        shutil.copyfile(path, destination)
        if digest(destination) != expected:
            raise ValueError("Source metadata changed during preparation")
    counts = {name: sum(r["split"] == name for r in rows) for name in ("train", "validation", "test")}
    pack.write(output / "annotations.json", rows)
    pack.write(output / "discovery.json", {"source_records": len(candidates), "new_test_documents": len(new), "excluded": excluded,
               "labelled_test_crops": sum(len(r["labelled_crops"]) for r in rows if r["split"] == "test"),
               "source_snapshots": [{"file": str(Path("source_updates") / tag / path.name), "sha256": sha} for tag, path, sha in snapshots]})
    pack.write(output / "manifest.json", {"images": len(rows), "original_documents": sum(r["transform"]["kind"] == "original" for r in rows),
               "augmented_images": sum(r["transform"]["kind"] != "original" for r in rows), **counts,
               "ocr_completed": len(previous), "test_used_for_training": False, "training_started": False,
               "layout_independence": False, "human_field_confirmation": False})
    (output / "verification.json").unlink()
    (output / "previous-index.html").write_text((baseline / "index.html").read_text(encoding="utf-8"), encoding="utf-8")
    cards = []
    for row in rows:
        if row["split"] != "test":
            continue
        image = str(Path(row["image"]).relative_to(output)).replace("\\", "/")
        crop_links = " · ".join(f'<a href="{info["file"]}">{html.escape(field)}</a>' for field, info in row["labelled_crops"].items())
        cards.append(f'<article><h2>{row["id"]} · test</h2><p>{html.escape(row["source_id"])}</p><a href="{image}"><img loading="lazy" src="{image}" alt="{row["id"]}"></a><p><a href="test/labels/{row["id"]}.json">Labels complets</a> · <a href="test/ocr/{row["id"]}.json">OCR</a></p><p>{crop_links}</p><pre>{html.escape(json.dumps(row["target"],ensure_ascii=False,indent=2))}</pre></article>')
    (output / "index.html").write_text('<!doctype html><html lang="fr"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Nouvelles factures de test</title><style>body{font:16px system-ui;margin:24px;background:#f3f5f7}main{display:grid;grid-template-columns:repeat(auto-fit,minmax(280px,1fr));gap:20px}article{padding:18px;background:white}img{width:100%;height:350px;object-fit:contain}pre{white-space:pre-wrap}</style>' + f'<h1>{len(new)} nouvelles factures réservées aux tests</h1><p>{len(rows)} images au total · {counts["train"]} entraînement · {counts["validation"]} validation · {counts["test"]} test.</p><p>Les recadrages sont des champs de ces factures, pas des documents indépendants. Les labels fournis restent à confirmer. Aucun entraînement n’utilise les nouvelles factures.</p><p><a href="previous-index.html">Lot précédent conservé</a> · <a href="discovery.json">Rapport de sélection</a></p><main>' + ''.join(cards) + '</main></html>', encoding="utf-8")
    (output / "README.md").write_text(f'''# Expanded invoice test pack

This self-contained pack has {len(rows)} images: {counts['train']} training, {counts['validation']} validation and {counts['test']} test images. The previous dataset is preserved unchanged. Newly discovered invoice identities and their labelled crops belong exclusively to test; no model training or test-set model evaluation was launched by this update.

- `annotations.json`: combined image/hash/parent/split records and supplied header targets.
- `images/`, `labels/`, `ocr/`, `source_annotations/`: preserved training/validation material.
- `test/images/`, `test/labels/`, `test/crops/`, `test/ocr/`: new original test images, full supplied annotations, field crops and OCR observations.
- `source_updates/`: current source JSON/CSV/crop-index snapshots; original annotations are supplied synthetic labels, not human-confirmed truth.
- `discovery.json`: selected counts, exclusion reasons and source metadata hashes. Re-rendered known invoice identities do not become independent test cases.
- `verification.json`: final image, crop, OCR-cache and three-way separation checks, created after OCR completes.
- `index.html`: new test gallery; `previous-index.html`: preserved earlier gallery.

OCR uses full images and automatic regions, never labelled crop boxes or target values. Field crops are parts of test documents, not independent invoices. Supplier/layout independence remains unproven. Human confirmation is required before ERP posting. Project instructions: `docs/expanded-invoice-tests.md`.
''', encoding="utf-8")
    return rows


def verify(output):
    pack.training.dataset_split(output)
    report = pack.verify(output)
    rows = json.loads((output / "annotations.json").read_text(encoding="utf-8"))
    crops = 0
    for row in rows:
        for crop in row.get("labelled_crops", {}).values():
            path = output / crop["file"]
            assert digest(path) == crop["sha256"]
            with pack.Image.open(path) as image:
                image.verify()
            crops += 1
    report.update({"test_documents": sum(r["split"] == "test" for r in rows), "labelled_crops_verified": crops,
                   "test_parent_and_invoice_identity_overlap": 0, "test_used_for_training": False})
    pack.write(output / "verification.json", report)
    return report


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", type=Path, default=Path("C:/Users/Joy/Downloads/facture_datasets"))
    parser.add_argument("--baseline", type=Path, default=Path("D:/PanlyackERP-training/dataset-150"))
    parser.add_argument("--output", type=Path, default=Path("D:/PanlyackERP-training/dataset-expanded"))
    parser.add_argument("--ocr-only", action="store_true")
    parser.add_argument("--gpu", action="store_true")
    args = parser.parse_args()
    if args.gpu:
        os.environ["OCR_GPU"] = "1"
    if not args.ocr_only:
        rows = extend(args.source, args.baseline, args.output)
        print(json.dumps({"images": len(rows), "new_test_documents": sum(r["split"] == "test" for r in rows)}), flush=True)
    pack.run_ocr(args.output)
    print(json.dumps(verify(args.output)), flush=True)


if __name__ == "__main__":
    main()

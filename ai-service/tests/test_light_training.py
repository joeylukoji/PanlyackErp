import importlib.util
import json
from pathlib import Path

import pytest
from PIL import Image


def load_training(tmp_path):
    path = Path(__file__).resolve().parents[2] / "scripts/train-light-invoices.py"
    spec = importlib.util.spec_from_file_location("light_training", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    module.PACK = tmp_path
    for name in ("local_50", "public_50"):
        (tmp_path / name).mkdir()
        (tmp_path / name / "annotations.json").write_text("[]", encoding="utf-8")
    return module


def test_training_excludes_archival_class_labels_and_preserves_decimal_targets(tmp_path):
    module = load_training(tmp_path)
    image = tmp_path / "local_50/invoice.png"
    Image.new("RGB", (20, 20), "white").save(image)
    row = {"id": "local-001", "image": "invoice.png", "source_kind": "local_synthetic_receipt",
           "source_labels": {"invoice_no": "A-1", "store": "Shop", "date": "01/02/2026", "total": 10.25}}
    (tmp_path / "local_50/annotations.json").write_text(json.dumps([row, {**row, "id": "duplicate"}]), encoding="utf-8")
    (tmp_path / "public_50/annotations.json").write_text(json.dumps([
        {"id": "public-001", "source_kind": "public_archival_scan", "review_status": "human_confirmed", "source_labels": {"label": 6}}
    ]), encoding="utf-8")
    rows = module.records()
    assert len(rows) == 1
    assert rows[0]["target"] == {"number": "A-1", "partner": "Shop", "issue_date": "01/02/2026", "total": "10.25"}


def test_training_refuses_missing_total_target(tmp_path):
    module = load_training(tmp_path)
    row = {"id": "supplier-001", "source_kind": "local_supplier_invoice", "source_labels": {"ground_truth": {"invoice_number": "A-1"}}}
    (tmp_path / "public_50/annotations.json").write_text(json.dumps([row]), encoding="utf-8")
    with pytest.raises(ValueError, match="Missing supplied target"):
        module.records()


def test_ocr_context_must_match_image_and_never_uses_target_labels(tmp_path):
    module = load_training(tmp_path)
    cache = tmp_path / "ocr.json"
    cache.write_text(json.dumps({"image_sha256": "wrong", "text": "Invoice ABC"}), encoding="utf-8")
    row = {"sha256": "actual", "ocr_file": str(cache), "target": {"number": "SECRET-TARGET"}}
    with pytest.raises(ValueError, match="does not match"):
        module.prompt_for(row)
    cache.write_text(json.dumps({"image_sha256": "actual", "text": "Invoice ABC"}), encoding="utf-8")
    prompt = module.prompt_for(row)
    assert "Invoice ABC" in prompt and "SECRET-TARGET" not in prompt

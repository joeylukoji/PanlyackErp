import hashlib
import importlib.util
import json
from pathlib import Path

from PIL import Image
import pytest


def module():
    path = Path(__file__).resolve().parents[2] / "scripts/extend-ocr-dataset.py"
    spec = importlib.util.spec_from_file_location("extend_dataset", path)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


def test_new_renders_of_seen_invoices_are_not_new_test_cases():
    data = module()
    target = {"number": "INV-1", "partner": "Shop", "issue_date": "01/01/2026", "total": "10.00"}
    old = [{"sha256": "known", "target": target}]
    rows = [
        {"source_id": "same-bytes", "sha256": "known", "target": target},
        {"source_id": "new-render", "sha256": "render", "target": {**target, "total": "11.00"}},
        {"source_id": "new-invoice", "sha256": "new", "target": {**target, "number": "INV-2"}},
        {"source_id": "duplicate-new", "sha256": "another-render", "target": {**target, "number": "INV-2"}},
    ]
    selected, excluded = data.select_unseen(rows, old)
    assert [r["source_id"] for r in selected] == ["new-invoice"]
    assert [r["reason"] for r in excluded] == ["already_present_image", "known_invoice_rendered_again", "duplicate_new_invoice"]


def test_training_never_returns_or_loads_test_ocr(tmp_path, monkeypatch):
    training = module().pack.training
    rows = []
    for index, (split, color) in enumerate(zip(("train", "validation", "test"), ("white", "black", "red"))):
        image = tmp_path / f"{split}.png"
        Image.new("RGB", (20, 20), color).save(image)
        sha = hashlib.sha256(image.read_bytes()).hexdigest()
        rows.append({"split": split, "image": str(image), "sha256": sha, "origin_sha256": sha,
                     "target": {"number": str(index), "partner": "Shop", "issue_date": "01/01/2026"}})
    (tmp_path / "annotations.json").write_text(json.dumps(rows), encoding="utf-8")
    seen = []
    monkeypatch.setattr(training, "prompt_for", lambda row: seen.append(row["split"]))
    train, validation = training.dataset_split(tmp_path)
    assert len(train) == len(validation) == 1
    assert seen == ["train", "validation"]
    rows[2]["target"] = rows[0]["target"]
    (tmp_path / "annotations.json").write_text(json.dumps(rows), encoding="utf-8")
    with pytest.raises(ValueError, match="identity leakage"):
        training.dataset_split(tmp_path)


def test_source_paths_cannot_escape_the_dataset(tmp_path):
    data = module()
    with pytest.raises(ValueError, match="unsafe"):
        data.contained(tmp_path, "../outside.json")

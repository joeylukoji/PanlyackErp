import importlib.util
from pathlib import Path

import pytest
from PIL import Image


def module():
    path = Path(__file__).resolve().parents[2] / "scripts/prepare-ocr-dataset.py"
    spec = importlib.util.spec_from_file_location("ocr_dataset", path)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


def test_augmentation_keeps_validation_parents_out_of_training(tmp_path):
    import hashlib
    images = []
    for index, color in enumerate(("white", "black")):
        path = tmp_path / f"source-{index}.png"
        Image.new("RGB", (40, 80), color).save(path)
        images.append({"id": str(index), "image": str(path), "sha256": hashlib.sha256(path.read_bytes()).hexdigest(), "target": {"number": "A", "total": "1.25"}})
    dataset = module()
    rows = dataset.prepare(images, tmp_path / "output", 4, {"1"})
    assert len(rows) == 4
    assert [r["id"] for r in rows if r["split"] == "validation"] == ["invoice-002"]
    assert {r["origin_sha256"] for r in rows if r["split"] == "train"} == {images[0]["sha256"]}
    assert all(r["target"] == images[0]["target"] for r in rows)
    with pytest.raises(ValueError, match="already exists"):
        dataset.prepare(images, tmp_path / "output", 4, {"1"})


def test_training_rejects_parent_leakage(tmp_path):
    import json
    dataset = module()
    (tmp_path / "annotations.json").write_text(json.dumps([
        {"split": "train", "origin_sha256": "same"}, {"split": "validation", "origin_sha256": "same"}
    ]), encoding="utf-8")
    with pytest.raises(ValueError, match="leakage"):
        dataset.training.dataset_split(tmp_path)

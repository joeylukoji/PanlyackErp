import importlib.util
import json
from pathlib import Path

import pytest


def evaluation():
    path = Path(__file__).resolve().parents[2] / "scripts/evaluate-light-invoices.py"
    spec = importlib.util.spec_from_file_location("light_evaluation", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_decimal_scoring_accepts_formatting_without_hiding_value_errors():
    matches = evaluation().total_matches
    assert matches("35.50", "35.5")
    assert matches("900719925474.123456", "900719925474.123456")
    assert not matches("900719925474.123455", "900719925474.123456")
    for bad in (35.5, None, True, "NaN", "Infinity", "3.55e1", "35,50", "$35.50"):
        assert not matches(bad, "35.5")


def test_offline_rescore_preserves_original_predictions_and_exact_counts(tmp_path):
    module = evaluation()
    report = {"documents": 1, "valid_json": 1, "split": "validation", "scope": "synthetic",
              "exact_fields": {"number": 0, "total": 0},
              "predictions": [{"id": "one", "expected": {"number": "INV-1", "total": "35.5"},
                               "parsed": {"number": "OTHER", "total": "35.50"},
                               "generated": "retained", "exact_fields": {"number": False, "total": False}}]}
    source = tmp_path / "evaluation.json"
    source.write_text(json.dumps(report), encoding="utf-8")
    before = source.read_bytes()
    output = module.rescore(source)
    result = json.loads(output.read_text(encoding="utf-8"))
    assert source.read_bytes() == before
    assert result["exact_fields"] == report["exact_fields"]
    assert result["normalized_fields"] == {"number": 0, "total": 1}
    assert result["predictions"][0]["generated"] == "retained"
    with pytest.raises(FileExistsError):
        module.rescore(source)

import importlib.util
from pathlib import Path


def test_public_context_keeps_provenance_and_omits_payee_and_payment_amount(tmp_path):
    spec = importlib.util.spec_from_file_location("accounting_context", Path(__file__).parents[2] / "scripts/build-accounting-context.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    root = tmp_path / "real-accounting/iowa-checkbook"
    root.mkdir(parents=True)
    (root / "ledger_first_10000_rows.csv").write_text("description,payee,amount\nOffice supplies,Private name,123.45\nOffice supplies,Other name,987.65\n")
    rows = module.build(tmp_path)
    assert len(rows) == 1 and rows[0]["text"] == "Office supplies"
    assert len(rows[0]["source_sha256"]) == 64
    assert "no balanced journal" in rows[0]["kind"]

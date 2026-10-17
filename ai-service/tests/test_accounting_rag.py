import json

import pytest
from pydantic import ValidationError

from app.accounting_advisor import AccountingFacts, policy_roles
from app.accounting_rag import retrieve


def facts(**overrides):
    return AccountingFacts(operation="SALE", settlement="PARTIAL", payment_methods=["BANK"],
                           products=["Coffee"], **overrides)


def test_retrieval_excludes_validation_and_test_references_and_filters_tax(tmp_path, monkeypatch):
    query = facts(tax_treatment="BUYER_SALES_TAX").model_dump()
    rows = [{"id": split, "split": split, "facts": query,
             "target": {"debit_roles": ["BANK", "ACCOUNTS_RECEIVABLE"], "credit_roles": ["SALES", "OUTPUT_TAX"]}}
            for split in ("train", "validation", "test")]
    path = tmp_path / "cases.json"
    path.write_text(json.dumps(rows))
    monkeypatch.setenv("ACCOUNTING_RAG_EXAMPLES_PATH", str(path))
    monkeypatch.delenv("ACCOUNTING_PUBLIC_CONTEXT_PATH", raising=False)
    references = retrieve(query)
    assert any(row["id"] == "train" for row in references)
    assert not any(row["id"] in ("test", "validation", "us-sale-no-tax") for row in references)
    assert any(row["id"] == "us-buyer-sales-tax" for row in references)


def test_us_tax_policy_distinguishes_purchase_cost_sales_liability_and_settlement():
    sale = policy_roles(facts(tax_treatment="BUYER_SALES_TAX"))
    assert sale.credit_roles == ["OUTPUT_TAX", "SALES"]
    purchase = AccountingFacts(operation="INVENTORY_PURCHASE", settlement="UNPAID", payment_methods=[],
                               products=["Rice"], tax_treatment="NONRECOVERABLE_PURCHASE_TAX")
    assert policy_roles(purchase).debit_roles == ["INVENTORY"]
    transfer = AccountingFacts(operation="CASH_TO_BANK", settlement="PAID", payment_methods=["CASH", "BANK"], products=["Deposit"])
    assert policy_roles(transfer).debit_roles == ["BANK"]
    assert policy_roles(transfer).credit_roles == ["CASH"]
    with pytest.raises(ValidationError):
        AccountingFacts(**{**transfer.model_dump(), "tax_treatment": "BUYER_SALES_TAX"})
    with pytest.raises(ValidationError):
        AccountingFacts(**{**transfer.model_dump(), "payment_methods": ["BANK"]})

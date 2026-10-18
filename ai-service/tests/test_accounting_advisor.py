import pytest
from fastapi.testclient import TestClient
from pydantic import ValidationError

from app.accounting_advisor import AccountingFacts, AccountRoles, assess, policy_roles
from app.main import app
import json


def facts(**updates):
    return AccountingFacts.model_validate({"operation": "INVENTORY_PURCHASE", "settlement": "PARTIAL",
                                           "payment_methods": ["CASH"], "products": ["Rice"], **updates})


def test_partial_inventory_purchase_and_expense_get_distinct_roles_without_money():
    expected = policy_roles(facts())
    assert expected.debit_roles == ["INVENTORY"]
    assert expected.credit_roles == ["ACCOUNTS_PAYABLE", "CASH"]
    expense = policy_roles(facts(operation="OPERATING_EXPENSE"))
    assert expense.debit_roles == ["OPERATING_EXPENSE"]
    sale = policy_roles(facts(operation="SALE", cost_recognized=True))
    assert sale.debit_roles == ["ACCOUNTS_RECEIVABLE", "CASH", "COGS"]
    assert sale.credit_roles == ["INVENTORY", "SALES"]


def test_model_cannot_authorize_posting_or_override_accounting_policy():
    wrong = AccountRoles(debit_roles=["OPERATING_EXPENSE"], credit_roles=["BANK"])
    result = assess(facts(), wrong)
    assert result.model_roles is None and result.policy_match is False
    assert result.human_confirmation_required is True and result.posting_authorized is False


def test_unknown_accounts_unsupported_tax_and_inconsistent_payment_facts_are_rejected():
    with pytest.raises(ValidationError):
        AccountRoles(debit_roles=["MADE_UP_ACCOUNT"], credit_roles=["CASH"])
    for updates in ({"settlement": "UNPAID"}, {"tax_rate": "16"}, {"cost_recognized": True},
                    {"payment_methods": ["CASH", "CASH"]}):
        with pytest.raises(ValidationError):
            facts(**updates)


def test_advisory_endpoint_is_authenticated_and_never_saves_postings(monkeypatch):
    monkeypatch.setenv("AI_API_KEY", "test-key")
    monkeypatch.setattr("app.main.propose", lambda value: assess(value, policy_roles(value)))
    client = TestClient(app)
    assert client.post("/v1/accounting/propose", json=facts().model_dump()).status_code == 401
    response = client.post("/v1/accounting/propose", headers={"Authorization": "Bearer test-key"}, json=facts().model_dump())
    assert response.status_code == 200
    assert response.json()["posting_authorized"] is False
    assert response.json()["policy_match"] is True


def test_isolated_model_worker_has_bounded_input_timeout_and_no_shell(monkeypatch):
    from app.accounting_advisor import propose
    monkeypatch.setenv("ACCOUNTING_ADAPTER_PATH", "local-pilot")
    calls = []
    class Worker:
        returncode = 0
        def __init__(self, command, **options):
            calls.append((command, options))
        def __enter__(self): return self
        def __exit__(self, *args): pass
        def communicate(self, value, timeout):
            calls[0][1].update(input=value, timeout=timeout)
            return policy_roles(facts()).model_dump_json(), ""
    monkeypatch.setattr("app.accounting_advisor.subprocess.Popen", Worker)
    result = propose(facts())
    assert result.policy_match is True and result.posting_authorized is False
    assert calls[0][0][-1] == "--predict"
    assert 0 < calls[0][1]["timeout"] <= 80 and not calls[0][1].get("shell", False)
    assert AccountingFacts.model_validate_json(calls[0][1]["input"]).products == ["Rice"]


def test_changed_retrieval_corpus_is_rejected_before_loading_weights(tmp_path, monkeypatch):
    from app.accounting_advisor import load_model
    (tmp_path / "report.json").write_text(json.dumps({"completed": True, "prompt_version": 2,
        "model": "HuggingFaceTB/SmolVLM-256M-Instruct", "revision": "7e3e67edbbed1bf9888184d9df282b700a323964", "dataset_sha256": "different"}))
    corpus = tmp_path / "cases.json"
    corpus.write_text("[]")
    monkeypatch.setenv("ACCOUNTING_RAG_EXAMPLES_PATH", str(corpus))
    with pytest.raises(ValueError, match="Retrieval corpus differs"):
        load_model(str(tmp_path))

import base64
from decimal import Decimal
from fastapi.testclient import TestClient
from app.main import app


def test_demo_requires_auth_and_is_explicit(monkeypatch):
    monkeypatch.setenv("AI_API_KEY", "test-key")
    monkeypatch.setenv("EXTRACTOR_MODE", "demo")
    client = TestClient(app)
    payload = {"mime_type": "application/pdf", "file_base64": base64.b64encode(b"%PDF-test").decode()}
    assert client.post("/v1/invoices/extract", json=payload).status_code == 401
    response = client.post("/v1/invoices/extract", json=payload, headers={"Authorization": "Bearer test-key"})
    assert response.status_code == 200
    invoice = response.json()
    assert invoice["extraction_mode"] == "demo" and invoice["warnings"]
    assert len(invoice["lines"]) == 14
    assert sum(Decimal(line["quantity"]) * Decimal(line["unit_price"]) for line in invoice["lines"]) == Decimal("537.40")
    payload["file_base64"] = "invalid!"
    assert client.post("/v1/invoices/extract", json=payload, headers={"Authorization": "Bearer test-key"}).status_code == 422

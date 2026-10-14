import base64
import io
import json
from decimal import Decimal

import httpx
import pypdfium2 as pdfium
from fastapi.testclient import TestClient

from app.main import app
from app.schemas import ExtractionRequest


def pdf_request(pages: int) -> ExtractionRequest:
    with pdfium.PdfDocument.new() as document:
        for _ in range(pages):
            document.new_page(400, 600).close()
        output = io.BytesIO()
        document.save(output)
    return ExtractionRequest(mime_type="application/pdf", file_base64=base64.b64encode(output.getvalue()).decode())


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


def test_ollama_receives_every_pdf_page_as_an_image(monkeypatch):
    monkeypatch.setenv("AI_API_KEY", "test-key")
    monkeypatch.setenv("EXTRACTOR_MODE", "ollama")
    seen_images = []

    def model_response(request):
        body = json.loads(request.content)
        seen_images.extend(body["images"])
        return httpx.Response(200, json={"response": json.dumps({
            "number": "PDF-1", "partner": "Supplier", "issue_date": "2026-10-13",
            "total": "1.00", "lines": [{"name": "Item", "quantity": "1", "unit_price": "1.00"}],
        })})

    real_client = httpx.AsyncClient
    monkeypatch.setattr("app.extractors.httpx.AsyncClient", lambda **kwargs: real_client(transport=httpx.MockTransport(model_response)))
    response = TestClient(app).post("/v1/invoices/extract", json=pdf_request(2).model_dump(), headers={"Authorization": "Bearer test-key"})
    assert response.status_code == 200
    assert response.json()["extraction_mode"] == "ollama"
    assert len(seen_images) == 2
    assert all(base64.b64decode(image).startswith(b"\x89PNG\r\n\x1a\n") for image in seen_images)


def test_ollama_rejects_pdf_over_page_limit(monkeypatch):
    monkeypatch.setenv("AI_API_KEY", "test-key")
    monkeypatch.setenv("EXTRACTOR_MODE", "ollama")
    response = TestClient(app).post("/v1/invoices/extract", json=pdf_request(6).model_dump(), headers={"Authorization": "Bearer test-key"})
    assert response.status_code == 422


def test_ollama_rejects_invalid_pdf(monkeypatch):
    monkeypatch.setenv("AI_API_KEY", "test-key")
    monkeypatch.setenv("EXTRACTOR_MODE", "ollama")
    payload = {"mime_type": "application/pdf", "file_base64": base64.b64encode(b"%PDF-not-a-document").decode()}
    response = TestClient(app).post("/v1/invoices/extract", json=payload, headers={"Authorization": "Bearer test-key"})
    assert response.status_code == 422

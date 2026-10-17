import asyncio
import base64
import io
import json

import httpx
from PIL import Image

from app.extractors import OllamaExtractor
from app.schemas import ExtractionRequest, Invoice


def document():
    output = io.BytesIO()
    Image.new("RGB", (500, 800), "white").save(output, "PNG")
    return ExtractionRequest(mime_type="image/png", file_base64=base64.b64encode(output.getvalue()).decode())


def invoice(total="6.00"):
    return {"number": "INV-1", "partner": "Store", "issue_date": "2026-10-17", "total": total,
            "lines": [{"name": "Rice 5kg", "quantity": "2", "unit_price": "3.00"}]}


def test_product_total_mismatch_gets_one_bounded_corrective_pass(monkeypatch):
    bodies = []
    def reply(request):
        bodies.append(json.loads(request.content))
        return httpx.Response(200, json={"response": json.dumps(invoice("3.00" if len(bodies) == 1 else "6.00"))})
    real_client = httpx.AsyncClient
    monkeypatch.setattr("app.extractors.httpx.AsyncClient", lambda **kwargs: real_client(transport=httpx.MockTransport(reply)))
    result = asyncio.run(OllamaExtractor("http://model", "vision").extract(document()))
    assert len(bodies) == 2 and "unit_price" in bodies[0]["prompt"]
    assert "line totals" in bodies[1]["prompt"]
    assert result.total == "6.00" and result.lines[0].name == "Rice 5kg"
    assert result.payments == []


def test_failed_correction_retains_original_draft_and_does_not_balance_by_invention(monkeypatch):
    bodies = []
    def reply(request):
        bodies.append(json.loads(request.content))
        if len(bodies) == 2:
            return httpx.Response(503)
        return httpx.Response(200, json={"response": json.dumps(invoice("99"))})
    real_client = httpx.AsyncClient
    monkeypatch.setattr("app.extractors.httpx.AsyncClient", lambda **kwargs: real_client(transport=httpx.MockTransport(reply)))
    result = asyncio.run(OllamaExtractor("http://model", "vision").extract(document()))
    assert result.total == "99" and len(bodies) == 2
    assert any("line totals" in warning for warning in result.warnings)
    assert any("correction" in warning.lower() for warning in result.warnings)


def test_detail_view_keeps_original_and_labels_the_extra_image(monkeypatch):
    bodies = []
    def reply(request):
        bodies.append(json.loads(request.content))
        return httpx.Response(200, json={"response": json.dumps(invoice())})
    real_client = httpx.AsyncClient
    monkeypatch.setattr("app.extractors.httpx.AsyncClient", lambda **kwargs: real_client(transport=httpx.MockTransport(reply)))
    result = asyncio.run(OllamaExtractor("http://model", "vision", use_details=True).extract(document()))
    assert len(bodies[0]["images"]) == 2
    assert bodies[0]["images"][0] == document().file_base64
    assert "detail view of page 1" in bodies[0]["prompt"]
    assert result.lines[0].quantity == "2"


def test_exact_diagnostics_do_not_hide_small_errors_or_invent_payment():
    from app.invoice_checks import diagnostics
    data = invoice("6.0001")
    data["payments"] = [{"method": "CASH", "amount": "7"}]
    issues = diagnostics(Invoice.model_validate(data))
    assert any("line totals" in message for message in issues)
    assert any("payments exceed" in message for message in issues)
    assert diagnostics(Invoice.model_validate(invoice())) == []


def test_purchase_partner_in_recipient_block_is_flagged_without_guessing_supplier():
    from app.invoice_checks import recipient_diagnostics
    data = invoice()
    data["partner"] = "Clinic"
    draft = Invoice.model_validate(data)
    def line(text, x, y):
        return {"text": text, "box": [[x,y],[x+200,y],[x+200,y+20],[x,y+20]]}
    evidence = [{"width": 1000, "height": 1400, "lines": [line("Supplier Inc", 100, 100), line("Facturé à",100,350), line("Clinic",100,390)]}]
    assert "recipient" in recipient_diagnostics(draft, evidence)[0]
    assert draft.partner == "Clinic"
    draft.partner = "Supplier Inc"
    assert recipient_diagnostics(draft, evidence) == []
    draft.partner, draft.direction = "Clinic", "SALE"
    assert recipient_diagnostics(draft, evidence) == []


def test_recipient_conflict_refocuses_the_single_retry_on_letterhead(monkeypatch):
    def line(text, y):
        return {"text": text, "box": [[10,y],[200,y],[200,y+15],[10,y+15]], "confidence": .9}
    monkeypatch.setattr("app.ocr.read_document", lambda data: {"width": 500, "height": 800,
        "text": "Supplier Inc\nBill to\nClinic", "lines": [line("Supplier Inc",40), line("Bill to",200), line("Clinic",225)]})
    bodies = []
    def reply(request):
        bodies.append(json.loads(request.content))
        result = invoice()
        result["partner"] = "Clinic" if len(bodies) == 1 else "Supplier Inc"
        result["currency"] = "EUR" if len(bodies) == 1 else "USD"
        result["lines"][0]["quantity"] = "2" if len(bodies) == 1 else "999"
        return httpx.Response(200, json={"response": json.dumps(result)})
    real_client = httpx.AsyncClient
    monkeypatch.setattr("app.extractors.httpx.AsyncClient", lambda **kwargs: real_client(transport=httpx.MockTransport(reply)))
    result = asyncio.run(OllamaExtractor("http://model", "vision", use_ocr=True, use_details=True).extract(document()))
    assert len(bodies) == 2 and result.partner == "Supplier Inc"
    assert result.currency == "EUR" and result.lines[0].quantity == "2"
    assert bodies[1]["images"][0] == document().file_base64
    assert bodies[0]["images"][-1] != bodies[1]["images"][-1]
    assert "letterhead at the TOP" in bodies[1]["prompt"]

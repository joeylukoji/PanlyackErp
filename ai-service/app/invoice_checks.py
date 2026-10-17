"""Exact arithmetic diagnostics for drafts; never silently repair extracted money."""
from decimal import Decimal, ROUND_HALF_UP, localcontext
import re
import unicodedata

from app.schemas import Invoice


def normalized(text: str) -> str:
    return re.sub(r"[^a-z0-9]+", " ", unicodedata.normalize("NFKD", text).encode("ascii", "ignore").decode().lower()).strip()


def header_supports_partner(partner: str, pages: list[dict]) -> bool:
    value = normalized(partner)
    return len(value) >= 3 and any(value in normalized(line["text"])
        for page in pages for line in page.get("lines", [])
        if len(line.get("box", [])) == 4 and line["box"][0][1] < page.get("height", 0)*.24)


def recipient_diagnostics(invoice: Invoice, pages: list[dict]) -> list[str]:
    """Flag a counterparty under a recipient label; do not guess the replacement issuer."""
    if invoice.direction != "PURCHASE":
        return []
    partner = normalized(invoice.partner)
    if len(partner) < 3:
        return []
    for page in pages:
        width, height = page.get("width", 0), page.get("height", 0)
        lines = [line for line in page.get("lines", []) if len(line.get("box", [])) == 4]
        recipients = [line for line in lines if re.match(r"^(facture a|bill to|billed to|sold to|ship to|envoye a|client|customer|destinataire)( |$)", normalized(line["text"]))]
        for line in lines:
            if partner not in normalized(line["text"]):
                continue
            x, y = line["box"][0]
            if y > height * .6:
                continue
            for label in recipients:
                left, top = label["box"][0]
                if 0 < y-top < height*.08 and abs(x-left) < width*.15:
                    return ["Extracted partner occurs below a customer/recipient label. For this purchase reread the supplier/issuer letterhead, not Bill to/Client; do not invent a supplier if unclear"]
    return []


def diagnostics(invoice: Invoice) -> list[str]:
    issues = []
    with localcontext() as context:
        context.prec = 40
        net, tax = Decimal(0), Decimal(0)
        quantum = Decimal("0.0001")
        for index, line in enumerate(invoice.lines, 1):
            quantity, price, rate = map(Decimal, (line.quantity, line.unit_price, line.tax_rate))
            if quantity <= 0:
                issues.append(f"Product row {index}: quantity must be positive")
            if rate > 100:
                issues.append(f"Product row {index}: tax percentage exceeds 100")
            if price != price.quantize(quantum) or rate != rate.quantize(quantum):
                issues.append(f"Product row {index}: price/tax precision exceeds four decimals")
            amount = (quantity * price).quantize(quantum, rounding=ROUND_HALF_UP)
            net += amount
            tax += (amount * rate / 100).quantize(quantum, rounding=ROUND_HALF_UP)
        if Decimal(invoice.total) != net + tax:
            issues.append(f"Declared total differs from line totals plus tax ({net + tax}); reread quantities, unit prices, discounts and taxes")
        if Decimal(invoice.total) <= 0:
            issues.append("Declared total must be positive")
        for value in [invoice.total] + [p.amount for p in invoice.payments]:
            if "." in value and len(value.split(".")[1]) > 4:
                issues.append("Total/payment precision exceeds four decimals; verify the printed amount")
        if sum((Decimal(payment.amount) for payment in invoice.payments), Decimal(0)) > Decimal(invoice.total):
            issues.append("Explicit payments exceed the declared total")
        if len({p.method for p in invoice.payments}) != len(invoice.payments):
            issues.append("Repeated payment method; check the original evidence")
    if not invoice.number.strip() or not invoice.partner.strip():
        issues.append("Invoice number or supplier is unreadable or missing")
    return issues

from datetime import date
from typing import Annotated, Literal
from pydantic import BaseModel, ConfigDict, Field, field_validator

DecimalString = Annotated[str, Field(pattern=r"^[0-9]{1,12}(\.[0-9]{1,6})?$")]


class Line(BaseModel):
    model_config = ConfigDict(extra="forbid")
    sku: str = Field(default="", max_length=80)
    name: str = Field(min_length=1, max_length=200)
    unit: str = Field(default="each", min_length=1, max_length=20)
    quantity: DecimalString
    unit_price: DecimalString
    tax_rate: DecimalString = "0"
    confidence: DecimalString = "0"


class Payment(BaseModel):
    method: Literal["CASH", "BANK", "MOBILE_MONEY", "CARD"]
    amount: DecimalString


class Invoice(BaseModel):
    model_config = ConfigDict(extra="forbid")
    direction: Literal["PURCHASE", "SALE"] = "PURCHASE"
    number: str = Field(max_length=100)
    partner: str = Field(max_length=200)
    issue_date: str = Field(
        pattern=r"^\d{4}-\d{2}-\d{2}$",
        description="Actual invoice issue date normalized to YYYY-MM-DD, not a due date or today's date.",
    )
    currency: Literal["USD", "EUR", "CDF", "ZAR"] = "USD"
    total: DecimalString
    lines: list[Line] = Field(min_length=1, max_length=100)
    payments: list[Payment] = Field(default_factory=list, max_length=4)
    confidence: DecimalString = "0"
    warnings: list[str] = Field(default_factory=list, max_length=20)
    extraction_mode: str = ""

    @field_validator('issue_date')
    @classmethod
    def valid_issue_date(cls, value: str) -> str:
        date.fromisoformat(value)
        return value


class ExtractionRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")
    file_base64: str = Field(min_length=1, max_length=14_000_000)
    mime_type: Literal["image/png", "image/jpeg", "application/pdf"]

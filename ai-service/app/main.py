import asyncio
import os
import secrets
from typing import Annotated
import httpx
from fastapi import Depends, FastAPI, Header, HTTPException
from app.extractors import DemoExtractor, Extractor, OllamaExtractor, validate_file
from app.schemas import ExtractionRequest, Invoice

app = FastAPI(title="PanlyackERP document extraction", version="0.1.0")
slots = asyncio.Semaphore(2)


def get_extractor() -> Extractor:
    mode = os.getenv("EXTRACTOR_MODE", "demo")
    if mode == "demo":
        return DemoExtractor()
    if mode == "ollama":
        return OllamaExtractor(os.getenv("MODEL_BASE_URL", "http://127.0.0.1:11434"), os.getenv("MODEL_NAME", "gemma3:4b"))
    raise HTTPException(503, "Unsupported extractor configuration")


def authorize(authorization: Annotated[str, Header()] = "") -> None:
    key = os.getenv("AI_API_KEY", "")
    if not key or not secrets.compare_digest(authorization, f"Bearer {key}"):
        raise HTTPException(401, "Invalid service credentials")


@app.get("/health/live")
async def live() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/health/ready")
async def ready(extractor: Annotated[Extractor, Depends(get_extractor)]) -> dict[str, str]:
    if not os.getenv("AI_API_KEY"):
        raise HTTPException(503, "AI_API_KEY is required")
    if isinstance(extractor, OllamaExtractor):
        try:
            async with httpx.AsyncClient(timeout=3.0) as client:
                response = await client.get(f"{extractor.base_url}/api/tags")
                response.raise_for_status()
                names = {m["name"] for m in response.json().get("models", [])}
                if extractor.model not in names:
                    raise HTTPException(503, "Configured model is not installed")
        except httpx.HTTPError as exc:
            raise HTTPException(503, "Model server unavailable") from exc
    return {"status": "ready", "mode": os.getenv("EXTRACTOR_MODE", "demo")}


@app.post("/v1/invoices/extract", response_model=Invoice, dependencies=[Depends(authorize)])
async def extract(request: ExtractionRequest, extractor: Annotated[Extractor, Depends(get_extractor)]) -> Invoice:
    try:
        validate_file(request)
    except ValueError as exc:
        raise HTTPException(422, str(exc)) from exc
    if slots.locked():
        raise HTTPException(429, "Extraction is busy; retry shortly")
    async with slots:
        try:
            return await extractor.extract(request)
        except ValueError as exc:
            raise HTTPException(422, "Document could not be extracted; check the file and limit PDFs to 5 pages") from exc
        except (httpx.HTTPError, KeyError) as exc:
            raise HTTPException(502, "Model service failed; no data has been posted") from exc

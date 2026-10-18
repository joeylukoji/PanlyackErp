import asyncio
import os
import secrets
from typing import Annotated
import httpx
from fastapi import Depends, FastAPI, Header, HTTPException
from app.extractors import DemoExtractor, Extractor, OllamaExtractor, validate_file
from app.schemas import ExtractionRequest, Invoice
from app.accounting_advisor import AccountingFacts, AccountAdvice, propose

app = FastAPI(title="PanlyackERP document extraction", version="0.1.0")
slots = asyncio.Semaphore(1)
# Serialize local OCR/vision and text inference on memory-constrained workstations.
active_inferences: set[asyncio.Task] = set()


async def run_inference(work, seconds):
    if slots.locked():
        raise HTTPException(429, "Local inference is busy; retry shortly")
    await slots.acquire()
    task = asyncio.create_task(work())
    active_inferences.add(task)
    def completed(done):
        active_inferences.discard(done)
        slots.release()
        if not done.cancelled():
            done.exception()
    task.add_done_callback(completed)
    # A cancelled to_thread await does not stop its native OCR/model worker.
    # Keep the slot until actual completion, including after an HTTP timeout.
    async with asyncio.timeout(seconds):
        return await asyncio.shield(task)


def get_extractor() -> Extractor:
    mode = os.getenv("EXTRACTOR_MODE", "demo")
    if mode == "demo":
        return DemoExtractor()
    if mode == "ollama":
        return OllamaExtractor(os.getenv("MODEL_BASE_URL", "http://127.0.0.1:11434"), os.getenv("MODEL_NAME", "gemma3:4b"), use_ocr=os.getenv("OCR_ENABLED", "0") == "1", use_details=os.getenv("VISION_DETAIL_ENABLED", "0") == "1")
    raise HTTPException(503, "Unsupported extractor configuration")


def authorize(authorization: Annotated[str, Header()] = "") -> None:
    key = os.getenv("AI_API_KEY", "")
    if not key or not secrets.compare_digest(authorization, f"Bearer {key}"):
        raise HTTPException(401, "Invalid service credentials")


@app.get("/health/live")
async def live() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/v1/accounting/propose", response_model=AccountAdvice, dependencies=[Depends(authorize)])
async def accounting_proposal(facts: AccountingFacts) -> AccountAdvice:
    try:
        return await run_inference(lambda: asyncio.to_thread(propose, facts), 90)
    except TimeoutError as exc:
        raise HTTPException(504, "Accounting advisor timed out; no posting performed") from exc
    except (ImportError, OSError, RuntimeError) as exc:
        raise HTTPException(503, "Local accounting advisor unavailable; no posting performed") from exc
    except (ValueError, KeyError) as exc:
        raise HTTPException(502, "Invalid accounting suggestion; no posting performed") from exc


@app.get("/health/ready")
async def ready(extractor: Annotated[Extractor, Depends(get_extractor)]) -> dict[str, str]:
    if not os.getenv("AI_API_KEY"):
        raise HTTPException(503, "AI_API_KEY is required")
    if isinstance(extractor, OllamaExtractor):
        if extractor.use_ocr:
            try:
                from app.ocr import warmup
                await asyncio.to_thread(warmup)
            except (ImportError, FileNotFoundError, RuntimeError) as exc:
                raise HTTPException(503, "OCR dependencies or local model files unavailable") from exc
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
    try:
        return await run_inference(lambda: extractor.extract(request), 90)
    except TimeoutError as exc:
        raise HTTPException(504, "Extraction timed out; no data has been posted") from exc
    except ValueError as exc:
        raise HTTPException(422, "Document could not be extracted; check the file and limit PDFs to 5 pages") from exc
    except (ImportError, FileNotFoundError, RuntimeError) as exc:
        raise HTTPException(503, "Local extraction engine unavailable") from exc
    except (httpx.HTTPError, KeyError) as exc:
        raise HTTPException(502, "Model service failed; no data has been posted") from exc

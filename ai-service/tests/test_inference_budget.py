import asyncio
import importlib

import pytest
from fastapi import HTTPException


def test_timeout_keeps_slot_until_worker_actually_finishes(monkeypatch):
    module = importlib.import_module("app.main")
    async def scenario():
        monkeypatch.setattr(module, "slots", asyncio.Semaphore(1))
        finished = asyncio.Event()
        async def worker():
            await finished.wait()
            return "done"
        with pytest.raises(TimeoutError):
            await module.run_inference(worker, .01)
        assert module.slots.locked()
        with pytest.raises(HTTPException) as error:
            await module.run_inference(worker, .01)
        assert error.value.status_code == 429
        finished.set()
        await asyncio.sleep(.01)
        assert not module.slots.locked() and not module.active_inferences
    asyncio.run(scenario())

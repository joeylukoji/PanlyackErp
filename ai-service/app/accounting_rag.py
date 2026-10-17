"""Small local retrieval corpus; training examples only, no embeddings or ERP database access."""
import json
import os
import re
from functools import lru_cache
from pathlib import Path


@lru_cache(maxsize=4)
def read_json(path: str):
    raw = Path(path).read_bytes()
    if len(raw) > 4_000_000:
        raise ValueError("Accounting retrieval corpus exceeds its size budget")
    return json.loads(raw)


def retrieve(facts: dict) -> list[dict]:
    result = []
    cards = read_json(str(Path(__file__).parent / "knowledge" / "us-accounting.json"))
    for card in cards:
        if facts["operation"] in card["operations"] and facts.get("tax_treatment", "NONE") in card["tax_treatments"]:
            result.append(card)
    dataset = os.getenv("ACCOUNTING_RAG_EXAMPLES_PATH", "")
    if dataset:
        candidates = []
        for row in read_json(dataset):
            other = row["facts"]
            if row["split"] != "train" or other["operation"] != facts["operation"] or other.get("tax_treatment", "NONE") != facts.get("tax_treatment", "NONE"):
                continue
            score = sum(other.get(key) == facts.get(key) for key in ("settlement", "payment_methods", "cost_recognized"))
            candidates.append((score, row))
        for _, row in sorted(candidates, key=lambda item: (-item[0], item[1]["id"]))[:1]:
            result.append({"id": row["id"], "source": "synthetic Go posting/settlement engine; not accountant-reviewed",
                           "facts": row["facts"], "roles": row["target"]})
    context = os.getenv("ACCOUNTING_PUBLIC_CONTEXT_PATH", "")
    if context:
        tokens = set(re.findall(r"[a-z]{4,}", " ".join(facts["products"]).lower()))
        candidates = [(len(tokens & set(re.findall(r"[a-z]{4,}", row["text"].lower()))), row) for row in read_json(context)]
        useful = sorted((item for item in candidates if item[0]), key=lambda item: (-item[0], item[1]["id"]))
        if useful:
            result.append(useful[0][1])
    # Keep instruction + references + facts inside the small text model context.
    return result[:4]

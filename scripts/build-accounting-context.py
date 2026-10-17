"""Build bounded public reference snippets, never journal ground truth or training targets."""
import argparse
import csv
import hashlib
import json
import re
from pathlib import Path


def build(root: Path) -> list[dict]:
    result = []
    ledger = root / "beancount-us/example.beancount"
    if ledger.exists():
        digest = hashlib.sha256(ledger.read_bytes()).hexdigest()
        blocks = re.split(r"(?=^\d{4}-\d{2}-\d{2} [*!] )", ledger.read_text(encoding="utf-8"), flags=re.M)
        for block in blocks[1:]:
            lines = block.splitlines()
            snippet = [lines[0]]
            for line in lines[1:]:
                if not line.startswith("  "):
                    break
                if line.strip() and not line.lstrip().startswith(";"):
                    snippet.append(line)
            if len(snippet) < 3:
                continue
            result.append({"id": f"beancount-{len(result)+1:03d}", "source": "https://github.com/beancount/beancount/blob/master/examples/example.beancount",
                           "source_sha256": digest, "kind": "generated educational ledger; context only, not US GAAP labels",
                           "text": "\n".join(snippet)[:600]})
            if len(result) >= 40:
                break
    public = root / "real-accounting/iowa-checkbook/ledger_first_10000_rows.csv"
    if public.exists():
        digest = hashlib.sha256(public.read_bytes()).hexdigest()
        with public.open(encoding="utf-8-sig", newline="") as stream:
            seen = set()
            for row in csv.DictReader(stream):
                # Keep public expenditure vocabulary, omit payee names, identifiers and amounts.
                terms = [value for key, value in row.items() if value and any(term in key.lower() for term in ("description", "object_name", "fund_name"))]
                text = "; ".join(terms)[:600]
                if not text or text in seen:
                    continue
                seen.add(text)
                result.append({"id": f"iowa-{len(seen):03d}", "source": "https://data.iowa.gov/catalog/dataset/1074",
                               "source_sha256": digest, "kind": "public government payment vocabulary; no balanced journal labels; State of Iowa / State Accounting Enterprise, CC BY 4.0, descriptions excerpted", "text": text})
                if len(seen) >= 40:
                    break
    if not result:
        raise ValueError("No supported local public sources found")
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, default=Path("var/datasets"))
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    rows = build(args.root)
    with args.output.open("x", encoding="utf-8") as stream:
        json.dump(rows, stream, indent=2, ensure_ascii=False)
    print(json.dumps({"snippets": len(rows), "output": str(args.output)}))


if __name__ == "__main__":
    main()

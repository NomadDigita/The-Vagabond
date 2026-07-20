#!/usr/bin/env python3
"""Audit real Unicode-symbol usage in tracked Go sources into an asset catalogue.

The output is the planning ground truth for replacing every Vagabond emoji with
a named TGS asset. It reads Git's tracked `HEAD` content, never modifies Go.
"""

from __future__ import annotations

import json
import subprocess
import unicodedata
from collections import Counter, defaultdict
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / "assets" / "visual-system" / "catalog"


def is_emoji_base(character: str) -> bool:
    code = ord(character)
    return (
        unicodedata.category(character) in {"So", "Sk"}
        or 0x2600 <= code <= 0x27BF
        or 0x1F000 <= code <= 0x1FAFF
    )


def extract_symbols(text: str) -> list[str]:
    symbols: list[str] = []
    index = 0
    while index < len(text):
        character = text[index]
        if not is_emoji_base(character):
            index += 1
            continue
        token = character
        index += 1
        while index < len(text) and ord(text[index]) in {0xFE0E, 0xFE0F, 0x20E3}:
            token += text[index]
            index += 1
        symbols.append(token)
    return symbols


def main() -> None:
    result = subprocess.run(
        ["git", "-C", str(ROOT), "grep", "-n", "--perl-regexp", r"[^\x00-\x7F]", "HEAD", "--", "*.go"],
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
        encoding="utf-8",
    )
    if result.returncode not in {0, 1}:
        raise SystemExit(result.stderr.strip() or "git grep failed")
    occurrences: Counter[str] = Counter()
    locations: dict[str, list[dict[str, object]]] = defaultdict(list)
    for line in result.stdout.splitlines():
        try:
            revision, path, line_number, source = line.split(":", 3)
            if revision != "HEAD":
                continue
            number = int(line_number)
        except ValueError:
            continue
        for symbol in extract_symbols(source):
            occurrences[symbol] += 1
            if len(locations[symbol]) < 8:
                locations[symbol].append({"path": path, "line": number, "context": source.strip()[:180]})
    assets = [
        {"unicode": symbol, "occurrences": count, "examples": locations[symbol]}
        for symbol, count in occurrences.most_common()
    ]
    OUT.mkdir(parents=True, exist_ok=True)
    (OUT / "emoji_usage_audit.json").write_text(json.dumps({"source": "git HEAD tracked *.go", "unique_symbols": len(assets), "occurrences": sum(occurrences.values()), "assets": assets}, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    lines = ["# Vagabond Unicode Asset Audit", "", f"Tracked Go sources contain **{len(assets)}** distinct symbol tokens across **{sum(occurrences.values())}** occurrences.", "", "| Unicode | Uses | Real source context |", "|---|---:|---|"]
    for asset in assets:
        context = asset["examples"][0]["context"].replace("|", "\\|") if asset["examples"] else ""
        lines.append(f"| {asset['unicode']} | {asset['occurrences']} | `{context}` |")
    (OUT / "emoji_usage_audit.md").write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(f"Wrote {len(assets)} unique symbol assets / {sum(occurrences.values())} occurrences.")


if __name__ == "__main__":
    main()

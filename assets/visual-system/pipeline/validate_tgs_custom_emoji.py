#!/usr/bin/env python3
"""Offline structural checks for Telegram TGS animated custom emoji.

TGS is gzip-compressed Lottie JSON.  This intentionally validates the
deterministic constraints that can be checked without a Telegram upload; a
client review is still required for visual fidelity and unsupported Lottie
features that Telegram's renderer may reject.
"""

from __future__ import annotations

import argparse
import gzip
import json
import sys
from pathlib import Path
from typing import Any


MAX_BYTES = 64 * 1024
CANVAS = 512
FPS = 60
MAX_FRAMES = FPS * 3


def walk(value: Any, path: str = "$"):
    if isinstance(value, dict):
        yield value, path
        for key, child in value.items():
            yield from walk(child, f"{path}.{key}")
    elif isinstance(value, list):
        for index, child in enumerate(value):
            yield from walk(child, f"{path}[{index}]")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("asset", type=Path, help=".tgs file to inspect")
    args = parser.parse_args()
    asset = args.asset.resolve()
    errors: list[str] = []
    warnings: list[str] = []

    if asset.suffix.lower() != ".tgs":
        errors.append("asset extension must be .tgs")
    if not asset.is_file():
        errors.append(f"missing asset: {asset}")
    if errors:
        print("FAIL: " + "; ".join(errors))
        return 1

    byte_count = asset.stat().st_size
    if byte_count > MAX_BYTES:
        errors.append(f"compressed file is {byte_count} bytes; cap is {MAX_BYTES}")
    try:
        with gzip.open(asset, "rt", encoding="utf-8") as handle:
            animation = json.load(handle)
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as error:
        errors.append(f"not valid gzip-compressed Lottie JSON: {error}")
        animation = {}

    if isinstance(animation, dict):
        if animation.get("w") != CANVAS or animation.get("h") != CANVAS:
            errors.append(f"canvas must be {CANVAS}x{CANVAS}; found {animation.get('w')}x{animation.get('h')}")
        if animation.get("fr") != FPS:
            errors.append(f"frame rate must be {FPS}; found {animation.get('fr')}")
        try:
            frames = float(animation.get("op")) - float(animation.get("ip", 0))
            if frames > MAX_FRAMES:
                errors.append(f"animation is {frames / FPS:.3f}s; cap is 3 seconds")
        except (TypeError, ValueError):
            errors.append("animation must provide numeric ip and op frame values")

        # The following flags mirror Telegram's published Bodymovin-TG
        # restrictions where those features have a stable Lottie signature.
        for node, path in walk(animation):
            ty = node.get("ty")
            if ty == 5:
                errors.append(f"text layer unsupported at {path}")
            if ty in {"sr", "mm", "rp", "gs"}:
                labels = {"sr": "star shape", "mm": "merge path", "rp": "repeater", "gs": "gradient stroke"}
                errors.append(f"{labels[ty]} unsupported at {path}")
            if node.get("ddd") == 1:
                errors.append(f"3D layer unsupported at {path}")
            if node.get("ao") == 1:
                errors.append(f"auto-oriented layer unsupported at {path}")
            if node.get("tm") not in (None, 0):
                errors.append(f"time remapping unsupported at {path}")
            if node.get("masksProperties"):
                errors.append(f"masks unsupported at {path}")
            if node.get("ef"):
                errors.append(f"layer effects unsupported at {path}")
            if isinstance(node.get("p"), str):
                errors.append(f"external/raster asset unsupported at {path}")

        if not animation.get("layers"):
            errors.append("animation has no layers")
        warnings.append("Loop closure and bounds are visual checks; render at frame 0 and final frame before upload.")

    if errors:
        for message in errors:
            print(f"FAIL: {message}")
        return 1
    print(
        f"PASS: valid TGS structure; {byte_count / 1024:.1f} KiB, "
        f"{animation['w']}x{animation['h']}, {animation['fr']} FPS, "
        f"{(float(animation['op']) - float(animation.get('ip', 0))) / FPS:.3f}s"
    )
    for message in warnings:
        print(f"WARN: {message}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

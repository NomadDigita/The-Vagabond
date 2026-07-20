#!/usr/bin/env python3
"""Validate a local asset against Telegram's regular-sticker media contract."""

from __future__ import annotations

import argparse
import gzip
import json
import subprocess
from pathlib import Path


MAX_TGS_BYTES = 64 * 1024
MAX_STATIC_BYTES = 512 * 1024
MAX_VIDEO_BYTES = 256 * 1024


def fail(message: str) -> None:
    raise SystemExit(f"INVALID: {message}")


def validate_tgs(path: Path) -> None:
    if path.stat().st_size > MAX_TGS_BYTES:
        fail(f"TGS is {path.stat().st_size} bytes; Telegram allows at most {MAX_TGS_BYTES}.")
    try:
        data = json.loads(gzip.decompress(path.read_bytes()).decode("utf-8"))
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as error:
        fail(f"not a valid gzipped Lottie TGS: {error}")
    if data.get("w") != 512 or data.get("h") != 512:
        fail("TGS canvas must be exactly 512x512.")
    frame_rate = data.get("fr")
    in_point, out_point = data.get("ip"), data.get("op")
    if not isinstance(frame_rate, (int, float)) or frame_rate <= 0:
        fail("TGS needs a positive frame rate.")
    if not isinstance(in_point, (int, float)) or not isinstance(out_point, (int, float)) or out_point <= in_point:
        fail("TGS needs a positive frame range.")
    if (out_point - in_point) / frame_rate > 3:
        fail("TGS animation must be no longer than three seconds.")


def validate_static(path: Path) -> None:
    if path.stat().st_size > MAX_STATIC_BYTES:
        fail(f"static sticker is {path.stat().st_size} bytes; Telegram allows at most {MAX_STATIC_BYTES}.")
    try:
        from PIL import Image
        with Image.open(path) as image:
            width, height = image.size
    except Exception as error:  # Pillow gives useful parser errors for malformed images.
        fail(f"cannot read image dimensions: {error}")
    if max(width, height) != 512 or min(width, height) > 512:
        fail(f"static sticker must have one side exactly 512px and the other no greater than 512px; got {width}x{height}.")


def validate_video(path: Path, ffprobe: str) -> None:
    if path.stat().st_size > MAX_VIDEO_BYTES:
        fail(f"video sticker is {path.stat().st_size} bytes; Telegram allows at most {MAX_VIDEO_BYTES}.")
    try:
        result = subprocess.run(
            [ffprobe, "-v", "error", "-show_entries", "format=duration:stream=codec_name,codec_type,width,height", "-of", "json", str(path)],
            check=True, text=True, capture_output=True,
        )
        probe = json.loads(result.stdout)
    except (OSError, subprocess.CalledProcessError, json.JSONDecodeError) as error:
        fail(f"ffprobe could not inspect video: {error}")
    streams = probe.get("streams", [])
    video = next((item for item in streams if item.get("codec_type") == "video"), None)
    if video is None or video.get("codec_name") != "vp9":
        fail("video sticker needs a VP9 video stream.")
    if any(item.get("codec_type") == "audio" for item in streams):
        fail("video sticker must not include audio.")
    width, height = video.get("width"), video.get("height")
    if not isinstance(width, int) or not isinstance(height, int) or max(width, height) != 512 or min(width, height) > 512:
        fail(f"video sticker must have one side exactly 512px and the other no greater than 512px; got {width}x{height}.")
    try:
        duration = float(probe["format"]["duration"])
    except (KeyError, TypeError, ValueError):
        fail("video sticker has no readable duration.")
    if duration > 3.05:  # Container rounding allowance; encoder target remains <=3 seconds.
        fail(f"video sticker duration {duration:.3f}s exceeds three seconds.")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("asset", type=Path)
    parser.add_argument("--ffprobe", default="ffprobe")
    args = parser.parse_args()
    path = args.asset.expanduser().resolve()
    if not path.is_file():
        fail(f"missing asset: {path}")
    suffix = path.suffix.lower()
    if suffix == ".tgs":
        validate_tgs(path)
        kind = "animated TGS"
    elif suffix in {".webp", ".png"}:
        validate_static(path)
        kind = "static image"
    elif suffix == ".webm":
        validate_video(path, args.ffprobe)
        kind = "video WebM"
    else:
        fail("asset must be .tgs, .webp, .png, or .webm.")
    print(f"VALID: {kind}; {path.name}; {path.stat().st_size} bytes")


if __name__ == "__main__":
    main()

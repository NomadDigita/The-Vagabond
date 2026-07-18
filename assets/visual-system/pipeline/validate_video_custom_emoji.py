#!/usr/bin/env python3
"""Validate a local VP9-alpha WebM against the Vagabond video-emoji contract.

This is deliberately an offline technical check. Telegram-client rendering and
sticker-set upload remain owner-side verification steps.

Example:
  python assets/visual-system/pipeline/validate_video_custom_emoji.py \
    assets/visual-system/animated/oracle_3d_v10/oracle_3d_v10.webm \
    --ffprobe C:/path/to/ffprobe.exe
"""

from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import sys
from pathlib import Path


MAX_DURATION_SECONDS = 3.0
MAX_BYTES = 256 * 1024


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("asset", type=Path)
    parser.add_argument("--ffprobe", help="Absolute path to ffprobe, or rely on PATH.")
    parser.add_argument("--ffmpeg", help="Absolute path to ffmpeg for alpha-plane decoding, or rely on PATH.")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    asset = args.asset.resolve()
    if not asset.is_file():
        raise SystemExit(f"Missing asset: {asset}")
    ffprobe = args.ffprobe or shutil.which("ffprobe")
    if not ffprobe:
        raise SystemExit("ffprobe is required; provide --ffprobe /absolute/path/to/ffprobe.")
    sibling_ffmpeg = Path(ffprobe).with_name("ffmpeg.exe")
    ffmpeg = args.ffmpeg or (str(sibling_ffmpeg) if sibling_ffmpeg.is_file() else shutil.which("ffmpeg"))
    if not ffmpeg:
        raise SystemExit("ffmpeg is required to validate decoded alpha; provide --ffmpeg /absolute/path/to/ffmpeg.")
    result = subprocess.run(
        [ffprobe, "-v", "error", "-show_streams", "-show_format", "-of", "json", str(asset)],
        check=True,
        capture_output=True,
        text=True,
    )
    metadata = json.loads(result.stdout)
    streams = metadata.get("streams", [])
    video = next((stream for stream in streams if stream.get("codec_type") == "video"), None)
    audio = [stream for stream in streams if stream.get("codec_type") == "audio"]
    errors: list[str] = []
    if video is None:
        errors.append("no video stream")
    else:
        if video.get("codec_name") != "vp9":
            errors.append(f"codec must be VP9, got {video.get('codec_name')}")
        if (video.get("width"), video.get("height")) != (100, 100):
            errors.append(f"frame must be 100x100, got {video.get('width')}x{video.get('height')}")
    duration = float(metadata.get("format", {}).get("duration", 0))
    if duration <= 0 or duration > MAX_DURATION_SECONDS:
        errors.append(f"duration must be >0 and <= {MAX_DURATION_SECONDS:g}s, got {duration:.3f}s")
    if audio:
        errors.append("audio stream present")
    if asset.stat().st_size > MAX_BYTES:
        errors.append(f"file must be <= {MAX_BYTES // 1024}KiB for this pipeline, got {asset.stat().st_size / 1024:.1f}KiB")
    # ffprobe may describe VP9-alpha as yuv420p while exposing alpha_mode=1 in
    # stream tags. Decode an RGBA frame instead of trusting either label.
    decoded = subprocess.run(
        [
            ffmpeg,
            "-v",
            "error",
            "-c:v",
            "libvpx-vp9",
            "-i",
            str(asset),
            "-frames:v",
            "1",
            "-f",
            "rawvideo",
            "-pix_fmt",
            "rgba",
            "-",
        ],
        check=True,
        capture_output=True,
    ).stdout
    expected_rgba_bytes = 100 * 100 * 4
    if len(decoded) != expected_rgba_bytes:
        errors.append(f"could not decode one 100x100 RGBA frame (got {len(decoded)} bytes)")
    else:
        alpha = decoded[3::4]
        if min(alpha) == 255:
            errors.append("decoded alpha plane is fully opaque; transparent background was lost")
        if max(alpha) == 0:
            errors.append("decoded alpha plane is fully transparent; object is absent")
    if errors:
        for error in errors:
            print(f"FAIL: {error}")
        raise SystemExit(1)
    print("PASS: VP9, decoded alpha, 100x100, silent, <=3s, and within 256KiB")
    print(f"Asset: {asset}")
    print(f"Duration: {duration:.3f}s | Size: {asset.stat().st_size / 1024:.1f}KiB")


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Create a small alpha VP9 WebM from an approved transparent PNG.

This is intentionally an offline finishing step for art that has already been
approved as a still.  It uses only FFmpeg: no Blender scene render, no web
service, and no API credential.  The animation is restrained by design: a
slow, reversible three-degree drift gives a crystal (or any glass object) a
living reflection without turning a carefully composed emoji into a spinning
sticker.

Example (PowerShell):
  python assets/visual-system/pipeline/animate_transparent_still.py `
    assets/visual-system/renders/crystal_3d_v12/crystal_3d_v12_100.png `
    --output assets/visual-system/animated/crystal_3d_v12/crystal_3d_v12.webm `
    --ffmpeg C:/.../ffmpeg.exe

Run validate_video_custom_emoji.py on the output before any Telegram upload.
"""

from __future__ import annotations

import argparse
import shutil
import subprocess
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path, help="Approved transparent PNG source.")
    parser.add_argument("--output", type=Path, required=True, help="Target VP9-alpha WebM.")
    parser.add_argument("--ffmpeg", help="Absolute ffmpeg path; otherwise PATH is used.")
    parser.add_argument("--fps", type=int, default=24)
    parser.add_argument("--seconds", type=float, default=2.0)
    parser.add_argument("--crf", type=int, default=43, help="VP9 quality; larger is smaller.")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    source = args.source.resolve()
    output = args.output.resolve()
    ffmpeg = args.ffmpeg or shutil.which("ffmpeg")
    if not source.is_file():
        raise SystemExit(f"Missing source PNG: {source}")
    if source.suffix.lower() != ".png":
        raise SystemExit("Source must be a transparent PNG, not a flattened screenshot.")
    if not ffmpeg:
        raise SystemExit("ffmpeg is required; pass --ffmpeg /absolute/path/to/ffmpeg.")
    if not 1 <= args.fps <= 30:
        raise SystemExit("--fps must be between 1 and 30 for Telegram video emoji.")
    if not 0 < args.seconds <= 3:
        raise SystemExit("--seconds must be greater than 0 and no more than 3.")
    if not 1 <= args.crf <= 63:
        raise SystemExit("--crf must be between 1 and 63.")

    output.parent.mkdir(parents=True, exist_ok=True)
    # The 108px work canvas prevents transparent corners from being cropped
    # while the source gently rocks.  `black@0` is deliberate: it preserves
    # alpha outside the original still instead of baking a dark background.
    # sin() starts and ends at zero over the two-second default, so Telegram's
    # loop point has no rotational jump.
    angle = f"0.0523598776*sin(2*PI*t/{args.seconds})"  # +/- 3 degrees
    video_filter = (
        "format=rgba,"
        "scale=108:108:flags=lanczos,"
        f"rotate='{angle}':ow=108:oh=108:c=black@0,"
        "crop=100:100:4:4,format=yuva420p"
    )
    command = [
        ffmpeg, "-y", "-loop", "1", "-framerate", str(args.fps), "-i", str(source),
        "-t", str(args.seconds), "-vf", video_filter,
        "-an", "-c:v", "libvpx-vp9", "-pix_fmt", "yuva420p",
        "-row-mt", "1", "-auto-alt-ref", "0", "-b:v", "0", "-crf", str(args.crf),
        "-deadline", "good", "-cpu-used", "4", str(output),
    ]
    subprocess.run(command, check=True)
    print(f"Created: {output}")
    print("Motion: transparent 3-degree reversible drift; no audio.")


if __name__ == "__main__":
    main()

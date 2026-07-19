#!/usr/bin/env python3
"""Compose the V13 crystal as a layered, animated alpha WebM.

The stable glass sphere/pedestal, rotating heart and independent sparkle
cycles are deliberately separate layers.  This avoids the blurry whole-object
drift used by V12 and makes the motion survive at Telegram's 100px size.
"""

from __future__ import annotations

import argparse
import math
import shutil
import subprocess
from pathlib import Path

from PIL import Image, ImageDraw, ImageEnhance, ImageFilter


FPS = 24
FRAMES = 48
ROOT = Path(__file__).resolve().parents[3]
VISUAL = ROOT / "assets" / "visual-system"
SOURCE = VISUAL / "source"
ASSET = "crystal_3d_v13"
FRAME_DIR = VISUAL / "animated" / ASSET / "frames"
OUTPUT = VISUAL / "animated" / ASSET / f"{ASSET}.webm"


def args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", type=Path, default=SOURCE / "crystal_3d_v13_base_chromakey.png")
    parser.add_argument("--heart", type=Path, default=SOURCE / "crystal_3d_v13_heart_chromakey.png")
    parser.add_argument("--ffmpeg", required=True)
    parser.add_argument("--frames", type=int, default=FRAMES)
    parser.add_argument("--no-encode", action="store_true")
    return parser.parse_args()


def key_green(path: Path) -> Image.Image:
    """Return a soft RGBA cutout from a deliberately flat green source."""
    image = Image.open(path).convert("RGBA")
    pixels = image.load()
    width, height = image.size
    for y in range(height):
        for x in range(width):
            red, green, blue, _ = pixels[x, y]
            dominance = green - max(red, blue)
            # Pure key green has very high dominance. A narrow transition keeps
            # bright rim pixels intact while removing antialiased green edges.
            alpha = max(0.0, min(1.0, (105.0 - dominance) / 42.0))
            desaturate = max(0.0, min(1.0, (dominance - 30.0) / 120.0))
            pixels[x, y] = (red, int(green * (1.0 - desaturate * 0.42)), blue, int(alpha * 255))
    return image


def star(size: int, opacity: float, phase: float) -> Image.Image:
    layer = Image.new("RGBA", (size, size))
    draw = ImageDraw.Draw(layer)
    center = size // 2
    core = max(2, int(size * 0.09))
    long = max(core + 2, int(size * (0.42 + 0.13 * phase)))
    alpha = int(255 * opacity)
    colour = (255, 247, 222, alpha)
    draw.polygon([(center, center - long), (center + core, center - core), (center, center + long), (center - core, center + core)], fill=colour)
    draw.polygon([(center - long, center), (center - core, center - core), (center + long, center), (center + core, center + core)], fill=colour)
    glow = layer.filter(ImageFilter.GaussianBlur(max(1, size // 12)))
    return Image.alpha_composite(glow, layer)


def scaled_heart(heart: Image.Image, frame: int, count: int) -> Image.Image:
    phase = math.tau * frame / count
    # A full turn is represented by the changing width, highlight and a small
    # tilt; at its narrowest it reads as a gemstone turning edge-on.
    width_factor = 0.24 + 0.76 * abs(math.cos(phase))
    tilt = math.degrees(math.sin(phase) * 0.13)
    brightness = 0.74 + 0.34 * (0.5 + 0.5 * math.cos(phase - 0.65))
    target_h = 188
    target_w = max(26, int(target_h * heart.width / heart.height * width_factor))
    result = heart.resize((target_w, target_h), Image.Resampling.LANCZOS)
    result = result.rotate(tilt, resample=Image.Resampling.BICUBIC, expand=True)
    return ImageEnhance.Brightness(result).enhance(brightness)


def render_frame(base: Image.Image, heart: Image.Image, index: int, count: int) -> Image.Image:
    canvas = base.copy()
    rotated = scaled_heart(heart, index, count)
    glow = rotated.copy().filter(ImageFilter.GaussianBlur(13))
    glow.putalpha(glow.getchannel("A").point(lambda value: int(value * 0.32)))
    position = ((512 - rotated.width) // 2, 144 - (rotated.height - 188) // 2)
    canvas.alpha_composite(glow, (position[0], position[1]))
    canvas.alpha_composite(rotated, position)
    # Reapply a faint glass reflection veil after the heart, making it read as
    # enclosed by the sphere instead of pasted on its front.
    veil = base.copy()
    veil.putalpha(veil.getchannel("A").point(lambda value: int(value * 0.18)))
    canvas.alpha_composite(veil)
    t = index / count
    for x, y, size, offset in ((166, 133, 60, 0.00), (360, 224, 43, 0.31), (205, 289, 35, 0.64)):
        pulse = 0.14 + 0.86 * max(0.0, math.sin(math.tau * (t + offset))) ** 4
        sparkle = star(size, pulse, pulse)
        canvas.alpha_composite(sparkle, (x - size // 2, y - size // 2))
    return canvas.resize((100, 100), Image.Resampling.LANCZOS)


def main() -> None:
    options = args()
    if not options.base.is_file() or not options.heart.is_file():
        raise SystemExit("Missing chroma-key source layer(s).")
    if not 2 <= options.frames <= 72:
        raise SystemExit("--frames must be between 2 and 72.")
    base = key_green(options.base).resize((512, 512), Image.Resampling.LANCZOS)
    heart = key_green(options.heart)
    FRAME_DIR.mkdir(parents=True, exist_ok=True)
    for index in range(options.frames):
        render_frame(base, heart, index, options.frames).save(FRAME_DIR / f"frame_{index + 1:04d}.png")
    if options.no_encode:
        return
    ffmpeg = options.ffmpeg or shutil.which("ffmpeg")
    if not ffmpeg:
        raise SystemExit("ffmpeg not found")
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    subprocess.run([ffmpeg, "-y", "-framerate", str(FPS), "-start_number", "1", "-i", str(FRAME_DIR / "frame_%04d.png"), "-frames:v", str(options.frames), "-an", "-c:v", "libvpx-vp9", "-pix_fmt", "yuva420p", "-row-mt", "1", "-auto-alt-ref", "0", "-b:v", "0", "-crf", "39", "-deadline", "good", "-cpu-used", "4", str(OUTPUT)], check=True)
    print(f"Created {OUTPUT}")


if __name__ == "__main__":
    main()

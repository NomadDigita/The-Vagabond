#!/usr/bin/env python3
"""Render a high-clarity V15 WebM candidate from the approved V13 crystal art.

This intentionally creates a separate review asset.  It keeps the physical
glass ball and gold base but makes the animated features larger at the actual
100px delivery size: one broad faceted heart turn, a sweeping specular slash,
and three high-contrast star pulses.  It does not alter V13.
"""

from __future__ import annotations

import argparse
import math
import subprocess
from pathlib import Path

from PIL import Image, ImageDraw, ImageEnhance, ImageFilter


FPS = 24
FRAMES = 48
ROOT = Path(__file__).resolve().parents[3]
VISUAL = ROOT / "assets" / "visual-system"
SOURCE = VISUAL / "source"
ASSET = "crystal_3d_v15_quality"
FRAME_DIR = VISUAL / "animated" / ASSET / "frames"
OUTPUT = VISUAL / "animated" / ASSET / f"{ASSET}.webm"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--ffmpeg", required=True)
    parser.add_argument("--frames", type=int, default=FRAMES)
    parser.add_argument("--no-encode", action="store_true")
    return parser.parse_args()


def key_green(path: Path) -> Image.Image:
    """Remove the flat chroma backing while retaining anti-aliased highlights."""
    image = Image.open(path).convert("RGBA")
    pixels = image.load()
    for y in range(image.height):
        for x in range(image.width):
            red, green, blue, _ = pixels[x, y]
            dominance = green - max(red, blue)
            alpha = max(0.0, min(1.0, (106.0 - dominance) / 42.0))
            green_bleed = max(0.0, min(1.0, (dominance - 30.0) / 120.0))
            pixels[x, y] = (red, int(green * (1.0 - green_bleed * 0.42)), blue, int(alpha * 255))
    return image


def add_star(canvas: Image.Image, x: int, y: int, size: int, intensity: float) -> None:
    """Place a crisp four-point star plus a small, controlled glow."""
    layer = Image.new("RGBA", canvas.size)
    draw = ImageDraw.Draw(layer)
    arm = max(5, int(size * (0.52 + intensity * 0.48)))
    core = max(2, size // 9)
    alpha = int(255 * intensity)
    colour = (255, 250, 231, alpha)
    draw.polygon([(x, y - arm), (x + core, y - core), (x, y + arm), (x - core, y + core)], fill=colour)
    draw.polygon([(x - arm, y), (x - core, y - core), (x + arm, y), (x + core, y + core)], fill=colour)
    glow = layer.filter(ImageFilter.GaussianBlur(max(2, size // 8)))
    glow.putalpha(glow.getchannel("A").point(lambda value: int(value * 0.52)))
    canvas.alpha_composite(glow)
    canvas.alpha_composite(layer)


def make_heart(heart: Image.Image, frame: int, count: int) -> Image.Image:
    phase = math.tau * frame / count
    # A slower full turn: the wide front view gets most of the animation time,
    # but a clearly visible edge-on moment proves actual rotation in Telegram.
    width_factor = 0.22 + 0.90 * (abs(math.cos(phase)) ** 0.55)
    height = 232
    width = max(34, int(height * heart.width / heart.height * width_factor))
    result = heart.resize((width, height), Image.Resampling.LANCZOS)
    result = result.rotate(math.degrees(math.sin(phase) * 0.085), Image.Resampling.BICUBIC, expand=True)
    return ImageEnhance.Contrast(ImageEnhance.Brightness(result).enhance(0.86 + 0.28 * (0.5 + 0.5 * math.cos(phase - 0.55)))).enhance(1.14)


def frame_image(base: Image.Image, heart: Image.Image, frame: int, count: int) -> Image.Image:
    canvas = base.copy()
    rotating = make_heart(heart, frame, count)
    position = ((512 - rotating.width) // 2, 120 - (rotating.height - 232) // 2)
    # Short, bright inner bloom: it makes the turning heart readable without
    # blurring every glass edge.
    bloom = rotating.filter(ImageFilter.GaussianBlur(9))
    bloom.putalpha(bloom.getchannel("A").point(lambda value: int(value * 0.34)))
    canvas.alpha_composite(bloom, position)
    canvas.alpha_composite(rotating, position)
    # Refractive veil restores the impression that the heart is inside glass.
    veil = base.copy()
    veil.putalpha(veil.getchannel("A").point(lambda value: int(value * 0.13)))
    canvas.alpha_composite(veil)
    t = frame / count
    # Larger, simpler stars survive the 100px target much more effectively
    # than micro-facets.
    for x, y, size, phase in ((154, 132, 68, 0.00), (366, 192, 54, 0.31), (223, 285, 44, 0.64)):
        intensity = 0.18 + 0.82 * max(0.0, math.sin(math.tau * (t + phase))) ** 5
        add_star(canvas, x, y, size, intensity)
    # A single moving highlight is deliberately broad and low-opacity: it
    # reads as a curved glass reflection rather than raster shimmer.
    slash = Image.new("RGBA", canvas.size)
    draw = ImageDraw.Draw(slash)
    sweep = 95 + int(110 * (0.5 + 0.5 * math.sin(math.tau * t)))
    draw.rounded_rectangle((sweep, 138, sweep + 22, 318), radius=11, fill=(255, 224, 255, 44))
    slash = slash.rotate(-25, center=(256, 256), resample=Image.Resampling.BICUBIC)
    slash = slash.filter(ImageFilter.GaussianBlur(5))
    canvas.alpha_composite(slash)
    return canvas.resize((100, 100), Image.Resampling.LANCZOS)


def main() -> None:
    options = parse_args()
    if not 2 <= options.frames <= 72:
        raise SystemExit("--frames must be between 2 and 72")
    base_path = SOURCE / "crystal_3d_v13_base_chromakey.png"
    heart_path = SOURCE / "crystal_3d_v13_heart_chromakey.png"
    base = key_green(base_path).resize((512, 512), Image.Resampling.LANCZOS)
    heart = key_green(heart_path)
    FRAME_DIR.mkdir(parents=True, exist_ok=True)
    for index in range(options.frames):
        frame_image(base, heart, index, options.frames).save(FRAME_DIR / f"frame_{index + 1:04d}.png")
    if options.no_encode:
        return
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    subprocess.run([
        options.ffmpeg, "-y", "-framerate", str(FPS), "-start_number", "1",
        "-i", str(FRAME_DIR / "frame_%04d.png"), "-frames:v", str(options.frames),
        "-an", "-c:v", "libvpx-vp9", "-pix_fmt", "yuva420p", "-row-mt", "1",
        "-auto-alt-ref", "0", "-b:v", "0", "-crf", "17", "-deadline", "good",
        "-cpu-used", "2", str(OUTPUT),
    ], check=True)
    print(f"Created {OUTPUT}")


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Build a crisp native-vector V14 crystal TGS custom emoji.

This is intentionally distinct from the WebM line: the shape language is
large, high-contrast vector geometry designed for Telegram's 512px / 60fps
TGS renderer.  It uses no raster assets, masks, effects, or Lottie star-shape
primitive.
"""

from __future__ import annotations

import gzip
import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / "assets" / "visual-system" / "animated" / "crystal_tgs_v14" / "crystal_tgs_v14.tgs"
FRAMES = 120


def prop(value):
    return {"a": 0, "k": value}


def keyed(values):
    return {"a": 1, "k": [{"t": frame, "s": value} for frame, value in values]}


def path(points, closed=True):
    return {"a": 0, "k": {"i": [[0, 0] for _ in points], "o": [[0, 0] for _ in points], "v": points, "c": closed}}


def fill(colour, opacity=100):
    return {"ty": "fl", "c": prop(colour), "o": prop(opacity), "r": 1, "nm": "fill"}


def stroke(colour, width, opacity=100):
    return {"ty": "st", "c": prop(colour), "o": prop(opacity), "w": prop(width), "lc": 2, "lj": 2, "ml": 4, "nm": "stroke"}


def transform(position=(256, 256), scale=(100, 100), opacity=100):
    return {"ty": "tr", "p": prop(list(position)), "a": prop([0, 0]), "s": scale if isinstance(scale, dict) else prop(list(scale)), "r": prop(0), "o": opacity if isinstance(opacity, dict) else prop(opacity), "sk": prop(0), "sa": prop(0)}


def layer(index, name, shapes, *, position=(256, 256), scale=(100, 100), opacity=100):
    return {"ddd": 0, "ind": index, "ty": 4, "nm": name, "sr": 1, "ks": transform(position, scale, opacity), "ao": 0, "shapes": shapes, "ip": 0, "op": FRAMES, "st": 0, "bm": 0}


def ellipse(size, colour, outline=None, outline_width=0):
    items = [{"ty": "el", "p": prop([0, 0]), "s": prop([size, size]), "nm": "orb"}, fill(colour)]
    if outline:
        items.append(stroke(outline, outline_width))
    items.append(transform())
    return items


def diamond(size):
    return path([[0, -size], [size, 0], [0, size], [-size, 0]])


def heart_path(scale=1.0):
    return path([[0, 126 * scale], [-122 * scale, 16 * scale], [-142 * scale, -75 * scale], [-76 * scale, -130 * scale], [0, -76 * scale], [76 * scale, -130 * scale], [142 * scale, -75 * scale], [122 * scale, 16 * scale]])


def star_shape(size):
    return path([[0, -size], [size * .22, -size * .22], [size, 0], [size * .22, size * .22], [0, size], [-size * .22, size * .22], [-size, 0], [-size * .22, -size * .22]])


def main():
    layers = []
    # Strong nested circles make the orb read as liquid glass at small scale.
    layers.append(layer(1, "violet outer orb", ellipse(440, [0.20, 0.035, 0.45, 1], [0.93, 0.74, 1, 1], 10)))
    layers.append(layer(2, "violet inner orb", ellipse(402, [0.36, 0.10, 0.74, 1], [0.74, 0.43, 1, 1], 6)))
    layers.append(layer(3, "crystal aura", ellipse(330, [0.49, 0.24, 0.93, 1], [0.95, 0.79, 1, 1], 3), opacity=42))
    # Gold pedestal is deliberately large and simple instead of photoreal micro-detail.
    pedestal = [path([[-145, 175], [-120, 128], [120, 128], [145, 175], [120, 208], [-120, 208]]), fill([0.76, 0.39, 0.06, 1]), stroke([1, 0.82, 0.32, 1], 8), transform()]
    layers.append(layer(4, "gold vagabond pedestal", pedestal))
    # Heart shrinks edge-on midway then returns, with facet diamonds moving with it.
    heart_scale = keyed([(0, [100, 100]), (30, [18, 104]), (60, [100, 100]), (90, [18, 104]), (120, [100, 100])])
    heart = [heart_path(), fill([0.83, 0.49, 1, 1]), stroke([1, 0.90, 1, 1], 7), transform(scale=heart_scale)]
    layers.append(layer(5, "rotating crystal heart", heart))
    for index, (x, y, size, colour) in enumerate(((-45, -16, 42, [0.98, 0.84, 1, 1]), (45, -16, 42, [0.65, 0.28, 0.96, 1]), (0, 52, 36, [1, 0.92, 1, 1]))):
        facet = [diamond(size), fill(colour, 72), stroke([1, 0.86, 1, 1], 3, 65), transform(position=(x, y), scale=heart_scale)]
        layers.append(layer(6 + index, f"heart facet {index}", facet))
    # Three independently pulsing manually-drawn four-point stars.
    for index, (x, y, size, offset) in enumerate(((-132, -120, 27, 0), (135, -15, 20, 20), (-90, 90, 16, 40))):
        pulse = keyed([(0 + offset, 12), (12 + offset, 100), (24 + offset, 12), (72 + offset, 12), (84 + offset, 100), (96 + offset, 12), (120 + offset, 12)])
        sparkle = [star_shape(size), fill([1, 0.96, 0.76, 1]), transform(position=(x, y), opacity=pulse)]
        layers.append(layer(9 + index, f"independent star glint {index}", sparkle))
    # Curved glass highlight is an ellipse stroke whose opacity glides around the loop.
    highlight_opacity = keyed([(0, 18), (30, 66), (60, 18), (90, 48), (120, 18)])
    highlight = [{"ty": "el", "p": prop([-70, -75]), "s": prop([210, 150]), "nm": "glass highlight ellipse"}, stroke([1, 0.92, 1, 1], 13), transform(opacity=highlight_opacity)]
    layers.append(layer(12, "moving glass highlight", highlight))
    composition = {"v": "5.5.7", "fr": 60, "ip": 0, "op": FRAMES, "w": 512, "h": 512, "nm": "The Vagabond Crystal v14", "ddd": 0, "assets": [], "layers": layers}
    OUT.parent.mkdir(parents=True, exist_ok=True)
    with gzip.open(OUT, "wb", compresslevel=9) as output:
        output.write(json.dumps(composition, separators=(",", ":")).encode("utf-8"))
    print(f"Created {OUT} ({OUT.stat().st_size} bytes)")


if __name__ == "__main__":
    main()

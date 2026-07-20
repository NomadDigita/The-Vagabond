#!/usr/bin/env python3
"""Build the first production family of sharp native-vector Vagabond TGS emoji.

Every asset is a separate 512px/60fps Lottie composition with a subject-led
silhouette and subject-specific motion.  There are no raster images, masks,
effects, expressions, or unsupported Lottie primitives.
"""

from __future__ import annotations

import gzip
import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / "assets" / "visual-system" / "animated"
FRAMES = 120
VIOLET = [0.31, 0.08, 0.64, 1]
LAVENDER = [0.79, 0.55, 1, 1]
GOLD = [0.93, 0.60, 0.12, 1]
STEEL = [0.64, 0.74, 0.86, 1]
OBSIDIAN = [0.055, 0.075, 0.12, 1]
WHITE = [1, 0.96, 0.84, 1]
RED = [0.92, 0.15, 0.22, 1]
CYAN = [0.15, 0.84, 1, 1]


def prop(value): return {"a": 0, "k": value}
def key(values): return {"a": 1, "k": [{"t": t, "s": v} for t, v in values]}
def path(points, closed=True): return {"a": 0, "k": {"i": [[0, 0]] * len(points), "o": [[0, 0]] * len(points), "v": points, "c": closed}}
def fill(c, opacity=100): return {"ty": "fl", "c": prop(c), "o": prop(opacity), "r": 1}
def stroke(c, width, opacity=100): return {"ty": "st", "c": prop(c), "o": prop(opacity), "w": prop(width), "lc": 2, "lj": 2, "ml": 4}
def tr(position=(0, 0), scale=(100, 100), rotation=0, opacity=100): return {"ty": "tr", "p": position if isinstance(position, dict) else prop(list(position)), "a": prop([0, 0]), "s": scale if isinstance(scale, dict) else prop(list(scale)), "r": rotation if isinstance(rotation, dict) else prop(rotation), "o": opacity if isinstance(opacity, dict) else prop(opacity), "sk": prop(0), "sa": prop(0)}


def layer(index, name, shapes, pos=(256, 256), scale=(100, 100), rotation=0, opacity=100):
    return {"ddd": 0, "ind": index, "ty": 4, "nm": name, "sr": 1, "ks": tr(pos, scale, rotation, opacity), "ao": 0, "shapes": shapes, "ip": 0, "op": FRAMES, "st": 0, "bm": 0}


def poly(points, colour, outline=WHITE, width=6, *, transform=None):
    return [path(points), fill(colour), stroke(outline, width), transform or tr()]


def ellipse(size, colour, outline=None, width=0, *, transform=None):
    values = [{"ty": "el", "p": prop([0, 0]), "s": prop(list(size))}, fill(colour)]
    if outline: values.append(stroke(outline, width))
    values.append(transform or tr())
    return values


def star(size):
    return path([[0,-size],[size*.22,-size*.22],[size,0],[size*.22,size*.22],[0,size],[-size*.22,size*.22],[-size,0],[-size*.22,-size*.22]])


def base_layers(index):
    # A subtle violet badge aids visibility but never replaces the subject.
    return [layer(index, "violet aura", ellipse((448, 448), VIOLET, LAVENDER, 8), opacity=46)]


def map_icon():
    layers = base_layers(1)
    folds = [[-170,-115],[-55,-145],[45,-110],[170,-145],[170,125],[45,95],[-55,128],[-170,96]]
    layers.append(layer(2, "folded expedition map", poly(folds, OBSIDIAN, GOLD, 9)))
    for i, x in enumerate((-55, 45)):
        layers.append(layer(3+i, "map fold", [path([[x,-140],[x,115]], False), stroke(STEEL, 7, 80), tr()]))
    route = [path([[-135,65],[-80,20],[-20,45],[32,-28],[102,-55],[135,-112]], False), stroke(CYAN, 11), tr()]
    layers.append(layer(5, "animated route", route))
    marker_pos = key([(0,[-135,65]),(48,[32,-28]),(96,[135,-112]),(120,[-135,65])])
    layers.append(layer(6, "route marker", ellipse((30,30), GOLD, WHITE, 4, transform=tr(position=marker_pos))))
    return layers


def raid_icon():
    layers = base_layers(1)
    for i, size in enumerate((300, 190, 80)):
        layers.append(layer(2+i, f"raid target ring {i}", ellipse((size,size), [0,0,0,0], RED if i<2 else GOLD, 16), opacity=90))
    lance_a = poly([[-155,125],[-118,155],[168,-128],[138,-160]], STEEL, WHITE, 5)
    lance_b = poly([[-155,-125],[-118,-155],[168,128],[138,160]], STEEL, WHITE, 5)
    layers.append(layer(5, "first raid lance", lance_a, rotation=key([(0,-4),(60,4),(120,-4)])))
    layers.append(layer(6, "second raid lance", lance_b, rotation=key([(0,4),(60,-4),(120,4)])))
    layers.append(layer(7, "raid impact", [star(48), fill(GOLD), tr(opacity=key([(0,15),(20,100),(40,15),(80,15),(100,100),(120,15)]))]))
    return layers


def base_icon():
    layers = base_layers(1)
    fortress = [[-175,150],[-175,-35],[-125,-35],[-125,-105],[-70,-105],[-70,-35],[70,-35],[70,-105],[125,-105],[125,-35],[175,-35],[175,150]]
    layers.append(layer(2, "fortified Vagabond base", poly(fortress, OBSIDIAN, GOLD, 10)))
    layers.append(layer(3, "base gate", poly([[-54,150],[-54,42],[54,42],[54,150]], VIOLET, LAVENDER, 7)))
    beacon = [star(32), fill(CYAN), tr(position=(0, -135), opacity=key([(0, 20), (30, 100), (60, 20), (90, 100), (120, 20)]))]
    layers.append(layer(4, "base beacon", beacon))
    return layers


def transport_icon():
    layers = base_layers(1)
    body = [[-178,85],[-178,-45],[-55,-45],[-20,-112],[90,-112],[154,-45],[180,-45],[180,85]]
    layers.append(layer(2, "armored transport body", poly(body, OBSIDIAN, GOLD, 9)))
    layers.append(layer(3, "cargo signal panel", poly([[-145,-14],[-62,-14],[-62,36],[-145,36]], VIOLET, LAVENDER, 5)))
    for i, x in enumerate((-105, 105)):
        spin = key([(0,0),(120,360)])
        wheel = ellipse((82,82), OBSIDIAN, STEEL, 10, transform=tr(position=(x,96), rotation=spin))
        layers.append(layer(4+i, f"transport wheel {i}", wheel))
        layers.append(layer(6+i, f"wheel hub {i}", ellipse((28,28), GOLD, WHITE, 4, transform=tr(position=(x,96), rotation=spin))))
    headlight = ellipse((28, 28), CYAN, WHITE, 4, transform=tr(position=(144, -48), opacity=key([(0, 35), (24, 100), (48, 35), (72, 100), (120, 35)])))
    layers.append(layer(8, "transport headlight", headlight))
    return layers


def combat_icon():
    layers = base_layers(1)
    sword = [[-22,142],[-5,142],[25,-135],[0,-175],[-25,-135]]
    layers.append(layer(2, "combat blade one", poly(sword, STEEL, WHITE, 5), rotation=key([(0,-46),(60,-40),(120,-46)])))
    layers.append(layer(3, "combat blade two", poly(sword, STEEL, WHITE, 5), rotation=key([(0,46),(60,40),(120,46)])))
    core = ellipse((78, 78), RED, GOLD, 8, transform=tr(scale=key([(0, [82, 82]), (30, [112, 112]), (60, [82, 82]), (90, [112, 112]), (120, [82, 82])])))
    layers.append(layer(4, "combat core", core))
    impact = [star(50), fill(WHITE), tr(opacity=key([(0, 0), (14, 100), (28, 0), (74, 0), (88, 100), (102, 0), (120, 0)]))]
    layers.append(layer(5, "combat impact spark", impact))
    return layers


def satellite_icon():
    layers = base_layers(1)
    dish = [path([[-155,-45],[100,-120],[145,38],[-98,110]]), fill(OBSIDIAN), stroke(STEEL, 9), tr(rotation=key([(0,-7),(60,7),(120,-7)]))]
    layers.append(layer(2, "satellite dish", dish))
    layers.append(layer(3, "signal receiver", ellipse((45,45), GOLD, WHITE, 5, transform=tr(position=(80,-70)))))
    for i, size in enumerate((130,205,280)):
        scale = key([(0,[20,20]),(60,[100,100]),(120,[20,20])])
        layers.append(layer(4+i, f"satellite radio ring {i}", ellipse((size,size), [0,0,0,0], CYAN, 7, transform=tr(position=(80,-70), scale=scale, opacity=key([(0,0),(24,80),(60,0),(84,80),(120,0)])))))
    return layers


def shield_icon():
    layers = base_layers(1)
    shield = [[0,-178],[145,-112],[118,74],[0,170],[-118,74],[-145,-112]]
    layers.append(layer(2, "Vagabond shield", poly(shield, OBSIDIAN, STEEL, 12)))
    layers.append(layer(3, "shield core", poly([[0,-105],[75,-65],[58,42],[0,100],[-58,42],[-75,-65]], VIOLET, LAVENDER, 7)))
    layers.append(layer(4, "shield charge", [star(38), fill(CYAN), tr(opacity=key([(0,18),(30,100),(60,18),(90,100),(120,18)]))]))
    return layers


def gear_icon():
    layers = base_layers(1)
    teeth=[]
    for i in range(16):
        a=i*22.5
        import math
        r=155
        teeth.append([r*math.cos(math.radians(a)), r*math.sin(math.radians(a))])
    spin=key([(0,0),(120,360)])
    layers.append(layer(2, "Vagabond gear", [path(teeth), fill(OBSIDIAN), stroke(GOLD,18), tr(rotation=spin)]))
    layers.append(layer(3, "gear hub", ellipse((126,126), VIOLET, LAVENDER, 9, transform=tr(rotation=spin))))
    layers.append(layer(4, "gear spark", [star(32), fill(WHITE), tr(position=(138,-112), opacity=key([(0,0),(18,100),(36,0),(90,0),(108,100),(120,0)]))]))
    return layers


def write(key, layers):
    data={"v":"5.5.7","fr":60,"ip":0,"op":FRAMES,"w":512,"h":512,"nm":f"The Vagabond {key}","ddd":0,"assets":[],"layers":layers}
    target=OUT/f"{key}_tgs_v14"/f"{key}_tgs_v14.tgs"
    target.parent.mkdir(parents=True, exist_ok=True)
    with gzip.open(target,"wb",compresslevel=9) as f: f.write(json.dumps(data,separators=(",",":")).encode())
    print(f"{key}: {target.stat().st_size} bytes")


def main():
    for key, builder in {"map":map_icon,"raid":raid_icon,"base":base_icon,"transport":transport_icon,"combat":combat_icon,"satellite":satellite_icon,"shield":shield_icon,"gear":gear_icon}.items():
        write(key,builder())


if __name__=="__main__": main()

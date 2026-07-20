#!/usr/bin/env python3
"""Build six native-vector narrative event stickers for The Vagabond.

This is deliberately separate from the general sticker builder.  Every scene
uses only Telegram-safe Lottie shapes, 512px transparent composition, and a
clear faction/event silhouette rather than a UI glyph.
"""

from __future__ import annotations

import gzip
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
CORE_PATH = ROOT / "assets/visual-system/pipeline/build_vagabond_tgs_core_v14.py"
spec = importlib.util.spec_from_file_location("vagabond_core", CORE_PATH)
core = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(core)

OUT = ROOT / "assets/visual-system/stickers/animated/v1-events"


def pulse(values):
    return core.key(values)


def aura(index, colour):
    return core.layer(index, "event aura", core.ellipse((465, 465), colour, core.LAVENDER, 7), opacity=36)


def commander(index, x, colour, facing=1):
    """Armoured survivor, used as a scene actor—not the subject itself."""
    head = core.ellipse((92, 92), core.OBSIDIAN, core.WHITE, 7, transform=core.tr(position=(x, -72)))
    visor = core.poly([[x-34, -87], [x+34, -87], [x+24, -58], [x-28, -58]], core.CYAN, core.WHITE, 4)
    body = core.poly([[x-58, 160], [x-45, 3], [x+48, 3], [x+72, 160]], colour, core.STEEL, 8)
    arm_x = x + (74 * facing)
    arm = [core.path([[x + (26 * facing), 34], [arm_x, -16]], False), core.stroke(core.STEEL, 18), core.tr()]
    return [
        core.layer(index, "armoured survivor helmet", head),
        core.layer(index + 1, "cyan expedition visor", visor),
        core.layer(index + 2, "armoured survivor body", body),
        core.layer(index + 3, "survivor raised arm", arm),
    ]


def steel_vanguard_last_stand():
    layers = [aura(1, core.CYAN)]
    shield = [[-126, -152], [0, -193], [126, -152], [151, -25], [86, 136], [0, 182], [-86, 136], [-151, -25]]
    layers.append(core.layer(2, "Steel Vanguard kinetic shield", core.poly(shield, core.OBSIDIAN, core.CYAN, 14), scale=pulse([(0,[91,91]), (35,[106,106]), (70,[91,91]), (120,[91,91])])))
    layers.append(core.layer(3, "shield faction crest", core.poly([[0,-101],[54,-51],[34,61],[0,99],[-34,61],[-54,-51]], core.CYAN, core.WHITE, 6)))
    breach = [core.path([[-213,-30],[-145,-10],[-185,44]], False), core.stroke(core.RED, 15), core.tr(opacity=pulse([(0,0),(22,100),(43,0),(82,0),(104,100),(120,0)]))]
    layers.append(core.layer(4, "incoming red breach blast", breach))
    layers.extend(commander(10, 0, core.STEEL))
    return layers


def rust_nomad_ambush():
    layers = [aura(1, core.GOLD)]
    dune = core.poly([[-220,139],[-115,54],[-20,118],[78,14],[220,119],[220,214],[-220,214]], core.GOLD, core.STEEL, 7)
    layers.append(core.layer(2, "rust nomad sand ridge", dune))
    crawler = [[-185,76],[-141,16],[-32,16],[10,-43],[105,-43],[165,22],[187,22],[187,91]]
    layers.append(core.layer(3, "rust nomad ambush crawler", core.poly(crawler, core.OBSIDIAN, core.GOLD, 10), pos=(256,260), rotation=pulse([(0,-2),(60,2),(120,-2)])))
    for index, x in enumerate((-103, 122)):
        layers.append(core.layer(4+index, "crawler wheel", core.ellipse((72,72), core.OBSIDIAN, core.STEEL, 9, transform=core.tr(position=(x,101), rotation=pulse([(0,0),(120,360)])))))
    dust = core.ellipse((120,42), core.WHITE, None, transform=core.tr(position=(-201,110), scale=pulse([(0,[25,25]),(45,[100,100]),(90,[25,25]),(120,[25,25])]), opacity=pulse([(0,0),(22,55),(66,0),(120,0)])))
    layers.append(core.layer(7, "ambush dust plume", dust))
    return layers


def rebellion_uprising():
    layers = [aura(1, core.RED)]
    fire = core.poly([[0,-68],[47,7],[24,88],[0,132],[-29,88],[-48,8]], core.GOLD, core.WHITE, 5)
    layers.append(core.layer(2, "rebellion campfire", fire, scale=pulse([(0,[82,82]),(30,[108,108]),(60,[82,82]),(90,[108,108]),(120,[82,82])])))
    pole = [core.path([[84,160],[84,-174]], False), core.stroke(core.STEEL, 13), core.tr()]
    banner = core.poly([[84,-168],[194,-128],[165,-23],[84,-55]], core.RED, core.WHITE, 7)
    layers.append(core.layer(3, "rebel banner pole", pole))
    layers.append(core.layer(4, "rebel uprising banner", banner, rotation=pulse([(0,-3),(60,4),(120,-3)])))
    signal = core.ellipse((74,74), [0,0,0,0], core.CYAN, 8, transform=core.tr(position=(84,-173), scale=pulse([(0,[20,20]),(60,[210,210]),(120,[20,20])]), opacity=pulse([(0,0),(12,100),(60,0),(72,100),(120,0)])))
    layers.append(core.layer(5, "rebellion broadcast", signal))
    layers.extend(commander(10, -104, core.RED, 1))
    return layers


def drone_hive_incursion():
    layers = [aura(1, core.VIOLET)]
    hive = core.poly([[0,-171],[131,-99],[151,38],[68,151],[-72,151],[-151,38],[-131,-99]], core.OBSIDIAN, core.VIOLET, 13)
    layers.append(core.layer(2, "rogue drone hive", hive, rotation=pulse([(0,-3),(60,3),(120,-3)])))
    optic = core.ellipse((69,69), core.RED, core.GOLD, 7, transform=core.tr(scale=pulse([(0,[78,78]),(30,[117,117]),(60,[78,78]),(90,[117,117]),(120,[78,78])])))
    layers.append(core.layer(3, "hive hostile optic", optic))
    for index, (x, y) in enumerate(((-175,-89),(175,-89),(-185,102),(185,102))):
        drone = core.poly([[x,y-24],[x+28,y],[x,y+24],[x-28,y]], core.STEEL, core.WHITE, 4)
        layers.append(core.layer(4+index, "attacking scout drone", drone, pos=pulse([(0,[256,256]),(60,[256+x*.09,256+y*.06]),(120,[256,256])])))
    return layers


def alliance_convocation():
    layers = [aura(1, core.CYAN)]
    left = core.poly([[-184,18],[-77,-53],[-16,5],[-82,80]], core.STEEL, core.WHITE, 8)
    right = core.poly([[184,18],[77,-53],[16,5],[82,80]], core.VIOLET, core.LAVENDER, 8)
    layers.append(core.layer(2, "alliance left gauntlet", left, pos=pulse([(0,[230,256]),(55,[256,256]),(120,[230,256])])))
    layers.append(core.layer(3, "alliance right gauntlet", right, pos=pulse([(0,[282,256]),(55,[256,256]),(120,[282,256])])))
    clasp = core.ellipse((88,88), core.GOLD, core.WHITE, 8, transform=core.tr(opacity=pulse([(0,0),(45,0),(55,100),(92,100),(120,0)]), scale=pulse([(0,[40,40]),(55,[100,100]),(92,[100,100]),(120,[40,40])])) )
    layers.append(core.layer(4, "alliance clasp energy", clasp))
    rings = core.ellipse((330,330), [0,0,0,0], core.CYAN, 9, transform=core.tr(scale=pulse([(0,[35,35]),(60,[105,105]),(120,[35,35])]), opacity=pulse([(0,0),(15,80),(60,0),(75,80),(120,0)])))
    layers.append(core.layer(5, "federation signal ring", rings))
    return layers


def wasteland_eclipse():
    layers = [aura(1, core.OBSIDIAN)]
    moon = core.ellipse((306,306), core.GOLD, core.WHITE, 9, transform=core.tr(position=(0,-18)))
    shadow = core.ellipse((292,292), core.OBSIDIAN, core.VIOLET, 8, transform=core.tr(position=pulse([(0,[43,-18]),(60,[0,-18]),(120,[43,-18])])))
    layers.append(core.layer(2, "wasteland eclipse moon", moon))
    layers.append(core.layer(3, "eclipse travelling shadow", shadow))
    ridge = core.poly([[-226,185],[-132,62],[-59,125],[17,32],[99,129],[161,67],[225,187]], core.OBSIDIAN, core.STEEL, 8)
    layers.append(core.layer(4, "wasteland black ridge", ridge))
    for index, x in enumerate((-150, 142)):
        beam = [core.path([[x,145],[x,-12]], False), core.stroke(core.CYAN, 9), core.tr(opacity=pulse([(0,15),(28,100),(56,15),(84,100),(120,15)]))]
        layers.append(core.layer(5+index, "distant outpost beacon", beam))
    return layers


SCENES = {
    "steel_vanguard_last_stand": steel_vanguard_last_stand,
    "rust_nomad_ambush": rust_nomad_ambush,
    "rebellion_uprising": rebellion_uprising,
    "drone_hive_incursion": drone_hive_incursion,
    "alliance_convocation": alliance_convocation,
    "wasteland_eclipse": wasteland_eclipse,
}


def write(key, build):
    animation = {"v":"5.5.7", "fr":60, "ip":0, "op":120, "w":512, "h":512,
                 "nm":f"The Vagabond Event Sticker {key}", "ddd":0, "assets":[], "layers":build()}
    target = OUT / key / f"{key}.tgs"
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(gzip.compress(json.dumps(animation,separators=(",", ":")).encode("utf-8"), compresslevel=9, mtime=0))
    print(f"{key}: {target.stat().st_size} bytes")


if __name__ == "__main__":
    for sticker, scene in SCENES.items():
        write(sticker, scene)

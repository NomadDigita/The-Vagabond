#!/usr/bin/env python3
"""Build six subject-led Vagabond character/reaction stickers as native TGS.

This is intentionally independent of the v1 scene builder.  The silhouettes
are hand-composed from Telegram-safe Lottie vectors: no raster imagery, masks,
effects, or expressions.  Every sticker has a real conversational meaning and
one readable character action rather than a generic UI symbol.
"""
from __future__ import annotations

import gzip
import importlib.util
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
CORE = ROOT / "assets/visual-system/pipeline/build_vagabond_tgs_core_v14.py"
spec = importlib.util.spec_from_file_location("vagabond_core", CORE)
core = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(core)
OUT = ROOT / "assets/visual-system/stickers/animated/v2_characters"


def L(ind, name, shapes, **kw):
    return core.layer(ind, name, shapes, **kw)


def pulse(values=((0, 20), (24, 100), (48, 20), (84, 20), (108, 100), (120, 20))):
    return core.key([(t, value) for t, value in values])


def commander(ind: int, *, arm="up", accent=None):
    """A recognisable graphite field commander: helmet, cyan visor, armour."""
    accent = accent or core.VIOLET
    head = core.poly([[-75,-153],[56,-153],[88,-102],[58,-40],[-75,-40],[-106,-98]], core.OBSIDIAN, core.STEEL, 9)
    visor = core.poly([[-72,-116],[52,-116],[36,-77],[-60,-77]], core.CYAN, core.WHITE, 5)
    torso = core.poly([[-107,-35],[78,-35],[119,139],[-132,139]], accent, core.STEEL, 10)
    badge = core.ellipse((34,34), core.GOLD, core.WHITE, 4, transform=core.tr(position=(-10,24)))
    layers = [L(ind, "field commander helmet", head), L(ind+1, "cyan field visor", visor), L(ind+2, "expedition armour", torso), L(ind+3, "rank beacon", badge)]
    if arm == "salute":
        shape = [core.path([[48,-25],[104,-108],[150,-138]], False), core.stroke(core.STEEL, 22), core.tr()]
    elif arm == "forward":
        shape = [core.path([[53,-20],[135,8],[181,-40]], False), core.stroke(core.STEEL, 23), core.tr()]
    elif arm == "cradle":
        shape = [core.path([[-84,16],[-8,92],[82,25]], False), core.stroke(core.STEEL, 24), core.tr()]
    else:
        shape = [core.path([[-66,2],[-136,58],[-170,16]], False), core.stroke(core.STEEL, 23), core.tr()]
    layers.append(L(ind+4, "commander gesture", shape))
    return layers


def rebel(ind: int):
    hood = core.poly([[-88,-164],[22,-187],[96,-105],[64,-22],[-95,-22],[-130,-105]], core.RED, core.WHITE, 9)
    face = core.poly([[-63,-121],[32,-129],[52,-54],[-67,-54]], core.OBSIDIAN, core.STEEL, 6)
    scarf = core.poly([[-113,-16],[87,-16],[117,145],[-144,145]], core.RED, core.GOLD, 9)
    eye = core.ellipse((24,16), core.CYAN, core.WHITE, 3, transform=core.tr(position=(-12,-87)))
    fist = core.ellipse((48,48), core.STEEL, core.WHITE, 5, transform=core.tr(position=(125,-102)))
    arm = [core.path([[50,0],[100,-58],[125,-100]], False), core.stroke(core.STEEL, 25), core.tr()]
    return [L(ind,"rebel hood",hood),L(ind+1,"rebel faceplate",face),L(ind+2,"rebel scarf",scarf),L(ind+3,"rebel cyan eye",eye),L(ind+4,"raised rebel arm",arm),L(ind+5,"raised rebel fist",fist)]


def medical_cross(ind, position=(138, -128)):
    h = core.poly([[-31,-11],[31,-11],[31,11],[-31,11]], core.WHITE, core.RED, 4)
    v = core.poly([[-11,-31],[11,-31],[11,31],[-11,31]], core.WHITE, core.RED, 4)
    return [L(ind, "medic horizontal mark", h, pos=position, scale=(90,90)), L(ind+1, "medic vertical mark", v, pos=position, scale=(90,90))]


def ack_commander():
    layers = core.base_layers(1) + commander(2, arm="salute")
    for n, p in enumerate(((145,-130),(172,-90))):
        layers.append(L(10+n, "acknowledgement signal", [core.star(20),core.fill(core.GOLD),core.tr(position=p, opacity=pulse())]))
    return layers


def crystal_joy():
    layers = core.base_layers(1) + commander(2, arm="cradle", accent=core.VIOLET)
    gem = [[0,-186],[64,-93],[38,27],[-41,27],[-65,-93]]
    layers.append(L(10,"rare violet crystal",core.poly(gem,core.LAVENDER,core.WHITE,8),pos=(276,215),scale=(105,105),rotation=core.key([(0,-7),(60,7),(120,-7)])))
    for n, p in enumerate(((215,42),(335,64),(362,178))):
        layers.append(L(11+n,"crystal joy glint",[core.star(25),core.fill(core.WHITE),core.tr(position=p,opacity=pulse())]))
    return layers


def raid_ready():
    layers = core.base_layers(1) + commander(2, arm="forward", accent=core.RED)
    route = [core.path([[-176,145],[-112,96],[-45,114],[45,35],[138,-16]],False),core.stroke(core.GOLD,12),core.tr()]
    layers.append(L(10,"raid route",route))
    target = core.ellipse((76,76), [0,0,0,0], core.RED, 10,
                          transform=core.tr(position=(138,-16), scale=core.key(
                              [(0,[70,70]), (60,[110,110]), (120,[70,70])]
                          )))
    layers.append(L(11, "raid target", target))
    return layers


def medic_recovery():
    layers = core.base_layers(1) + commander(2, arm="low", accent=core.CYAN)
    patch = core.ellipse((92,92),core.WHITE,core.RED,7,transform=core.tr(position=(118,32),scale=core.key([(0,[80,80]),(30,[108,108]),(60,[80,80]),(90,[108,108]),(120,[80,80])])) )
    layers.append(L(10,"field medical patch",patch))
    layers.extend(medical_cross(11,(374,288)))
    layers.append(L(13,"recovery glint",[core.star(28),core.fill(core.CYAN),core.tr(position=(129,-118),opacity=pulse())]))
    return layers


def defiant_survivor():
    layers = core.base_layers(1) + rebel(2)
    banner = core.poly([[0,-165],[94,-130],[94,-29],[0,-62]],core.RED,core.GOLD,7)
    pole = [core.path([[0,-188],[0,170]],False),core.stroke(core.STEEL,12),core.tr()]
    layers.append(L(10,"rebellion banner",banner,pos=(396,222),rotation=core.key([(0,-5),(60,5),(120,-5)])))
    layers.append(L(11,"banner pole",pole,pos=(396,260)))
    return layers


def survivor_bond():
    layers = core.base_layers(1)
    left = commander(2, arm="forward", accent=core.CYAN)
    right = commander(8, arm="low", accent=core.RED)
    for layer in right:
        layer["ks"]["p"] = core.prop([372, 256])
        layer["ks"]["s"] = core.prop([76,76])
        layer["ks"]["r"] = core.prop(0)
    for layer in left:
        layer["ks"]["p"] = core.prop([160, 268])
        layer["ks"]["s"] = core.prop([76,76])
    clasp = core.ellipse((58,58),core.GOLD,core.WHITE,6,transform=core.tr(position=(256,287),scale=core.key([(0,[72,72]),(60,[112,112]),(120,[72,72])])) )
    layers += left + right + [L(16,"survivor forearm clasp",clasp)]
    return layers


SCENES = {
    "commander_acknowledges": ack_commander,
    "crystal_joy": crystal_joy,
    "raid_ready": raid_ready,
    "field_medic_recovery": medic_recovery,
    "defiant_survivor": defiant_survivor,
    "survivor_bond": survivor_bond,
}


def build(key: str, scene) -> Path:
    data = {"v":"5.5.7","fr":60,"ip":0,"op":120,"w":512,"h":512,"nm":f"The Vagabond Sticker {key}","ddd":0,"assets":[],"layers":scene()}
    target = OUT / key / f"{key}.tgs"
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(gzip.compress(json.dumps(data,separators=(",",":")).encode(),compresslevel=9,mtime=0))
    return target


if __name__ == "__main__":
    for name, scene in SCENES.items():
        target = build(name, scene)
        print(f"{name}: {target.stat().st_size} bytes")

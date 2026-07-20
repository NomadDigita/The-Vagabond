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


def warning_icon():
    layers = base_layers(1)
    triangle = [[0, -185], [172, 142], [-172, 142]]
    layers.append(layer(2, "hazard warning triangle", poly(triangle, OBSIDIAN, GOLD, 14)))
    bar = poly([[-18, -75], [18, -75], [18, 48], [-18, 48]], RED, WHITE, 4)
    layers.append(layer(3, "warning mark", bar, scale=key([(0, [92, 92]), (30, [115, 115]), (60, [92, 92]), (90, [115, 115]), (120, [92, 92])])))
    dot = ellipse((35, 35), RED, WHITE, 4, transform=tr(position=(0, 92)))
    layers.append(layer(4, "warning dot", dot))
    return layers


def failure_icon():
    layers = base_layers(1)
    slash = [[-135, -165], [-88, -165], [0, -50], [88, -165], [135, -165], [45, 0], [135, 165], [88, 165], [0, 50], [-88, 165], [-135, 165], [-45, 0]]
    layers.append(layer(2, "fractured failure cross", poly(slash, RED, WHITE, 9), rotation=key([(0, -4), (60, 4), (120, -4)])))
    fracture = [path([[-105, 0], [0, -28], [105, 0]], False), stroke(GOLD, 8), tr(opacity=key([(0, 20), (30, 100), (60, 20), (90, 100), (120, 20)]))]
    layers.append(layer(3, "failure fracture", fracture))
    return layers


def ai_mech_icon():
    layers = base_layers(1)
    head = [[-150, -80], [-98, -138], [98, -138], [150, -80], [150, 92], [92, 142], [-92, 142], [-150, 92]]
    layers.append(layer(2, "AI mech head", poly(head, OBSIDIAN, STEEL, 11)))
    visor = poly([[-112, -30], [112, -30], [80, 45], [-80, 45]], VIOLET, CYAN, 8)
    layers.append(layer(3, "AI scanning visor", visor))
    scan = [path([[-88, -18], [88, -18]], False), stroke(WHITE, 9), tr(position=key([(0, [0, -10]), (60, [0, 50]), (120, [0, -10])]))]
    layers.append(layer(4, "visor sweep", scan))
    antenna = [path([[0, -138], [0, -195]], False), stroke(STEEL, 9), tr()]
    layers.append(layer(5, "AI antenna", antenna))
    signal = ellipse((28, 28), CYAN, WHITE, 4, transform=tr(position=(0, -200), opacity=key([(0, 25), (30, 100), (60, 25), (90, 100), (120, 25)])))
    layers.append(layer(6, "AI signal", signal))
    return layers


def electricity_icon():
    layers = base_layers(1)
    bolt = [[-38, -190], [92, -190], [25, -58], [118, -58], [-78, 192], [-34, 36], [-122, 36]]
    layers.append(layer(2, "primary energy bolt", poly(bolt, RED, GOLD, 10), scale=key([(0, [92, 92]), (24, [110, 110]), (48, [92, 92]), (84, [110, 110]), (120, [92, 92])])))
    for i, (x, y) in enumerate(((-135, -72), (128, 18), (-102, 115))):
        layers.append(layer(3 + i, f"electricity spark {i}", [star(30 - i * 5), fill(GOLD), tr(position=(x, y), opacity=key([(0 + i * 15, 0), (12 + i * 15, 100), (24 + i * 15, 0), (120 + i * 15, 0)]))]))
    return layers


def scrap_icon():
    layers = base_layers(1)
    pile = [[-175, 128], [-145, -15], [-72, 45], [-25, -125], [55, -38], [142, -92], [172, 128]]
    layers.append(layer(2, "salvage pile", poly(pile, OBSIDIAN, STEEL, 10)))
    layers.append(layer(3, "salvage plate", poly([[-132, 20], [-28, -38], [88, 25], [48, 95], [-78, 95]], VIOLET, GOLD, 7)))
    weld = [star(48), fill(GOLD), tr(position=(18, -30), opacity=key([(0, 0), (18, 100), (36, 0), (74, 0), (92, 100), (110, 0), (120, 0)]))]
    layers.append(layer(4, "salvage weld spark", weld))
    return layers


def hq_icon():
    layers = base_layers(1)
    tower = [[-118, 158], [-118, -82], [-65, -148], [65, -148], [118, -82], [118, 158]]
    layers.append(layer(2, "terminal HQ tower", poly(tower, OBSIDIAN, STEEL, 11)))
    screen = poly([[-72, -55], [72, -55], [72, 35], [-72, 35]], VIOLET, CYAN, 7)
    layers.append(layer(3, "terminal status screen", screen))
    mast = [path([[0, -148], [0, -205]], False), stroke(STEEL, 9), tr()]
    layers.append(layer(4, "HQ antenna", mast))
    ping = ellipse((50, 50), CYAN, WHITE, 4, transform=tr(position=(0, -210), scale=key([(0, [35, 35]), (60, [120, 120]), (120, [35, 35])]), opacity=key([(0, 100), (60, 0), (120, 100)])))
    layers.append(layer(5, "HQ signal ping", ping))
    return layers


def camp_icon():
    layers = base_layers(1)
    tent = [[-170, 140], [0, -145], [170, 140]]
    layers.append(layer(2, "outpost tent", poly(tent, OBSIDIAN, GOLD, 11)))
    flap = poly([[0, -145], [62, 140], [0, 140]], VIOLET, LAVENDER, 6)
    layers.append(layer(3, "tent flap", flap))
    fire = [star(42), fill(GOLD), tr(position=(0, 96), scale=key([(0, [75, 75]), (30, [120, 120]), (60, [75, 75]), (90, [112, 112]), (120, [75, 75])]))]
    layers.append(layer(4, "camp fire", fire))
    return layers


def economy_icon():
    layers = base_layers(1)
    vault = ellipse((300, 300), OBSIDIAN, GOLD, 14)
    layers.append(layer(2, "economy vault door", vault))
    hub = ellipse((94, 94), VIOLET, LAVENDER, 8, transform=tr(rotation=key([(0, 0), (120, 360)])))
    layers.append(layer(3, "vault hub", hub))
    for i, angle in enumerate((0, 72, 144, 216, 288)):
        import math
        x, y = 105 * math.cos(math.radians(angle)), 105 * math.sin(math.radians(angle))
        spoke = [path([[0, 0], [x, y]], False), stroke(GOLD, 10), tr(rotation=key([(0, 0), (120, 360)]))]
        layers.append(layer(4 + i, f"vault spoke {i}", spoke))
    return layers


def workshop_icon():
    layers = base_layers(1)
    factory = [[-178, 145], [-178, -40], [-90, -40], [-90, -125], [-40, -125], [-40, -40], [40, -40], [40, -90], [95, -90], [95, -40], [178, -40], [178, 145]]
    layers.append(layer(2, "heavy workshop", poly(factory, OBSIDIAN, GOLD, 10)))
    conveyor = [path([[-132, 76], [132, 76]], False), stroke(STEEL, 16), tr()]
    layers.append(layer(3, "workshop conveyor", conveyor))
    weld = [star(38), fill(CYAN), tr(position=key([(0, [-90, 76]), (60, [90, 76]), (120, [-90, 76])]), opacity=key([(0, 20), (30, 100), (60, 20), (90, 100), (120, 20)]))]
    layers.append(layer(4, "workshop weld", weld))
    return layers


def research_icon():
    layers = base_layers(1)
    flask = [[-72, -165], [72, -165], [54, -35], [135, 112], [90, 158], [-90, 158], [-135, 112], [-54, -35]]
    layers.append(layer(2, "research flask", poly(flask, OBSIDIAN, STEEL, 10)))
    fluid = poly([[-100, 78], [100, 78], [128, 120], [88, 148], [-88, 148], [-128, 120]], VIOLET, LAVENDER, 5)
    layers.append(layer(3, "research compound", fluid, scale=key([(0, [94, 94]), (60, [108, 108]), (120, [94, 94])])))
    molecule = [star(30), fill(CYAN), tr(position=(0, 28), opacity=key([(0, 20), (30, 100), (60, 20), (90, 100), (120, 20)]))]
    layers.append(layer(4, "research reaction", molecule))
    return layers


def mutation_icon():
    layers = base_layers(1)
    left = [path([[-94, -170], [-18, -105], [-94, -35], [-18, 38], [-94, 115], [-18, 175]], False), stroke(LAVENDER, 16), tr()]
    right = [path([[94, -170], [18, -105], [94, -35], [18, 38], [94, 115], [18, 175]], False), stroke(CYAN, 16), tr()]
    layers.append(layer(2, "mutation helix left", left, rotation=key([(0, -5), (60, 5), (120, -5)])))
    layers.append(layer(3, "mutation helix right", right, rotation=key([(0, 5), (60, -5), (120, 5)])))
    for i, y in enumerate((-104, -35, 38, 112)):
        rung = [path([[-45, y], [45, y]], False), stroke(GOLD, 9), tr(opacity=key([(0 + i * 12, 25), (30 + i * 12, 100), (60 + i * 12, 25), (120 + i * 12, 25)]))]
        layers.append(layer(4 + i, f"mutation rung {i}", rung))
    return layers


def mining_icon():
    layers = base_layers(1)
    handle = [path([[-118, 152], [118, -142]], False), stroke(GOLD, 22), tr(rotation=key([(0, -7), (60, 7), (120, -7)]))]
    head = [path([[-148, -105], [-55, -185], [18, -128], [128, -55], [72, 12]], False), stroke(STEEL, 28), tr(rotation=key([(0, -7), (60, 7), (120, -7)]))]
    layers.append(layer(2, "mining pick handle", handle))
    layers.append(layer(3, "mining pick head", head))
    ore = poly([[-58, 70], [0, 8], [62, 70], [40, 148], [-42, 148]], VIOLET, LAVENDER, 7)
    layers.append(layer(4, "mined crystal ore", ore))
    return layers


def radar_icon():
    layers = base_layers(1)
    layers.append(layer(2, "expedition radar ring", ellipse((330, 330), [0, 0, 0, 0], CYAN, 10)))
    layers.append(layer(3, "expedition radar core", ellipse((54, 54), GOLD, WHITE, 6)))
    sweep = poly([[0, 0], [148, -62], [148, 10]], CYAN, CYAN, 1, transform=tr(rotation=key([(0, 0), (120, 360)])))
    layers.append(layer(4, "radar sweep", sweep))
    target = ellipse((28, 28), RED, WHITE, 4, transform=tr(position=(92, -85), opacity=key([(0, 10), (24, 100), (48, 10), (72, 100), (120, 10)])))
    layers.append(layer(5, "radar target", target))
    return layers


def ranking_icon():
    layers = base_layers(1)
    cup = [[-130, -138], [130, -138], [98, 35], [45, 82], [45, 130], [105, 130], [105, 170], [-105, 170], [-105, 130], [-45, 130], [-45, 82], [-98, 35]]
    layers.append(layer(2, "ranking trophy", poly(cup, GOLD, WHITE, 10)))
    shine = [star(32), fill(WHITE), tr(position=(-54, -70), opacity=key([(0, 15), (30, 100), (60, 15), (120, 15)]))]
    layers.append(layer(3, "trophy shine", shine))
    return layers


def warehouse_icon():
    layers = base_layers(1)
    shell = [[-170, 152], [-170, -48], [0, -162], [170, -48], [170, 152]]
    layers.append(layer(2, "armoured warehouse", poly(shell, OBSIDIAN, STEEL, 11)))
    door = poly([[-95, 152], [-95, 8], [95, 8], [95, 152]], VIOLET, CYAN, 7)
    layers.append(layer(3, "warehouse bay door", door, pos=key([(0, [0, 0]), (60, [0, 24]), (120, [0, 0])])))
    return layers


def market_icon():
    layers = base_layers(1)
    booth = [[-162, 148], [-162, -50], [-110, -128], [110, -128], [162, -50], [162, 148]]
    layers.append(layer(2, "market terminal", poly(booth, OBSIDIAN, GOLD, 10)))
    arrow_up = [path([[-70, 62], [0, -8], [70, 62]], False), stroke(CYAN, 16), tr(position=key([(0, [0, 20]), (60, [0, -15]), (120, [0, 20])]))]
    arrow_down = [path([[-70, 5], [0, 75], [70, 5]], False), stroke(LAVENDER, 16), tr(position=key([(0, [0, -20]), (60, [0, 15]), (120, [0, -20])]))]
    layers.append(layer(3, "market rise", arrow_up))
    layers.append(layer(4, "market fall", arrow_down))
    return layers


def troops_icon():
    layers = base_layers(1)
    helmet = [[-150, 42], [-120, -90], [-60, -145], [60, -145], [120, -90], [150, 42], [98, 92], [-98, 92]]
    layers.append(layer(2, "troop helmet", poly(helmet, OBSIDIAN, STEEL, 11)))
    visor = poly([[-106, -12], [106, -12], [70, 40], [-70, 40]], RED, WHITE, 5)
    layers.append(layer(3, "troop visor", visor, scale=key([(0, [92, 92]), (60, [105, 105]), (120, [92, 92])])))
    return layers


def vehicle_icon():
    layers = base_layers(1)
    hull = [[-178, 98], [-138, -18], [-58, -18], [-20, -78], [105, -78], [158, 8], [178, 98]]
    layers.append(layer(2, "combat vehicle hull", poly(hull, OBSIDIAN, GOLD, 10)))
    for i, x in enumerate((-100, 102)):
        wheel = ellipse((62, 62), STEEL, CYAN, 6, transform=tr(position=(x, 112), rotation=key([(0, 0), (120, 360)])))
        layers.append(layer(3 + i, f"vehicle wheel {i}", wheel))
    return layers


def deconstruct_icon():
    layers = base_layers(1)
    ring = ellipse((300, 300), [0, 0, 0, 0], LAVENDER, 15)
    layers.append(layer(2, "deconstruction cycle", ring, rotation=key([(0, 0), (120, -360)])))
    shard = poly([[-38, -182], [60, -115], [0, -60], [-60, -115]], RED, WHITE, 4)
    layers.append(layer(3, "deconstruction break", shard, rotation=key([(0, 0), (120, 360)])))
    return layers


def infrastructure_icon():
    layers = base_layers(1)
    crane = [path([[-145, 158], [-145, -162], [135, -162], [28, -78]], False), stroke(GOLD, 18), tr()]
    layers.append(layer(2, "infrastructure crane", crane))
    hook = [path([[28, -78], [28, 48]], False), stroke(STEEL, 10), tr(position=key([(0, [0, -18]), (60, [0, 18]), (120, [0, -18])]))]
    layers.append(layer(3, "crane lifting hook", hook))
    block = poly([[-45, 52], [45, 52], [45, 142], [-45, 142]], VIOLET, CYAN, 6)
    layers.append(layer(4, "raised construction block", block, pos=key([(0, [0, -18]), (60, [0, 18]), (120, [0, -18])])))
    return layers


def defense_icon():
    layers = base_layers(1)
    bunker = [[-150, 152], [-112, 20], [-55, -38], [55, -38], [112, 20], [150, 152]]
    layers.append(layer(2, "defense bunker", poly(bunker, OBSIDIAN, STEEL, 11)))
    cannon = [path([[0, 6], [0, -166]], False), stroke(GOLD, 24), tr(rotation=key([(0, -14), (60, 14), (120, -14)]))]
    layers.append(layer(3, "defense cannon", cannon))
    flash = [star(34), fill(CYAN), tr(position=key([(0, [-40, -155]), (60, [40, -155]), (120, [-40, -155])]), opacity=key([(0, 0), (30, 100), (60, 0), (120, 0)]))]
    layers.append(layer(4, "defense muzzle flash", flash))
    return layers


def clan_icon():
    layers = base_layers(1)
    pole = [path([[-112, 168], [-112, -172]], False), stroke(STEEL, 15), tr()]
    layers.append(layer(2, "clan banner pole", pole))
    flag = poly([[-104, -160], [150, -112], [-104, -35]], VIOLET, GOLD, 8, transform=tr(position=key([(0, [0, 0]), (60, [0, -10]), (120, [0, 0])])))
    layers.append(layer(3, "clan banner", flag))
    crest = [star(32), fill(WHITE), tr(position=(-25, -103), scale=key([(0, [80, 80]), (60, [110, 110]), (120, [80, 80])]))]
    layers.append(layer(4, "clan crest", crest))
    return layers


def boss_icon():
    layers = base_layers(1)
    head = [[-132, -104], [-70, -166], [70, -166], [132, -104], [132, 105], [70, 158], [-70, 158], [-132, 105]]
    layers.append(layer(2, "boss mech skull", poly(head, OBSIDIAN, RED, 12)))
    for i, x in enumerate((-55, 55)):
        eye = ellipse((52, 42), RED, WHITE, 5, transform=tr(position=(x, -28), opacity=key([(0, 35), (30, 100), (60, 35), (120, 35)])))
        layers.append(layer(3 + i, f"boss eye {i}", eye))
    jaw = [path([[-76, 78], [-35, 112], [0, 78], [35, 112], [76, 78]], False), stroke(STEEL, 11), tr()]
    layers.append(layer(5, "boss jaw", jaw))
    return layers


def warlord_icon():
    layers = base_layers(1)
    crown = [[-158, 112], [-158, -118], [-70, -42], [0, -160], [70, -42], [158, -118], [158, 112]]
    layers.append(layer(2, "warlord crown", poly(crown, GOLD, WHITE, 11), scale=key([(0, [92, 92]), (60, [108, 108]), (120, [92, 92])])))
    gem = ellipse((44, 44), RED, WHITE, 5, transform=tr(position=(0, 42), opacity=key([(0, 30), (30, 100), (60, 30), (120, 30)])))
    layers.append(layer(3, "warlord crown gem", gem))
    return layers


def world_icon():
    layers = base_layers(1)
    layers.append(layer(2, "world sphere", ellipse((330, 330), OBSIDIAN, CYAN, 11)))
    meridian = [path([[0, -160], [0, 160]], False), stroke(LAVENDER, 9), tr(rotation=key([(0, -25), (60, 25), (120, -25)]))]
    equator = [path([[-155, 0], [155, 0]], False), stroke(LAVENDER, 9), tr()]
    layers.append(layer(3, "world meridian", meridian))
    layers.append(layer(4, "world equator", equator))
    return layers


def cannon_icon():
    layers = base_layers(1)
    turret = ellipse((210, 116), OBSIDIAN, STEEL, 10)
    layers.append(layer(2, "tactical cannon turret", turret, pos=(256, 320)))
    barrel = [path([[0, 44], [0, -172]], False), stroke(GOLD, 30), tr(rotation=key([(0, -10), (60, 10), (120, -10)]))]
    layers.append(layer(3, "tactical cannon barrel", barrel))
    blast = [star(38), fill(CYAN), tr(position=(0, -175), opacity=key([(0, 0), (28, 100), (56, 0), (120, 0)]))]
    layers.append(layer(4, "cannon blast", blast))
    return layers


def silo_icon():
    layers = base_layers(1)
    shell = [[-118, 158], [-118, -36], [-70, -126], [0, -172], [70, -126], [118, -36], [118, 158]]
    layers.append(layer(2, "strategic silo", poly(shell, OBSIDIAN, STEEL, 11)))
    core = [star(46), fill(RED), tr(position=(0, 24), opacity=key([(0, 30), (30, 100), (60, 30), (120, 30)]))]
    layers.append(layer(3, "silo warning core", core))
    return layers


def alliance_icon():
    layers = base_layers(1)
    left = poly([[-175, -58], [-70, -148], [15, -63], [-68, 35]], VIOLET, LAVENDER, 9)
    right = poly([[175, 58], [70, 148], [-15, 63], [68, -35]], GOLD, WHITE, 9)
    layers.append(layer(2, "alliance left clasp", left, pos=key([(0, [236, 256]), (60, [255, 256]), (120, [236, 256])])))
    layers.append(layer(3, "alliance right clasp", right, pos=key([(0, [276, 256]), (60, [257, 256]), (120, [276, 256])])))
    spark = [star(28), fill(CYAN), tr(opacity=key([(0, 0), (30, 100), (60, 0), (120, 0)]))]
    layers.append(layer(4, "alliance lock", spark))
    return layers


def rebellion_icon():
    layers = base_layers(1)
    flag = poly([[-130, -150], [145, -104], [-130, -25]], RED, WHITE, 9, transform=tr(rotation=key([(0, -4), (60, 4), (120, -4)])))
    pole = [path([[-135, -170], [-135, 170]], False), stroke(STEEL, 15), tr()]
    layers.append(layer(2, "rebellion flag", flag))
    layers.append(layer(3, "rebellion pole", pole))
    flame = [star(38), fill(GOLD), tr(position=(-8, 96), opacity=key([(0, 30), (30, 100), (60, 30), (120, 30)]))]
    layers.append(layer(4, "rebellion fire", flame))
    return layers


def jet_icon():
    layers = base_layers(1)
    jet = [[0, -185], [44, -32], [154, 70], [48, 65], [0, 168], [-48, 65], [-154, 70], [-44, -32]]
    layers.append(layer(2, "expedition jet", poly(jet, OBSIDIAN, STEEL, 10), rotation=key([(0, -7), (60, 7), (120, -7)])))
    exhaust = [star(32), fill(CYAN), tr(position=(0, 158), opacity=key([(0, 15), (30, 100), (60, 15), (120, 15)]))]
    layers.append(layer(3, "jet exhaust", exhaust))
    return layers


def commander_icon():
    layers = base_layers(1)
    head = ellipse((180, 180), OBSIDIAN, STEEL, 10, transform=tr(position=(0, -38)))
    layers.append(layer(2, "commander profile", head))
    shoulders = poly([[-170, 162], [-110, 62], [110, 62], [170, 162]], VIOLET, LAVENDER, 9)
    layers.append(layer(3, "commander shoulders", shoulders))
    insignia = [star(24), fill(GOLD), tr(position=(0, 100), opacity=key([(0, 20), (30, 100), (60, 20), (120, 20)]))]
    layers.append(layer(4, "commander insignia", insignia))
    return layers


def exploration_icon():
    layers = base_layers(1)
    compass = ellipse((300, 300), OBSIDIAN, GOLD, 10)
    layers.append(layer(2, "exploration compass", compass))
    needle = poly([[0, -138], [42, 28], [0, 138], [-42, 28]], RED, WHITE, 6, transform=tr(rotation=key([(0, 0), (120, 360)])))
    layers.append(layer(3, "exploration needle", needle))
    return layers


def write(key, layers):
    data={"v":"5.5.7","fr":60,"ip":0,"op":FRAMES,"w":512,"h":512,"nm":f"The Vagabond {key}","ddd":0,"assets":[],"layers":layers}
    target=OUT/f"{key}_tgs_v14"/f"{key}_tgs_v14.tgs"
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(gzip.compress(json.dumps(data, separators=(",", ":")).encode(), compresslevel=9, mtime=0))
    print(f"{key}: {target.stat().st_size} bytes")


def main():
    for key, builder in {"map":map_icon,"raid":raid_icon,"base":base_icon,"transport":transport_icon,"combat":combat_icon,"satellite":satellite_icon,"shield":shield_icon,"gear":gear_icon,"warning":warning_icon,"failure":failure_icon,"ai_mech":ai_mech_icon,"electricity":electricity_icon,"scrap":scrap_icon,"hq":hq_icon,"camp":camp_icon,"economy":economy_icon,"workshop":workshop_icon,"research":research_icon,"mutation":mutation_icon,"mining":mining_icon,"radar":radar_icon,"ranking":ranking_icon,"warehouse":warehouse_icon,"market":market_icon,"troops":troops_icon,"vehicle":vehicle_icon,"deconstruct":deconstruct_icon,"infrastructure":infrastructure_icon,"defense":defense_icon,"clan":clan_icon,"boss":boss_icon,"warlord":warlord_icon,"world":world_icon,"cannon":cannon_icon,"silo":silo_icon,"alliance":alliance_icon,"rebellion":rebellion_icon,"jet":jet_icon,"commander":commander_icon,"exploration":exploration_icon}.items():
        write(key,builder())


if __name__=="__main__": main()

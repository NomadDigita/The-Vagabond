#!/usr/bin/env python3
"""Build native-vector V14 social and meta-system TGS custom emoji."""
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
CYAN = [0.15, 0.84, 1, 1]
RED = [0.92, 0.15, 0.22, 1]

def prop(value): return {"a": 0, "k": value}
def key(values): return {"a": 1, "k": [{"t": t, "s": v} for t, v in values]}
def path(points, closed=True): return {"a": 0, "k": {"i": [[0, 0]] * len(points), "o": [[0, 0]] * len(points), "v": points, "c": closed}}
def fill(c, opacity=100): return {"ty": "fl", "c": prop(c), "o": prop(opacity), "r": 1}
def stroke(c, width, opacity=100): return {"ty": "st", "c": prop(c), "o": prop(opacity), "w": prop(width), "lc": 2, "lj": 2, "ml": 4}
def tr(position=(0, 0), scale=(100, 100), rotation=0, opacity=100):
    return {"ty":"tr","p":position if isinstance(position,dict) else prop(list(position)),"a":prop([0,0]),"s":scale if isinstance(scale,dict) else prop(list(scale)),"r":rotation if isinstance(rotation,dict) else prop(rotation),"o":opacity if isinstance(opacity,dict) else prop(opacity),"sk":prop(0),"sa":prop(0)}
def layer(index, name, shapes, pos=(256,256), scale=(100,100), rotation=0, opacity=100):
    return {"ddd":0,"ind":index,"ty":4,"nm":name,"sr":1,"ks":tr(pos,scale,rotation,opacity),"ao":0,"shapes":shapes,"ip":0,"op":FRAMES,"st":0,"bm":0}
def poly(points, colour, outline=WHITE, width=6, transform=None): return [path(points),fill(colour),stroke(outline,width),transform or tr()]
def ellipse(size, colour, outline=None, width=0, transform=None):
    result=[{"ty":"el","p":prop([0,0]),"s":prop(list(size))},fill(colour)]
    if outline: result.append(stroke(outline,width))
    result.append(transform or tr())
    return result
def star(size): return path([[0,-size],[size*.22,-size*.22],[size,0],[size*.22,size*.22],[0,size],[-size*.22,size*.22],[-size,0],[-size*.22,-size*.22]])
def base(): return [layer(1,"violet clarity aura",ellipse((448,448),VIOLET,LAVENDER,8),opacity=46)]

def federation_icon():
    layers=base()
    left=[[-150,-115],[-25,-145],[-8,-42],[-58,6],[-18,105],[-145,143]]
    right=[[150,-115],[25,-145],[8,-42],[58,6],[18,105],[145,143]]
    layers.append(layer(2,"federation shield left",poly(left,OBSIDIAN,GOLD,10),rotation=key([(0,-5),(60,3),(120,-5)])))
    layers.append(layer(3,"federation shield right",poly(right,OBSIDIAN,GOLD,10),rotation=key([(0,5),(60,-3),(120,5)])))
    layers.append(layer(4,"alliance link",[path([[-64,0],[-24,0],[0,26],[24,0],[64,0]],False),stroke(CYAN,12),tr(opacity=key([(0,35),(30,100),(60,35),(90,100),(120,35)]))]))
    layers.append(layer(5,"federation seal",[star(30),fill(WHITE),tr(position=(0,-70),opacity=key([(0,0),(18,100),(36,0),(84,0),(102,100),(120,0)]))]))
    return layers

def missions_icon():
    layers=base()
    layers.append(layer(2,"mission contract board",poly([[-142,-166],[120,-166],[150,-132],[150,168],[-142,168]],OBSIDIAN,GOLD,10)))
    layers.append(layer(3,"mission header",poly([[-105,-120],[86,-120],[86,-86],[-105,-86]],VIOLET,LAVENDER,4)))
    for i,y in enumerate((-40,12,64)):
        layers.append(layer(4+i,"mission task line",[path([[-92,y],[74,y]],False),stroke(STEEL,9),tr(opacity=key([(i*12,35),(22+i*12,100),(50+i*12,35),(120,35)]))]))
    layers.append(layer(7,"mission completion check",[path([[-104,92],[-50,132],[110,34]],False),stroke(CYAN,18),tr(opacity=key([(0,0),(30,100),(75,100),(105,0),(120,0)]))]))
    return layers

def destinations_icon():
    layers=base()
    layers.append(layer(2,"destination planet",ellipse((242,242),OBSIDIAN,STEEL,10,transform=tr(position=(-45,30)))))
    layers.append(layer(3,"destination orbit",[path([[-185,25],[-88,-115],[92,-117],[190,12],[95,128],[-90,135],[-185,25]],False),stroke(LAVENDER,9),tr(rotation=key([(0,-8),(60,8),(120,-8)]))]))
    pin=[[104,-174],[158,-108],[134,-20],[104,22],[74,-20],[50,-108]]
    layers.append(layer(4,"destination waypoint",poly(pin,RED,WHITE,7),scale=key([(0,[88,88]),(30,[108,108]),(60,[88,88]),(90,[108,108]),(120,[88,88])])))
    layers.append(layer(5,"waypoint core",ellipse((28,28),GOLD,WHITE,4,transform=tr(position=(104,-96)))))
    return layers

def settings_icon():
    layers=base()
    for i,(y,x) in enumerate(((-105,-60),(0,70),(105,-20))):
        layers.append(layer(2+i,"settings control rail",[path([[-168,y],[168,y]],False),stroke(STEEL,15),tr()]))
        end=x+35 if i%2==0 else x-35
        layers.append(layer(5+i,"settings slider",ellipse((58,58),VIOLET,GOLD,7,transform=tr(position=key([(0,[x,y]),(60,[end,y]),(120,[x,y])])))))
    layers.append(layer(8,"settings calibration spark",[star(28),fill(CYAN),tr(position=(162,-145),opacity=key([(0,0),(20,100),(40,0),(88,0),(108,100),(120,0)]))]))
    return layers

def feedback_icon():
    layers=base()
    bubble=[[-170,-126],[126,-126],[166,-82],[166,88],[30,88],[-52,154],[-35,88],[-170,88]]
    layers.append(layer(2,"commander feedback transmission",poly(bubble,OBSIDIAN,GOLD,10)))
    for i,x in enumerate((-82,0,82)):
        layers.append(layer(3+i,"feedback signal dot",ellipse((34,34),CYAN,WHITE,3,transform=tr(position=(x,-14),scale=key([(i*15,[45,45]),(20+i*15,[110,110]),(40+i*15,[45,45]),(120,[45,45])])))))
    layers.append(layer(6,"uplink ray",[path([[122,-85],[183,-138]],False),stroke(LAVENDER,8),tr(opacity=key([(0,0),(25,100),(50,0),(85,0),(110,100),(120,0)]))]))
    return layers

def profile_stats_icon():
    layers=base()
    badge=[[0,-174],[126,-108],[108,66],[0,164],[-108,66],[-126,-108]]
    layers.append(layer(2,"commander profile badge",poly(badge,OBSIDIAN,GOLD,11)))
    layers.append(layer(3,"commander crest",ellipse((76,76),VIOLET,LAVENDER,7,transform=tr(position=(0,-58)))))
    for i,(x,h) in enumerate(((-72,58),(0,104),(72,145))):
        rect=[[x-20,112],[x-20,112-h],[x+20,112-h],[x+20,112]]
        layers.append(layer(4+i,"profile stat bar",poly(rect,CYAN,WHITE,3,transform=tr(scale=key([(0,[100,55]),(60,[100,100]),(120,[100,55])])))))
    return layers

def composition(name,layers): return {"v":"5.7.4","fr":60,"ip":0,"op":FRAMES,"w":512,"h":512,"nm":name,"ddd":0,"assets":[],"layers":layers}
ASSETS={"federation":federation_icon,"missions":missions_icon,"destinations":destinations_icon,"settings":settings_icon,"feedback":feedback_icon,"profile_stats":profile_stats_icon}
def main():
    for key_name,builder in ASSETS.items():
        folder=OUT/f"{key_name}_tgs_v14"; folder.mkdir(parents=True,exist_ok=True)
        target=folder/f"{key_name}_tgs_v14.tgs"
        with gzip.open(target,"wb",compresslevel=9) as handle: handle.write(json.dumps(composition(f"Vagabond {key_name} V14",builder()),separators=(",",":"),ensure_ascii=True).encode())
        print(f"{key_name}: {target} ({target.stat().st_size} bytes)")
if __name__ == "__main__": main()

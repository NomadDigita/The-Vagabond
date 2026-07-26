#!/usr/bin/env python3
"""Build the sharp V14 native-vector fleet and mobility emoji family.

Each symbol is purpose-drawn around a real Vagabond vehicle silhouette.  The
files use only Telegram-safe Lottie shape primitives and retain transparency.
"""
from __future__ import annotations

import gzip
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / "assets" / "visual-system" / "animated"
FRAMES = 120
OBSIDIAN=[.055,.075,.12,1]; VIOLET=[.31,.08,.64,1]; LAVENDER=[.79,.55,1,1]
GOLD=[.93,.60,.12,1]; STEEL=[.64,.74,.86,1]; WHITE=[1,.96,.84,1]
CYAN=[.15,.84,1,1]; RED=[.92,.15,.22,1]

def p(v): return {"a":0,"k":v}
def k(v): return {"a":1,"k":[{"t":t,"s":x} for t,x in v]}
def tr(pos=(0,0), scale=(100,100), rot=0, opacity=100):
    return {"ty":"tr","p":pos if isinstance(pos,dict) else p(list(pos)),"a":p([0,0]),"s":scale if isinstance(scale,dict) else p(list(scale)),"r":rot if isinstance(rot,dict) else p(rot),"o":opacity if isinstance(opacity,dict) else p(opacity),"sk":p(0),"sa":p(0)}
def path(points, closed=True): return {"ty":"sh","ks":p({"i":[[0,0]]*len(points),"o":[[0,0]]*len(points),"v":points,"c":closed})}
def fill(col): return {"ty":"fl","c":p(col),"o":p(100),"r":1}
def stroke(col,w): return {"ty":"st","c":p(col),"o":p(100),"w":p(w),"lc":2,"lj":2,"ml":p(4)}
def poly(points, col, edge=WHITE, w=7, transform=None): return [path(points),fill(col),stroke(edge,w),transform or tr()]
def ell(size,col,edge=None,w=0,transform=None):
    a=[{"ty":"el","p":p([0,0]),"s":p(list(size))},fill(col)]
    if edge: a.append(stroke(edge,w))
    a.append(transform or tr()); return a
def star(n=35): return path([[0,-n],[n*.24,-n*.24],[n,0],[n*.24,n*.24],[0,n],[-n*.24,n*.24],[-n,0],[-n*.24,-n*.24]])
def L(i,name,shapes,**kw): return {"ddd":0,"ind":i,"ty":4,"nm":name,"sr":1,"ks":tr(**kw),"ao":0,"shapes":shapes,"ip":0,"op":FRAMES,"st":0,"bm":0}
def aura(): return [L(1,"Vagabond fleet aura",ell((448,448),VIOLET,LAVENDER,8),opacity=46)]
def pulse(): return k([(0,20),(24,100),(48,20),(84,100),(120,20)])

def scout_walker():
    a=aura(); body=[[-125,-30],[-52,-112],[80,-96],[138,-30],[95,28],[-78,38]]
    a += [L(2,"scout walker hull",poly(body,OBSIDIAN,STEEL,10)), L(3,"scout eye",ell((55,38),CYAN,WHITE,5,transform=tr(pos=(22,-43),opacity=pulse())))]
    for i,x in enumerate((-80,62)):
        a.append(L(4+i,f"walker leg {i}",[path([[x,28],[x-26,148],[x+38,148]],False),stroke(GOLD,17),tr(pos=(0,0),rot=k([(0,-4 if i==0 else 4),(60,4 if i==0 else -4),(120,-4 if i==0 else 4)]))]))
    return a

def cargo_ship():
    a=aura(); ship=[[-182,58],[-88,-70],[112,-70],[182,5],[132,105],[-146,105]]
    a += [L(2,"cargo ship hull",poly(ship,OBSIDIAN,GOLD,10)), L(3,"cargo containers",poly([[-68,-45],[54,-45],[54,30],[-68,30]],VIOLET,LAVENDER,6))]
    engine = ell((48,48), CYAN, WHITE, 4, transform=tr(pos=(-138,40), scale=k([(0,[70,70]),(60,[125,125]),(120,[70,70])])))
    a.append(L(4,"cargo engine glow",engine))
    return a

def battlecruiser():
    a=aura(); hull=[[-192,52],[-78,-108],[108,-80],[192,8],[88,108],[-126,108]]
    a += [L(2,"battlecruiser armored hull",poly(hull,OBSIDIAN,STEEL,11)), L(3,"battlecruiser command deck",poly([[-72,-58],[62,-58],[94,-16],[-38,-16]],VIOLET,CYAN,6))]
    for i,y in enumerate((-20,48)):
        a.append(L(4+i,f"cruiser rail cannon {i}",[path([[18,y],[172,y-20]],False),stroke(GOLD,18),tr()]))
    a.append(L(6,"cruiser firing flash",[star(38),fill(WHITE),tr(pos=(176,-40),opacity=pulse())]))
    return a

def bomber():
    a=aura(); wing=[[-188,12],[-45,-72],[0,-152],[45,-72],[188,12],[72,64],[24,142],[-24,142],[-72,64]]
    a += [L(2,"bomber swept wings",poly(wing,OBSIDIAN,RED,10)), L(3,"bomber cockpit",ell((56,90),VIOLET,CYAN,6,transform=tr(pos=(0,-22))))]
    bomb = ell((35,55), GOLD, WHITE, 4, transform=tr(pos=k([(0,[0,50]),(60,[0,170]),(120,[0,50])]), opacity=k([(0,0),(12,100),(72,100),(84,0),(120,0)])))
    a.append(L(4,"bomb drop",bomb))
    return a

def doomsday_rig():
    a=aura(); rig=[[-166,142],[-166,-82],[-100,-142],[100,-142],[166,-82],[166,142]]
    reactor = ell((112,112), VIOLET, GOLD, 9, transform=tr(scale=k([(0,[82,82]),(30,[112,112]),(60,[82,82]),(90,[112,112]),(120,[82,82])])) )
    a += [L(2,"doomsday siege rig",poly(rig,OBSIDIAN,RED,12)), L(3,"doomsday reactor",reactor)]
    for i,x in enumerate((-116,116)):
        a.append(L(4+i,f"doomsday stabilizer {i}",[path([[x,90],[x,180]],False),stroke(STEEL,18),tr()]))
    a.append(L(6,"doomsday charge",[star(53),fill(CYAN),tr(opacity=pulse())]))
    return a

def hauler():
    a=aura(); body=[[-184,78],[-184,-52],[-58,-52],[-15,-120],[94,-120],[155,-52],[184,-52],[184,78]]
    a += [L(2,"ore hauler body",poly(body,OBSIDIAN,GOLD,10)),L(3,"hauler ore bay",poly([[-142,-24],[-45,-24],[-45,38],[-142,38]],VIOLET,LAVENDER,6))]
    for i,x in enumerate((-108,110)):
        wheel = ell((78,78), OBSIDIAN, STEEL, 9, transform=tr(pos=(x,96), rot=k([(0,0),(120,360)])))
        a.append(L(4+i,f"hauler wheel {i}",wheel))
    a.append(L(6,"hauler beacon",ell((25,25),CYAN,WHITE,4,transform=tr(pos=(142,-54),opacity=pulse()))))
    return a

BUILDERS={"scout_walker":scout_walker,"cargo_ship":cargo_ship,"battlecruiser":battlecruiser,"bomber":bomber,"doomsday_rig":doomsday_rig,"hauler":hauler}
def write(name, layers):
    data={"v":"5.5.7","fr":60,"ip":0,"op":FRAMES,"w":512,"h":512,"nm":f"The Vagabond {name}","ddd":0,"assets":[],"layers":layers}
    path=OUT/f"{name}_tgs_v14"/f"{name}_tgs_v14.tgs"; path.parent.mkdir(parents=True,exist_ok=True)
    path.write_bytes(gzip.compress(json.dumps(data,separators=(",",":")).encode(),compresslevel=9,mtime=0)); print(f"{name}: {path.stat().st_size} bytes")
if __name__=="__main__":
    for name,make in BUILDERS.items(): write(name,make())

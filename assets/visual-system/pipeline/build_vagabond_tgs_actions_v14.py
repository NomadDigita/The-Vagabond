#!/usr/bin/env python3
"""Build the V14 action-system emoji family.

These are native Telegram TGS compositions: transparent 512px canvas, 60 fps,
two-second loop, and only straightforward vector paths, fills, strokes, and
transform animation.  The assets are deliberately subject-led: an action must
read as its game operation before its motion is noticed.
"""
from __future__ import annotations

import gzip
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / "assets" / "visual-system" / "animated"
FRAMES = 120
VIOLET=[.31,.08,.64,1]; LAVENDER=[.79,.55,1,1]; GOLD=[.93,.60,.12,1]
STEEL=[.64,.74,.86,1]; OBSIDIAN=[.055,.075,.12,1]; WHITE=[1,.96,.84,1]
CYAN=[.15,.84,1,1]; RED=[.92,.15,.22,1]

def prop(v): return {"a":0,"k":v}
def key(v): return {"a":1,"k":[{"t":t,"s":s} for t,s in v]}
def path(points, closed=True): return {"ty":"sh","ks":{"a":0,"k":{"i":[[0,0]]*len(points),"o":[[0,0]]*len(points),"v":points,"c":closed}}}
def fill(c, opacity=100): return {"ty":"fl","c":prop(c),"o":prop(opacity),"r":1}
def stroke(c,w,opacity=100): return {"ty":"st","c":prop(c),"o":prop(opacity),"w":prop(w),"lc":2,"lj":2,"ml":4}
def tr(position=(0,0),scale=(100,100),rotation=0,opacity=100):
    return {"ty":"tr","p":position if isinstance(position,dict) else prop(list(position)),"a":prop([0,0]),"s":scale if isinstance(scale,dict) else prop(list(scale)),"r":rotation if isinstance(rotation,dict) else prop(rotation),"o":opacity if isinstance(opacity,dict) else prop(opacity),"sk":prop(0),"sa":prop(0)}
def layer(index,name,shapes,pos=(256,256),scale=(100,100),rotation=0,opacity=100):
    return {"ddd":0,"ind":index,"ty":4,"nm":name,"sr":1,"ks":tr(pos,scale,rotation,opacity),"ao":0,"shapes":shapes,"ip":0,"op":FRAMES,"st":0,"bm":0}
def poly(points,c,outline=WHITE,width=6,transform=None): return [path(points),fill(c),stroke(outline,width),transform or tr()]
def ellipse(size,c,outline=None,width=0,transform=None):
    r=[{"ty":"el","p":prop([0,0]),"s":prop(list(size))},fill(c)]
    if outline: r.append(stroke(outline,width))
    return r+[transform or tr()]
def star(size): return path([[0,-size],[size*.22,-size*.22],[size,0],[size*.22,size*.22],[0,size],[-size*.22,size*.22],[-size,0],[-size*.22,-size*.22]])
def base(index=1): return [layer(index,"violet action aura",ellipse((448,448),VIOLET,LAVENDER,8),opacity=46)]

def hyperspeed():
    ls=base(); ship=[[-180,8],[-55,-73],[135,-24],[178,0],[135,24],[-55,73]]
    ls.append(layer(2,"hyperspeed craft",poly(ship,OBSIDIAN,GOLD,10),scale=key([(0,[94,94]),(60,[108,108]),(120,[94,94])])) )
    for i,y in enumerate((-80,0,80)):
        ls.append(layer(3+i,"hyperspeed trail",[path([[-208,y],[-70,y]],False),stroke(CYAN,14),tr(opacity=key([(0,15),(24,100),(55,15),(80,100),(120,15)]))]))
    ls.append(layer(6,"jump core",[star(34),fill(WHITE),tr(position=(-22,0),opacity=key([(0,15),(30,100),(60,15),(90,100),(120,15)]))]))
    return ls

def teleport():
    ls=base(); portal=ellipse((304,304),[0,0,0,0],CYAN,14,transform=tr(rotation=key([(0,0),(120,360)])))
    ls.append(layer(2,"teleport portal",portal)); ls.append(layer(3,"portal inner",ellipse((210,210),OBSIDIAN,LAVENDER,9)))
    body=poly([[-38,118],[-50,-22],[0,-122],[50,-22],[38,118]],GOLD,WHITE,6,transform=tr(scale=key([(0,[65,65]),(50,[112,112]),(120,[65,65])]),opacity=key([(0,15),(20,100),(82,100),(120,15)])))
    ls.append(layer(4,"teleported traveller",body))
    return ls

def orbital():
    ls=base(); ls.append(layer(2,"orbital planet",ellipse((198,198),OBSIDIAN,STEEL,10)))
    orbit=ellipse((390,170),[0,0,0,0],LAVENDER,8,transform=tr(rotation=-24)); ls.append(layer(3,"orbital trajectory",orbit))
    satellite=poly([[-22,-12],[-55,-38],[-55,38],[22,12],[55,38],[55,-38]],GOLD,WHITE,5,transform=tr(position=key([(0,[167,-92]),(30,[0,-125]),(60,[-167,-92]),(90,[0,125]),(120,[167,-92])])) )
    ls.append(layer(4,"orbital maneuver craft",satellite)); return ls

def repair_units():
    ls=base(); bot=poly([[-120,108],[-120,-40],[-72,-105],[72,-105],[120,-40],[120,108]],OBSIDIAN,STEEL,10)
    ls.append(layer(2,"field repair drone",bot)); ls.append(layer(3,"drone visor",poly([[-78,-42],[78,-42],[54,10],[-54,10]],VIOLET,CYAN,6)))
    wrench=poly([[-22,-142],[16,-142],[16,-30],[58,12],[30,40],[-12,-2],[-22,-2]],GOLD,WHITE,5,transform=tr(position=key([(0,[0,20]),(60,[0,-6]),(120,[0,20])]),rotation=key([(0,-8),(60,8),(120,-8)])))
    ls.append(layer(4,"repair wrench",wrench)); return ls

def repair_buildings():
    ls=base(); shell=poly([[-170,145],[-170,-36],[-105,-36],[-105,-132],[105,-132],[105,-36],[170,-36],[170,145]],OBSIDIAN,GOLD,10)
    ls.append(layer(2,"damaged building",shell)); crack=[path([[-52,-95],[-18,-42],[-45,15],[16,50],[-12,118]],False),stroke(RED,12),tr()]
    ls.append(layer(3,"building damage",crack)); plus=[path([[-42,0],[42,0]],False),stroke(CYAN,15),tr(position=(76,-30))]+[path([[0,-42],[0,42]],False),stroke(CYAN,15),tr(position=(76,-30))]
    ls.append(layer(4,"construction repair signal",plus,opacity=key([(0,20),(30,100),(60,20),(90,100),(120,20)]))); return ls

def manual_scan():
    ls=base(); ls.append(layer(2,"manual scan lens",ellipse((258,258),OBSIDIAN,CYAN,12,transform=tr(position=(-35,-35)))))
    ls.append(layer(3,"lens handle",[path([[58,58],[155,155]],False),stroke(GOLD,27),tr()]))
    line=[path([[-128,0],[128,0]],False),stroke(WHITE,8),tr(position=key([(0,[-35,-108]),(60,[-35,108]),(120,[-35,-108])]))]
    ls.append(layer(4,"manual scan line",line)); target=[star(26),fill(RED),tr(position=(35,26),opacity=key([(0,0),(42,100),(72,0),(120,0)]))]
    ls.append(layer(5,"scan target",target)); return ls

def write(name,builder):
    data={"v":"5.5.7","fr":60,"ip":0,"op":FRAMES,"w":512,"h":512,"nm":f"The Vagabond {name}","ddd":0,"assets":[],"layers":builder()}
    target=OUT/f"{name}_tgs_v14"/f"{name}_tgs_v14.tgs"; target.parent.mkdir(parents=True,exist_ok=True)
    target.write_bytes(gzip.compress(json.dumps(data,separators=(",",":"),allow_nan=False).encode(),compresslevel=9,mtime=0)); print(f"{name}: {target.stat().st_size} bytes")

def main():
    for name,builder in {"hyperspeed":hyperspeed,"teleport":teleport,"orbital":orbital,"repair_units":repair_units,"repair_buildings":repair_buildings,"manual_scan":manual_scan}.items(): write(name,builder)
if __name__=="__main__": main()

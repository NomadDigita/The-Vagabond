#!/usr/bin/env python3
"""Build the first narrative Vagabond sticker collection as native-vector TGS."""
import gzip, importlib.util, json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
CORE = ROOT / "assets/visual-system/pipeline/build_vagabond_tgs_core_v14.py"
spec = importlib.util.spec_from_file_location("core", CORE)
core = importlib.util.module_from_spec(spec); spec.loader.exec_module(core)
OUT = ROOT / "assets/visual-system/stickers/animated/v1"

SCENES = {
 "vagabond_salute":"commander", "crystal_discovery":"mutation", "raid_charge":"vehicle",
 "outpost_home":"base", "factory_forge":"factory", "tactical_win":"ranking",
 "shield_hold":"shield", "market_deal":"market", "radio_call":"radio",
 "research_breakthrough":"research", "wasteland_weather":"exploration", "diplomatic_pact":"diplomacy",
 "iron_warden_guard":"defense", "waste_phantom_salvage":"scrap", "rookie_survivor_arrives":"hq",
 "drone_nest_warlord":"boss", "field_medic_patch":"research", "outpost_under_attack":"warning",
 "defeat_but_alive":"commander", "level_up":"warlord", "recruit_join":"alliance",
 "world_boss_awakens":"boss", "rebellion_signal":"rebellion", "raid_victory_convoy":"transport",
}

def actor(index, pose, colour):
    head = core.ellipse((96,96), core.OBSIDIAN, core.WHITE, 7, transform=core.tr(position=(-105,-70)))
    body = core.poly([[-168,170],[-150,15],[-58,15],[-24,170]], colour, core.STEEL, 8)
    arm = [core.path([[-118,40],[-42,pose]],False), core.stroke(core.STEEL,17), core.tr()]
    visor = core.poly([[-142,-83],[-68,-83],[-78,-56],[-132,-56]], core.CYAN, core.WHITE, 4)
    return [core.layer(index,"Vagabond field commander",head), core.layer(index+1,"commander armour",body), core.layer(index+2,"commander gesture",arm), core.layer(index+3,"commander visor",visor)]

def write(key, source):
    layers = source() + actor(20, -130 if key in {"vagabond_salute","level_up","rebellion_signal"} else 95, core.VIOLET)
    data={"v":"5.5.7","fr":60,"ip":0,"op":120,"w":512,"h":512,"nm":f"The Vagabond Sticker {key}","ddd":0,"assets":[],"layers":layers}
    target=OUT/key/f"{key}.tgs"; target.parent.mkdir(parents=True,exist_ok=True)
    target.write_bytes(gzip.compress(json.dumps(data,separators=(",",":")).encode(),compresslevel=9,mtime=0))
    print(f"{key}: {target.stat().st_size} bytes")

for key, source in SCENES.items(): write(key, getattr(core, f"{source}_icon"))

# The Vagabond — Event & Faction Stickers

This independent six-sticker family is for live event announcements, faction
identity, and high-emotion chat moments. It uses native transparent 512px TGS
animation: bold silhouettes, vector-only construction, and a readable hold.

| Sticker | Chat intent | Subject-led scene | Motion cue |
|---|---|---|---|
| `steel_vanguard_last_stand` | hold the line | A Steel Vanguard defender stands inside a physical kinetic shield | Shield flexes under a red breach blast |
| `rust_nomad_ambush` | ambush / hit convoy | Rust Nomad crawler rises over a dune | Wheels turn and dust kicks outward |
| `rebellion_uprising` | rebel event | Survivor raises the red rebellion banner beside a campfire | Banner lifts, broadcast ring pulses |
| `drone_hive_incursion` | hostile swarm | Rogue drone hive directs four scout drones | Hive tilts and hostile optic pulses |
| `alliance_convocation` | alliance formed | Steel and violet gauntlets clasp at the federation signal | Hands meet, gold energy resolves into a cyan ring |
| `wasteland_eclipse` | world event | Eclipse passes over an outpost ridge | Moon shadow crosses and distant beacons blink |

## Delivery

Run `python assets/visual-system/pipeline/build_vagabond_event_stickers_v1.py`.
Each output belongs beneath `stickers/animated/v1-events/`; it does not modify
the general sticker builder or any game code.

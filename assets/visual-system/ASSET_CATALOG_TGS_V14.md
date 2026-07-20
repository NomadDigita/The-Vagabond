# The Vagabond - TGS V14 Production Catalogue

V14 is the Telegram-confirmed sharp-vector format: 512x512 canvas, 60 FPS,
two-second loop, transparent background, native `.tgs`. Each entry below is a
real game object or state, not a recolored generic badge.

| Asset | Game meaning | Silhouette | Motion | Deliverable |
|---|---|---|---|---|
| `map` | Expedition / map navigation | Folded field map with route line | Route marker travels the plotted path | `animated/map_tgs_v14/map_tgs_v14.tgs` |
| `raid` | Raid launched / hostile target | Bullseye with crossed lances | Lances breathe apart; impact spark fires | `animated/raid_tgs_v14/raid_tgs_v14.tgs` |
| `base` | Home base / stronghold | Fortified gate and two towers | Cyan roof beacon flashes | `animated/base_tgs_v14/base_tgs_v14.tgs` |
| `transport` | Convoy / vehicle movement | Armored cargo crawler with cab and wheels | Wheels turn; headlamp pulses | `animated/transport_tgs_v14/transport_tgs_v14.tgs` |
| `combat` | Battle / attack | Crossed steel blades around a red core | Blades tense; core beats; impact spark flashes | `animated/combat_tgs_v14/combat_tgs_v14.tgs` |
| `satellite` | Reconnaissance / communications | Angled receiver dish and emitter | Radio rings transmit from receiver | `animated/satellite_tgs_v14/satellite_tgs_v14.tgs` |
| `shield` | Defense / protection | Six-sided Vagabond shield | Internal charge star pulses | `animated/shield_tgs_v14/shield_tgs_v14.tgs` |
| `gear` | Crafting / mechanics | Large functional cog and hub | Cog makes a complete turn; spark catches at the rim | `animated/gear_tgs_v14/gear_tgs_v14.tgs` |
| `warning` | Hazard / caution | Industrial warning triangle and exclamation | Mark pulses with a clear alert rhythm | `animated/warning_tgs_v14/warning_tgs_v14.tgs` |
| `failure` | Failed action / system break | Fractured red failure cross | Break line flashes through the cross | `animated/failure_tgs_v14/failure_tgs_v14.tgs` |
| `ai_mech` | AI / machine intelligence | Mech head, antenna, and scanning visor | Visor sweep traverses the face; antenna signal blinks | `animated/ai_mech_tgs_v14/ai_mech_tgs_v14.tgs` |
| `electricity` | Power / energy | Heavy energy bolt with satellite sparks | Bolt surges; separate gold sparks discharge | `animated/electricity_tgs_v14/electricity_tgs_v14.tgs` |
| `scrap` | Salvage / materials | Uneven salvage pile and welded plate | Welding spark bursts from the join | `animated/scrap_tgs_v14/scrap_tgs_v14.tgs` |
| `hq` | Terminal HQ / command centre | Tall fortified command tower and antenna | Command signal expands from the antenna | `animated/hq_tgs_v14/hq_tgs_v14.tgs` |
| `camp` | Outpost camp | Field tent, ground line, and signal fire | Campfire flickers in the tent opening | `animated/camp_tgs_v14/camp_tgs_v14.tgs` |
| `economy` | System economy / tax | Armoured vault with exchange hub | Hub spokes rotate like a working market mechanism | `animated/economy_tgs_v14/economy_tgs_v14.tgs` |
| `workshop` | Heavy workshop | Stepped factory roof and conveyor | Welding spark travels the conveyor line | `animated/workshop_tgs_v14/workshop_tgs_v14.tgs` |
| `research` | Research laboratory | Scientific flask with contained compound | Reaction star pulses inside the fluid | `animated/research_tgs_v14/research_tgs_v14.tgs` |
| `mutation` | Mutation core | Full double-helix with gold rungs | Helix twists while rungs light in sequence | `animated/mutation_tgs_v14/mutation_tgs_v14.tgs` |
| `mining` | Active mining | Heavy pickaxe striking a crystal ore node | Pickaxe swings; violet ore holds the impact point | `animated/mining_tgs_v14/mining_tgs_v14.tgs` |
| `radar` | Scan targets / expedition radar | Large radar ring, sweeping wedge, and target | Sweep completes a full scan; target pings | `animated/radar_tgs_v14/radar_tgs_v14.tgs` |

## Production rule

All future Vagabond emoji must declare these four things before generation:

1. Exact in-game meaning and Unicode/custom-emoji replacement target.
2. A subject-recognizable silhouette at inline size.
3. Motion that belongs to that subject rather than a copied global pulse.
4. A dedicated isolated Telegram test-set slug before production adoption.

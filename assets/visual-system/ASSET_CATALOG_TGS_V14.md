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

## Production rule

All future Vagabond emoji must declare these four things before generation:

1. Exact in-game meaning and Unicode/custom-emoji replacement target.
2. A subject-recognizable silhouette at inline size.
3. Motion that belongs to that subject rather than a copied global pulse.
4. A dedicated isolated Telegram test-set slug before production adoption.

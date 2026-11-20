# ADR 0149: Decisions from `internal/engine/tick/relicconvoys.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/relicconvoys.go` and moved here so the source no longer carries them. Each entry is the original comment.

## RARE_WORLD_FEATURES_PLAN.md Phase 1: Relic Convoys.

Removed from `internal/engine/tick/relicconvoys.go:12`.

~~~~text
RARE_WORLD_FEATURES_PLAN.md Phase 1: Relic Convoys.

A vanishingly rare, server-wide race: a pre-war relic convoy surfaces
at a random coordinate, broadcast to every player at once, and the
first person to tap Claim (see internal/bot/handlers/relicconvoy.go)
wins a permanent title plus a cash windfall. There is no travel time
and no coordinate-proximity requirement - the entire mechanic IS the
race, deliberately kept mechanically simple so the "rare and
exciting" read isn't buried under friction. See the plan doc's
section 1 for the full design rationale.
~~~~


## relicConvoySpawnChance is the per-tick probability of a new relic

Removed from `internal/engine/tick/relicconvoys.go:23`.

~~~~text
relicConvoySpawnChance is the per-tick probability of a new relic
convoy spawning, checked only when no convoy is currently active (see
relicConvoySpawn below) - so this is genuinely "how rare is the next
one", not diluted by a backlog. At the tick engine's ~60s cadence,
0.15% averages roughly one spawn every ~11 hours of continuous
uptime - a real "stop what you're doing" moment, not background hum.

A var, not a const, so relicconvoys_test.go can deterministically
force a spawn (set to 1.0, restored via t.Cleanup) instead of
looping thousands of real-Postgres round trips waiting for a
legitimate 0.15% roll to land.
~~~~


## relicConvoyWindowMinutes is how long an unclaimed relic convoy stays

Removed from `internal/engine/tick/relicconvoys.go:36`.

~~~~text
relicConvoyWindowMinutes is how long an unclaimed relic convoy stays
live before vanishing - long enough that players across timezones/
session patterns have a real shot, short enough to stay an event
rather than a standing fixture.
~~~~


## relicConvoyMinReward/relicConvoyMaxReward bound the cash windfall a

Removed from `internal/engine/tick/relicconvoys.go:42`.

~~~~text
relicConvoyMinReward/relicConvoyMaxReward bound the cash windfall a
claimed relic pays out. Scaled off the game's own Crystal-conversion
economy (crystal_exchange.go's dollars rate of 2000/Crystal) rather
than an arbitrary number - roughly 6-10 Crystal's worth of cash for
doing nothing but tapping first.
~~~~


## relicNames is the curated flavor pool a new convoy's name is drawn

Removed from `internal/engine/tick/relicconvoys.go:52`.

~~~~text
relicNames is the curated flavor pool a new convoy's name is drawn
from, so the broadcast headline reads as a named, memorable event
("The Last Aurora Convoy is passing through...") rather than a
generic "convoy #4821".
~~~~


## relicConvoySpawn is the "relic_convoy_spawn" tick phase - see

Removed from `internal/engine/tick/relicconvoys.go:69`.

~~~~text
relicConvoySpawn is the "relic_convoy_spawn" tick phase - see
engine.go's ProcessTick phase list. Rolls a new convoy into existence
only when none is currently active, so at most one is ever live at a
time, server-wide.
~~~~


## No seeded world geography yet (e.g. a brand new deployment) -

Removed from `internal/engine/tick/relicconvoys.go:89`.

~~~~text
No seeded world geography yet (e.g. a brand new deployment) -
nothing to anchor a convoy to, just skip this tick rather
than erroring the whole phase.
~~~~


## relicConvoyExpire is the "relic_convoy_expire" tick phase - clears

Removed from `internal/engine/tick/relicconvoys.go:116`.

~~~~text
relicConvoyExpire is the "relic_convoy_expire" tick phase - clears
convoys nobody claimed in time, with a "moved on" broadcast, matching
the weather engine's own "conditions have cleared" pattern rather
than silently vanishing.
~~~~

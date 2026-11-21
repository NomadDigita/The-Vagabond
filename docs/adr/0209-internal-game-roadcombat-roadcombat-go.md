# ADR 0209: Decisions from `internal/game/roadcombat/roadcombat.go`

Status: Accepted

These decisions were written as comments in `internal/game/roadcombat/roadcombat.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package roadcombat owns the deterministic, testable rules for Phase 3/4 of

Removed from `internal/game/roadcombat/roadcombat.go:1`.

~~~~text
Package roadcombat owns the deterministic, testable rules for Phase 3/4 of
MMO_WORLD_EVOLUTION_PLAN.md: turning a timer-only march into a route-aware
journey, and resolving a field battle when two expeditions meet on the
road. Like internal/game/worldintel, this package holds pure policy only -
no database or Telegram access - so balance changes can be tested without
a live bot.
~~~~


## EncounterRadius is how close (in world coordinate units) two moving

Removed from `internal/game/roadcombat/roadcombat.go:14`.

~~~~text
EncounterRadius is how close (in world coordinate units) two moving
expeditions must be before a road encounter can trigger.
~~~~


## EncounterRollChance is the per-tick probability that two expeditions

Removed from `internal/game/roadcombat/roadcombat.go:18`.

~~~~text
EncounterRollChance is the per-tick probability that two expeditions
within EncounterRadius of each other actually notice one another, once
eligible. Kept below 1.0 so a slow-moving convoy doesn't guarantee an
encounter the instant it enters range, and so parties that choose
"Continue" aren't immediately re-rolled into another encounter next tick.
~~~~


## EncounterCooldown is how long two expeditions that just resolved an

Removed from `internal/game/roadcombat/roadcombat.go:25`.

~~~~text
EncounterCooldown is how long two expeditions that just resolved an
encounter (by fighting or passing) are exempt from re-rolling against each
other, so a "Continue" choice doesn't just get re-asked next tick.
~~~~


## ResponseWindow is how long both commanders have to choose Attack/Continue

Removed from `internal/game/roadcombat/roadcombat.go:30`.

~~~~text
ResponseWindow is how long both commanders have to choose Attack/Continue
before the encounter auto-resolves as a peaceful pass.
~~~~


## RouteProgress returns how far through the CURRENT leg (outbound march or

Removed from `internal/game/roadcombat/roadcombat.go:40`.

~~~~text
RouteProgress returns how far through the CURRENT leg (outbound march or
return march) an expedition has travelled, as a 0..1 fraction. It is
deliberately based on the leg's own start time and planned duration
rather than the mutable resolve_time, so a later delay (fuel-depletion
pause, encounter freeze) cannot make an army's position snap backward -
progress simply holds at whatever fraction it had reached and does not
exceed 1.0.
~~~~


## CurrentPosition interpolates an expedition's current world position along

Removed from `internal/game/roadcombat/roadcombat.go:62`.

~~~~text
CurrentPosition interpolates an expedition's current world position along
its stored route snapshot. During "marching" the army moves origin ->
destination; during "returning" it moves destination -> origin (heading
home along the same corridor). Any other state is treated as stationary
at the destination (matches "engaged", where the force has arrived).
~~~~


## FieldForce is the subset of a raid_forces row relevant to a mobile

Removed from `internal/game/roadcombat/roadcombat.go:100`.

~~~~text
FieldForce is the subset of a raid_forces row relevant to a mobile
field battle: no base, no defense grid, no turrets - just what each
commander actually marched out with, plus the tech/supply modifiers that
travel with the column itself.
~~~~


## attacker-side mech multiplier input, floor 1

Removed from `internal/game/roadcombat/roadcombat.go:114`.

~~~~text
attacker-side mech multiplier input, floor 1
~~~~


## rations/ammo both depleted -> offense penalty, mirrors base-raid rule

Removed from `internal/game/roadcombat/roadcombat.go:115`.

~~~~text
rations/ammo both depleted -> offense penalty, mirrors base-raid rule
~~~~


## electricity/logistics depleted -> mech/capital tech multiplier disabled (Phase 5 milestone

Removed from `internal/game/roadcombat/roadcombat.go:116`.

~~~~text
electricity/logistics depleted -> mech/capital tech multiplier disabled (Phase 5 milestone 4), distinct from SuppliesOut's blanket penalty
~~~~


## unitAttackRatings mirrors the flat per-unit attack contributions used by

Removed from `internal/game/roadcombat/roadcombat.go:126`.

~~~~text
unitAttackRatings mirrors the flat per-unit attack contributions used by
the base-raid combat resolver in internal/engine/tick/engine.go
(resolveRaidCombats), so a field battle "feels" consistent with a raid
against a real base rather than introducing a disconnected second power
curve. Soldiers/Mechs keep their existing 15.0 base multiplier; capital
units use content.Units' canonical AttackRating values, duplicated here
as plain constants (this package intentionally has zero dependencies, so
callers pass in only the numbers, not a content.Unit) to keep it testable
without importing the content package.
~~~~


## Power computes a force's raw combat rating for a field battle. Unlike a

Removed from `internal/game/roadcombat/roadcombat.go:145`.

~~~~text
Power computes a force's raw combat rating for a field battle. Unlike a
base raid there is no defense grid, turret counter-play, or biome/weather
term here - those are base-specific; two marching columns fight on equal
terrain. Weather is still applied by the caller as part of offenseMod,
since it affects a field battle exactly as it affects a march.
~~~~


## High-tech contributions offline: mechs fight as unenhanced

Removed from `internal/game/roadcombat/roadcombat.go:165`.

~~~~text
High-tech contributions offline: mechs fight as unenhanced
infantry (no multiplier) and capital units lose their tech
edge, but nobody gets a blanket offense penalty the way
SuppliesOut applies - this is "disables high-tech
contributions," not "the whole column can't fight."
~~~~


## fraction of A's TotalUnits() destroyed

Removed from `internal/game/roadcombat/roadcombat.go:192`.

~~~~text
fraction of A's TotalUnits() destroyed
~~~~


## fraction of B's TotalUnits() destroyed

Removed from `internal/game/roadcombat/roadcombat.go:193`.

~~~~text
fraction of B's TotalUnits() destroyed
~~~~


## ResolveBattle applies the same win/lose casualty shape used for base

Removed from `internal/game/roadcombat/roadcombat.go:196`.

~~~~text
ResolveBattle applies the same win/lose casualty shape used for base
raids (winner ~10-12% losses, loser ~30-35% losses) to a symmetric
mobile-vs-mobile fight. A draw (power within 5% of each other) applies
moderate losses to both sides and awards no cargo.
~~~~


## CargoShare is capacity-limited cargo a road-battle winner can capture

Removed from `internal/game/roadcombat/roadcombat.go:289`.

~~~~text
CargoShare is capacity-limited cargo a road-battle winner can capture
from the loser's carried loot. Real warfare doesn't let a small patrol
walk off with a fully loaded convoy's entire hold, so capture is capped
as a fraction of what the loser is carrying, further capped by the
winner's own free carrying capacity if provided (capacityLimit <= 0 means
"no additional cap beyond the capture fraction").
~~~~


## --- Phase 5: route weather incidents and reinforcement convoys ---

Removed from `internal/game/roadcombat/roadcombat.go:308`.

~~~~text
--- Phase 5: route weather incidents and reinforcement convoys ---

These live in the same package as the route/field-battle policy above
because they are governed by the same physical model (a column at a
point on the route, blocked by something external, resumed via the
paused_at/leg_started_at shift) rather than because they are
thematically identical to combat.
~~~~


## IncidentBaseRollChance is the per-tick chance of a local weather

Removed from `internal/game/roadcombat/roadcombat.go:316`.

~~~~text
IncidentBaseRollChance is the per-tick chance of a local weather
incident on a column travelling through a continent with no matching
active regional event - freak weather can still happen on a clear day.
~~~~


## IncidentElevatedRollChance is the per-tick chance once the column's

Removed from `internal/game/roadcombat/roadcombat.go:321`.

~~~~text
IncidentElevatedRollChance is the per-tick chance once the column's
current continent has a matching active world_events entry (e.g. an
acid_rain event raising flood odds).
~~~~


## IncidentMatchesActiveWeather reports whether an active continent-wide

Removed from `internal/game/roadcombat/roadcombat.go:326`.

~~~~text
IncidentMatchesActiveWeather reports whether an active continent-wide
world event (from internal/engine/world) makes a given local route
incident type more likely, so Phase 5 reads the existing weather engine
as an input signal instead of rolling a second, disconnected one.
~~~~


## IncidentDuration maps a rolled severity (1..3) to how long the temporary

Removed from `internal/game/roadcombat/roadcombat.go:345`.

~~~~text
IncidentDuration maps a rolled severity (1..3) to how long the temporary
camp must hold before conditions clear, matching "which might take a day
or even longer" - a minor incident clears in about half a day, a severe
one can run a day and a half.
~~~~


## ConvoySpeedCoordinateUnitsPerMinute is how fast a dedicated resupply

Removed from `internal/game/roadcombat/roadcombat.go:360`.

~~~~text
ConvoySpeedCoordinateUnitsPerMinute is how fast a dedicated resupply
convoy covers ground - deliberately slower than a combat column's
implicit speed (a marching column's minutes are balance-tuned per raid,
not distance-derived) so a convoy is a real logistics commitment, not a
courier that outruns the very expedition it's chasing.
~~~~


## MinConvoyTravelMinutes floors convoy travel time so a resupply run to an

Removed from `internal/game/roadcombat/roadcombat.go:367`.

~~~~text
MinConvoyTravelMinutes floors convoy travel time so a resupply run to an
adjacent tile still takes a meaningful, non-instant amount of time.
~~~~

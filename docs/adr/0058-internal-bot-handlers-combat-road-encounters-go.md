# ADR 0058: Decisions from `internal/bot/handlers/combat_road_encounters.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/combat_road_encounters.go` and moved here so the source no longer carries them. Each entry is the original comment.

## isRealPlayer reports whether a user_id belongs to an actual Telegram

Removed from `internal/bot/handlers/combat_road_encounters.go:17`.

~~~~text
isRealPlayer reports whether a user_id belongs to an actual Telegram
account, as opposed to an AI faction's synthetic negative telegram_id
(see cmd/bot/main.go's seedAICivilizations). This is the handlers-
package twin of internal/engine/tick/engine.go's isRealPlayer -
duplicated rather than exported across packages because it's a single
comparison, and because this package and the tick engine hold different
transaction contexts that shouldn't be coupled just to share one line.

Needed here specifically because an AI-launched raid (Phase 6's
decision loop, internal/engine/tick/aidecisions.go) can now be either
side of a road encounter a HUMAN resolves by tapping Attack/Continue -
every "notify the other commander" site in this file was written back
when "the other commander" always meant a real player, and is unguarded
without this check. See BUGS_AND_INCONSISTENCIES.md for the sibling bug
this was found alongside (the tick-engine side of the same issue,
already fixed there).
~~~~


## loadRoadFieldForce loads the mobile combat force, supply status, and

Removed from `internal/bot/handlers/combat_road_encounters.go:37`.

~~~~text
loadRoadFieldForce loads the mobile combat force, supply status, and
military tech level for one side of a road encounter, in the shape
roadcombat.Power/CasualtiesFor expect. It intentionally ignores buggies,
ships, jets, nukes, and every logistics/transport unit - those units
escort and supply the column but don't fight in a field battle, mirroring
how they're excluded from the base-raid attack rating.
~~~~


## applyRoadSurvivors overwrites a raid_forces row's combat-unit columns

Removed from `internal/bot/handlers/combat_road_encounters.go:71`.

~~~~text
applyRoadSurvivors overwrites a raid_forces row's combat-unit columns
with the post-battle survivor counts. Non-combat columns (buggies,
transports, cargo ships) are untouched - they never took part in the
fight and still need to carry the column (and any captured loot) home.
~~~~


## captureCargo moves a capped share of every stolen_* resource type the

Removed from `internal/bot/handlers/combat_road_encounters.go:85`.

~~~~text
captureCargo moves a capped share of every stolen_* resource type the
loser is carrying over to the winner, matching the "steal resources (not
just one resource, but many, including Crystal)" requirement even for a
road ambush rather than only a base raid. Returns a human-readable
summary of what changed hands (for the battle report), or "" if the
loser had nothing worth taking.
~~~~


## HandleRoadEncounterCallback resolves the Attack/Continue decision from

Removed from `internal/bot/handlers/combat_road_encounters.go:122`.

~~~~text
HandleRoadEncounterCallback resolves the Attack/Continue decision from
the Expedition Radar panel for a Phase 4 road encounter
(MMO_WORLD_EVOLUTION_PLAN.md). Attack is unilateral and resolves
immediately - a real ambush doesn't wait for the victim's permission.
Continue is reciprocal: it only resolves the encounter once BOTH sides
have chosen it (or the response window times out), matching "War may
happen, or both parties may simply continue on their way."
~~~~


## myRaidID used to travel in callback_data, but combined with

Removed from `internal/bot/handlers/combat_road_encounters.go:156`.

~~~~text
myRaidID used to travel in callback_data, but combined with
encounterID that pushed the button's callback_data past
Telegram's 64-byte cap, so Telegram silently refused to attach
the button at all - the exact "the button won't respond" bug
report this fix addresses. Deriving it server-side from the
caller's own encampment is both shorter and safer (no longer
trusting a client-supplied raid ID for something security-
relevant).
~~~~


## BUG FIX (2026-07-26): result.AWon already means "my side won" -

Removed from `internal/bot/handlers/combat_road_encounters.go:248`.

~~~~text
BUG FIX (2026-07-26): result.AWon already means "my side won" -
ResolveBattle(myPower, otherPower) was called with my side as
its "A" argument, independent of which raid happens to be
road_encounters.raid_a vs raid_b (an unrelated canonical
ordering used only for the pending-pair unique index). The
previous code compared result.AWon against (myRaidID ==
raidAID), which silently flipped the winner backwards - and
therefore the cargo-capture direction, the winner_raid_id
column, and the "WON"/"LOST" battle-report headline for BOTH
sides - every time the attacker happened to be raid_b in that
ordering. Fixed: just use result.AWon directly.
~~~~


## Both survivors continue their journey from the exact point they

Removed from `internal/bot/handlers/combat_road_encounters.go:273`.

~~~~text
Both survivors continue their journey from the exact point they
paused - a road battle costs lives and cargo, not the whole trip.
A side reduced to zero combat units has nothing left to march with
and is pulled from the road entirely.
~~~~


## aiGarrisonReserveFraction is the fraction of an AI faction's current

Removed from `internal/bot/handlers/combat_road_encounters.go:349`.

~~~~text
aiGarrisonReserveFraction is the fraction of an AI faction's current
workshop_inventory.soldiers/mechs treated as its "always-home" defense
reserve for road-base encounters, per
AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 1.4. A real player's
garrisoned_soldiers/garrisoned_mechs is a deliberate choice made once on
their Hero Commander panel and then read verbatim; no AI faction has
ever made that choice (garrisoned_soldiers/garrisoned_mechs reads 0,0
for every one of them), so reading that column for an AI base would
make it a free, uncontested win for any human column that stumbles into
it - not parity, a trivially exploitable farm. Computed fresh at
encounter-resolution time rather than stored, the same way a human's
actual mobilized-vs-garrisoned split is a live, in-the-moment decision.

Deliberately a separate constant from aidecisions.go's
aiFractionOfGarrisonCommitted (which caps a raid's OUTGOING commitment
at 65%, implying ~35% stays home) rather than reusing it or its
complement: "how much a raid commits" and "how much is left over to
defend an ambush" are different tuning knobs that happen to start in
the same neighborhood, not the same knob wearing two hats.
~~~~


## loadBaseGarrisonForce loads a passive base's standing home-defense

Removed from `internal/bot/handlers/combat_road_encounters.go:370`.

~~~~text
loadBaseGarrisonForce loads a passive base's standing home-defense
reserve as a roadcombat.FieldForce. This is deliberately a lighter
force than the base's full raid-defense computation (no defense grid,
turrets, or drafted-but-not-yet-launched units): a road ambush is a
field skirmish against whoever the base kept at home, not a full siege.
A commander who wants to actually raid the base's economy for loot
still needs to launch a real raid through the normal /raid flow, with
all of that system's existing protections (shields, pacts, etc.)
intact.

For a real player, this reads workshop_inventory.garrisoned_soldiers/
garrisoned_mechs - units the owner explicitly withheld from every
draft, see hero.go's Manual Defense Garrison. For an AI faction, see
aiGarrisonReserveFraction's doc comment above: that column is never
set, so this instead computes a synthetic reserve from the faction's
live soldiers/mechs pool.
~~~~


## applyBaseGarrisonCasualties writes back a road-base-encounter's

Removed from `internal/bot/handlers/combat_road_encounters.go:413`.

~~~~text
applyBaseGarrisonCasualties writes back a road-base-encounter's
casualties (not survivors - see below for why) to whichever pool the
defending force's garrison actually came from. For a real player, that
pool is garrisoned_soldiers/garrisoned_mechs (the entire thing was
committed, so this is equivalent to writing back survivors). For an AI
faction, only aiGarrisonReserveFraction of its live soldiers/mechs was
ever committed to this specific skirmish - the rest of the faction's
force wasn't involved and must not be touched - so this subtracts the
casualties from the live pool instead of overwriting it with the
(partial) survivor count.
~~~~


## HandleRoadBaseEncounterCallback resolves the Attack/Continue decision for

Removed from `internal/bot/handlers/combat_road_encounters.go:440`.

~~~~text
HandleRoadBaseEncounterCallback resolves the Attack/Continue decision for
a road-vs-base encounter (MMO_WORLD_EVOLUTION_PLAN.md Phase 4 milestone
2). Only the expedition's commander gets a choice here - the base didn't
choose to be in the road's path, so there is no symmetric "both sides
must agree to continue" step like the expedition-vs-expedition version;
Attack resolves immediately, and doing nothing before the deadline is a
peaceful pass (handled by the tick engine's expireRoadBaseEncounters).
~~~~


## The expedition continues its journey from exactly where it paused,

Removed from `internal/bot/handlers/combat_road_encounters.go:528`.

~~~~text
The expedition continues its journey from exactly where it paused,
unless it has nothing left to march with. No resource looting here
(see loadBaseGarrisonForce) - a skirmish costs lives, not the base's
economy, which stays behind the normal /raid protections.
~~~~


## resolveRoadBaseEncounterContinue is the explicit-Continue / timeout

Removed from `internal/bot/handlers/combat_road_encounters.go:592`.

~~~~text
resolveRoadBaseEncounterContinue is the explicit-Continue / timeout
counterpart for a road-vs-base encounter - same shape as
resolveRoadEncounterContinue, but only one raid to unfreeze since the
base side was never paused (it was never moving to begin with).
~~~~


## resolveRoadEncounterContinue is the mutual-Continue counterpart to the

Removed from `internal/bot/handlers/combat_road_encounters.go:612`.

~~~~text
resolveRoadEncounterContinue is the mutual-Continue counterpart to the
tick engine's timeout resolution (internal/engine/tick/engine.go
resolveEncounterAsContinue) - duplicated at this small size rather than
exported across packages, since the two call sites hold different
transaction contexts (bot handler vs. tick engine) and the logic is a
handful of UPDATEs, not enough to justify a shared dependency.
~~~~


## convoySupplyPackage is the default resupply amount (in the picker's

Removed from `internal/bot/handlers/combat_road_encounters.go:647`.

~~~~text
convoySupplyPackage is the default resupply amount (in the picker's
25/50/75/100 tiers) a standard single-pair convoy carries per
field-supply gauge (rations/ammo/electricity/logistics are all 0-100
scales, see attacker_* columns on raids) when the player doesn't
change anything. A real logistics operation restocks a meaningful
chunk, not a full instant refill - the stranded column still has to
make the most of it and may need a second convoy on a very long
remaining leg, unless the player commits more pairs or a bigger
package via HandleConvoyConfigPanel below.
~~~~


## convoyScrapCostBase / convoyScrapCostPerUnit price a convoy by distance:

Removed from `internal/bot/handlers/combat_road_encounters.go:658`.

~~~~text
convoyScrapCostBase / convoyScrapCostPerUnit price a convoy by distance:
a nearby resupply is cheap, reinforcing a column halfway across the
world costs real materiel, matching "using [logistics] mechanics should
be very expensive" in spirit (this is the Phase 5 equivalent of Phase
4's expensive march speed-up).
~~~~


## convoyMaxPairs caps how many Hauler+Tanker pairs a single convoy order

Removed from `internal/bot/handlers/combat_road_encounters.go:667`.

~~~~text
convoyMaxPairs caps how many Hauler+Tanker pairs a single convoy order
can commit at once - the player-facing answer to "let me send more
units/more resources with the convoy": each additional pair both adds
to the total delivered (capped at a full 100% resupply) and to the
transports actually placed at risk if the convoy is ambushed en route.
~~~~


## convoyCarriedPct is how much of each field-supply gauge (0-100) the

Removed from `internal/bot/handlers/combat_road_encounters.go:678`.

~~~~text
convoyCarriedPct is how much of each field-supply gauge (0-100) the
convoy actually delivers: each committed pair contributes pct
percentage points, stacking up to a hard 100% cap - so 2 pairs at 50%
or 1 pair at 100% both deliver a full resupply, just at different
cost/risk trade-offs.
~~~~


## convoyCost scales the base distance-priced Scrap cost by both fill

Removed from `internal/bot/handlers/combat_road_encounters.go:691`.

~~~~text
convoyCost scales the base distance-priced Scrap cost by both fill
level (pct, relative to the original 50% baseline so the default
selection reproduces the original flat cost exactly) and pair count,
while Metal cost - modeling the transports' own fuel/upkeep - scales
with pair count alone, regardless of how full they're loaded.
~~~~


## convoyTargetDistance re-derives a stranded column's frozen current

Removed from `internal/bot/handlers/combat_road_encounters.go:702`.

~~~~text
convoyTargetDistance re-derives a stranded column's frozen current
position (same logic HandleDispatchConvoy already used) and returns
its distance from home - shared by the config panel (to preview cost)
and the actual dispatch handler (to charge the same cost it just
previewed).
~~~~


## HandleConvoyConfigPanel is the interactive picker: how much of each

Removed from `internal/bot/handlers/combat_road_encounters.go:741`.

~~~~text
HandleConvoyConfigPanel is the interactive picker: how much of each
field-supply gauge to carry per pair (25/50/75/100%) and how many
Hauler+Tanker pairs to commit (more pairs = more delivered, capped at
a full 100% resupply, and more risk if the convoy is ambushed).
Answers "let me choose how much to send" directly - a convoy used to
always be a hardcoded 50% package with exactly 1 Hauler + 1 Tanker and
no way to send more. Every button here re-renders this same panel with
the new selection except the final Confirm, which hands off to
HandleDispatchConvoy with the chosen pct/pairs baked into its
callback data.
~~~~


## Plain (uncolored) buttons here rather than keyboards.Styled - the

Removed from `internal/bot/handlers/combat_road_encounters.go:829`.

~~~~text
Plain (uncolored) buttons here rather than keyboards.Styled - the
styled-button helper only supports SendStyled (always posts a new
message, no in-place edit), and this panel needs to edit itself in
place on every tap so re-selecting a tier doesn't spam a fresh
message each time. The ✅/🚚/❌ prefixes carry the same meaning
color would have.
~~~~


## HandleDispatchConvoy implements the "reinforcement resources... transported

Removed from `internal/bot/handlers/combat_road_encounters.go:867`.

~~~~text
HandleDispatchConvoy implements the "reinforcement resources... transported
by dedicated resource units" requirement: a commander with a column
halted at movement_state = 'awaiting_reinforcement' can send a convoy
from home carrying rations/ammo/electricity/logistics, gated on actually
having the Hauler+Tanker pairs to commit (matching the existing
raid-launch rule that transports must be real, staged units) and a
scrap/metal cost that scales with the real distance to the stranded
column's current, frozen position, the chosen package size, and pair
count - see HandleConvoyConfigPanel, which is what actually presents
the picker; args[1]/args[2] (pct/pairs) arrive pre-selected from
there, defaulting to the original flat 50%/1-pair behavior if called
with just a raid ID (kept for any other caller/back-compat).
~~~~


## Since the column is frozen (movement_state != 'moving'), paused_at

Removed from `internal/bot/handlers/combat_road_encounters.go:939`.

~~~~text
Since the column is frozen (movement_state != 'moving'), paused_at
anchors its true current position - it isn't advancing while it
waits, so this is exactly where the convoy needs to travel to.
~~~~


## breakCampCrystalPerSeverityHour / breakCampMinCrystalCost /

Removed from `internal/bot/handlers/combat_road_encounters.go:994`.

~~~~text
breakCampCrystalPerSeverityHour / breakCampMinCrystalCost /
breakCampMaxCrystalCost price Phase 5 milestone 5's "pay to clear a
weather camp early" option: cost scales with severity and how much time
is actually left (clearing a camp that's about to expire anyway should
be cheap; clearing a severe camp with a day and a half left should be
genuinely expensive), with a documented floor and ceiling so it's never
free and never unboundedly punishing.
~~~~


## handleBreakCampEarly implements the priced early-clear: paying Crystal

Removed from `internal/bot/handlers/combat_road_encounters.go:1005`.

~~~~text
handleBreakCampEarly implements the priced early-clear: paying Crystal
(deliberately - Crystal is the rarest resource, so this is an
"exceptional, expensive" bypass, matching the plan's milestone 5
requirement) resolves the active route_incidents row immediately via the
same leg/resolve_time-shift resume logic used everywhere else in Phase
3-5, rather than a separate, disconnected "instant done" shortcut.
~~~~

# ADR 0085: Decisions from `internal/bot/handlers/jobs.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/jobs.go` and moved here so the source no longer carries them. Each entry is the original comment.

## sendConfirmCard renders the standard Confirm/Cancel prompt card used

Removed from `internal/bot/handlers/jobs.go:30`.

~~~~text
sendConfirmCard renders the standard Confirm/Cancel prompt card used
by every resource-spending job below (2026-08-05, by explicit
project-owner direction - "let there be confirm buttons for all
jobs... in a situation whereby you mistakenly pressed teleport, it
will just automatically go"). actionKey is the short callback-data
prefix registered in cmd/bot/main.go as "\f"+actionKey+"_c" (confirm)
and "\f"+actionKey+"_x" (cancel) - see HandleTeleport/
HandleGhostProtocol above for the pattern this generalizes.
~~~~


## sendCancelledCard is the standard reply for every "❌ Cancel" callback

Removed from `internal/bot/handlers/jobs.go:50`.

~~~~text
sendCancelledCard is the standard reply for every "❌ Cancel" callback
below - nothing was ever charged or executed, so this just closes out
the card with a plain confirmation of that.
~~~~


## HandleJobsPanel is the "🛠️ Odd Jobs" mother-keyboard entry point -

Removed from `internal/bot/handlers/jobs.go:58`.

~~~~text
HandleJobsPanel is the "🛠️ Odd Jobs" mother-keyboard entry point -
plants the JobsNavigation child keyboard (via sendPanelWithNav, see
navhelper.go's doc comment for why a plain c.Send with two reply-markup
arguments would silently drop one of them) and shows a cost/cooldown
reference summary so a player can decide what to run without needing
to trigger each job blind. Every button on the JobsNavigation keyboard
fires its job directly - see that function's doc comment in
internal/bot/keyboards/navigation.go for why there's no further
sub-panel here. As of 2026-08-05, every resource-spending job (all
except Gather Sunlight, which only ever gains resources with no
downside to confirm against, and the pure-informational scan/trade
aliases) shows its own inline Confirm/Cancel card before actually
executing - see HandleTeleport's doc comment for the full rationale.
~~~~


## hyperspeedTarget is the nearest-resolving accelerable mission - either

Removed from `internal/bot/handlers/jobs.go:89`.

~~~~text
hyperspeedTarget is the nearest-resolving accelerable mission - either
an outbound/returning raid (raids.resolve_time) or a scout party
already on its way home (scout_missions.return_eta, phase='returning').
2026-08-06 direct request: "job HyperSpeed should be able to work to
HyperSpeed long range scouting units returning home" - previously
HyperSpeed only ever looked at the raids table, so a player with no
active raid but a scout party inbound had nothing to accelerate even
though one clearly existed. A scout mission still in its 'searching'
phase has no fixed ETA to cut in half (that's the whole point of an
open-ended search - see scoutMissionPingInterval's doc comment in
internal/engine/tick/scoutmissions.go), so only 'returning' qualifies,
mirroring how a raid only qualifies once it has a resolve_time at all.
~~~~


## hyperspeedQuerier is satisfied by both *sql.DB (preview) and *sql.Tx

Removed from `internal/bot/handlers/jobs.go:107`.

~~~~text
hyperspeedQuerier is satisfied by both *sql.DB (preview) and *sql.Tx
(execution, where the row must be locked FOR UPDATE) - findHyperSpeedTarget
runs the same two candidate queries either way and returns whichever
resolves soonest.
~~~~


## HandleHyperSpeed (/newjobhyperspeed) shaves time off your earliest

Removed from `internal/bot/handlers/jobs.go:157`.

~~~~text
HandleHyperSpeed (/newjobhyperspeed) shaves time off your earliest
active raid's remaining travel, matching the SpaceHunt tip about
launching HyperSpeed before departing a raid.
HandleHyperSpeed (/newjobhyperspeed) shows a Confirm/Cancel preview
before spending Electricity to halve your nearest active mission's
remaining time - 2026-08-05 fix, same rationale as HandleTeleport.
Since 2026-08-06, "nearest active mission" spans both raids and
returning scout parties - see findHyperSpeedTarget's doc comment.
See doHyperSpeed for the execution core.
~~~~


## HandleHyperSpeedConfirmCallback fires when a player taps "✅

Removed from `internal/bot/handlers/jobs.go:192`.

~~~~text
HandleHyperSpeedConfirmCallback fires when a player taps "✅
Confirm" on the HyperSpeed card. Re-derives the caller's encampment
server-side and calls doHyperSpeed, which re-validates funds and the
target mission fresh.
~~~~


## doHyperSpeed is the testable core of HandleHyperSpeedConfirmCallback -

Removed from `internal/bot/handlers/jobs.go:220`.

~~~~text
doHyperSpeed is the testable core of HandleHyperSpeedConfirmCallback -
unchanged logic from the pre-2026-08-05 HandleHyperSpeed, just moved
behind the new confirm callback and returning its result instead of
sending it directly. Since 2026-08-06 it picks the nearest-resolving
target across both raids and returning scout missions (see
findHyperSpeedTarget) and updates whichever table that target came
from.
~~~~


## HandleExtendPlanet (/newjobextendplanet) shows a Confirm/Cancel

Removed from `internal/bot/handlers/jobs.go:275`.

~~~~text
HandleExtendPlanet (/newjobextendplanet) shows a Confirm/Cancel
preview before permanently increasing storage capacity - 2026-08-05
fix, same rationale as HandleTeleport. A real, growing investment
rather than a one-time bonus, and cost grows every level, so
confirming first matters more here as a player progresses. See
doExtendPlanet for the execution core.
~~~~


## doExtendPlanet is the testable core of HandleExtendPlanetConfirmCallback -

Removed from `internal/bot/handlers/jobs.go:331`.

~~~~text
doExtendPlanet is the testable core of HandleExtendPlanetConfirmCallback -
unchanged logic from the pre-2026-08-05 HandleExtendPlanet, just moved
behind the new confirm callback.
~~~~


## teleportCooldown and teleportCostFraction: 2026-08-05 rebalance, by

Removed from `internal/bot/handlers/jobs.go:367`.

~~~~text
teleportCooldown and teleportCostFraction: 2026-08-05 rebalance, by
explicit project-owner direction ("increase the price for Teleporting
... to almost unaffordable, make it super costly, make players see
why"). Teleport used to cost a flat 1000 Electricity - negligible
against typical late-game balances (a live report showed a player
sitting on 7M+ Electricity, i.e. the "cost" was ~0.014% of holdings
and functionally free). Now priced the same way as Ghost Protocol
below - a FRACTION of current holdings across five resources, so it
scales with wealth instead of going stale as the economy grows - but
at a third of Ghost Protocol's severity, preserving the intentional
tier gap between "frequent utility hop" (24h cooldown, relocate
only) and "rare emergency measure" (90-day cooldown, relocate AND
erase every intel lock on this base).
~~~~


## HandleTeleport (/newjobteleport) shows a cost-preview Confirm/Cancel

Removed from `internal/bot/handlers/jobs.go:385`.

~~~~text
HandleTeleport (/newjobteleport) shows a cost-preview Confirm/Cancel
card rather than relocating immediately - 2026-08-05 fix, by explicit
project-owner direction ("let there be confirm buttons for all jobs
... in a situation whereby you mistakenly pressed teleport, it will
just automatically go"). Nothing is charged or moved until
HandleTeleportConfirmCallback fires. See doTeleport for the actual
execution core.
~~~~


## HandleTeleportConfirmCallback fires when a player taps "✅ Confirm

Removed from `internal/bot/handlers/jobs.go:429`.

~~~~text
HandleTeleportConfirmCallback fires when a player taps "✅ Confirm
Teleport" on the card HandleTeleport rendered. Re-derives the
caller's encampment server-side (never trusts anything from
callback_data for this) and re-validates the cooldown fresh -
doTeleport re-checks it independently of HandleTeleport's earlier
check, since time may have passed (or a second Teleport already
happened) between the prompt being shown and Confirm being tapped.
~~~~


## doTeleport is the testable core of HandleTeleportConfirmCallback,

Removed from `internal/bot/handlers/jobs.go:462`.

~~~~text
doTeleport is the testable core of HandleTeleportConfirmCallback,
following the same pattern as doGhostProtocol below: no
telebot.Context dependency, so it can be exercised directly against a
real database in tests. Returns the message to send and, on failure,
a non-nil error (in which case the message is plain-text, not HTML).
~~~~


## ghostProtocolCooldown and ghostProtocolCostFraction are deliberately

Removed from `internal/bot/handlers/jobs.go:512`.

~~~~text
ghostProtocolCooldown and ghostProtocolCostFraction are deliberately
severe compared to /newjobteleport's cheap, frequent utility cost -
per the project owner's own words ("very very very costly," "once in
3 months"). Cost is a fraction of *current* holdings rather than a
fixed absolute number, so it stays meaningfully painful regardless of
where the live economy's typical balances land - there's no historical
usage data yet to calibrate a fixed number against (see
AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 3.4 and its "Open
questions" item 1). Treat both as tunable constants to revisit after a
few weeks of real usage, same spirit as aidecisions.go's
aiFairnessNormalBandAbove/aiOverdueRaidThreshold constants.
~~~~


## HandleGhostProtocol (/ghostprotocol) shows a cost-preview

Removed from `internal/bot/handlers/jobs.go:528`.

~~~~text
HandleGhostProtocol (/ghostprotocol) shows a cost-preview
Confirm/Cancel card rather than executing immediately - 2026-08-05
fix, same rationale as HandleTeleport above, arguably even more
important here given how much more severe and rarer (90-day
cooldown) this action is. Nothing is charged, relocated, or erased
until HandleGhostProtocolConfirmCallback fires. See doGhostProtocol
for the actual execution core (unchanged).

This is a separate, far more severe action than /newjobteleport -
see AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 3.4 for why
the existing cheap/frequent teleport was deliberately NOT
repurposed for this. In addition to relocating (reusing
/newjobteleport's random-coordinate logic), this deletes every
known_locations row where this encampment is the target - every
scout/attacker who'd locked this base's position loses that lock
and must rediscover it from scratch. encampment_discoveries (the
permanent "have you ever heard of this entity" relationship) is
untouched - only the coordinate lock resets.
~~~~


## HandleGhostProtocolConfirmCallback fires when a player taps "✅

Removed from `internal/bot/handlers/jobs.go:581`.

~~~~text
HandleGhostProtocolConfirmCallback fires when a player taps "✅
Confirm Ghost Protocol" on the card HandleGhostProtocol rendered.
Re-derives the caller's encampment server-side and calls the
unchanged doGhostProtocol core, which re-validates the cooldown
fresh - see HandleTeleportConfirmCallback's doc comment for why that
re-check matters even though HandleGhostProtocol already checked it
once when the card was shown.
~~~~


## doGhostProtocol is the testable core of HandleGhostProtocolConfirmCallback,

Removed from `internal/bot/handlers/jobs.go:615`.

~~~~text
doGhostProtocol is the testable core of HandleGhostProtocolConfirmCallback,
following the same pattern as admin.go's doSetTaxRate: no telebot.Context
dependency, so it can be exercised directly against a real database in
tests. Returns the message to send and, on failure, a non-nil error
(in which case the message is a plain-text failure notice, not HTML).
~~~~


## doOrbitalManeuver is the testable core of

Removed from `internal/bot/handlers/jobs.go:713`.

~~~~text
doOrbitalManeuver is the testable core of
HandleOrbitalManeuverConfirmCallback - unchanged logic from the
pre-2026-08-05 HandleOrbitalManeuver, just moved behind the new
confirm callback.
~~~~


## HandleRepairUnits (/newjobrepairunits) shows a Confirm/Cancel preview

Removed from `internal/bot/handlers/jobs.go:748`.

~~~~text
HandleRepairUnits (/newjobrepairunits) shows a Confirm/Cancel preview
before field-repairing a small batch of Soldiers for a Scrap cost -
2026-08-05 fix, same rationale as HandleTeleport. See doRepairUnits
for the execution core.
~~~~


## doRepairUnits is the testable core of HandleRepairUnitsConfirmCallback -

Removed from `internal/bot/handlers/jobs.go:795`.

~~~~text
doRepairUnits is the testable core of HandleRepairUnitsConfirmCallback -
unchanged logic from the pre-2026-08-05 HandleRepairUnits, just moved
behind the new confirm callback.
~~~~


## HandleRepairBuildings (/newjobrepairbuildings) shows a Confirm/Cancel

Removed from `internal/bot/handlers/jobs.go:828`.

~~~~text
HandleRepairBuildings (/newjobrepairbuildings) shows a Confirm/Cancel
preview before spending Scrap to rush any in-progress building
upgrade - 2026-08-05 fix, same rationale as HandleTeleport. See
doRepairBuildings for the execution core.
~~~~


## doRepairBuildings is the testable core of

Removed from `internal/bot/handlers/jobs.go:881`.

~~~~text
doRepairBuildings is the testable core of
HandleRepairBuildingsConfirmCallback - unchanged logic from the
pre-2026-08-05 HandleRepairBuildings, just moved behind the new
confirm callback.
~~~~

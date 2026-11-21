# ADR 0147: Decisions from `internal/engine/tick/factiondefeat.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/factiondefeat.go` and moved here so the source no longer carries them. Each entry is the original comment.

## RARE_WORLD_FEATURES_PLAN.md Phase 3: Legacy Epitaphs.

Removed from `internal/engine/tick/factiondefeat.go:14`.

~~~~text
RARE_WORLD_FEATURES_PLAN.md Phase 3: Legacy Epitaphs.

The game's first permanent-defeat state, deliberately scoped to AI
factions only (see the plan doc's design principle 3 - real players
are never wiped by anything in this feature set). This file
deliberately does NOT touch resolveRaidCombats (engine.go's large,
well-tested combat resolution function) at all - checkForFactionDefeat
is a brand new, independent, read-then-conditionally-write tick phase
that runs AFTER combat resolution each tick and asks a simple
question per AI faction: is this faction now at (near-)zero
resources AND (near-)zero garrison? If yes, and it wasn't already
marked defeated, mark it now. See the plan doc's section 3.1 for the
full "why this is safe to add" rationale.
~~~~


## factionDefeatResourceThreshold reuses engine.go's own

Removed from `internal/engine/tick/factiondefeat.go:28`.

~~~~text
factionDefeatResourceThreshold reuses engine.go's own
economicCollapseWarningThreshold (50.0) rather than defining a
second, potentially-drifting constant with the same intended
meaning - "critically depleted" and "totally defeated" should stay
in lockstep if that number is ever tuned.
~~~~


## epitaphTemplates are the flavor templates a newly-defeated faction's

Removed from `internal/engine/tick/factiondefeat.go:35`.

~~~~text
epitaphTemplates are the flavor templates a newly-defeated faction's
epitaph is drawn from. %s is the faction's own name; %s (second) is
the finishing attacker's name, or "the wasteland itself" when no
raid history exists (see epitaphAttackerFlavor below) - e.g. a
faction that starved out from a weather/tax pass instead of combat.
~~~~


## checkForFactionDefeat is the "faction_defeat_check" tick phase,

Removed from `internal/engine/tick/factiondefeat.go:48`.

~~~~text
checkForFactionDefeat is the "faction_defeat_check" tick phase,
running after raid_combat each tick (see engine.go's phase list).
For every AI faction not already marked defeated: total resources
(scrap+metal+crystal+dollars) at or below
factionDefeatResourceThreshold AND garrison (soldiers+mechs) at or
below zero, BOTH conditions - not just resources, so a faction that's
merely poor but still garrisoned (or vice versa) isn't prematurely
retired.
~~~~


## Conqueror bonus: a small, one-time "you get to remember doing

Removed from `internal/engine/tick/factiondefeat.go:106`.

~~~~text
Conqueror bonus: a small, one-time "you get to remember doing
this" cash payoff to whichever real player delivered the
final blow - notified directly, not just via the regional
broadcast everyone else gets.
~~~~


## mostRecentAttacker looks up the most recent completed raid against

Removed from `internal/engine/tick/factiondefeat.go:122`.

~~~~text
mostRecentAttacker looks up the most recent completed raid against
defenderID to credit as the "final blow" in that faction's epitaph.
Returns the attacker's display name, whether that attacker is a real
player (vs. another AI faction - both can finish off a faction, but
only a real player gets the conqueror bonus below), and the
attacker's Telegram user_id (0 if not a real player / not found).
Falls back to ("", false, 0) - which checkForFactionDefeat renders as
"the wasteland itself" - when there's no raid history at all, e.g. a
faction that starved out from a weather/tax pass instead of combat.
~~~~


## conquerorBonusDollars is the flat one-time cash payoff a real player

Removed from `internal/engine/tick/factiondefeat.go:147`.

~~~~text
conquerorBonusDollars is the flat one-time cash payoff a real player
receives for delivering the finishing blow to an AI faction - scaled
like Phase 1's relic reward (a genuine, felt windfall) rather than a
token amount, since finishing off a faction permanently is meant to
feel like a real, memorable accomplishment.
~~~~


## payConquerorBonus credits attackerUserID's own encampment with

Removed from `internal/engine/tick/factiondefeat.go:154`.

~~~~text
payConquerorBonus credits attackerUserID's own encampment with
conquerorBonusDollars (storage-cap clamped, same as every other
resource-gain path) and sends them a direct notification distinct
from the regional "a faction has fallen" broadcast everyone else
gets.
~~~~


## Attacker's encampment no longer exists (account deleted since

Removed from `internal/engine/tick/factiondefeat.go:162`.

~~~~text
Attacker's encampment no longer exists (account deleted since
the raid) - nothing to pay, not an error worth failing the
whole tick phase over.
~~~~

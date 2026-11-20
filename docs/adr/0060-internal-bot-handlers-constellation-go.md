# ADR 0060: Decisions from `internal/bot/handlers/constellation.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/constellation.go` and moved here so the source no longer carries them. Each entry is the original comment.

## RARE_WORLD_FEATURES_PLAN.md Phase 5: Constellation Alliances.

Removed from `internal/bot/handlers/constellation.go:16`.

~~~~text
RARE_WORLD_FEATURES_PLAN.md Phase 5: Constellation Alliances.

A rare, temporary, N-ary cross-clan structure sitting above the
existing pairwise clan_diplomacy pacts (diplomacy.go). See the plan
doc's section 5.1 for why this needed its own header+membership
table pair (constellations/constellation_members) rather than
reusing clan_diplomacy's pairwise shape. The tick.Engine side
(dividend payout, expiry) lives in internal/engine/tick/constellations.go.
~~~~


## constellationDurationDays is how long an activated Constellation

Removed from `internal/bot/handlers/constellation.go:25`.

~~~~text
constellationDurationDays is how long an activated Constellation
lasts before automatically dissolving - long enough to feel like a
real commitment, short enough to stay a special, opt-in moment
rather than a silent permanent power bloc (see the plan doc's design
principle 2 - "rare means rare").
~~~~


## constellationMaxMembers is the largest a Constellation can be

Removed from `internal/bot/handlers/constellation.go:32`.

~~~~text
constellationMaxMembers is the largest a Constellation can be
(founder + up to 2 invited clans) - kept small so "which clans are
in my Constellation" stays an easy, memorable fact rather than
approaching a second clan-membership system.
~~~~


## getMyClanForConstellation mirrors diplomacy.go's

Removed from `internal/bot/handlers/constellation.go:46`.

~~~~text
getMyClanForConstellation mirrors diplomacy.go's
getMyClanForDiplomacy exactly (can't call that one directly - it's a
method on a different receiver type).
~~~~


## HandleConstellationForm (/constellation_form Name, ClanA, ClanB)

Removed from `internal/bot/handlers/constellation.go:62`.

~~~~text
HandleConstellationForm (/constellation_form Name, ClanA, ClanB)
lets a Clan Leader propose forming a Constellation with 1-2 other
named Clans (comma-separated so both the Constellation's own name
and Clan names can contain spaces - see plan doc 5.3). Creates the
constellations row (status 'forming'), an 'accepted' member row for
the founder's own Clan, and one 'pending' row per invited Clan.
~~~~


## HandleConstellationRespondCallback fires when an invited Clan

Removed from `internal/bot/handlers/constellation.go:268`.

~~~~text
HandleConstellationRespondCallback fires when an invited Clan
Leader taps Accept/Reject on HandleConstellationPanel. A single
reject dissolves the whole forming Constellation (see plan doc 5.3
point 4) rather than leaving it stuck at partial acceptance forever.
Once every invited member has accepted, the Constellation activates:
status -> 'active', activated_at/expires_at set, and a broadcast
announces it to every member Clan's players.
~~~~


## broadcastConstellationActivated notifies every player in every member

Removed from `internal/bot/handlers/constellation.go:373`.

~~~~text
broadcastConstellationActivated notifies every player in every member
Clan directly (via notifications.Queue, not a region-scoped
broadcast - member Clans' players may be spread across regions) that
their Constellation has activated.
~~~~

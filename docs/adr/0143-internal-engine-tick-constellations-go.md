# ADR 0143: Decisions from `internal/engine/tick/constellations.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/constellations.go` and moved here so the source no longer carries them. Each entry is the original comment.

## RARE_WORLD_FEATURES_PLAN.md Phase 5: Constellation Alliances.

Removed from `internal/engine/tick/constellations.go:12`.

~~~~text
RARE_WORLD_FEATURES_PLAN.md Phase 5: Constellation Alliances.

The handler side (formation, accept/reject) lives in
internal/bot/handlers/constellation.go. This file is the tick.Engine
side: the joint dividend payout and automatic expiry for active
Constellations.
~~~~


## constellationDividendDollars is the flat "Alignment Dividend" every

Removed from `internal/engine/tick/constellations.go:19`.

~~~~text
constellationDividendDollars is the flat "Alignment Dividend" every
member Clan's encampments each receive once per day while their
Constellation is active - deliberately a flat administrative bonus
rather than a percentage-of-resource-generation change, so this
phase never has to touch the resource-tick formulas Phase 2's Bloom
bonus already extended (see the plan doc's section 5.4 - keeps the
two features' code paths fully independent).
~~~~


## constellationDividend is the "constellation_dividend" tick phase.

Removed from `internal/engine/tick/constellations.go:28`.

~~~~text
constellationDividend is the "constellation_dividend" tick phase.
For every active Constellation whose last_dividend_at is more than
24 hours ago (or never set), every member Clan's encampments each
receive constellationDividendDollars, storage-cap clamped like every
other resource-gain path, with a direct notification per recipient.
Tracked via last_dividend_at rather than assuming the tick cadence
lines up with a calendar day - see plan doc 5.4.
~~~~


## constellationExpire is the "constellation_expire" tick phase -

Removed from `internal/engine/tick/constellations.go:108`.

~~~~text
constellationExpire is the "constellation_expire" tick phase -
active Constellations past their expires_at become 'dissolved' with
a "the Constellation has ended" broadcast to every member Clan,
matching every other expiry phase in this feature set (Phase
1.4/2/4.5's precedent).
~~~~

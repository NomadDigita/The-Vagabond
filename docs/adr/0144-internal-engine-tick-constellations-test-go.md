# ADR 0144: Decisions from `internal/engine/tick/constellations_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/constellations_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## seedActiveConstellation creates an 'active' Constellation with two

Removed from `internal/engine/tick/constellations_test.go:10`.

~~~~text
seedActiveConstellation creates an 'active' Constellation with two
member Clans (founder + one invited, both 'accepted'), each led by a
distinct seeded encampment - returning the constellation id and both
member encampment ids.
~~~~


## TestConstellationDividend_PaysEveryMemberClanOnce is the direct test

Removed from `internal/engine/tick/constellations_test.go:53`.

~~~~text
TestConstellationDividend_PaysEveryMemberClanOnce is the direct test
for RARE_WORLD_FEATURES_PLAN.md Phase 5.4: an active Constellation
due a dividend (last_dividend_at NULL) should pay
constellationDividendDollars to every member Clan's encampments and
set last_dividend_at.
~~~~


## expired 1 hour ago

Removed from `internal/engine/tick/constellations_test.go:121`.

~~~~text
expired 1 hour ago
~~~~

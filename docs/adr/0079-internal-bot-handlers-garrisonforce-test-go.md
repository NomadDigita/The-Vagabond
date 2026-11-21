# ADR 0079: Decisions from `internal/bot/handlers/garrisonforce_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/garrisonforce_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestLoadBaseGarrisonForce_AIFactionUsesSyntheticReserveFromLivePool

Removed from `internal/bot/handlers/garrisonforce_test.go:34`.

~~~~text
TestLoadBaseGarrisonForce_AIFactionUsesSyntheticReserveFromLivePool
verifies AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 1.4's fix:
an AI faction's garrisoned_soldiers/garrisoned_mechs is never set by
anyone, so reading it directly (the old behavior) always returned a
free-win zero garrison. It should instead derive a reserve from the
faction's live soldiers/mechs pool.
~~~~


## TestLoadBaseGarrisonForce_HumanReadsGarrisonedColumnUnchanged confirms

Removed from `internal/bot/handlers/garrisonforce_test.go:65`.

~~~~text
TestLoadBaseGarrisonForce_HumanReadsGarrisonedColumnUnchanged confirms
the human path is untouched: it still reads the manual, explicit
garrisoned_soldiers/garrisoned_mechs column, not the live pool.
~~~~


## TestApplyBaseGarrisonCasualties_AIFactionDebitsLivePoolNotWholeReserve

Removed from `internal/bot/handlers/garrisonforce_test.go:93`.

~~~~text
TestApplyBaseGarrisonCasualties_AIFactionDebitsLivePoolNotWholeReserve
verifies the write-back side: only the casualties are subtracted from
the AI faction's live pool - the 80% that was never committed to this
skirmish must be untouched.
~~~~

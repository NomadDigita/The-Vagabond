# ADR 0061: Decisions from `internal/bot/handlers/constellation_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/constellation_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## seedClanWithLeader creates a Clan led by telegramID (also seeding

Removed from `internal/bot/handlers/constellation_test.go:9`.

~~~~text
seedClanWithLeader creates a Clan led by telegramID (also seeding
that user's own encampment via seedExchangeCamp, so tests exercising
the dividend payout later have somewhere for cash to land), returning
the clan's id.
~~~~


## Directly exercise the accept transition's SQL (the same

Removed from `internal/bot/handlers/constellation_test.go:100`.

~~~~text
Directly exercise the accept transition's SQL (the same
statements HandleConstellationRespondCallback runs) rather than
going through the telebot.Context-dependent handler, matching
this package's doX-core testing convention elsewhere.
~~~~

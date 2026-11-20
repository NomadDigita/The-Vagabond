# ADR 0097: Decisions from `internal/bot/handlers/ranking.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/ranking.go` and moved here so the source no longer carries them. Each entry is the original comment.

## displayName returns the escaped name for leaderboard display, prefixed

Removed from `internal/bot/handlers/ranking.go:37`.

~~~~text
displayName returns the escaped name for leaderboard display, prefixed
with 🤖 for AI factions so parity in the score doesn't come at the cost
of honest labeling in the text (AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md
section 1.2).
~~~~


## rankTable renders a medal-ranked list as a native Rich Message

Removed from `internal/bot/handlers/ranking.go:48`.

~~~~text
rankTable renders a medal-ranked list as a native Rich Message
<table> (Bot API 10.1) - the same real-columns approach exchange.go's
market panel established for its listings board, applied here so the
leaderboard's three sections (which are exactly this "rank, name,
score" shape) get the same aligned, scannable layout instead of the
emoji+em-dash card lines every panel used before Rich Messages were
available. header names the score column (e.g. "Score", "Rating");
rows must already be display-ready strings (medal, name, formatted
score) - this function only lays them out, it doesn't format them.
~~~~


## ── Top Players ────────────────────────────────────────────────

Removed from `internal/bot/handlers/ranking.go:76`.

~~~~text
── Top Players ────────────────────────────────────────────────
AI factions are ranked identically to humans here (full parity -
they earned the score the same way), but are visually marked with
a 🤖 prefix so the distinction stays honest in the display line.
See AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 1.2.
~~~~


## RARE_WORLD_FEATURES_PLAN.md Phase 5.6: a small

Removed from `internal/bot/handlers/ranking.go:158`.

~~~~text
RARE_WORLD_FEATURES_PLAN.md Phase 5.6: a small
banner-emoji suffix for any Clan currently in an
ACTIVE Constellation - a visible, "arrived" flourish
on the leaderboard itself, not just on /constellation.
~~~~

# ADR 0171: Decisions from `internal/game/devconsole/console.go`

Status: Accepted

These decisions were written as comments in `internal/game/devconsole/console.go` and moved here so the source no longer carries them. Each entry is the original comment.

## MemoryScope namespaces this feature's conversational history inside

Removed from `internal/game/devconsole/console.go:13`.

~~~~text
MemoryScope namespaces this feature's conversational history inside
ai_memory, distinct from every other Phase B+ scope.
~~~~


## newPlayerListCap bounds how many individual new-player rows are

Removed from `internal/game/devconsole/console.go:17`.

~~~~text
newPlayerListCap bounds how many individual new-player rows are
listed in the prompt (Snapshot.NewPlayerCount still carries the true
total even if the list itself is capped) — a signup surge shouldn't
blow up the prompt size or the token bill for a report that's meant
to be a summary, not a full export.
~~~~


## topPlayerCount mirrors the "Top Skilled"/ranking panel's usual

Removed from `internal/game/devconsole/console.go:24`.

~~~~text
topPlayerCount mirrors the "Top Skilled"/ranking panel's usual
depth for a quick read, not the full 15 HandleRankingPanel shows —
this report is a summary, not a leaderboard reprint.
~~~~


## Console is the Phase J entry point. Unlike every other Phase B-I

Removed from `internal/game/devconsole/console.go:34`.

~~~~text
Console is the Phase J entry point. Unlike every other Phase B-I
package, this one has no per-player ErrNoEncampment-style guard —
it reports on the whole game, not one player's own state. Admin
gating happens in the handler layer (matching
internal/bot/handlers/admin.go's existing IsAdmin pattern), not
here — this package doesn't know about Telegram admin IDs at all.
~~~~


## windowSince returns the cutoff time for a "last N days" window,

Removed from `internal/game/devconsole/console.go:49`.

~~~~text
windowSince returns the cutoff time for a "last N days" window,
defaulting to 7 if windowDays isn't positive. Shared by
BuildSnapshot and RunIntent so "last N days" always means the same
thing everywhere in this package.
~~~~


## buildNewPlayers reads the newPlayerListCap most recently registered

Removed from `internal/game/devconsole/console.go:103`.

~~~~text
buildNewPlayers reads the newPlayerListCap most recently registered
users in the window, LEFT JOINed to their encampment (if any) so a
player who registered but never finished onboarding still shows up
— with an honest "no outpost yet" home continent rather than being
silently dropped or guessed at.
~~~~


## buildTopPlayers reuses the exact same scoring.ScoreExpr formula the

Removed from `internal/game/devconsole/console.go:135`.

~~~~text
buildTopPlayers reuses the exact same scoring.ScoreExpr formula the
player-facing Global Ranking panel already uses, so this report's
"top players" never disagrees with what players themselves see on
/ranking.
~~~~


## Recommend produces a fresh AI activity-report summary for the last

Removed from `internal/game/devconsole/console.go:185`.

~~~~text
Recommend produces a fresh AI activity-report summary for the last
windowDays days. callerUserID is the requesting admin's Telegram ID,
used only for ai_memory attribution/cost tracking the same way
every other Phase B-I package attributes its calls — this method
itself does not check whether callerUserID is actually an admin
(see Console's doc comment for why that's the handler's job).

Read-only: nothing in this method changes any player, setting, or
game data — it only reads and reports.
~~~~

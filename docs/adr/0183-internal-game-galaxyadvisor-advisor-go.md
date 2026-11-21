# ADR 0183: Decisions from `internal/game/galaxyadvisor/advisor.go`

Status: Accepted

These decisions were written as comments in `internal/game/galaxyadvisor/advisor.go` and moved here so the source no longer carries them. Each entry is the original comment.

## MemoryScope namespaces this feature's conversational history inside

Removed from `internal/game/galaxyadvisor/advisor.go:13`.

~~~~text
MemoryScope namespaces this feature's conversational history inside
ai_memory, distinct from every other Phase B+ scope.
~~~~


## recentNewsLimit mirrors the 5-headline limit

Removed from `internal/game/galaxyadvisor/advisor.go:17`.

~~~~text
recentNewsLimit mirrors the 5-headline limit
internal/bot/handlers/world.go's HandleWorldFeed already uses for
its own news panel, so this advisor sees exactly what the player
would see on that screen — not a different, inconsistent slice of
history.
~~~~


## BuildSnapshot loads the calling player's home continent, every

Removed from `internal/game/galaxyadvisor/advisor.go:38`.

~~~~text
BuildSnapshot loads the calling player's home continent, every
continent's current world event (via the shared
internal/engine/world helpers the tick engine and world-feed panel
already use), and the most recent sector news headlines.
~~~~


## Recommend produces a fresh AI briefing for the given player. It

Removed from `internal/game/galaxyadvisor/advisor.go:92`.

~~~~text
Recommend produces a fresh AI briefing for the given player. It
stores both turns in ai_memory under MemoryScope.

Read-only: nothing in this method changes any world event, marches
any fleet, or queues any construction — it only reads and advises.
~~~~

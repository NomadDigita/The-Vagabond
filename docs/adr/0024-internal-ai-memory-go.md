# ADR 0024: Decisions from `internal/ai/memory.go`

Status: Accepted

These decisions were written as comments in `internal/ai/memory.go` and moved here so the source no longer carries them. Each entry is the original comment.

## MemoryStore persists short conversational history per (user, scope)

Removed from `internal/ai/memory.go:8`.

~~~~text
MemoryStore persists short conversational history per (user, scope)
so a follow-up call to the same AI feature has continuity without
the caller re-assembling context by hand. "Scope" separates
unrelated conversations for the same player, e.g. "fleet_commander"
vs "npc:trader_rell", so memory never bleeds across features or NPCs.

This is deliberately a thin, generic layer: Phase I (NPC
Intelligence) and Phase G (Guild Assistant) will likely want richer,
summarized long-term memory. That should be built as a decorator
around this interface (e.g. a SummarizingMemoryStore that
periodically compacts old rows via a Provider call), not by changing
this interface's contract.
~~~~

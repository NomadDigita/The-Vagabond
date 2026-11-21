# ADR 0091: Decisions from `internal/bot/handlers/npc_intel.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/npc_intel.go` and moved here so the source no longer carries them. Each entry is the original comment.

## NPCIntelHandler exposes Phase I (AI NPC Intelligence). New command

Removed from `internal/bot/handlers/npc_intel.go:12`.

~~~~text
NPCIntelHandler exposes Phase I (AI NPC Intelligence). New command
(/npc_intel) — distinct from /recon_ai (the static composition
numbers, in combat.go) and /fleet_commander (the attack/no-attack
call, Phase C) — this gives a composition-specific tactical read of
the Rogue Drone Nest against the player's own fleet.
~~~~


## buildNPCIntelKeyboard renders the inline keyboard attached to every

Removed from `internal/bot/handlers/npc_intel.go:25`.

~~~~text
buildNPCIntelKeyboard renders the inline keyboard attached to every
NPC Intelligence report: a single refresh button. Like Battle
Analyst, Guild Assistant, and Galaxy Advisor, there's no goal
selection — there's only one Nest (scaled to the player's level) to
read.
~~~~


## renderNPCIntelReport runs a fresh tactical read and returns the

Removed from `internal/bot/handlers/npc_intel.go:37`.

~~~~text
renderNPCIntelReport runs a fresh tactical read and returns the
formatted text plus its attached keyboard, shared by the
/npc_intel command and its refresh callback so the two can never
drift apart.
~~~~


## ── /npc_intel ───────────────────────────────────────────────────────

Removed from `internal/bot/handlers/npc_intel.go:49`.

~~~~text
── /npc_intel ───────────────────────────────────────────────────────

Gives a tactical, composition-specific read of the Rogue Drone Nest
(scaled to the player's level) against the player's own current
mobile fleet. Read-only: never launches a raid or moves any unit.
~~~~


## ── callback: npc_intel_refresh ──────────────────────────────────────

Removed from `internal/bot/handlers/npc_intel.go:73`.

~~~~text
── callback: npc_intel_refresh ──────────────────────────────────────

Re-runs the tactical read on demand (a real new AI Foundation call,
subject to the usual cost/cache/budget rules) and posts a fresh
report.
~~~~

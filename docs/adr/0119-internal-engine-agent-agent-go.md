# ADR 0119: Decisions from `internal/engine/agent/agent.go`

Status: Accepted

These decisions were written as comments in `internal/engine/agent/agent.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Calculate fuel deductions incorporating Science Tech and Biological Mutations

Removed from `internal/engine/agent/agent.go:83`.

~~~~text
Calculate fuel deductions incorporating Science Tech and Biological Mutations
Base upkeep and floor tuned 2026-07-26 per Asiwaju's balance
call: automation should cost real electricity even at max tech
investment, not trail off to a token fee.
~~~~


## Floor: even maxed tech still pays 40% of base upkeep

Removed from `internal/engine/agent/agent.go:88`.

~~~~text
Floor: even maxed tech still pays 40% of base upkeep
~~~~


## Autopilot Industrial mode (Metal, Hydrogen). Former separate

Removed from `internal/engine/agent/agent.go:123`.

~~~~text
Autopilot Industrial mode (Metal, Hydrogen). Former separate
Iron/Oil gains are now folded directly into Metal.
Same Surplus Preservation cap rule as `collector`: no
further passive gain once a resource is at/above cap.
~~~~


## Autopilot Precious mode (Crystal, Dollars, Neuro Cores).

Removed from `internal/engine/agent/agent.go:142`.

~~~~text
Autopilot Precious mode (Crystal, Dollars, Neuro Cores).
Former separate Silver/Gold/Diamond gains are now folded
directly into Crystal. Same cap rule as the other
collector modes.
~~~~


## Same "Module levels cannot exceed your Outpost Core level"

Removed from `internal/engine/agent/agent.go:169`.

~~~~text
Same "Module levels cannot exceed your Outpost Core level"
rule enforced on the manual upgrade path (camp.go
HandleUpgradeCallback: `currentLvl >= campLvl` is blocked).
Only consider modules still below that cap so the agent
can't auto-build past what a manual upgrade would allow.
~~~~


## Either no modules exist yet, or every module is

Removed from `internal/engine/agent/agent.go:187`.

~~~~text
Either no modules exist yet, or every module is
already capped at the Outpost Core level. Seed a
starter tent only if nothing exists at all.
~~~~


## Same Hangar capacity rule enforced on the manual Recruit

Removed from `internal/engine/agent/agent.go:233`.

~~~~text
Same Hangar capacity rule enforced on the manual Recruit
Soldier path (factory.go HandleCraftItemCallback):
maxCapacity = 500 + hangarLvl*200 (10x pass on the original
50 + hangarLvl*20 - level 1 now holds 700 instead of 70,
level 2 holds 900 instead of 90, etc.), blocked once
totalUnits hits that cap. The agent must not be able to
auto-recruit past what a manual recruit would allow.
~~~~

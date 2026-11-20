# ADR 0148: Decisions from `internal/engine/tick/factiondefeat_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/factiondefeat_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## seedEncampment already leaves resources/workshop_inventory at

Removed from `internal/engine/tick/factiondefeat_test.go:21`.

~~~~text
seedEncampment already leaves resources/workshop_inventory at
their zero defaults - nothing further to seed for this case.
~~~~


## conquerorBonusDollars (5000) + a one-time 2000 "conqueror"

Removed from `internal/engine/tick/factiondefeat_test.go:157`.

~~~~text
conquerorBonusDollars (5000) + a one-time 2000 "conqueror"
achievement bonus (NEXT_GENERATION_FEATURES_PLAN.md Phase 1,
threshold 1 on progression.TriggerFactionDefeated, unlocked by
this same finishing blow via checkForFactionDefeat's
progression.RecordEvent call) = 7000.
~~~~


## TestFactionDefeat_ExcludedFromAIPools confirms a defeated faction no

Removed from `internal/engine/tick/factiondefeat_test.go:190`.

~~~~text
TestFactionDefeat_ExcludedFromAIPools confirms a defeated faction no
longer appears in the AI-decision, AI-jobs, or AI-growth pools (Phase
3.4) - it stops scouting, raiding, growing, and running jobs, but
its row persists (permanent ruin, discoverable per 3.5).
~~~~


## The row itself must still exist (permanent ruin).

Removed from `internal/engine/tick/factiondefeat_test.go:209`.

~~~~text
The row itself must still exist (permanent ruin).
~~~~

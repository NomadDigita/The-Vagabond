# ADR 0203: Decisions from `internal/game/prestige/prestige_test.go`

Status: Accepted

These decisions were written as comments in `internal/game/prestige/prestige_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## below campLevelRequirement (25)

Removed from `internal/game/prestige/prestige_test.go:69`.

~~~~text
below campLevelRequirement (25)
~~~~


## TestDoPrestige_RejectsIneligibleCamp confirms DoPrestige refuses to

Removed from `internal/game/prestige/prestige_test.go:172`.

~~~~text
TestDoPrestige_RejectsIneligibleCamp confirms DoPrestige refuses to
run (and changes nothing) for a camp that doesn't meet the
eligibility gate - the panel already hides the button, but the core
function itself must not trust that alone.
~~~~


## TestDoPrestige_StacksAcrossMultiplePrestiges confirms

Removed from `internal/game/prestige/prestige_test.go:192`.

~~~~text
TestDoPrestige_StacksAcrossMultiplePrestiges confirms
prestige_bonus_pct accumulates (not resets) across repeated
Prestige cycles - "stacking forever" is the whole point of the
mechanic.
~~~~

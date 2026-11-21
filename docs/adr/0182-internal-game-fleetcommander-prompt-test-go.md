# ADR 0182: Decisions from `internal/game/fleetcommander/prompt_test.go`

Status: Accepted

These decisions were written as comments in `internal/game/fleetcommander/prompt_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## See ADR-016 in PROJECT_MASTER_PLAN.md: a response cut off mid-object

Removed from `internal/game/fleetcommander/prompt_test.go:65`.

~~~~text
See ADR-016 in PROJECT_MASTER_PLAN.md: a response cut off mid-object
(hit MaxTokens) is now distinguished from one that never contained
JSON at all, so the player sees a more specific message.
~~~~


## See PROJECT_MASTER_PLAN.md ADR-015 / §1.8 — reproduces a real

Removed from `internal/game/fleetcommander/prompt_test.go:92`.

~~~~text
See PROJECT_MASTER_PLAN.md ADR-015 / §1.8 — reproduces a real
production failure where prose or a raw newline around/inside an
otherwise-valid JSON object made json.Unmarshal fail outright.
~~~~


## Level 20 crosses every enrichment threshold confirmed in

Removed from `internal/game/fleetcommander/prompt_test.go:148`.

~~~~text
Level 20 crosses every enrichment threshold confirmed in
content.RogueNestComposition (turrets, guardians, observers,
integrity tech, shields, and the hero-superpower threshold at
level 20), so this exercises the full enriched path, not just
the legacy Soldiers/Mechs/Drones/Jets fields.
~~~~


## The enriched data must actually reach the rendered prompt, not

Removed from `internal/game/fleetcommander/prompt_test.go:171`.

~~~~text
The enriched data must actually reach the rendered prompt, not
just live unused on the struct.
~~~~

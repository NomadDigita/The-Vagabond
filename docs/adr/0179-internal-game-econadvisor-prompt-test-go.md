# ADR 0179: Decisions from `internal/game/econadvisor/prompt_test.go`

Status: Accepted

These decisions were written as comments in `internal/game/econadvisor/prompt_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## See ADR-016 in PROJECT_MASTER_PLAN.md: a response cut off mid-object

Removed from `internal/game/econadvisor/prompt_test.go:80`.

~~~~text
See ADR-016 in PROJECT_MASTER_PLAN.md: a response cut off mid-object
(hit MaxTokens) is now distinguished from one that never contained
JSON at all, so the player sees a more specific message.
~~~~


## See PROJECT_MASTER_PLAN.md ADR-015 / §1.8 — reproduces a real

Removed from `internal/game/econadvisor/prompt_test.go:107`.

~~~~text
See PROJECT_MASTER_PLAN.md ADR-015 / §1.8 — reproduces a real
production failure where prose or a raw newline around/inside an
otherwise-valid JSON object made json.Unmarshal fail outright.
~~~~

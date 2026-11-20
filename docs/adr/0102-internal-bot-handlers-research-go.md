# ADR 0102: Decisions from `internal/bot/handlers/research.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/research.go` and moved here so the source no longer carries them. Each entry is the original comment.

## MaxResearchLevel caps every tech node. Raised from the old 5-level cap to

Removed from `internal/bot/handlers/research.go:23`.

~~~~text
MaxResearchLevel caps every tech node. Raised from the old 5-level cap to
give the tree real long-term progression depth.
~~~~


## researchNode describes one branch of the tree. dbColumn must match a real

Removed from `internal/bot/handlers/research.go:27`.

~~~~text
researchNode describes one branch of the tree. dbColumn must match a real
column on research_states.
~~~~


## used in callback data

Removed from `internal/bot/handlers/research.go:30`.

~~~~text
used in callback data
~~~~

# ADR 0134: Decisions from `internal/engine/tick/aidiplomacy_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/aidiplomacy_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestGrowAICivilizationsCanProposeAndAcceptPacts verifies both

Removed from `internal/engine/tick/aidiplomacy_test.go:10`.

~~~~text
TestGrowAICivilizationsCanProposeAndAcceptPacts verifies both
diplomacy paths: a Leader-role AI faction with a pending proposal
addressed to it eventually responds (mirrors
HandleDiplomacyRespondCallback), and, separately, a Leader-role AI
faction with no pending proposal eventually proposes a new pact to an
unrelated clan (mirrors proposePact/HandleProposeAlliance/HandleProposeNAP).
~~~~


## Zero out the independent leave/disband roll - this test needs

Removed from `internal/engine/tick/aidiplomacy_test.go:21`.

~~~~text
Zero out the independent leave/disband roll - this test needs
its tracked clan(s) to survive the full 300-tick loop below; see
aiClanLeaveChance's own doc comment in engine.go.
~~~~


## TestPickFairAIRaidTargetRespectsActivePact verifies a target whose

Removed from `internal/engine/tick/aidiplomacy_test.go:108`.

~~~~text
TestPickFairAIRaidTargetRespectsActivePact verifies a target whose
clan has an active pact with the attacking faction's clan is excluded
from raid selection - the same rule HasActivePact enforces for
human-launched raids.
~~~~


## in-band otherwise

Removed from `internal/engine/tick/aidiplomacy_test.go:126`.

~~~~text
in-band otherwise
~~~~

# ADR 0169: Decisions from `internal/game/devconsole/balance.go`

Status: Accepted

These decisions were written as comments in `internal/game/devconsole/balance.go` and moved here so the source no longer carries them. Each entry is the original comment.

## This file implements balance-suggestion tooling: real unit

Removed from `internal/game/devconsole/balance.go:1`.

~~~~text
This file implements balance-suggestion tooling: real unit
usage/outcome statistics from completed raids, with AI commentary
on what they might imply for game balance.

Honesty boundary, stated up front because it matters: these are
CORRELATIONAL statistics from real games, not a controlled
experiment. A unit showing a high "win rate when used" could mean
the unit is strong, or could just mean stronger/more experienced
players happen to favor it — the data alone can't tell those apart.
SystemPrompt instructs the model to flag this explicitly rather than
hand admins a false sense of causal certainty. This mirrors the
project's existing discipline (ADR-017/018/019) of never presenting
a proxy or a correlation as more certain than it actually is.
~~~~


## mobilizableUnitColumns are raid_forces' own mobilized-count columns

Removed from `internal/game/devconsole/balance.go:25`.

~~~~text
mobilizableUnitColumns are raid_forces' own mobilized-count columns
— a fixed, hardcoded list (never derived from user or model input),
so building a column name into SQL here carries no injection risk.
~~~~


## 0 if RaidsUsedIn is 0 — see BuildBalanceSnapshot

Removed from `internal/game/devconsole/balance.go:41`.

~~~~text
0 if RaidsUsedIn is 0 — see BuildBalanceSnapshot
~~~~


## Snapshot is the real, deterministic usage/outcome data this

Removed from `internal/game/devconsole/balance.go:66`.

~~~~text
Snapshot is the real, deterministic usage/outcome data this
pass was built from - not model output, so it's set by
RecommendBalance after parsing rather than unmarshaled from
JSON. FormatBalanceForTelegram uses it to render an actual
numeric table (real percentages) with the model's UnitNotes
shown as commentary alongside it, rather than asking the model
to reproduce numbers it was only ever supposed to reason about.
~~~~


## FormatBalanceForTelegram renders a BalanceRecommendation as a

Removed from `internal/game/devconsole/balance.go:126`.

~~~~text
FormatBalanceForTelegram renders a BalanceRecommendation as a
Telegram HTML-mode message. This was plain, unformatted text until
the 2026-08-01 UI pass below - it's the one AI Developer Console
report the original HTML polish campaign (see
BUGS_AND_INCONSISTENCIES.md waves 1-11) never reached, which is why
it still looked like a flat wall of text next to every other panel
in the game. All model-generated fields are escaped via
ai.HTMLEscape before being wrapped in a tag.

Design choice worth stating explicitly: the unit table below is
built from rec.Snapshot's real UnitUsageStat numbers, not from the
model's UnitNotes text. The model was only ever asked to comment on
the data, not to reproduce it - rendering the real percentages
ourselves and showing the model's commentary as clearly-labeled
prose alongside it keeps the ground-truth numbers and the AI's
interpretation of them visually distinct, rather than trusting an
LLM to transcribe a number correctly into a table cell.
~~~~


## no deployments this window - a real 0% win rate would be misleading here

Removed from `internal/game/devconsole/balance.go:165`.

~~~~text
no deployments this window - a real 0% win rate would be misleading here
~~~~


## BuildBalanceSnapshot computes real usage/outcome correlations for

Removed from `internal/game/devconsole/balance.go:195`.

~~~~text
BuildBalanceSnapshot computes real usage/outcome correlations for
every mobilizable unit type across completed raids in the last
windowDays days. "Apparent win" reuses the same stolen-resources
heuristic already established for the attacker side in
fleetcommander.BuildCombatHistory and battleanalyst — see those
packages' own doc comments for why it's a proxy, not an
authoritative outcome column.
~~~~


## RecommendBalance produces a fresh AI balance-commentary pass for the

Removed from `internal/game/devconsole/balance.go:238`.

~~~~text
RecommendBalance produces a fresh AI balance-commentary pass for the
last windowDays days. It stores both turns in ai_memory under its
own scope, distinct from the weekly-report and NL-query scopes.

Read-only: nothing in this method changes any unit, cost, or game
setting — it only reads and comments.
~~~~

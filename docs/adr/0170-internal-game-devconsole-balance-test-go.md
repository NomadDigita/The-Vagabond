# ADR 0170: Decisions from `internal/game/devconsole/balance_test.go`

Status: Accepted

These decisions were written as comments in `internal/game/devconsole/balance_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## This is a load-bearing safety property of the whole feature: the

Removed from `internal/game/devconsole/balance_test.go:22`.

~~~~text
This is a load-bearing safety property of the whole feature: the
model must be told explicitly not to state balance verdicts as
fact from correlational data.
~~~~


## See ADR-016: a response cut off mid-object is distinguished from one

Removed from `internal/game/devconsole/balance_test.go:107`.

~~~~text
See ADR-016: a response cut off mid-object is distinguished from one
that never contained JSON at all.
~~~~


## TestFormatBalanceForTelegram_RendersRealDataTable is the regression

Removed from `internal/game/devconsole/balance_test.go:152`.

~~~~text
TestFormatBalanceForTelegram_RendersRealDataTable is the regression
test for the 2026-08-01 UI pass: the unit-usage table must be built
from Snapshot's real numbers, not from anything the model wrote, so
a made-up or mistranscribed AI figure can never end up looking like
ground truth. It also checks the zero-usage case renders as "—"
rather than a misleading literal 0% win rate (there's no rate to
speak of if the unit was never deployed).
~~~~


## wraiths was never deployed (RaidsUsedIn: 0) - must show "—", not "0",

Removed from `internal/game/devconsole/balance_test.go:175`.

~~~~text
wraiths was never deployed (RaidsUsedIn: 0) - must show "—", not "0",
for its win rate, since a real 0% would misleadingly imply it lost
every time it was used.
~~~~


## TestFormatBalanceForTelegram_CautionUsesExpandableQuote guards the

Removed from `internal/game/devconsole/balance_test.go:183`.

~~~~text
TestFormatBalanceForTelegram_CautionUsesExpandableQuote guards the
other half of the same UI pass: per-unit AI cautions render as a
collapsible blockquote rather than dumped inline, since they can run
long and this was previously a flat, unformatted wall of text (the
one AI Developer Console report the original HTML polish campaign
never reached).
~~~~


## TestFormatBalanceForTelegram_NilSnapshotOmitsTable makes sure the

Removed from `internal/game/devconsole/balance_test.go:205`.

~~~~text
TestFormatBalanceForTelegram_NilSnapshotOmitsTable makes sure the
table section is skipped cleanly (not a nil-pointer panic) when
Snapshot wasn't set - the case every hand-built test
BalanceRecommendation above exercises, since none of them set it.
~~~~

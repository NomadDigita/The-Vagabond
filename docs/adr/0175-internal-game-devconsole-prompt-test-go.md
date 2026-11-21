# ADR 0175: Decisions from `internal/game/devconsole/prompt_test.go`

Status: Accepted

These decisions were written as comments in `internal/game/devconsole/prompt_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## See ADR-016 in PROJECT_MASTER_PLAN.md: a response cut off mid-object

Removed from `internal/game/devconsole/prompt_test.go:131`.

~~~~text
See ADR-016 in PROJECT_MASTER_PLAN.md: a response cut off mid-object
is distinguished from one that never contained JSON at all.
~~~~


## TestFormatForTelegram_NarrativesUseExpandableQuote is the regression

Removed from `internal/game/devconsole/prompt_test.go:187`.

~~~~text
TestFormatForTelegram_NarrativesUseExpandableQuote is the regression
test for the 2026-08-01 UI pass: the three free-text AI narrative
fields can legitimately run to several sentences each, so they now
render as collapsible blockquotes instead of sitting inline as one
dense wall of prose between the report's other sections.
~~~~


## TestFormatForTelegram_TopPlayersTableFromRealSnapshot guards the

Removed from `internal/game/devconsole/prompt_test.go:210`.

~~~~text
TestFormatForTelegram_TopPlayersTableFromRealSnapshot guards the
same ground-truth-vs-AI-paraphrase property as the Balance Report's
unit table: the Top Players table must come from Snapshot's real
scores, not from anything the model wrote in TopPerformerNarrative.
~~~~


## TestFormatForTelegram_NilSnapshotOmitsTable makes sure the table

Removed from `internal/game/devconsole/prompt_test.go:235`.

~~~~text
TestFormatForTelegram_NilSnapshotOmitsTable makes sure the table
section is skipped cleanly (not a nil-pointer panic) when Snapshot
wasn't set - the exact case every existing hand-built test
Recommendation above exercises, since none of them set Snapshot.
~~~~

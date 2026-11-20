# ADR 0055: Decisions from `internal/bot/handlers/changelog_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/changelog_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## changelogTestDB wraps rankingTestDB with an extra truncate for the

Removed from `internal/bot/handlers/changelog_test.go:10`.

~~~~text
changelogTestDB wraps rankingTestDB with an extra truncate for the
two changelog tables, which aren't covered by rankingTestDB's own
TRUNCATE list (no FK to users/encampments to cascade from) - needed
here specifically because these tests depend on exact ordering and
counts, unlike most other tests in this package which just filter by
a known seeded ID and don't care what else is in the table.
~~~~


## TestFetchChangelogPage_FallsBackToMostRecentWhenCaughtUp verifies the

Removed from `internal/bot/handlers/changelog_test.go:129`.

~~~~text
TestFetchChangelogPage_FallsBackToMostRecentWhenCaughtUp verifies the
one deliberate deviation from the plan's literal "oldest" instruction:
once nothing is unread, show the most recent entries instead.
~~~~

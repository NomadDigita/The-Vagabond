# ADR 0054: Decisions from `internal/bot/handlers/changelog.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/changelog.go` and moved here so the source no longer carries them. Each entry is the original comment.

## changelogPageSize matches the plan doc's "at least 5" instruction

Removed from `internal/bot/handlers/changelog.go:34`.

~~~~text
changelogPageSize matches the plan doc's "at least 5" instruction
literally - see FEEDBACK_CHANGELOG_NLP_PLAN.md milestone 2.
~~~~


## doPublishChangelog is the testable core shared by the admin panel's

Removed from `internal/bot/handlers/changelog.go:53`.

~~~~text
doPublishChangelog is the testable core shared by the admin panel's
guided-input flow and (if ever needed) a direct call - matching this
session's doX convention. Stores the entry, then broadcasts it to
every real player via the already-built QueueToAllPlayers (see
AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 5.2) - the actual
"dispatch it to users" half of the milestone 2 ask. Category
"general" (non-mutable): a release note is exactly the kind of thing
notifications/preferences.go's own contract says must never be
muteable, same reasoning as the scout-mission discovery-alert fix
earlier this session.
~~~~


## The entry is already saved - a broadcast failure shouldn't

Removed from `internal/bot/handlers/changelog.go:89`.

~~~~text
The entry is already saved - a broadcast failure shouldn't
make the admin think publishing failed outright and retry,
duplicating the entry. Surface it as a partial-success notice
instead of a hard error.
~~~~


## fetchChangelogPage returns up to changelogPageSize entries for

Removed from `internal/bot/handlers/changelog.go:107`.

~~~~text
fetchChangelogPage returns up to changelogPageSize entries for
userID: the oldest entries they haven't read yet, oldest-first, so a
player with a backlog catches up in chronological order instead of
landing mid-story. If they have no unread backlog at all, falls back
to the most recent entries instead (nothing to "catch up" on, so
most-recent is the useful default) - reported via the caughtUp
return value so the caller knows whether to mark anything as read.
~~~~


## Fully caught up - show the most recent entries instead, without

Removed from `internal/bot/handlers/changelog.go:139`.

~~~~text
Fully caught up - show the most recent entries instead, without
touching changelog_reads (they're already read, nothing new to
mark).
~~~~


## HandleChangelogMoreCallback is the "Show 5 More" button - since

Removed from `internal/bot/handlers/changelog.go:226`.

~~~~text
HandleChangelogMoreCallback is the "Show 5 More" button - since
entries are marked read on view, the "next" oldest-unread batch is
just whatever fetchChangelogPage returns on a fresh call, no
offset/pagination state needed.
~~~~


## HandlePublishChangelogPendingInput consumes an admin's guided-input

Removed from `internal/bot/handlers/changelog.go:235`.

~~~~text
HandlePublishChangelogPendingInput consumes an admin's guided-input
reply for publishing a changelog entry. Expected format: category on
the first line, title on the second, body on the rest - see
adminPromptFor's "changelog" case in admin.go for the prompt text
shown to the admin.
~~~~

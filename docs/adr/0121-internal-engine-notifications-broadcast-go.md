# ADR 0121: Decisions from `internal/engine/notifications/broadcast.go`

Status: Accepted

These decisions were written as comments in `internal/engine/notifications/broadcast.go` and moved here so the source no longer carries them. Each entry is the original comment.

## broadcastQuerier is satisfied by both *sql.DB and *sql.Tx - same

Removed from `internal/engine/notifications/broadcast.go:8`.

~~~~text
broadcastQuerier is satisfied by both *sql.DB and *sql.Tx - same
convention as querier in notifications.go, extended with the raw
QueryContext this file needs to enumerate recipients before calling
Queue() for each one.
~~~~


## QueueToRegion delivers message to every real player whose encampment

Removed from `internal/engine/notifications/broadcast.go:17`.

~~~~text
QueueToRegion delivers message to every real player whose encampment
currently sits in region - the "regional" broadcast shape from
AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 5.2, used for weather
and anything else tied to a continent/region. AI factions are excluded
(e.is_ai_faction = FALSE): they have no Telegram session to notify.
Goes through the same Queue() mute-aware path as any other
notification, so if category is ever made mutable, muted players are
still respected.
~~~~


## QueueToAllPlayers delivers message to every real player, server-wide -

Removed from `internal/engine/notifications/broadcast.go:55`.

~~~~text
QueueToAllPlayers delivers message to every real player, server-wide -
the "global" broadcast shape from
AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 5.2, used for things
like a tax rate change. AI factions are excluded via their synthetic
users.state = 'ai_faction' marker (see cmd/bot/main.go's AI civilization
seeding) - they have no Telegram session to notify either.
~~~~

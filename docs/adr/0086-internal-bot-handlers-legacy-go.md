# ADR 0086: Decisions from `internal/bot/handlers/legacy.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/legacy.go` and moved here so the source no longer carries them. Each entry is the original comment.

## LegacyHandler is the panel side of RARE_WORLD_FEATURES_PLAN.md Phase

Removed from `internal/bot/handlers/legacy.go:12`.

~~~~text
LegacyHandler is the panel side of RARE_WORLD_FEATURES_PLAN.md Phase
3 - the tick.Engine side (defeat detection, epitaph generation, pool
exclusion) lives in internal/engine/tick/factiondefeat.go.
~~~~


## HandleLegacyPanel (/legacy) shows the Hall of the Fallen - every

Removed from `internal/bot/handlers/legacy.go:23`.

~~~~text
HandleLegacyPanel (/legacy) shows the Hall of the Fallen - every
permanently-defeated AI faction, most recent first, as a Rich
Message <table> (name / defeated date / epitaph excerpt), matching
the Hall of Relics (relicconvoy.go) and ranking.go's leaderboard
table pattern. This is deliberately AI-faction-only content - see
the plan doc's design principle 3: real players are never
permanently defeated by anything in this feature set, so this panel
will never show a real player's outpost.
~~~~


## truncateEpitaph shortens a full epitaph to a table-friendly excerpt,

Removed from `internal/bot/handlers/legacy.go:79`.

~~~~text
truncateEpitaph shortens a full epitaph to a table-friendly excerpt,
appending an ellipsis when it had to cut - the full epitaph is still
shown in full wherever it's the primary content (the exploration
discovery message in engine.go's resolveExplorationDiscovery), this
is only for the Hall of the Fallen's compact table column.
~~~~

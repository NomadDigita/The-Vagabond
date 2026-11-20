# ADR 0211: Decisions from `internal/game/scoring/scoring.go`

Status: Accepted

These decisions were written as comments in `internal/game/scoring/scoring.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package scoring computes the canonical player "score" used for the

Removed from `internal/game/scoring/scoring.go:1`.

~~~~text
Package scoring computes the canonical player "score" used for the
Global Ranking board, matching SpaceHunt's classic point-total ranking.
The formula lives here as a single reusable SQL fragment so every
consumer (ranking panel, future admin tools, future "top skilled"
breakdowns) computes score identically rather than drifting apart.
~~~~


## ScoreExpr is a SQL scalar subquery expression that computes one

Removed from `internal/game/scoring/scoring.go:8`.

~~~~text
ScoreExpr is a SQL scalar subquery expression that computes one
encampment's total score, given its `encampments e` alias is in scope.
Composed of:
  - Base: outpost level * 1000
  - Structures: sum of all Defense Grid / utility module levels * 500
  - Economy: total banked resource value (Dollars weighted higher) * 0.05
  - Military: weighted workshop inventory value (pure combat units and
    capital ships weighted far higher than transport/utility units)
~~~~


## MilitaryScoreExpr isolates just the military/combat component, used for

Removed from `internal/game/scoring/scoring.go:34`.

~~~~text
MilitaryScoreExpr isolates just the military/combat component, used for
the "Top Skilled" (combat-only) leaderboard category.
~~~~

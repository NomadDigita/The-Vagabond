# ADR 0153: Decisions from `internal/engine/tick/roadencounter_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/roadencounter_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## testDB applies the full real schema to a throwaway Postgres. Phase 7

Removed from `internal/engine/tick/roadencounter_test.go:15`.

~~~~text
testDB applies the full real schema to a throwaway Postgres. Phase 7
milestone 5 asks for tests covering "all policy boundaries" - the road-
encounter tick passes are exactly the code that broke twice this session
from parallel-branch merge collisions, so they're the highest-value
target for real (not just gofmt/vet) test coverage.
~~~~


## Every test in this file shares one database (spinning up a fresh

Removed from `internal/engine/tick/roadencounter_test.go:36`.

~~~~text
Every test in this file shares one database (spinning up a fresh
Postgres per test would be far slower), so each test must start
from a clean slate - otherwise a leftover row from an earlier test
(or an earlier test's caught bug) can silently satisfy another
test's "did an encounter appear" check without that test's own
tick call having done anything at all.
~~~~


## world_state (a singleton row, id=1) and world_news (append-only,

Removed from `internal/engine/tick/roadencounter_test.go:47`.

~~~~text
world_state (a singleton row, id=1) and world_news (append-only,
no FK to anything the truncate above cascades from) both survive
the truncate untouched, so they'd otherwise leak state across
every test in this shared-DB test binary - e.g. aispawning_test.go
needs world_state.last_ai_spawn_at to start NULL every time, not
carry over whatever the previous test in the run left behind.
~~~~


## world_bosses (NEXT_GENERATION_FEATURES_PLAN.md Phase 8) has no FK

Removed from `internal/engine/tick/roadencounter_test.go:56`.

~~~~text
world_bosses (NEXT_GENERATION_FEATURES_PLAN.md Phase 8) has no FK
to encampments/users/coordinates either, so it survives the
CASCADE truncate above untouched too - world_boss_attacks/
world_boss_contributions/player_gear/crafting_queue DO cascade
(they all reference encampments), but world_bosses itself needs
its own reset: delete any extra rows a test inserted, and put
the fixed launch set's mutable state (current_hp, total_kills)
back to its defaults so tier-gating and boss-kill tests start
from a clean slate instead of accumulating kills across the
whole shared-DB test binary run.
~~~~


## runUntilCondition repeatedly calls fn (each in its own transaction)

Removed from `internal/engine/tick/roadencounter_test.go:128`.

~~~~text
runUntilCondition repeatedly calls fn (each in its own transaction)
until done() reports true or the retry budget is exhausted. Necessary
because the actual encounter roll is probabilistic
(roadcombat.EncounterRollChance = 0.35, not 1.0) - the production code
has no injectable RNG, so a bounded retry loop is the honest way to test
it without either flaking or weakening the real per-tick odds just to
make a test deterministic. 40 attempts makes the chance of a false
failure (every attempt missing) about 0.65^40 ≈ 4e-8.
~~~~


## Both raids must be frozen, not just one - this was exactly the kind

Removed from `internal/engine/tick/roadencounter_test.go:175`.

~~~~text
Both raids must be frozen, not just one - this was exactly the kind
of asymmetric-freeze bug that a scrambled merge could reintroduce.
~~~~


## Two expeditions from the SAME commander, converging on each other -

Removed from `internal/engine/tick/roadencounter_test.go:197`.

~~~~text
Two expeditions from the SAME commander, converging on each other -
must never generate a road_encounters row against themselves.
~~~~


## Discovery must be reciprocal even when nobody has decided to fight

Removed from `internal/engine/tick/roadencounter_test.go:291`.

~~~~text
Discovery must be reciprocal even when nobody has decided to fight
yet - matches discoverRouteContacts' existing behavior.
~~~~


## Force the response window to have already lapsed, then run the

Removed from `internal/engine/tick/roadencounter_test.go:321`.

~~~~text
Force the response window to have already lapsed, then run the
expiry pass directly - this is the "nobody tapped a button in time"
path, distinct from the explicit-Continue path in the bot handler.
~~~~

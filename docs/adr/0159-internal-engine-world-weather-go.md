# ADR 0159: Decisions from `internal/engine/world/weather.go`

Status: Accepted

These decisions were written as comments in `internal/engine/world/weather.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Continents mirrors coordinates.region's existing quadrant scheme

Removed from `internal/engine/world/weather.go:23`.

~~~~text
Continents mirrors coordinates.region's existing quadrant scheme
(see relocateZeroCoordinates in cmd/bot/main.go) so that world events
line up with where players actually spawn instead of an unrelated
separate zone list.
~~~~


## eventPool is every non-nominal world event that can currently roll.

Removed from `internal/engine/world/weather.go:29`.

~~~~text
eventPool is every non-nominal world event that can currently roll.
Solar Flare/Radiation Storm/Acid Rain are the original three; EMP,
Supply Crisis, Disease, and Sandstorm are Phase 7 item 12's additions;
Bloom is RARE_WORLD_FEATURES_PLAN.md Phase 2's addition - the
wasteland's one purely POSITIVE event, and deliberately weighted
rarer than the rest (see weatherPool/pickWeightedEvent below) so a
"rare and beautiful" moment doesn't become a routine one.
~~~~


## weatherPool assigns bloom a weight of 1 against every other event's

Removed from `internal/engine/world/weather.go:46`.

~~~~text
weatherPool assigns bloom a weight of 1 against every other event's
weight of 4 - roughly 4x rarer than any single existing event, while
leaving the other seven events' odds relative to EACH OTHER exactly
as uniform as eventPool's old flat rand.Intn(len(eventPool)) draw
always made them. Built from eventPool at init time (a range loop,
not a second hand-maintained literal) so a future addition to
eventPool can't silently fall out of sync with this weighting table
by only updating one of the two.
~~~~


## pickWeightedEvent draws one event type from weatherPool, respecting

Removed from `internal/engine/world/weather.go:66`.

~~~~text
pickWeightedEvent draws one event type from weatherPool, respecting
each entry's relative weight - the weighted-draw replacement for
eventPool's old flat rand.Intn(len(eventPool)) selection.
~~~~


## Unreachable given the loop above always exhausts roll < total,

Removed from `internal/engine/world/weather.go:81`.

~~~~text
Unreachable given the loop above always exhausts roll < total,
but keeps the function total rather than relying on that
invariant silently forever.
~~~~


## RunWeatherPass rolls world events independently per continent, instead

Removed from `internal/engine/world/weather.go:90`.

~~~~text
RunWeatherPass rolls world events independently per continent, instead
of one global front affecting the whole game. Each continent persists
its current event for 2 hours (via expires_at), then rolls a 10%
chance per tick to start a new one once clear. A continent with an
event still running is left alone (persistence); a continent whose
event just expired gets a "conditions have cleared" headline before
it's eligible to roll again.
~~~~


## Per-continent outcome for this pass, logged once at the end

Removed from `internal/engine/world/weather.go:98`.

~~~~text
Per-continent outcome for this pass, logged once at the end
regardless of whether anything happened. Previously this
function only logged on an actual trigger, so a live deployment
that only ever rolled misses produced zero log output from this
phase - indistinguishable from the phase not running at all when
diagnosing "no world events have ever fired" reports. See
LastTickStatus in internal/engine/tick/engine.go for the
companion admin-visible confirmation that ticks are executing.
~~~~


## The most recent event for this continent exists but has

Removed from `internal/engine/world/weather.go:125`.

~~~~text
The most recent event for this continent exists but has
expired - clear it out and announce the all-clear before
considering a fresh roll this same pass.
~~~~


## NOTE (2026-08-02, confirmed live via Render logs): world_events

Removed from `internal/engine/world/weather.go:156`.

~~~~text
NOTE (2026-08-02, confirmed live via Render logs): world_events
was originally created by migrations/001_initial_schema.sql with
title VARCHAR(150) NOT NULL and starts_at TIMESTAMPTZ NOT NULL
(plus description/multiplier, both nullable/defaulted). This
package's simplified 4-column model (id/event_type/continent/
expires_at) was added later via schema.go's `CREATE TABLE IF NOT
EXISTS world_events`, which is a no-op against a table that
already exists - so on any deployment whose DB predates that
change, the live table still carries the old NOT NULL columns
this INSERT never populated. Every single roll-hit was failing
at the DB layer with "null value in column title violates
not-null constraint", silently eating the whole weather pass
(RunWeatherPass returns the error, aborting the tx) - this is
the actual root cause of "zero world events have ever fired"
despite the roll logic itself always having been correct.
Supplying title/starts_at here fixes it directly; see schema.go
for the accompanying idempotent ALTER TABLE that also relaxes
the constraint with a DEFAULT, so a *fresh* database (no legacy
columns at all) and this codebase's own migration order can't
reintroduce the same trap.
~~~~


## Same headline text pushed directly, so the world_news feed and

Removed from `internal/engine/world/weather.go:187`.

~~~~text
Same headline text pushed directly, so the world_news feed and
the direct notification never say different things (section 5.3).
~~~~


## eventHeadline builds the full news-feed announcement for a freshly

Removed from `internal/engine/world/weather.go:226`.

~~~~text
eventHeadline builds the full news-feed announcement for a freshly
triggered event, matching the original three's tone and mechanical
callouts. See internal/bot/handlers/world.go for the matching
in-panel description text, and each mechanical consumer
(internal/engine/tick/engine.go, internal/engine/resource/resource.go,
internal/bot/handlers/combat.go, internal/bot/handlers/camp.go) for
where these actually apply their effects.
~~~~

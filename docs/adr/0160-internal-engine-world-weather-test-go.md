# ADR 0160: Decisions from `internal/engine/world/weather_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/world/weather_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Deterministically force a new event to roll on the very first pass:

Removed from `internal/engine/world/weather_test.go:69`.

~~~~text
Deterministically force a new event to roll on the very first pass:
eventRollChance is 10%, so run the pass in a retry loop rather than
depending on a lucky single roll (same probabilistic-but-bounded
pattern the tick-engine tests already use elsewhere).
~~~~


## The Europe player must never receive a notification mentioning

Removed from `internal/engine/world/weather_test.go:102`.

~~~~text
The Europe player must never receive a notification mentioning
Africa - cross-region isolation is already unit-tested directly in
QueueToRegion's own tests, so this just confirms weather.go passes
the right continent through, not a stale/wrong one.
~~~~


## TestPickWeightedEvent_BloomIsRarerThanEveryOtherEvent is

Removed from `internal/engine/world/weather_test.go:148`.

~~~~text
TestPickWeightedEvent_BloomIsRarerThanEveryOtherEvent is
RARE_WORLD_FEATURES_PLAN.md Phase 2.2's direct test: bloom must draw
meaningfully less often than any other single event over a large
sample. This is inherently statistical (pickWeightedEvent is a real
weighted random draw, not a deterministic function), so the
assertion is a generous tolerance band, not an exact ratio - bloom's
weight (1) against every other event's weight (4) means it should
land close to 1-in-11 of all draws (1 / (1*1 + 4*7)); the test only
requires it stays well under half of a "fair" 1-in-8 share, which a
correctly-weighted draw will clear by a wide margin every run while
still catching a regression back to a flat, unweighted pool.
~~~~


## An unweighted flat pool of 8 events would give bloom ~12.5%

Removed from `internal/engine/world/weather_test.go:167`.

~~~~text
An unweighted flat pool of 8 events would give bloom ~12.5%
(1-in-8); the weighted pool should land close to ~9% (1-in-11).
Assert well under half of the flat-pool share (6.25%) as a loose
but meaningful regression guard against weighting silently being
dropped or inverted.
~~~~

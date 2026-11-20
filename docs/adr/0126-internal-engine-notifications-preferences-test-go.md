# ADR 0126: Decisions from `internal/engine/notifications/preferences_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/notifications/preferences_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## testDB applies the full real schema (not a hand-rolled subset) to a

Removed from `internal/engine/notifications/preferences_test.go:14`.

~~~~text
testDB applies the full real schema (not a hand-rolled subset) to a
throwaway Postgres, so these tests exercise notification_preferences
exactly as it's actually defined, not a copy that could quietly drift
from the real migration.
~~~~


## TestMutableCategoriesIsTheOnlyGate is the core safety property Phase 7

Removed from `internal/engine/notifications/preferences_test.go:45`.

~~~~text
TestMutableCategoriesIsTheOnlyGate is the core safety property Phase 7
milestone 2 requires: "without suppressing combat, discovery, or
supply-loss alerts". This doesn't touch a database at all - it checks
the actual policy object every other test and every call site relies on,
so a careless future edit that adds "combat" to the map gets caught
here specifically, not just incidentally by some other test failing.
~~~~


## Even if some future bug wrote a preference row that a non-mutable

Removed from `internal/engine/notifications/preferences_test.go:69`.

~~~~text
Even if some future bug wrote a preference row that a non-mutable
category should be muted, IsCategoryMuted must not honor it - the
MutableCategories check has to happen before any row is even
consulted, per the doc comment on IsCategoryMuted.
~~~~


## mute_route_status is irrelevant here - "general" isn't in

Removed from `internal/engine/notifications/preferences_test.go:169`.

~~~~text
mute_route_status is irrelevant here - "general" isn't in
MutableCategories at all, so it must always be delivered.
~~~~

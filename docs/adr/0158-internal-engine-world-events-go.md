# ADR 0158: Decisions from `internal/engine/world/events.go`

Status: Accepted

These decisions were written as comments in `internal/engine/world/events.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Queryer is satisfied by both *sql.DB and *sql.Tx, so callers already

Removed from `internal/engine/world/events.go:8`.

~~~~text
Queryer is satisfied by both *sql.DB and *sql.Tx, so callers already
inside a tick transaction and callers making a one-off query from a
bot handler can both use the same lookup helpers below.
~~~~


## ActiveEventFor returns the currently active world event type for a

Removed from `internal/engine/world/events.go:16`.

~~~~text
ActiveEventFor returns the currently active world event type for a
given continent ("nominal" if none is active, the row has expired, or
the continent is unrecognized/empty - e.g. a rogue AI nest with no
real region). Every mechanical consumer of world events (combat
resolution, passive resource generation, march timing, construction
flavor text) should resolve its own encampment's continent via
coordinates.region and call this instead of reading world_state
directly, which is no longer written to.
~~~~


## ActiveEventsByContinent returns every continent's currently active

Removed from `internal/engine/world/events.go:39`.

~~~~text
ActiveEventsByContinent returns every continent's currently active
event type in one query, for callers that process many encampments in
a single pass (e.g. the passive resource tick) and would otherwise pay
for one query per encampment. Continents with no active event are
simply absent from the map - callers should treat a missing key the
same as "nominal".
~~~~

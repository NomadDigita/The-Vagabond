# ADR 0108: Decisions from `internal/bot/handlers/spawnlocation.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/spawnlocation.go` and moved here so the source no longer carries them. Each entry is the original comment.

## continentCoordinate rolls a random (x, y) inside the correct quadrant

Removed from `internal/bot/handlers/spawnlocation.go:12`.

~~~~text
continentCoordinate rolls a random (x, y) inside the correct quadrant
for a given continent, matching the quadrant convention established at
onboarding: Africa is +x+y, Europe -x+y, Asia +x-y, Americas -x-y.
Every per-continent system in the game (world events/weather via
internal/engine/world.Continents, and anything built on top of it
later) assumes a base's region string is always one of exactly these
four - see allocateCoordinate's doc comment for why that matters.
~~~~


## allocateCoordinate rolls a fresh (x, y) for the given continent,

Removed from `internal/bot/handlers/spawnlocation.go:32`.

~~~~text
allocateCoordinate rolls a fresh (x, y) for the given continent,
retrying on the rare (x, y) collision, and returns the new
coordinates row's UUID ready to attach to an encampment.

continent MUST be one of world.Continents. Passing anything else
(the literal string "Unknown Sector" was the previous bug in
/newjobteleport and /ghostprotocol) silently excludes that base from
every per-continent system forever - weather events are looked up by
exact continent name, so a base with a bogus region just never
matches any of them again.

seed lets callers vary determinism: onboarding seeds per-player (a
stable, evenly-spread initial placement), while /newjobteleport and
/ghostprotocol want a fresh independent reroll each time, so they
pass a time-based seed instead.
~~~~


## randomContinent picks one of the four playable continents uniformly

Removed from `internal/bot/handlers/spawnlocation.go:68`.

~~~~text
randomContinent picks one of the four playable continents uniformly
at random, for callers (teleport, Ghost Protocol) that want a genuine
reroll rather than onboarding's per-player deterministic spread.
~~~~


## flavorTowns and flavorCountriesByContinent back flavorLocation below.

Removed from `internal/bot/handlers/spawnlocation.go:75`.

~~~~text
flavorTowns and flavorCountriesByContinent back flavorLocation below.
These are deliberately fictional (no real-world country names), to
keep the tone consistent with the game's post-collapse setting.
~~~~


## flavorLocation deterministically derives a cosmetic town + country

Removed from `internal/bot/handlers/spawnlocation.go:93`.

~~~~text
flavorLocation deterministically derives a cosmetic town + country
pair for a coordinate. It's a pure function of (x, y) rather than a
stored value - deliberately not a new DB column - so the same
coordinate always describes itself the same way on every screen
(onboarding, the returning-player dashboard, teleport, Ghost
Protocol) without a migration or any risk of the display drifting
from the stored coordinate.
~~~~


## locationDescriptor renders the standard "you are here" line used

Removed from `internal/bot/handlers/spawnlocation.go:112`.

~~~~text
locationDescriptor renders the standard "you are here" line used
everywhere a player's base location is shown. HTML-safe as-is: every
input is either a fixed continent name or drawn from the fixed
flavorTowns/flavorCountriesByContinent pools above, never player-
authored text, so nothing here needs htmlEscape.
~~~~

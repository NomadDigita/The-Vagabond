# ADR 0213: Decisions from `internal/game/worldintel/worldintel.go`

Status: Accepted

These decisions were written as comments in `internal/game/worldintel/worldintel.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package worldintel owns the small, deterministic rules that govern what an

Removed from `internal/game/worldintel/worldintel.go:1`.

~~~~text
Package worldintel owns the small, deterministic rules that govern what an
outpost is allowed to know about the wider world. Database reads/writes stay
in handlers and tick passes; keeping balance math here makes it testable
without Telegram or Postgres.
~~~~


## ExplorationDiscoveryChance returns the probability that a completed

Removed from `internal/game/worldintel/worldintel.go:11`.

~~~~text
ExplorationDiscoveryChance returns the probability that a completed
exploration dispatch reveals a previously unknown target. Scout Walkers are
the expedition's survey capability, so each one helps, but the probability
has a hard cap to keep discovery meaningful rather than guaranteed.
~~~~


## Raised from 0.35 base (2026-07-26): exploration itself used to be

Removed from `internal/game/worldintel/worldintel.go:19`.

~~~~text
Raised from 0.35 base (2026-07-26): exploration itself used to be
the real bottleneck (a single shared, contested site per continent -
see explorationSiteTemplate's doc comment in
internal/bot/handlers/exploration.go), so this roll rarely even got
a chance to fire. Now that every dispatch is personal and always
available, this base is tuned so a fresh outpost with zero Scout
Walkers still has roughly a 92% chance of at least one first-contact
within 5 expeditions (1-(1-0.45)^5), not just a "maybe, eventually."
~~~~


## RadarWarningMinutes returns how much remaining travel time a defender gets

Removed from `internal/game/worldintel/worldintel.go:30`.

~~~~text
RadarWarningMinutes returns how much remaining travel time a defender gets
when its sensors first identify an incoming column. Stronger radar and home
recon units improve warning distance. Stealth routes reduce, rather than
erase, detection: capable defenses should always retain a narrow chance to
see a hostile force before impact.
~~~~


## Never reveal before half of a journey has elapsed, and ensure a

Removed from `internal/game/worldintel/worldintel.go:54`.

~~~~text
Never reveal before half of a journey has elapsed, and ensure a
non-stealth march has at least a small actionable warning window.
~~~~

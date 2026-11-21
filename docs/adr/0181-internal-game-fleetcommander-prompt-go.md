# ADR 0181: Decisions from `internal/game/fleetcommander/prompt.go`

Status: Accepted

These decisions were written as comments in `internal/game/fleetcommander/prompt.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package fleetcommander implements Phase C of the AI Systems Roadmap:

Removed from `internal/game/fleetcommander/prompt.go:1`.

~~~~text
Package fleetcommander implements Phase C of the AI Systems Roadmap:
the AI Fleet Commander. Like internal/game/governor, this package is
the seam where game state meets the ai.Service — internal/ai itself
stays game-agnostic per ADR-001 in PROJECT_MASTER_PLAN.md.

This file has zero I/O so it can be unit tested directly. All
database access lives in commander.go.
~~~~


## TargetProfile describes what the fleet might be sent against.

Removed from `internal/game/fleetcommander/prompt.go:28`.

~~~~text
TargetProfile describes what the fleet might be sent against.
TargetProfile describes what the fleet might be sent against.

TurretGrid, IntegrityTechLvl, Shields, and HeroSuperpower mirror the
enriched internal/game/content.RogueNestForce fields added by the
parallel SpaceHunt Phase 7 work ("Rogue AI scaling via real player
subsystems") — a rogue nest now stands up individually-typed
turrets, research levels, and (at high tiers) a hero-equivalent
superpower, the same way a real player base would, rather than a
single flat bonus. TurretBonus is kept for backward compatibility
with anything reading only the legacy summed total.
~~~~


## human-readable, e.g. "Moderate", "Severe" (PvE only)

Removed from `internal/game/fleetcommander/prompt.go:42`.

~~~~text
human-readable, e.g. "Moderate", "Severe" (PvE only)
~~~~


## legacy flat defense-rating modifier bonus, 0 if none

Removed from `internal/game/fleetcommander/prompt.go:44`.

~~~~text
legacy flat defense-rating modifier bonus, 0 if none
~~~~


## TurretGrid is a turret-type→level map (e.g. "light_laser": 4),

Removed from `internal/game/fleetcommander/prompt.go:46`.

~~~~text
TurretGrid is a turret-type→level map (e.g. "light_laser": 4),
empty if the target has no Defense Grid breakdown available.
~~~~


## IntegrityTechLvl mirrors research_states.integrity_tech_lvl (or

Removed from `internal/game/fleetcommander/prompt.go:49`.

~~~~text
IntegrityTechLvl mirrors research_states.integrity_tech_lvl (or
its PvE equivalent) — 0 if not applicable/unknown.
~~~~


## Shields mirrors nuclear_shields count — 0 if none.

Removed from `internal/game/fleetcommander/prompt.go:52`.

~~~~text
Shields mirrors nuclear_shields count — 0 if none.
~~~~


## HeroSuperpower names an active hero-equivalent ability the

Removed from `internal/game/fleetcommander/prompt.go:54`.

~~~~text
HeroSuperpower names an active hero-equivalent ability the
target can bring to the fight (e.g. "Kinetic Barrier"), empty if
none.
~~~~


## CombatHistorySummary is a rough win/loss proxy built from completed

Removed from `internal/game/fleetcommander/prompt.go:60`.

~~~~text
CombatHistorySummary is a rough win/loss proxy built from completed
raids where the player was the attacker. See BuildCombatHistory in
commander.go for the exact (heuristic) definition of "win" — the
`raids` table has no explicit outcome column, so this is an
approximation flagged as technical debt in PROJECT_MASTER_PLAN.md.
~~~~


## summed attacker_losses across analyzed raids

Removed from `internal/game/fleetcommander/prompt.go:68`.

~~~~text
summed attacker_losses across analyzed raids
~~~~


## Action mirrors the roadmap's required recommendation vocabulary:

Removed from `internal/game/fleetcommander/prompt.go:72`.

~~~~text
Action mirrors the roadmap's required recommendation vocabulary:
attack, retreat, reinforce, scout, wait, split fleets.
~~~~


## one of the Action constants, as free text from the model

Removed from `internal/game/fleetcommander/prompt.go:87`.

~~~~text
one of the Action constants, as free text from the model
~~~~


## e.g. "high", "medium", "low"

Removed from `internal/game/fleetcommander/prompt.go:88`.

~~~~text
e.g. "high", "medium", "low"
~~~~


## only meaningful if Recommendation == split_fleet

Removed from `internal/game/fleetcommander/prompt.go:91`.

~~~~text
only meaningful if Recommendation == split_fleet
~~~~


## FellBackToRawText is true when JSON parsing failed and

Removed from `internal/game/fleetcommander/prompt.go:93`.

~~~~text
FellBackToRawText is true when JSON parsing failed and
Reasoning holds the model's raw, unparsed text instead.
~~~~


## Truncated is true when the fallback happened because the

Removed from `internal/game/fleetcommander/prompt.go:96`.

~~~~text
Truncated is true when the fallback happened because the
model's response was cut off mid-object (almost always meaning
MaxTokens was hit before the model finished). See ADR-016 in
PROJECT_MASTER_PLAN.md.
~~~~


## BuildUserPrompt renders own fleet + target + history into the data

Removed from `internal/game/fleetcommander/prompt.go:117`.

~~~~text
BuildUserPrompt renders own fleet + target + history into the data
block the model reasons over. Sorted map iteration keeps output
deterministic for internal/ai's cache key hashing.
~~~~


## ParseRecommendation decodes the model's response text, tolerating a

Removed from `internal/game/fleetcommander/prompt.go:183`.

~~~~text
ParseRecommendation decodes the model's response text, tolerating a
markdown code fence the way governor.ParseRecommendation does. On
failure it falls back to Reasoning=<raw text> so the player always
gets something usable.
~~~~


## See ADR-015: real providers occasionally leave a raw,

Removed from `internal/game/fleetcommander/prompt.go:198`.

~~~~text
See ADR-015: real providers occasionally leave a raw,
unescaped newline/tab inside a string value.
~~~~


## actionEmoji gives each recommendation a distinct glyph so players

Removed from `internal/game/fleetcommander/prompt.go:208`.

~~~~text
actionEmoji gives each recommendation a distinct glyph so players
can scan the outcome at a glance before reading the reasoning.
~~~~


## FormatForTelegram renders a Recommendation as a Telegram HTML-mode

Removed from `internal/game/fleetcommander/prompt.go:219`.

~~~~text
FormatForTelegram renders a Recommendation as a Telegram HTML-mode
message. All fields are LLM-generated text and are escaped via
ai.HTMLEscape before being wrapped in a tag - see internal/ai/render.go.
~~~~

# ADR 0167: Decisions from `internal/game/content/roguenest.go`

Status: Accepted

These decisions were written as comments in `internal/game/content/roguenest.go` and moved here so the source no longer carries them. Each entry is the original comment.

## RogueNestForce is the composition of an AI "Rogue Drone Nest" defense,

Removed from `internal/game/content/roguenest.go:3`.

~~~~text
RogueNestForce is the composition of an AI "Rogue Drone Nest" defense,
scaled to the attacking player's own outpost level. Higher-level players
face a correspondingly larger, more varied, better-equipped garrison -
not just more of the same single unit type.

Phase 7: the Nest no longer fights with a single flat "TurretBonus". It
now stands up the same subsystems a real player base would - an
individually-typed Defense Grid, garrison-only Guardians/Observers, a
research (Integrity Tech) level, nuclear shields, and (at high tiers) a
hero-equivalent superpower - so it is resolved by the combat engine
through the exact same code path as a real defender, not a bespoke
fallback formula. TurretBonus is kept only as a legacy total for any
display code that hasn't been updated to the per-turret breakdown.
~~~~


## "" until the Nest is dangerous enough to field a Warlord-equivalent

Removed from `internal/game/content/roguenest.go:37`.

~~~~text
"" until the Nest is dangerous enough to field a Warlord-equivalent
~~~~


## legacy flat total (sum of turret levels * 0.08), kept for older display code only

Removed from `internal/game/content/roguenest.go:39`.

~~~~text
legacy flat total (sum of turret levels * 0.08), kept for older display code only
~~~~


## RogueNestComposition computes the Rogue Drone Nest's defense force for a

Removed from `internal/game/content/roguenest.go:42`.

~~~~text
RogueNestComposition computes the Rogue Drone Nest's defense force for a
given attacker level. This is deterministic and shared by both the real
combat resolution and the /recon_ai scouting preview, so what a player
scouts is exactly what they'll actually fight - no surprises.
~~~~


## Defense Grid: the Nest stands up turrets in the same tiered order a

Removed from `internal/game/content/roguenest.go:65`.

~~~~text
Defense Grid: the Nest stands up turrets in the same tiered order a
player would build them, one type at a time, so it "feels" like a
real base rather than a monolithic bonus.
~~~~


## Extreme-tier Nests field a Warlord: a hero-equivalent that grants

Removed from `internal/game/content/roguenest.go:99`.

~~~~text
Extreme-tier Nests field a Warlord: a hero-equivalent that grants
one of the same superpowers a player Commander can roll, so top-end
players genuinely feel like they're fighting a fortified commander,
not a spreadsheet.
~~~~

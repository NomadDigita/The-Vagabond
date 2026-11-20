# ADR 0107: Decisions from `internal/bot/handlers/silo.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/silo.go` and moved here so the source no longer carries them. Each entry is the original comment.

## BUGFIX: this used to be `WHERE e.id != $1 LIMIT 3` - any 3

Removed from `internal/bot/handlers/silo.go:51`.

~~~~text
BUGFIX: this used to be `WHERE e.id != $1 LIMIT 3` - any 3
encampments in the entire game, in whatever order Postgres felt
like returning them, with no relation to what this commander has
actually scouted. Every other targeting surface in the game (the
raid board's HandleRaidBoard, above) gates on encampment_discoveries
- you have to scout a target before you can act on it, the fog-of-
war rule this game is built around. The Silo was the one exception,
silently bypassing it while also being effectively random about
which handful of (mostly un-scouted, unreachable-by-raid) outposts
it even offered. Now it shows the same discovered targets the raid
board would, most-recently-scouted first.
~~~~


## HandleLaunchPiercingMissileCallback fires a Piercing Missile: a Silo

Removed from `internal/bot/handlers/silo.go:258`.

~~~~text
HandleLaunchPiercingMissileCallback fires a Piercing Missile: a Silo
weapon built to punch through defenses rather than raid strength. It's
far harder for an Anti-Missile Battery to intercept than a standard
Nuke (half the intercept chance, half the cap) and is never stopped by
Nuclear Shields at all - but instead of vaporizing troops, it directly
strips levels off the target's Defense Grid turrets.
~~~~


## Half the intercept chance and half the cap of a standard ICBM -

Removed from `internal/bot/handlers/silo.go:316`.

~~~~text
Half the intercept chance and half the cap of a standard ICBM -
piercing warheads are built specifically to slip past this.
~~~~


## Piercing Missiles ignore Nuclear Shields entirely - that's the

Removed from `internal/bot/handlers/silo.go:341`.

~~~~text
Piercing Missiles ignore Nuclear Shields entirely - that's the
entire point of the weapon - and strip one level off EVERY Defense
Grid turret type instead of vaporizing troops or a random facility.
~~~~

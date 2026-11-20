# ADR 0084: Decisions from `internal/bot/handlers/hero.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/hero.go` and moved here so the source no longer carries them. Each entry is the original comment.

## HandleGarrisonPanel shows the player's current garrison reservation

Removed from `internal/bot/handlers/hero.go:123`.

~~~~text
HandleGarrisonPanel shows the player's current garrison reservation
(how many Soldiers/Mechs are locked at home vs. free to draft into a
campaign) and lets them adjust it. This is the "player manually assigns
defensive units" / "withdraw defenders anytime" half of the Hero
Commander system - garrisoned units always still count for defense
(nothing is subtracted from combat), they're simply protected from
being pulled into an outgoing raid draft.
~~~~


## HandleGarrisonAdjustCallback moves units between the general draftable

Removed from `internal/bot/handlers/hero.go:177`.

~~~~text
HandleGarrisonAdjustCallback moves units between the general draftable
pool and the locked garrison reserve. "inc" reserves more units at home
(soldiers in steps of 10, mechs in steps of 5); "dec" withdraws them
back to the general pool, freeing them up to draft into a raid again.
~~~~


## NEXT_GENERATION_FEATURES_PLAN.md Phase 6: unlocked skill

Removed from `internal/bot/handlers/hero.go:273`.

~~~~text
NEXT_GENERATION_FEATURES_PLAN.md Phase 6: unlocked skill
nodes discount the Scrap cost of training (EffectTrainingCostPct)
and a hero-equipped gear item (see herodepth.HeroXPBonusPct)
boosts the XP earned - two independent Phase 6 systems this
one action exercises together.
~~~~


## HandleHeroGearPanel (⚙️ Hero Gear button) shows every owned item

Removed from `internal/bot/handlers/hero.go:421`.

~~~~text
HandleHeroGearPanel (⚙️ Hero Gear button) shows every owned item
with a "hero-equip" button - distinct from Phase 3's camp-level
/gear equip, since a hero can independently carry one item for its
own XP-gain bonus (see herodepth.HeroXPBonusPct) while the camp
separately benefits from whatever's equipped at the camp level.
~~~~

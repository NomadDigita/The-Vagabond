# ADR 0202: Decisions from `internal/game/prestige/prestige.go`

Status: Accepted

These decisions were written as comments in `internal/game/prestige/prestige.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package prestige is NEXT_GENERATION_FEATURES_PLAN.md Phase 4:

Removed from `internal/game/prestige/prestige.go:1`.

~~~~text
Package prestige is NEXT_GENERATION_FEATURES_PLAN.md Phase 4:
Prestige / Rebirth Loop - a deliberate, irreversible reset of the
economic game back to its starting point, in exchange for a
permanent, stacking resource-generation bonus. The classic idle-
genre answer to "a maxed-out veteran has nothing left to do."

What resets: camp level, every building (modules table), every
research track (all 7 columns on research_states - this build's
actual research surface, wider than the plan doc's illustrative
"four tracks" description), and current resources (reset to the
same starting baseline onboarding.go grants a brand-new account).

What is deliberately kept: gear (Phase 3), achievements (Phase 1),
hero level/skills/story progress (Phase 6), clan membership, and any
Legacy/Relic history - prestige resets the economic/building game,
not a player's identity or social standing. Garrison (soldiers/
mechs in workshop_inventory) is also kept - the plan doc's own list
of what resets names camp level, buildings, research, and resources
specifically, and does not mention troops.
~~~~


## Querier is satisfied by both *sql.DB and *sql.Tx, matching the same

Removed from `internal/game/prestige/prestige.go:28`.

~~~~text
Querier is satisfied by both *sql.DB and *sql.Tx, matching the same
convention gear.Querier/progression.Querier/herodepth.Querier
already establish.
~~~~


## campLevelRequirement/researchLevelRequirement are Prestige's

Removed from `internal/game/prestige/prestige.go:36`.

~~~~text
campLevelRequirement/researchLevelRequirement are Prestige's
eligibility gate - deliberately steep (matching the plan doc's own
"meaningful requirement" framing), since resetting the economic game
should only be available to someone who has genuinely finished it,
not someone looking for a shortcut.
~~~~


## researchColumns is every column on research_states this build

Removed from `internal/game/prestige/prestige.go:48`.

~~~~text
researchColumns is every column on research_states this build
actually exposes (research.go's own fetchResearchLevels reads all
seven) - Prestige requires and resets every one of them, not just
the plan doc's illustrative four.
~~~~


## EligibilityResult is CheckEligibility's return shape - Eligible is

Removed from `internal/game/prestige/prestige.go:57`.

~~~~text
EligibilityResult is CheckEligibility's return shape - Eligible is
the single yes/no the panel gates the Prestige button on; Missing is
a human-readable breakdown of exactly what's still short, so a
not-yet-eligible player sees a clear "what's missing" list rather
than a bare refusal.
~~~~


## No research_states row yet at all - every track is

Removed from `internal/game/prestige/prestige.go:89`.

~~~~text
No research_states row yet at all - every track is
effectively level 1, well under the requirement.
~~~~


## DoPrestige performs the actual reset - deliberately NOT wrapped in

Removed from `internal/game/prestige/prestige.go:119`.

~~~~text
DoPrestige performs the actual reset - deliberately NOT wrapped in
its own transaction (the caller, HandlePrestigeConfirmCallback,
already runs this inside one alongside the confirmation flow, the
same "caller owns the transaction" convention doPostListing/
doBuyListingQty already establish).
~~~~


## ResourceBonusPctFor returns campID's current cumulative Prestige

Removed from `internal/game/prestige/prestige.go:159`.

~~~~text
ResourceBonusPctFor returns campID's current cumulative Prestige
bonus - resource.go reads this at the exact same call site
gear.ResourceBonusPctFor/herodepth.ResourceBonusPctFor/Phase 2's
Bloom multiplier already use, stacking with (not replacing) any of
them.
~~~~

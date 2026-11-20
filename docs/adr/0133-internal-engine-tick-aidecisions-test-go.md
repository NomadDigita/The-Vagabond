# ADR 0133: Decisions from `internal/engine/tick/aidecisions_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/aidecisions_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestAIScoutCanDiscoverAnotherAIFaction verifies aiScout no longer

Removed from `internal/engine/tick/aidecisions_test.go:123`.

~~~~text
TestAIScoutCanDiscoverAnotherAIFaction verifies aiScout no longer
excludes AI factions as scouting targets - AI-vs-AI conflict is now in
scope (AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 2, superseding
AI_FACTION_DECISION_LOOP_PLAN.md's "deliberately out of scope" note).
Faction Beta is seeded first (closer/earlier established), so the
existing danger/established_at ordering picks it before the player.
~~~~


## TestPickFairAIRaidTargetOnlyAllowsAttackingAtOrAboveOwnLevel verifies

Removed from `internal/engine/tick/aidecisions_test.go:168`.

~~~~text
TestPickFairAIRaidTargetOnlyAllowsAttackingAtOrAboveOwnLevel verifies
the 2026-08-01 fairness inversion: a faction may only raid a target at
its own level or above, never below - "lower level can attack higher
level but higher level can't attack lower level," per the project
owner. Runs pickFairAIRaidTarget many times since it randomizes
between the normal and wide band on each call; a below-level target
must never appear in any of them, and an at-or-above-level target must
appear at least once.
~~~~


## far below - must always be excluded

Removed from `internal/engine/tick/aidecisions_test.go:185`.

~~~~text
far below - must always be excluded
~~~~


## 1 level BELOW - the exact case the project owner called out

Removed from `internal/engine/tick/aidecisions_test.go:188`.

~~~~text
1 level BELOW - the exact case the project owner called out
~~~~


## as the one that "should almost never" happen anymore: must be excluded now, even though th

Removed from `internal/engine/tick/aidecisions_test.go:189`.

~~~~text
as the one that "should almost never" happen anymore: must be excluded now, even though the old
(pre-2026-08-01) rule would have allowed it.
~~~~


## 2 levels ABOVE, inside the normal band - a valid "punch up" target.

Removed from `internal/engine/tick/aidecisions_test.go:193`.

~~~~text
2 levels ABOVE, inside the normal band - a valid "punch up" target.
~~~~


## exactly level - allowed, "at or above" includes equal.

Removed from `internal/engine/tick/aidecisions_test.go:196`.

~~~~text
exactly level - allowed, "at or above" includes equal.
~~~~


## TestPickOverdueRaidTargetGuaranteesEligibilityRegardlessOfProbability

Removed from `internal/engine/tick/aidecisions_test.go:237`.

~~~~text
TestPickOverdueRaidTargetGuaranteesEligibilityRegardlessOfProbability
verifies the second half of the 2026-08-01 direction: a discovered,
level-4+ real player who hasn't been raided in aiOverdueRaidThreshold
is returned by pickOverdueRaidTarget - the guarantee mechanism that
bypasses the probability roll entirely - while a target raided
recently, a target below the level floor, and another AI faction (not
eligible for this specific guarantee) are all correctly excluded.
~~~~


## well above the level-4 floor, never raided - should be guaranteed-eligible

Removed from `internal/engine/tick/aidecisions_test.go:253`.

~~~~text
well above the level-4 floor, never raided - should be guaranteed-eligible
~~~~


## below aiOverdueMinTargetLevel (4) - must be excluded from the guarantee

Removed from `internal/engine/tick/aidecisions_test.go:256`.

~~~~text
below aiOverdueMinTargetLevel (4) - must be excluded from the guarantee
~~~~


## AI-vs-AI is deliberately excluded from this guarantee

Removed from `internal/engine/tick/aidecisions_test.go:262`.

~~~~text
AI-vs-AI is deliberately excluded from this guarantee
~~~~


## recentlyRaidedTarget was raided 1 hour ago - well inside the 10-hour threshold.

Removed from `internal/engine/tick/aidecisions_test.go:269`.

~~~~text
recentlyRaidedTarget was raided 1 hour ago - well inside the 10-hour threshold.
~~~~


## TestAIFactionCanRaidAnotherAIFactionRarely mirrors

Removed from `internal/engine/tick/aidecisions_test.go:404`.

~~~~text
TestAIFactionCanRaidAnotherAIFactionRarely mirrors
TestEvaluateRoadBaseEncountersExcludesAIFactionBases's probabilistic
pattern from roadencounter_test.go: AI-vs-AI conflict is in scope
(AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 2), and running many
iterations at aiVsAIRaidProbabilityWhenEligible should produce at least
one AI-vs-AI raid (comfortably so, now that the probability was raised
2026-08-01 toward a twice-daily real-world cadence rather than staying
rare background texture), and it should never generate a public
world_news headline.
~~~~


## Garrison is finite and each raid debits it - replenish so later

Removed from `internal/engine/tick/aidecisions_test.go:447`.

~~~~text
Garrison is finite and each raid debits it - replenish so later
iterations can still commit forces even after earlier raids fired.
~~~~


## TestAILaunchedRaidParticipatesInRoadEncounters is the exit-criteria

Removed from `internal/engine/tick/aidecisions_test.go:465`.

~~~~text
TestAILaunchedRaidParticipatesInRoadEncounters is the exit-criteria
test AI_FACTION_DECISION_LOOP_PLAN.md calls "the single most important
test in this plan" - it proves an AI-launched raid is a real column
through the Phase 3/4 road-encounter system, not just a row that
resolves in isolation.
~~~~


## Force it to the midpoint of its route, same trick seedMarchingRaid

Removed from `internal/engine/tick/aidecisions_test.go:501`.

~~~~text
Force it to the midpoint of its route, same trick seedMarchingRaid
uses, so it's guaranteed to be in encounter range of a human column
converging on the same point.
~~~~


## AI raid marches from (0,0) toward the player at (10,0); a human

Removed from `internal/engine/tick/aidecisions_test.go:511`.

~~~~text
AI raid marches from (0,0) toward the player at (10,0); a human
column marching the reverse direction converges at the midpoint.
~~~~

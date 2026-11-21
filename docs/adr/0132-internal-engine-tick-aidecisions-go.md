# ADR 0132: Decisions from `internal/engine/tick/aidecisions.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/aidecisions.go` and moved here so the source no longer carries them. Each entry is the original comment.

## aiDecisionCadence is how often a single AI faction gets to decide

Removed from `internal/engine/tick/aidecisions.go:12`.

~~~~text
aiDecisionCadence is how often a single AI faction gets to decide
anything. See AI_FACTION_DECISION_LOOP_PLAN.md's "Step 1: cadence gate"
for why 20 minutes was picked (roughly one of the 8 seeded factions
deciding something every 2-3 minutes world-wide).
~~~~


## aiRaidProbabilityWhenEligible is rolled even when a faction has a valid,

Removed from `internal/engine/tick/aidecisions.go:18`.

~~~~text
aiRaidProbabilityWhenEligible is rolled even when a faction has a valid,
fair target - so AI aggression reads as a threat that sometimes doesn't
come, not a metronome. See the plan doc's "Step 3: intent selection".
~~~~


## aiVsAIRaidProbabilityWhenEligible is the separate probability used when

Removed from `internal/engine/tick/aidecisions.go:23`.

~~~~text
aiVsAIRaidProbabilityWhenEligible is the separate probability used when
a fair target happens to be another AI faction rather than a human.
Raised 2026-08-01 from its original 0.12 ("rare background texture")
per explicit project owner direction: AI-vs-AI raids should actually
reach roughly twice-daily frequency, not stay a rare curiosity. At the
20-minute cadence (aiDecisionCadence) each faction gets ~72 decisions/
day; 0.35 assumes a fair AI-vs-AI target is discovered and eligible on
a meaningful minority of those cycles, which combined with the
probability should land in the right neighborhood - this is a
first-pass estimate to be corrected from real play, not a derived
number, exactly like every other constant in this file.
~~~~


## aiFractionOfGarrisonCommitted caps how much of a faction's home garrison

Removed from `internal/engine/tick/aidecisions.go:36`.

~~~~text
aiFractionOfGarrisonCommitted caps how much of a faction's home garrison
a single raid can commit, so a faction that raids never strips its own
defense to zero - mirrors the spirit of the human "Manual Defense
Garrison" concept (combat_road_encounters.go's loadBaseGarrisonForce),
without reusing that exact mechanism (AI factions don't have a player-
set garrison reserve to read).
~~~~


## aiFairnessNormalBandAbove, aiFairnessWideBandAbove, and

Removed from `internal/engine/tick/aidecisions.go:44`.

~~~~text
aiFairnessNormalBandAbove, aiFairnessWideBandAbove, and
aiFairnessWideBandChance replace the old "how far below itself may a
faction raid" guardrail (aiMaxLevelsBelowSelfForFairTarget) with the
inverted rule the project owner set 2026-08-01: "lower level can
attack higher level but higher level can't attack lower level." A
faction may now only raid a target at its own level or above, never
below - this protects newer/weaker players and weaker AI factions
from being farmed by anything stronger, while still letting a weak
side "punch up" for a real fight. It applies identically regardless of
whether the target is a human or another AI faction - the project
owner was explicit that near-level-or-higher is the same rule on both
sides (e.g. a level-9 faction reaching down to hit a level-7 one
"should almost never" happen anymore, exactly the case this inversion
closes off).

aiFairnessNormalBandAbove is how far above its own level a faction
will normally consider. aiFairnessWideBandAbove is an occasionally-
used wider version so a faction that's currently the strongest thing
it has discovered within the normal band doesn't go permanently idle
- "sometimes widen the fairness band generally," per the project
owner - and aiFairnessWideBandChance is how often the wide band gets
used instead of the normal one. First-pass tuning guesses, same
caveat as every other constant here: loosen or tighten after
observing real play.
~~~~


## aiOverdueRaidThreshold and aiOverdueMinTargetLevel implement the

Removed from `internal/engine/tick/aidecisions.go:74`.

~~~~text
aiOverdueRaidThreshold and aiOverdueMinTargetLevel implement the
second half of the project owner's 2026-08-01 direction: independent
of the fairness band and the probability roll above, a real player at
or above aiOverdueMinTargetLevel who hasn't been the defender in any
raid for at least aiOverdueRaidThreshold, and who at least one AI
faction has already legitimately discovered and could raid under the
attack-up-only rule, gets raided on that faction's very next decision
cycle - no roll, no exception. This is what "sometimes... a dedicated
hasn't-been-raided-in-N-hours becomes eligible" means in practice: it
guarantees periodic pressure on higher-level players instead of
leaving it to chance whether the probability roll ever produces it.
10 hours means a continuously-eligible level-10+ player sees roughly
2-3 guaranteed hits/day from this mechanism alone (on top of whatever
the normal roll adds), which is what the "2 to 4 raids daily for
level 10+" ask was actually asking for. Deliberately restricted to
real players, not AI-vs-AI - AI-vs-AI frequency is handled entirely by
aiVsAIRaidProbabilityWhenEligible above. First-pass guess, same
tuning caveat as the rest of this file.
~~~~


## decideAIFactionActions is the second half of Phase 6 (persistent AI

Removed from `internal/engine/tick/aidecisions.go:97`.

~~~~text
decideAIFactionActions is the second half of Phase 6 (persistent AI
civilizations) - see AI_FACTION_DECISION_LOOP_PLAN.md for the full
design this implements. growAICivilizations (Phase 6 foundational tier)
already makes AI factions grow passively; this makes them periodically
decide to scout an undiscovered target or launch a real raid, using
nothing but what they've legitimately discovered through
encampment_discoveries - the same no-omniscience rule a human plays by.
~~~~


## Mark the decision as made immediately - this is both the

Removed from `internal/engine/tick/aidecisions.go:134`.

~~~~text
Mark the decision as made immediately - this is both the
cadence gate for next time and (combined with the FOR UPDATE
above) the concurrency guard: a second tick worker reading
this row after this UPDATE commits will see a fresh
ai_last_decision_at and skip it.
~~~~


## A faction that leads or belongs to a clan with an active pact

Removed from `internal/engine/tick/aidecisions.go:171`.

~~~~text
A faction that leads or belongs to a clan with an active pact
(alliance or NAP) against another clan must not raid that clan's
members - the same rule HasActivePact enforces for human-launched
raids in combat.go. Resolved once per decision and threaded
through both target pickers below, rather than reimplementing the
clan lookup in each. NULL (not in a clan) simply excludes nothing,
since no clan_diplomacy row can match a NULL clan id.
~~~~


## countUndiscoveredTargets mirrors the WHERE clause

Removed from `internal/engine/tick/aidecisions.go:218`.

~~~~text
countUndiscoveredTargets mirrors the WHERE clause
resolveExplorationDiscovery uses to find a human's next undiscovered
target, adapted for an AI faction as the observer. Other AI factions now
count as valid undiscovered targets too - AI-vs-AI conflict is in scope,
see AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 2 (which
supersedes AI_FACTION_DECISION_LOOP_PLAN.md's "deliberately out of
scope" note).
~~~~


## aiScout discovers exactly one undiscovered target (a real player or,

Removed from `internal/engine/tick/aidecisions.go:240`.

~~~~text
aiScout discovers exactly one undiscovered target (a real player or,
since this is now in scope, another AI faction) in the faction's home
continent - the AI-faction-as-observer counterpart to
resolveExplorationDiscovery. No notification is sent to the discovered
party: a human's own exploration discovering a rival doesn't notify
the rival either (discovery is silent both ways today), so this doesn't
introduce a new asymmetry.
~~~~


## pickFairAIRaidTarget returns an already-discovered target (a real

Removed from `internal/engine/tick/aidecisions.go:284`.

~~~~text
pickFairAIRaidTarget returns an already-discovered target (a real
player, or - now that AI-vs-AI conflict is in scope, see
AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 2 - another AI
faction) this faction could fairly raid, or nil if none qualifies.
"Fairly" means at or above the faction's own level, up to
aiFairnessNormalBandAbove levels above (or, on the
aiFairnessWideBandChance roll, up to aiFairnessWideBandAbove) - the
attack-up-only rule described where those constants are defined. The
band applies identically regardless of who's on the other side;
AI-vs-AI doesn't get a special exemption from it. factionClanID (may
be NULL/invalid if the faction isn't in a clan) excludes any target
whose clan has an active alliance or NAP with the faction's clan - the
same rule HasActivePact enforces for human-launched raids.
~~~~


## pickOverdueRaidTarget implements the guarantee mechanism described at

Removed from `internal/engine/tick/aidecisions.go:341`.

~~~~text
pickOverdueRaidTarget implements the guarantee mechanism described at
aiOverdueRaidThreshold's definition: an already-discovered real player
(never another AI faction - AI-vs-AI frequency is handled separately
by aiVsAIRaidProbabilityWhenEligible) at or above aiOverdueMinTargetLevel,
at or above this faction's own level (still no exemption from the
attack-up-only rule - this only removes the probability roll, not the
other guardrails), who hasn't been raided by anyone in at least
aiOverdueRaidThreshold. A diplomatic pact still blocks the guarantee too
(see factionClanID's doc on pickFairAIRaidTarget) - a truce shouldn't
have a "unless the timer runs out" loophole. Returns nil if no such
target exists, in which case the caller falls through to the normal
probabilistic path.
~~~~


## launchAIRaid creates a real raids row with factionID as attacker_id,

Removed from `internal/engine/tick/aidecisions.go:398`.

~~~~text
launchAIRaid creates a real raids row with factionID as attacker_id,
shaped exactly like a human-launched raid (see
AI_FACTION_DECISION_LOOP_PLAN.md's "Step 6" for why this precision
matters - the entire downstream combat/road/loot/notification pipeline
only works correctly if every field a human raid sets is set here too).
aiFleeGarrisonThreshold is the "reduced below some threshold by
repeated raiding" heuristic from section 3.4 - a faction with fewer
than this many total soldiers+mechs left is treated as desperate
enough to consider fleeing. Deliberately a small absolute number
rather than a fraction of historical peak: simpler to reason about,
and this session's own note on aiMaxLevelsBelowSelfForFairTarget
applies here too - a tunable starting guess, not a precisely-right
number.

aiFleeCooldown and aiFleeCostFraction mirror jobs.go's
ghostProtocolCooldown/ghostProtocolCostFraction exactly (an AI faction
gets no special exemption from Ghost Protocol's real cost/cadence) -
duplicated here rather than imported since bot/handlers and
engine/tick are separate packages and those constants are
intentionally unexported; if either value changes, update both.
~~~~


## maybeAIFlee is the AI decision loop's use of Ghost Protocol (see

Removed from `internal/engine/tick/aidecisions.go:424`.

~~~~text
maybeAIFlee is the AI decision loop's use of Ghost Protocol (see
jobs.go's HandleGhostProtocol for the human-facing equivalent and the
full design rationale). Reuses the exact same cooldown and
proportional-cost formula, so an AI faction gets no special exemption
from Ghost Protocol's real cost - it just happens to be cheap in
absolute terms when the faction is already down to near-nothing,
which is exactly when this triggers.
~~~~


## Nobody reads a newsfeed of two bots fighting each other - suppress

Removed from `internal/engine/tick/aidecisions.go:543`.

~~~~text
Nobody reads a newsfeed of two bots fighting each other - suppress
the public world_news headline for AI-vs-AI raids, but still keep
the audit trail in ai_faction_decisions below (that should record
everything, headline or not).
~~~~

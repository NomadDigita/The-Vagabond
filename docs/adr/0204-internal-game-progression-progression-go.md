# ADR 0204: Decisions from `internal/game/progression/progression.go`

Status: Accepted

These decisions were written as comments in `internal/game/progression/progression.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package progression is the shared engine behind

Removed from `internal/game/progression/progression.go:1`.

~~~~text
Package progression is the shared engine behind
NEXT_GENERATION_FEATURES_PLAN.md Phase 1 (Achievements & Milestones)
and Phase 2 (Daily/Weekly Quest Board) - built together per that
document's own suggested build order, since both are event-driven
off the same real action call sites.

Definitions (which achievements/quests exist, what triggers them,
what they reward) are static Go data, not DB tables - the same
convention this codebase already uses for other seed-flavor content
(weather.go's eventPool, relicconvoys.go's relicNames,
factiondefeat.go's epitaphTemplates). Only per-player progress is
persisted.

A single call, RecordEvent, is the one integration point every real
action handler needs - it updates the player's lifetime counter for
a trigger, unlocks any achievement whose threshold is now met, and
advances any active quest sharing that trigger as its goal type, all
in one round trip. Callers don't need to know which achievements or
quests exist, only which trigger key their action corresponds to.
~~~~


## Querier is satisfied by both *sql.DB and *sql.Tx, matching the same

Removed from `internal/game/progression/progression.go:32`.

~~~~text
Querier is satisfied by both *sql.DB and *sql.Tx, matching the same
convention notifications.querier and storagecap.Queryer already
establish - callers already holding an open transaction (most tick-
engine call sites) reuse it directly; handler call sites without one
(most bot handlers) pass their *sql.DB.
~~~~


## Trigger keys - the vocabulary every RecordEvent call site and every

Removed from `internal/game/progression/progression.go:43`.

~~~~text
Trigger keys - the vocabulary every RecordEvent call site and every
AchievementDef/QuestDef below share. Adding a new achievement or
quest that reuses an EXISTING trigger needs zero new call sites;
adding one that needs a genuinely new signal does need one new
RecordEvent call at the relevant action's completion point.
~~~~


## AchievementDef is a permanent, one-time (per player) unlock. Trigger

Removed from `internal/game/progression/progression.go:60`.

~~~~text
AchievementDef is a permanent, one-time (per player) unlock. Trigger
+ Threshold together mean "unlock once the player's lifetime counter
for Trigger reaches Threshold" - a Threshold of 1 is a simple
one-shot achievement, a higher Threshold is a milestone.
~~~~


## combat, economy, exploration, social

Removed from `internal/game/progression/progression.go:69`.

~~~~text
combat, economy, exploration, social
~~~~


## Achievements is the full launch catalog - deliberately a smaller,

Removed from `internal/game/progression/progression.go:75`.

~~~~text
Achievements is the full launch catalog - deliberately a smaller,
real, fully-wired set (18) rather than the plan doc's illustrative
"40-50" figure, matching this project's own established practice of
shipping a genuine v1 and expanding data-only later (adding another
entry here needs no new code if it reuses an existing Trigger).
~~~~


## --- Combined-arms milestones (deliberately reuse existing

Removed from `internal/game/progression/progression.go:104`.

~~~~text
--- Combined-arms milestones (deliberately reuse existing
triggers at higher thresholds rather than inventing new ones,
demonstrating the "adding an achievement needs no new code"
property this package is designed around) ---
~~~~


## QuestPeriod is "daily" or "weekly" - determines how a quest's

Removed from `internal/game/progression/progression.go:113`.

~~~~text
QuestPeriod is "daily" or "weekly" - determines how a quest's
period_start is computed (see periodStart below) and therefore how
often its progress resets.
~~~~


## PeriodOnce is NEXT_GENERATION_FEATURES_PLAN.md Phase 17's

Removed from `internal/game/progression/progression.go:121`.

~~~~text
PeriodOnce is NEXT_GENERATION_FEATURES_PLAN.md Phase 17's
"Getting Started" checklist - a one-time onboarding quest chain
that reuses this same progress-tracking machinery instead of
building a third one. Its period_start never advances (see
periodStart below), so a PeriodOnce quest's single
player_quest_progress row is permanent, not reset like
daily/weekly quests are.
~~~~


## QuestDef is a repeatable, period-scoped goal. Unlike achievements,

Removed from `internal/game/progression/progression.go:131`.

~~~~text
QuestDef is a repeatable, period-scoped goal. Unlike achievements,
completing a quest doesn't auto-grant its reward for a real player -
see ClaimQuest below - a real player has to visit /quests and tap
Claim, giving the panel a reason to be visited. An AI faction
(negative user_id) has no panel to visit, so RecordEvent auto-claims
on its behalf at the moment of completion instead of leaving it
permanently unclaimed - see RecordEvent's own comment.
~~~~


## "" if the reward is dollars-only

Removed from `internal/game/progression/progression.go:145`.

~~~~text
"" if the reward is dollars-only
~~~~


## ignored if RewardResourceType == ""

Removed from `internal/game/progression/progression.go:146`.

~~~~text
ignored if RewardResourceType == ""
~~~~


## Quests is the fixed daily/weekly board - deliberately a fixed set

Removed from `internal/game/progression/progression.go:149`.

~~~~text
Quests is the fixed daily/weekly board - deliberately a fixed set
shown every period rather than a randomly-rerolled 3-of-N subset
(NEXT_GENERATION_FEATURES_PLAN.md's Phase 2 draft described
rotation; this is a documented scope simplification for the initial
build - see that doc's Phase 2 status update after this commit).
~~~~


## --- NEXT_GENERATION_FEATURES_PLAN.md Phase 17: "Getting Started"

Removed from `internal/game/progression/progression.go:163`.

~~~~text
--- NEXT_GENERATION_FEATURES_PLAN.md Phase 17: "Getting Started"
checklist. A one-time (PeriodOnce - never resets) onboarding
quest chain walking a new player through the four actions this
project's own onboarding audit found were the biggest early
systems a brand-new survivor might never discover organically:
their first raid, first research upgrade, first Clan join, and
first market interaction. Deliberately reuses the exact same
trigger keys the daily/weekly board and several achievements
already use - a new player completing these also makes progress
on "first_raid"/"first_clan"/etc. achievements and any active
daily quest sharing the same trigger, for free, in the same
RecordEvent call.
~~~~


## PeriodStart is the exported form of periodStart, for display-only

Removed from `internal/game/progression/progression.go:181`.

~~~~text
PeriodStart is the exported form of periodStart, for display-only
lookups from outside this package (the /quests panel needs to query
player_quest_progress by the same period_start key RecordEvent
writes with, without duplicating the Monday-boundary math).
~~~~


## periodStart returns the boundary timestamp for a quest's current

Removed from `internal/game/progression/progression.go:189`.

~~~~text
periodStart returns the boundary timestamp for a quest's current
period - the start of today (UTC) for daily quests, the most recent
Monday 00:00 UTC for weekly quests. Used as part of
player_quest_progress's composite key, so a new period is naturally
a fresh row rather than needing a separate rollover/reset tick phase.
~~~~


## A fixed, never-advancing boundary - every PeriodOnce quest

Removed from `internal/game/progression/progression.go:196`.

~~~~text
A fixed, never-advancing boundary - every PeriodOnce quest
keys to the same single row forever, rather than a fresh row
each period the way daily/weekly quests do.
~~~~


## ISO week: Monday start. time.Weekday Sunday=0 ... Saturday=6.

Removed from `internal/game/progression/progression.go:206`.

~~~~text
ISO week: Monday start. time.Weekday Sunday=0 ... Saturday=6.
~~~~


## days since Monday

Removed from `internal/game/progression/progression.go:207`.

~~~~text
days since Monday
~~~~


## campIDFor resolves a userID's encampment id - needed to credit

Removed from `internal/game/progression/progression.go:211`.

~~~~text
campIDFor resolves a userID's encampment id - needed to credit
reward dollars via storagecap. Returns "" if not found (e.g. an AI
faction whose row was deleted, or a malformed userID) - callers
treat that as "nothing to credit" rather than an error, matching
this codebase's general "best-effort side-channel, never block the
action it's attached to" convention for achievement/notification
side effects.
~~~~


## creditReward pays dollars (storage-cap clamped, same as every other

Removed from `internal/game/progression/progression.go:224`.

~~~~text
creditReward pays dollars (storage-cap clamped, same as every other
resource-gain path in this codebase) to userID's encampment and
queues a notification. Silently does nothing if the encampment can't
be resolved.
~~~~


## only real players read notifications

Removed from `internal/game/progression/progression.go:238`.

~~~~text
only real players read notifications
~~~~


## RecordEvent is the single integration point every real action

Removed from `internal/game/progression/progression.go:243`.

~~~~text
RecordEvent is the single integration point every real action
handler calls at its own natural completion point (e.g.
doBuyListingQty after a successful purchase, doDispatchScoutMission
after dispatch, checkForFactionDefeat when crediting a real finishing
attacker). It:

 1. Increments the player's lifetime counter for trigger by amount.
 2. Unlocks any Achievements whose Threshold is now met and not
    already unlocked, crediting RewardDollars and notifying.
 3. Advances progress on any currently-active Quest sharing this
    trigger, marking it completed once GoalAmount is reached. For a
    real player (positive userID), the reward sits unclaimed until
    they tap Claim on /quests - see ClaimQuest. For an AI faction
    (negative userID, which has no panel to visit), completion
    auto-claims immediately so the reward isn't permanently stranded -
    matching NEXT_GENERATION_FEATURES_PLAN.md Phase 2's AI-parity
    note.

Errors are logged-and-swallowed by callers (this is always called
with `_ = progression.RecordEvent(...)`), matching this codebase's
established convention for notification/side-effect calls that
should never block or roll back the primary action they're attached
to.
~~~~


## claimQuestLocked grants a quest's reward and marks it claimed - the

Removed from `internal/game/progression/progression.go:325`.

~~~~text
claimQuestLocked grants a quest's reward and marks it claimed - the
shared core behind both RecordEvent's AI auto-claim path and
ClaimQuest's real-player panel path below.
~~~~

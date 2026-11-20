# ADR 0205: Decisions from `internal/game/progression/progression_test.go`

Status: Accepted

These decisions were written as comments in `internal/game/progression/progression_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## seedProgressionPlayer seeds a user + encampment + resources row

Removed from `internal/game/progression/progression_test.go:36`.

~~~~text
seedProgressionPlayer seeds a user + encampment + resources row
(needed since RecordEvent's reward crediting looks up the
encampment by user_id) with plenty of storage headroom so reward
crediting isn't accidentally clamped in a way that would make a test
assertion ambiguous between "reward wasn't paid" and "reward was
paid but clamped."
~~~~


## "first_raid" unlocks at TriggerRaidCompleted threshold 1.

Removed from `internal/game/progression/progression_test.go:74`.

~~~~text
"first_raid" unlocks at TriggerRaidCompleted threshold 1.
~~~~


## first_raid (1), raider_10 (10) both share TriggerRaidCompleted.

Removed from `internal/game/progression/progression_test.go:108`.

~~~~text
first_raid (1), raider_10 (10) both share TriggerRaidCompleted.
~~~~


## TestRecordEvent_AdvancesQuestProgressAndMarksComplete is the direct

Removed from `internal/game/progression/progression_test.go:143`.

~~~~text
TestRecordEvent_AdvancesQuestProgressAndMarksComplete is the direct
test for Phase 2: progress accumulates toward a quest's goal and
completed_at is set once the goal is reached, but the reward is NOT
auto-credited for a real player - it stays unclaimed until
ClaimQuest is called (see the next test).
~~~~


## daily_market needs 2 TriggerMarketPurchase events.

Removed from `internal/game/progression/progression_test.go:154`.

~~~~text
daily_market needs 2 TriggerMarketPurchase events.
~~~~


## Reward should NOT be auto-credited for the QUEST specifically

Removed from `internal/game/progression/progression_test.go:185`.

~~~~text
Reward should NOT be auto-credited for the QUEST specifically
before claiming - but note first_purchase (an achievement
sharing TriggerMarketPurchase, threshold 1) DOES auto-unlock and
pay its own 250 reward from the very first RecordEvent call
above, since achievement rewards are always auto-granted
(unlike quest rewards). 250 here is entirely first_purchase's
achievement payout, not any quest payout.
~~~~


## daily_market needs 2 purchases; only record 1.

Removed from `internal/game/progression/progression_test.go:245`.

~~~~text
daily_market needs 2 purchases; only record 1.
~~~~


## TestRecordEvent_AIFactionAutoClaims is the direct test for the

Removed from `internal/game/progression/progression_test.go:255`.

~~~~text
TestRecordEvent_AIFactionAutoClaims is the direct test for the
AI-parity design in NEXT_GENERATION_FEATURES_PLAN.md Phase 2: an AI
faction (negative user_id) has no /quests panel to visit, so
completing a quest auto-claims and credits the reward immediately
rather than leaving it permanently stranded.
~~~~


## TestRecordEvent_UnknownTriggerIsANoOp confirms a trigger key with no

Removed from `internal/game/progression/progression_test.go:289`.

~~~~text
TestRecordEvent_UnknownTriggerIsANoOp confirms a trigger key with no
matching achievement or quest definition doesn't error - a safe
no-op, so future call sites can pass a trigger that isn't yet used
by any definition without breaking anything.
~~~~


## TestRecordEvent_OnboardingQuestNeverResets is the direct test for

Removed from `internal/game/progression/progression_test.go:311`.

~~~~text
TestRecordEvent_OnboardingQuestNeverResets is the direct test for
NEXT_GENERATION_FEATURES_PLAN.md Phase 17's "Getting Started"
checklist: a PeriodOnce quest's period_start must be identical
across calls separated by real time (not just within the same test
run) - it's a permanent, one-time chain, not a daily/weekly one.
~~~~


## A daily/weekly period's boundary depends on time.Now(), so

Removed from `internal/game/progression/progression_test.go:318`.

~~~~text
A daily/weekly period's boundary depends on time.Now(), so
compute a second call after a small real delay and confirm
PeriodOnce's boundary didn't move - unlike PeriodDaily/PeriodWeekly,
which legitimately could (if this test happened to straddle
midnight/Monday UTC, an extremely rare flake risk those periods
already accept elsewhere in this file).
~~~~

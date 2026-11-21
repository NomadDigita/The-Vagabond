# ADR 0136: Decisions from `internal/engine/tick/aigovernance_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/aigovernance_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## seedAIClanLeader creates a clan led by the given AI faction's user_id,

Removed from `internal/engine/tick/aigovernance_test.go:10`.

~~~~text
seedAIClanLeader creates a clan led by the given AI faction's user_id,
returning the clan id - a small helper shared by the clan-war and
federation tests below, both of which require the faction to already
lead a clan (mirrors user_clans.role = 'Leader', the same gate
HandleDeclareClanWarCallback and HandleFoundFederation/HandleJoinFederation
use for humans).
~~~~


## TestGrowAICivilizationsCanDeclareClanWar verifies a Leader-role AI

Removed from `internal/engine/tick/aigovernance_test.go:28`.

~~~~text
TestGrowAICivilizationsCanDeclareClanWar verifies a Leader-role AI
faction eventually declares war on an available rival clan, mirroring
HandleDeclareClanWarCallback's exact mechanics.
~~~~


## This test needs its tracked clan to survive the whole 300-tick

Removed from `internal/engine/tick/aigovernance_test.go:36`.

~~~~text
This test needs its tracked clan to survive the whole 300-tick
loop below; zero out the independent leave/disband roll so it
can't randomly disband the clan out from under the war-declare
check being tested here (see aiClanLeaveChance's own doc comment).
~~~~


## TestGrowAICivilizationsWontDeclareWarWhenNotLeader verifies a

Removed from `internal/engine/tick/aigovernance_test.go:79`.

~~~~text
TestGrowAICivilizationsWontDeclareWarWhenNotLeader verifies a
rank-and-file AI clan member (not the Leader) never declares a war -
mirrors HandleDeclareClanWarCallback's "Leaders only" guard.
~~~~


## Zero out the independent leave/disband roll - without this, the

Removed from `internal/engine/tick/aigovernance_test.go:87`.

~~~~text
Zero out the independent leave/disband roll - without this, the
AI member can leave "Human-Led Clan" partway through the loop
and later found or join an entirely different clan as Leader,
whose own legitimate war declaration could then involve the
still-existing (but now AI-vacated) original clan as the
opponent, which the warCount check below can't distinguish from
the actual behavior under test. See aiClanLeaveChance's comment.
~~~~


## TestGrowAICivilizationsCanFoundOrJoinFederation verifies a Leader-role

Removed from `internal/engine/tick/aigovernance_test.go:137`.

~~~~text
TestGrowAICivilizationsCanFoundOrJoinFederation verifies a Leader-role
AI faction with enough Crystal eventually gets its clan into a
federation - either by founding one (HandleFoundFederation's mechanics)
or joining an existing one (HandleJoinFederation's mechanics).
~~~~


## 500 iterations turned out to be statistically marginal: only the

Removed from `internal/engine/tick/aigovernance_test.go:161`.

~~~~text
500 iterations turned out to be statistically marginal: only the
"found" branch of the 1%-chance/tick roll can ever succeed in this
test (there's no other federation for the 50% "join" branch to
find), so the real per-tick success probability is ~0.5%, giving
500 tries roughly an 8% false-failure rate - caught by a shuffled,
repeated-count test run, not a rare fluke worth ignoring. 3000
iterations brings the false-failure rate below one in a million.
~~~~


## Replenish crystal each round so the founding path stays

Removed from `internal/engine/tick/aigovernance_test.go:181`.

~~~~text
Replenish crystal each round so the founding path stays
available even after a listing/buying roll spent some.
~~~~


## TestGrowAICivilizationsCanQueueForArena verifies a funded AI faction

Removed from `internal/engine/tick/aigovernance_test.go:195`.

~~~~text
TestGrowAICivilizationsCanQueueForArena verifies a funded AI faction
eventually joins the arena queue, mirroring HandleJoinQueueCallback's
exact mechanics (entry fee debited, arena_queue row inserted).
~~~~


## TestGrowAICivilizationsCanDisbandClanAsLeader is the direct test for

Removed from `internal/engine/tick/aigovernance_test.go:239`.

~~~~text
TestGrowAICivilizationsCanDisbandClanAsLeader is the direct test for
the AI clan leave/disband gap the project owner flagged 2026-08-09:
a Leader-role AI faction must eventually disband a clan it leads
(DELETE FROM clans, cascading to user_clans) rather than being
permanently stuck leading it forever.
~~~~


## TestGrowAICivilizationsCanDisbandClanAsLeader_NotifiesRealMembers

Removed from `internal/engine/tick/aigovernance_test.go:285`.

~~~~text
TestGrowAICivilizationsCanDisbandClanAsLeader_NotifiesRealMembers
confirms a real human member of an AI-led clan is notified when
their AI Leader disbands it - the clan disappearing out from under
them should never be silent.
~~~~


## TestGrowAICivilizationsCanLeaveClanAsMember confirms a rank-and-file

Removed from `internal/engine/tick/aigovernance_test.go:338`.

~~~~text
TestGrowAICivilizationsCanLeaveClanAsMember confirms a rank-and-file
(non-Leader) AI clan member eventually leaves without touching the
clan itself - the clan should survive with the human Leader still in
charge.
~~~~


## TestGrowAICivilizationsReviewsApplicationsToLedClan is the direct

Removed from `internal/engine/tick/aigovernance_test.go:393`.

~~~~text
TestGrowAICivilizationsReviewsApplicationsToLedClan is the direct
test for the related "applications to an AI-led clan sit pending
forever" gap found alongside the leave/disband fix: an AI Leader
must eventually accept a pending real-player application to its
clan, rather than nothing ever reviewing it.
~~~~


## Same fragility as the declare-war/federation tests above: zero

Removed from `internal/engine/tick/aigovernance_test.go:403`.

~~~~text
Same fragility as the declare-war/federation tests above: zero
out the independent leave/disband roll so the Leader's clan
can't be randomly disbanded before it gets a chance to review
the pending application this test is actually checking.
~~~~


## TestGrowAICivilizationsCraftsAndEquipsGear is the direct test for

Removed from `internal/engine/tick/aigovernance_test.go:459`.

~~~~text
TestGrowAICivilizationsCraftsAndEquipsGear is the direct test for
NEXT_GENERATION_FEATURES_PLAN.md Phase 3's AI-parity crafting
heuristic: an AI faction with enough resources eventually crafts and
equips a Common/Rare gear item on its own, without ever visiting a
/gear panel.
~~~~

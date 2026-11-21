# ADR 0092: Decisions from `internal/bot/handlers/onboarding.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/onboarding.go` and moved here so the source no longer carries them. Each entry is the original comment.

## generateReferralCode derives a referral code directly from a player's

Removed from `internal/bot/handlers/onboarding.go:28`.

~~~~text
generateReferralCode derives a referral code directly from a player's
Telegram ID. Because Telegram IDs are themselves unique, base36-encoding
the ID guarantees a collision-free code with no DB round-trip or retry
loop needed (unlike the old `telegramID % 1_000_000` scheme, which could
map two different players to the same code).
~~~~


## referralMilestones defines one-time bonus payouts unlocked once a

Removed from `internal/bot/handlers/onboarding.go:37`.

~~~~text
referralMilestones defines one-time bonus payouts unlocked once a
referrer's total referral count reaches the given threshold, on top of
the standard per-referral reward.
~~~~


## topUpAllResources adds amount to all nine resources for a camp,

Removed from `internal/bot/handlers/onboarding.go:49`.

~~~~text
topUpAllResources adds amount to all nine resources for a camp,
respecting the storage cap (via storagecap.Clamp) the same way any
other post-creation resource grant does - unlike the initial Welcome
Pack insert, which deliberately starts a brand-new player above their
tiny starting cap and is never clamped.
~~~~


## Brand new user - capture a referral code from the /start payload if

Removed from `internal/bot/handlers/onboarding.go:177`.

~~~~text
Brand new user - capture a referral code from the /start payload if
present (e.g. /start REF123456), before the faction picker.
~~~~


## nameChangeCostCrystal and nameChangeCostDollars are deliberately steep -

Removed from `internal/bot/handlers/onboarding.go:215`.

~~~~text
nameChangeCostCrystal and nameChangeCostDollars are deliberately steep -
SpaceHunt's /name command is a rare, deliberate vanity purchase, not
something players do casually.
~~~~


## HandleRenameOutpost implements SpaceHunt's "Change your username"

Removed from `internal/bot/handlers/onboarding.go:221`.

~~~~text
HandleRenameOutpost implements SpaceHunt's "Change your username"
feature. The new name applies everywhere the player is identified:
battle reports, the Global Ranking board, World Boss/Rebellion
leaderboards, and raid targeting.
~~~~


## Every player gets a referral code the moment their account is

Removed from `internal/bot/handlers/onboarding.go:344`.

~~~~text
Every player gets a referral code the moment their account is
created (or upgraded from 'onboarding' to 'active'), instead of
waiting for them to first run /refer. COALESCE on conflict means
an existing code (e.g. set by an earlier session) is never
clobbered.
~~~~


## Temporary placeholder only - HandleOnboardingPendingInput

Removed from `internal/bot/handlers/onboarding.go:379`.

~~~~text
Temporary placeholder only - HandleOnboardingPendingInput
(users.state = 'naming', set below) overwrites this with the
player's own chosen name before they see anything else. Never
player-facing as a "final" name.
~~~~


## Welcome Pack: every brand-new, non-referred survivor starts

Removed from `internal/bot/handlers/onboarding.go:394`.

~~~~text
Welcome Pack: every brand-new, non-referred survivor starts
with 25,000 of all nine resources - a flat, equal baseline
across the board rather than the old lopsided split (1,000
Scrap/50 Rations/250 Electricity with everything else in the
low hundreds). Referred players effectively double this (see
the referral-bonus block below), since it tops up the same
baseline by another 25,000 of everything. The small per-
faction bonus below is a flavor differentiator layered on top
of the shared baseline, not a replacement for it.
~~~~


## 7-day Cognitive Agent (Premium) trial for every new survivor.

Removed from `internal/bot/handlers/onboarding.go:434`.

~~~~text
7-day Cognitive Agent (Premium) trial for every new survivor.
This only ever runs inside the isNewCamp guard, so it can't be
replayed to keep extending someone's trial.
~~~~


## Gate on naming: the very first thing a brand-new survivor does

Removed from `internal/bot/handlers/onboarding.go:439`.

~~~~text
Gate on naming: the very first thing a brand-new survivor does
after picking a faction is name their outpost (see
HandleOnboardingPendingInput), before they see their resources,
location, or anything else. state flips back to 'active' once
naming completes.
~~~~


## Referral reward: if this new survivor arrived via a referral code,

Removed from `internal/bot/handlers/onboarding.go:451`.

~~~~text
Referral reward: if this new survivor arrived via a referral code,
grant both them and their referrer a resource bonus. Gated on
isNewCamp so this can only ever fire once per player — previously
this ran on every HandleFactionCallback invocation, letting anyone
replay the still-live "join faction" button to farm free resources
indefinitely.
~~~~


## referralBonusPerResource tops up all nine resources by the same

Removed from `internal/bot/handlers/onboarding.go:460`.

~~~~text
referralBonusPerResource tops up all nine resources by the same
amount as the base Welcome Pack (startingBaseline above), so a
referred player's total starting stake becomes a clean 2x
(50,000) of every resource rather than the old flat 50,000
Metal / 500 Crystal / 50,000 Neuro Cores special-case, which
left most resources untouched and wildly overshot others
(Metal/Neuro would've ended up at 3x once stacked on top of
the now-25,000 baseline, Crystal barely above 1x). Applied to
the referrer too, as their reward for bringing in a new
survivor - not meant to double their whole economy, just a
flat top-up like any other referral bonus.
~~~~


## gettingStartedGuideText is the step-by-step quickstart, shared between the

Removed from `internal/bot/handlers/onboarding.go:537`.

~~~~text
gettingStartedGuideText is the step-by-step quickstart, shared between the
first-spawn welcome message and the standalone /guide command so a player
can always call it back up later.
~~~~


## outpostNameRegex validates any player-chosen outpost name - both the

Removed from `internal/bot/handlers/onboarding.go:558`.

~~~~text
outpostNameRegex validates any player-chosen outpost name - both the
mandatory first-naming prompt below and the paid /name rename command
(HandleRenameOutpost) share this exact rule.
~~~~


## sendNamingPrompt is the very first thing a brand-new survivor sees

Removed from `internal/bot/handlers/onboarding.go:563`.

~~~~text
sendNamingPrompt is the very first thing a brand-new survivor sees
after picking a faction - before their resources, location, or
anything else. Also re-sent from HandleStart if someone abandons
onboarding mid-naming and comes back via /start.
~~~~


## HandleOnboardingPendingInput consumes a plain-text reply as the

Removed from `internal/bot/handlers/onboarding.go:576`.

~~~~text
HandleOnboardingPendingInput consumes a plain-text reply as the
player's chosen outpost name, but only while users.state = 'naming'
(set by HandleFactionCallback the moment a brand-new camp is
created). Registered in cmd/bot/main.go's OnText chain, same pattern
as admin.go's HandleAdminPendingInput: returns handled=false
immediately for anyone not mid-naming, so it's a no-op for every
other message in the game.
~~~~


## sendWelcomeComplete fires once, right after a brand-new survivor

Removed from `internal/bot/handlers/onboarding.go:624`.

~~~~text
sendWelcomeComplete fires once, right after a brand-new survivor
finishes the mandatory naming step above: it reveals their faction,
resources, and where in the wastes they've settled (a named town and
country within their assigned continent - see spawnlocation.go's
flavorLocation - rather than raw coordinates or a bare continent
name).
~~~~

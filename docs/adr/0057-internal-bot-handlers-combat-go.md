# ADR 0057: Decisions from `internal/bot/handlers/combat.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/combat.go` and moved here so the source no longer carries them. Each entry is the original comment.

## HandleRaidBoardPageCallback fires when a player taps "▶️ More" or

Removed from `internal/bot/handlers/combat.go:64`.

~~~~text
HandleRaidBoardPageCallback fires when a player taps "▶️ More" or
"◀️ Back" on the Tactical Target Matrix. 2026-08-06 fix: the matrix
used to hardcode `ORDER BY d.last_seen_at DESC LIMIT 5`, so a player
who had scouted more than 5 targets could never see anything past
their 5 most-recently-seen ones - older-but-still-valid discoveries
just silently vanished from the board with no way to reach them. This
paginates instead of truncating: page is carried in callback_data (see
cmd/bot/main.go's "\fraid_board_page" registration) and every older
discovery stays reachable via ◀️/▶️.
~~~~


## sendRaidBoardPage renders one page of the Tactical Target Matrix.

Removed from `internal/bot/handlers/combat.go:85`.

~~~~text
sendRaidBoardPage renders one page of the Tactical Target Matrix.
edit controls whether the result replaces the message the callback
came from (page nav) or is sent fresh (initial /raid open) - matches
the c.Edit vs c.Send split used by every other paginated panel's
callback in this codebase.
~~~~


## Co-Op lobbies and world-contact sections are page-1-only - they

Removed from `internal/bot/handlers/combat.go:217`.

~~~~text
Co-Op lobbies and world-contact sections are page-1-only - they
aren't part of the paginated discovery list and would otherwise
repeat identically on every page.
~~~~


## The Rogue Drone Nest has no real encampments row and no real

Removed from `internal/bot/handlers/combat.go:258`.

~~~~text
The Rogue Drone Nest has no real encampments row and no real
coordinate (see worldintel.RogueDroneNestKey's doc comment) -
this used to hardcode a fake "(Sector 1,1)" here regardless of
the player's actual position, which was just wrong information
dressed up as a coordinate. Say what it actually is instead.
~~~~


## Speedup/Abort per active expedition gets pulled into its own small

Removed from `internal/bot/handlers/combat.go:316`.

~~~~text
Speedup/Abort per active expedition gets pulled into its own small
colored message instead of unstyled buttons mixed into the shared
HUD - see keyboards/styled.go.
~~~~


## Road encounters: pending Attack/Continue decisions for any of this

Removed from `internal/bot/handlers/combat.go:395`.

~~~~text
Road encounters: pending Attack/Continue decisions for any of this
commander's moving expeditions. The queued notification only tells
them to check here, since plain-text notifications can't carry
inline buttons.
~~~~


## Road-contact Attack/Continue decisions get pulled out of the

Removed from `internal/bot/handlers/combat.go:400`.

~~~~text
Road-contact Attack/Continue decisions get pulled out of the
shared panel keyboard and sent as their own small, colored
messages below (see the end of this function) - Telegram Bot
API 9.4 (Feb 2026) added a real "style" field for exactly this
kind of binary choice (danger/success/primary), and a genuine
Attack-vs-Continue decision is the clearest possible case for it
in this game. See keyboards/styled.go for how this works without
needing a telebot.v3 library upgrade.
~~~~


## Road-vs-base encounters: same idea as the road_encounters block

Removed from `internal/bot/handlers/combat.go:453`.

~~~~text
Road-vs-base encounters: same idea as the road_encounters block
above, but the other side is a passive base rather than a second
expedition, so only this commander gets an Attack/Continue choice.
~~~~


## Real remaining time until resolve_time - the same field

Removed from `internal/bot/handlers/combat.go:533`.

~~~~text
Real remaining time until resolve_time - the same field
resolvePendingEspionageMissions actually checks - not a
fixed 120s window from created_at. The outbound scan and
return-flight legs are both distance-based (see
HandleSpyCallback's steps*1.5-minutes-per-tile formula)
and can genuinely take many hours for a long-distance
target; showing a fake 2-minute countdown made this look
permanently stuck at "0s remaining" long before the real
flight was anywhere close to done.
~~~~


## Sent as separate messages, after the main HUD, rather than folded

Removed from `internal/bot/handlers/combat.go:624`.

~~~~text
Sent as separate messages, after the main HUD, rather than folded
into its keyboard: each is its own real decision with a deadline,
and Bot API 9.4's colored buttons only work via the raw send path
(see keyboards/styled.go) - they can't be mixed into the same
telebot.ReplyMarkup.Inline() call as the HUD's other buttons.
~~~~


## HandleAutoScanToggle toggles the SpaceHunt-style "Automatic Scan" job:

Removed from `internal/bot/handlers/combat.go:642`.

~~~~text
HandleAutoScanToggle toggles the SpaceHunt-style "Automatic Scan" job:
when enabled, the tick engine periodically runs a lightweight scan
against a random rival outpost and reports it directly to the player,
without them needing to manually run /scout each time.
HandleReconAICallback lets a player scout the Rogue Drone Nest before
committing forces. Since the nest's composition is deterministic (scaled
to the player's own level via content.RogueNestComposition), this
reveals EXACTLY what they'll face - no surprises, no wasted losses from
blind attacks, matching a real scouting report rather than a guess.
~~~~


## Enforce the intercept window: the deadline is whatever resolve_time

Removed from `internal/bot/handlers/combat.go:967`.

~~~~text
Enforce the intercept window: the deadline is whatever resolve_time
currently represents (outbound arrival before the tick engine flips
`resolved`, or the return-to-orbit deadline afterward). Without this
check the satellite stays interceptable indefinitely, even long
after it should have safely landed.
~~~~


## BUGFIX: the Observer unit's own flavor text promises it "boosts

Removed from `internal/bot/handlers/combat.go:996`.

~~~~text
BUGFIX: the Observer unit's own flavor text promises it "boosts
counter-espionage odds when stationed at home" (internal/game/content/units.go),
but this formula never referenced observers at all - the unit's
stated purpose was simply never wired in. 3% per Observer, capped
at 30%, mirrors the same shape as its (separate, smaller) raid
early-warning bonus elsewhere, just weighted higher since
counter-espionage is this unit's actual primary purpose.
~~~~


## startRaidDraft is the shared core behind both HandleLaunchRaidCallback

Removed from `internal/bot/handlers/combat.go:1163`.

~~~~text
startRaidDraft is the shared core behind both HandleLaunchRaidCallback
(a player tapping a target in the /raid matrix) and
HandleNLPLaunchRaid below (a player typing "raid <name>") - 2026-08-05
extraction, so a natural-language launch opens the exact same,
already-proven draft/launch-style picker a manual tap does, rather
than duplicating this discovery-check-and-draft-reset logic. Nothing
is actually committed here: this only resets campaign_drafts'
composition to 0 for the given target and shows the customizer HUD -
the player still has to choose their own force composition and tap
one of the three Launch buttons (which go through the unmodified,
unchanged HandleConfirmHangarLaunchCallback) to actually send anyone
anywhere. That's the real confirmation step for this whole flow, by
design - deliberately not touched or reimplemented by this addition.
~~~~


## resolveRaidTargetByName searches the caller's OWN discovered targets

Removed from `internal/bot/handlers/combat.go:1220`.

~~~~text
resolveRaidTargetByName searches the caller's OWN discovered targets
(never the global encampment table - see the same
encampment_discoveries scoping startRaidDraft re-checks below) for an
outpost name or commander first name matching a case-insensitive
partial match of name, for the natural-language "raid <name>"
command. Returns the matched target's encampment ID and a
human-readable label. If zero or more than one target matches, an
error is returned instead of guessing - raiding is a real commitment
of a player's own forces, so an ambiguous or unmatched name should
never silently resolve to some target the player didn't actually
mean.
~~~~


## HandleNLPLaunchRaid is nlpcommand.ActionLaunchRaid's execution

Removed from `internal/bot/handlers/combat.go:1264`.

~~~~text
HandleNLPLaunchRaid is nlpcommand.ActionLaunchRaid's execution
(2026-08-05 addition, by explicit project-owner request - "launching
raids on this particular person"). No Confirm/Cancel card, matching
the existing manual flow's own UX exactly: tapping a target in the
/raid matrix has never itself required a confirmation, because
nothing is committed by opening the draft board - see
startRaidDraft's doc comment for why the real confirmation is the
mandatory next step (choosing a composition and tapping Launch),
which this does not skip or shortcut.
~~~~


## Manual Defense Garrison: units reserved as permanent home defense

Removed from `internal/bot/handlers/combat.go:1308`.

~~~~text
Manual Defense Garrison: units reserved as permanent home defense
are never presented as draftable, protecting them from being sent
out on a raid by accident.
~~~~


## HandleAddDraftCommand implements "/add <n> <unit>" - a bulk text

Removed from `internal/bot/handlers/combat.go:1459`.

~~~~text
HandleAddDraftCommand implements "/add <n> <unit>" - a bulk text
shortcut for the same draft adjustment the +/- buttons perform, so a
player doesn't have to tap a button 100 times to draft 100 Soldiers.
~~~~


## BUGFIX: Nukes mobilized into a marching raid (nukes_mobilized on

Removed from `internal/bot/handlers/combat.go:1577`.

~~~~text
BUGFIX: Nukes mobilized into a marching raid (nukes_mobilized on
raid_forces) were never actually used anywhere in raid combat
resolution - they contributed zero attack power, never appeared in
the battle report, and always returned home 100% intact regardless
of outcome. Nukes' real, fully-implemented mechanic is the
Strategic Silo's direct-launch strike (HandleLaunchICBMCallback) -
interception odds, Nuclear Shields, structural damage, the works.
Rather than inventing a second, redundant "nuke joins the raid"
mechanic from scratch, retire the dead one and point players at
the real one.
~~~~


## Manual Defense Garrison: reserved units are excluded from the

Removed from `internal/bot/handlers/combat.go:1620`.

~~~~text
Manual Defense Garrison: reserved units are excluded from the
enforced draftable maximum, not just hidden from display.
~~~~


## Bulk selection: cycle the persisted step size (x1 -> x10 -> x100 ->

Removed from `internal/bot/handlers/combat.go:1643`.

~~~~text
Bulk selection: cycle the persisted step size (x1 -> x10 -> x100 ->
MAX -> x1...) instead of adjusting a specific unit type. -1 is the
MAX sentinel: inc/dec on any unit then jumps straight to its cap or
to zero, rather than moving a fixed amount.
~~~~


## The target matrix is only a convenience view. Re-check discovery in

Removed from `internal/bot/handlers/combat.go:1811`.

~~~~text
The target matrix is only a convenience view. Re-check discovery in
the launch transaction so a stale or forged callback cannot turn a
globally-known ID into an unauthorised raid.
~~~~


## Phase 7 (item 12): per-continent world events. myRegion (the

Removed from `internal/bot/handlers/combat.go:1832`.

~~~~text
Phase 7 (item 12): per-continent world events. myRegion (the
attacker's own outpost region) is already resolved above.
~~~~


## Phase 7 (item 11): diplomacy. An active alliance or non-aggression

Removed from `internal/bot/handlers/combat.go:1884`.

~~~~text
Phase 7 (item 11): diplomacy. An active alliance or non-aggression
pact between the attacker's and defender's Clans blocks the raid
outright, mirroring how clan membership already protects clanmates.
~~~~


## Was previously `_ = ...Scan(...)`, silently discarded - which is

Removed from `internal/bot/handlers/combat.go:2001`.

~~~~text
Was previously `_ = ...Scan(...)`, silently discarded - which is
exactly how the two-INSERT-statements-concatenated-with-no-
separator bug this replaces went unnoticed: a guaranteed SQL
syntax error aborted the transaction (rolling back the resource/
unit deduction above), yet the code continued straight through
to a "campaign launched" success animation regardless. See
BUGS_AND_INCONSISTENCIES.md for the full writeup.
~~~~


## A speed-up pulls the arrival clock forward, so it must also pull

Removed from `internal/bot/handlers/combat.go:2136`.

~~~~text
A speed-up pulls the arrival clock forward, so it must also pull
the route-leg clock forward by the same 30 minutes, or the next
route-progress read would still show the column crawling along
its old (slower) pace while its actual arrival time jumped.
~~~~

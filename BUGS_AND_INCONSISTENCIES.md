# Bugs & Inconsistencies Found — UI Polish Wave 1 (2026-07-26)

Found while auditing handlers for rich-text formatting. No gameplay logic
was changed as part of this pass — this is a log of things worth fixing
next, grouped by how serious they are.

## Real bugs (affect what players see or can do)

1. **Heavy Workshop panel shows a fake outpost name.**
   `internal/bot/handlers/factory.go`, `HandleFactoryPanel` — the panel
   hard-codes `"Outpost Name: Military Engineering Hangar"` instead of
   querying the player's actual encampment name (every other sector
   panel — Camp, Sector Map — does the real lookup). Every player sees
   the same placeholder name.

2. **No Telegram parse mode was set anywhere prior to this commit.**
   All 39 handler files sent plain text. This isn't a player-visible
   "bug" exactly, but it means none of the intentional-looking
   formatting (dashes as pseudo-headers, `[brackets]` as pseudo-buttons)
   was ever rendering as anything other than literal characters — the
   game has been running with strictly plain-text output since launch.
   Fixed for the 10 panels touched in this commit; ~28 files still need
   the same treatment (see PROJECT_MASTER_PLAN.md follow-up note).

3. **Handler alias inconsistency: `factory.go` imports telebot as `gopkg`
   instead of the `telebot` name every other handler uses.** Not a bug,
   but a real trap for future edits/merges — a careless copy-paste of a
   `telebot.X` reference into factory.go, or a `gopkg.X` reference out
   of it, fails to compile in a way that's easy to misdiagnose.

4. **Promoting a clan member sets a role that nothing else recognizes.**
   ~~`clan.go`, `HandlePromoteMemberCallback` sets `role = 'Co-Leader'`,
   but every permission check in the file (`HandleGuildIcon`,
   `HandleGuildDescription`, `HandleDeclareClanWarCallback`,
   `HandleKickMemberCallback`, etc.) checks `role == "Leader"` exactly.~~
   **Fixed in UI Polish Wave 6.** Added a `canManageClan(role)` helper
   (`role == "Leader" || role == "Co-Leader"`) and wired it into the
   Applications Inbox, Accept/Reject, Manage Members roster access,
   Kick, Guild Icon, and Guild Description checks. Co-Leader is
   deliberately narrower than Leader, not identical: a Co-Leader can
   only Kick a `Soldier` (not the Leader or another Co-Leader, checked
   server-side in `HandleKickMemberCallback`, not just hidden in the
   UI), cannot Promote anyone, and cannot Declare War or rename the
   clan (those stay Leader-only - one is a 48h commitment for the
   whole clan, the other spends Crystal). Promoting someone is a real
   power grant now, not a cosmetic label.

5. **Notification dispatcher had no guard against synthetic (AI-faction)
   user IDs.** Surfaced while reviewing the Phase 6 AI decision loop
   (`internal/engine/tick/aidecisions.go`, merged by a parallel session -
   see `AI_FACTION_DECISION_LOOP_PLAN.md`/`AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md`),
   which is the first time an AI faction can ever be a raid's
   `attacker_id` rather than only ever a defender/target. AI factions'
   seeded `users` rows use a synthetic *negative* `telegram_id` (see
   `seedAICivilizations`), which no real Telegram chat exists for.
   `notifications.Queue()`/the raw `INSERT INTO notifications` call
   sites have no check for this, and the dispatcher's `drainQueue()`
   silently `continue`s past a failed send without ever marking that row
   `is_sent` - so a notification queued for an AI faction wouldn't just
   fail once, it would retry forever, permanently wasting one of the
   dispatcher's `LIMIT 10` drain slots every 3s poll. The AI decision
   loop's own raid-launch code already guards its *first* "target found"
   notification (`if !target.isAIFaction`), but every downstream
   attacker-facing site in `resolveRaidCombats`, `discoverRouteContacts`,
   `evaluateRoadEncounters`, and `evaluateRoadBaseEncounters` - all
   written back when "attacker" always meant a real player - was still
   unguarded, and an AI-launched raid flows through every one of them.
   **Fixed:** added `isRealPlayer(userID int64) bool { return userID >
   0 }` in `engine.go` (real Telegram IDs are always positive) and
   wrapped all seven of those call sites plus the one in
   `roadbaseencounter.go`.

6. **CRITICAL, LIVE BUG (now fixed): every solo raid launch was silently
   broken.** `HandleConfirmHangarLaunchCallback` in `combat.go` - wired to
   the real "🚀 Launch Direct/Stealth/Safe" buttons via
   `bot.Handle("\fconfirm_launch", ...)` - built its `raids` INSERT as two
   `INSERT INTO raids (...) VALUES (...)` clauses concatenated with no
   semicolon or separator, a guaranteed Postgres syntax error (extended-
   protocol `Parse` messages can't contain more than one statement). The
   error was silently discarded (`_ = tx.QueryRowContext(...).Scan(...)`),
   which aborted the entire transaction on execution - rolling back the
   resource/unit deduction that happened earlier in the same `tx` - while
   the handler still sent a full "campaign launched" success animation
   regardless. Net effect: tap Launch, watch the animation, nothing
   actually happens - no raid, resources untouched (rolled back), no
   error shown. This was previously found and documented as *dormant*
   (unreachable, since nothing called this function) - it is not dormant;
   it's the main solo-raid dispatch path. **Fixed:** merged into one
   valid INSERT (kept the `leg_started_at`/`leg_total_minutes` columns
   the tick engine actually reads for position tracking -
   `route_progress`/`route_progress_at`/`route_leg_minutes` already
   default/COALESCE to sane values everywhere they're read, confirmed by
   tracing every read site) and stopped discarding the insert/commit
   errors, so a genuine failure now tells the player instead of lying to
   them.

7. **The co-op raid-launch path had the identical missing-columns gap.**
   `autoLaunchExpiredStagedRaids` (the tick pass that promotes a staged
   co-op raid to `state = 'marching'`) only ever set `state` and
   `resolve_time` - never `origin_x/origin_y/destination_x/destination_y/
   origin_region/destination_region/leg_started_at/leg_total_minutes`.
   Both `evaluateRoadEncounters` and `evaluateRoadBaseEncounters` filter
   on `origin_x IS NOT NULL`, so a co-op raid launched this way was
   invisible to the entire Phase 3/4/5 road-encounter/weather system.
   **Fixed** to populate the same columns the solo path (item 6 above)
   now does.

8. **Repo-wide follow-up audit on the `isRealPlayer` notification guard
   (item 5 above): found several more real gaps it missed.** Went
   through essentially every `INSERT INTO notifications`/
   `notifications.Queue` call site in `internal/engine/tick` (~50 of
   them) checking whether the `userID` involved could ever be an AI
   faction's synthetic negative ID. Fixed: `applyActiveLogisticsConsumption`'s
   8 rations/ammo/electricity/logistics threshold alerts (an AI-launched
   raid's supplies deplete through this same shared function); the
   espionage target-side notification in `resolvePendingEspionageMissions`
   (AI factions are valid spy targets) plus the direct synchronous
   Telegram `Send` in `HandleSpyCallback` (added a package-local
   `isRealPlayer` in `internal/bot/handlers` - can't share the tick
   package's unexported one across packages); weather/route-incident
   pause alerts; the raid radar proximity warning; reciprocal
   route-contact discovery (a base can reciprocally discover the
   expedition that discovered it, and that base can be an AI faction);
   and the top-3 leaderboard tax-payout notification, now reachable by AI
   factions since the AI-parity leaderboard feature landed. **Also
   found something older and more impactful than any of the above**: the
   defender-side checks in `resolveRaidCombats` used `r.defenderUserID !=
   0` rather than `> 0` - `!= 0` does *not* exclude AI factions' negative
   synthetic IDs, so this specific gap has existed since Phase 6's
   foundational tier first shipped (AI factions have always been valid
   raid *targets*, long before any AI decision loop existed to make them
   attackers too). Fixed both sites the same way. Cross-checked several
   more `!= 0` guards (world boss, clan war, cognitive agent, exploration
   dispatch, co-op lobby, arena, idle-miner) and confirmed those are
   genuinely player-only features AI factions can't currently reach -
   left as-is rather than churning code with no behavioral effect.

## Confirmed still-open items from prior sessions (re-verified today)

4. **`coordinates.danger_level` is stored but never read by any game
   system** — confirmed again: the only reference in the entire repo is
   the struct tag in `internal/models/models.go`. Nothing sets combat
   difficulty, encounter rates, or AI behavior from it. Either wire it
   up or remove the column/field to stop it looking load-bearing.

5. **~~Guild roles are still binary (`Leader` / `Soldier`)~~ — no longer
   accurate.** This note predates UI Polish Wave 6 (`clan.go`'s
   `canManageClan()` helper): Co-Leader is now a real, narrower-than-
   Leader permission tier (Applications, Kick-a-Soldier-only, Guild
   Icon/Description), not a cosmetic label. See item 4 above.

## Cosmetic/UX inconsistencies fixed in this pass

- Section headers across panels used three different visual styles
  (full-width `━━━` box borders, bracketed `[Title]`, or plain
  `TITLE:` with a colon) with no shared convention. Standardized the
  10 touched panels on a lighter `divider` + bold-header pattern that
  also now actually renders as bold instead of literal text.
- Numeric readouts (HP, resource counts, coordinates) were mixed
  plain digits and decorated strings inconsistently; touched panels
  now consistently monospace these via `<code>` so columns stay
  aligned.

## Not yet audited (flagging so nothing gets assumed "checked")

Waves 2-3 covered: battlereport.go (shared raid + World Boss combat
report renderer), the notifications dispatcher (auto-detects HTML now),
combat.go's Target Matrix + Expedition Panel, silo.go's full ICBM/
Piercing Missile flow, exchange.go, deconstruct.go, and the highest-
traffic tick-engine notifications (return-march loot, campaign
engagement, boss loot, clan war, arena, espionage).

Wave 4 covered: all 9 AI-advisor `FormatForTelegram` outputs (Governor,
Fleet Commander, Economy/Research/Battle/Guild/Galaxy/NPC advisors, Dev
Console).

Wave 5 covered: clan.go's full guild-panel UI (not just its
notification calls) - and surfaced bug #4 above, since fixed in wave 6.

Wave 6 covered: research.go, ether.go, crystal_exchange.go, rebellion.go, jobs.go, agent.go - plus the bug #4 Co-Leader permission fix itself (see above). `jobs.go`'s two cooldown messages (`HandleTeleport`, `HandleGatherSunlight`) were deliberately left plain - low-value one-liners, not worth the churn.

Wave 7 covered: admin.go (panel activation, guided-input prompts, both the
inline-callback and standalone /admin_metrics variants of the server
metrics panel, the two-tap db-reset confirmation flow, and every
gift/tax/faction/broadcast confirmation + player-facing notification) and
combat_road_encounters.go (road battle/skirmish reports for both
commanders on both sides, convoy dispatch, break-camp-early, and every
"resolved without engaging" / "column destroyed" notification). Escaped
admin-supplied free text (broadcast body, gifted usernames) and
user-authored encampment names (`ar.attackerName`/`ar.defenderName` in
admin.go's DB-reset abort alerts, `attackerName` in the road-skirmish
defender report) since these now carry HTML tags and pass through the
notification dispatcher's auto-detect-HTML path (see wave 2-3 note above)
- previously safe as plain text, these would now risk a "can't parse
entities" 400 error on a raw `&`/`<`/`>` in a player's chosen name.

Infra note: this sandbox can install a real Go toolchain
(`apt-get install golang-1.22-go`) - a capability prior sessions didn't
have. The default module proxy (proxy.golang.org) is still blocked by
the network allowlist, but `GOPROXY=direct` plus a temporary
`replace gopkg.in/telebot.v3 => github.com/go-telebot/telebot/v3 v3.3.8`
directive in go.mod (github.com and codeload.github.com are allowlisted)
lets `go build`/`go vet`/`go test` run for real. That replace line is
build-verification-only and must never be committed - go.mod/go.sum are
restored to their original state after every verification pass. All 203+
tests plus a full `go build ./...` and `go vet ./...` passed clean against
both wave 7 files using this method.

Still plain text / not yet audited: diplomacy.go, federation.go,
starvation.go (the tick notifications in `internal/engine/starvation`,
not a handler file), the remaining ~15 tick-engine notifications (tax
collection, exploration discovery, ETA/proximity alerts), and world.go.
Recommend diplomacy.go and federation.go as wave 8 (the two largest
remaining unaudited handler files at 294 and 267 lines).

Wave 8 covered: diplomacy.go and federation.go. Both rich-formatted;
escaped user-authored Clan names (`otherName`/`myClanName`/`targetName`
in diplomacy.go) and Federation names (`name`/`fedName` in
federation.go - federations only ever get a player-chosen `name` at
founding time, `icon`/`description` are DB defaults per migration 014
and never player-set, but `description` was escaped anyway as a
defense-in-depth measure since nothing currently prevents a future
change from letting it be set). Also fixed a real (if purely cosmetic)
terminology inconsistency found during the audit: both files called
the Clan's top role "King"/"Clan King" throughout (player-facing text,
code comments, and the `isKing` variable name) while `clan.go` - the
actual clan/guild system - consistently calls the exact same role
(`clans.leader_id`) "Leader" everywhere. Renamed every occurrence in
both files to "Leader"/`isLeader` for consistency; the underlying
`leader_id == userID` check was already correct and is unchanged.

Still plain text / not yet audited: starvation.go (tick notifications
in `internal/engine/starvation`, not a handler file), the remaining
~15 tick-engine notifications (tax collection, exploration discovery,
ETA/proximity alerts), and world.go. No large unaudited handler files
remain above ~250 lines - recommend wave 9 target the tick-engine
notification strings directly (grep `internal/engine/` for
`INSERT INTO notifications` / `notifications.Queue` calls not already
covered by a polished handler file) rather than another whole-file
pass.

Wave 9 covered a first pass of the tick-engine notification strings
(as recommended above): `collectDailyTax`'s Top-3 payout alert, the
full march-supply-depletion cluster in `resolveExpeditionRationsAndAmmo`
(rations/ammo/electricity/logistics threshold warnings, column-halted,
high-tech-offline - 10 sites), the radar proximity warning, and the
salvage/return-march ETA notification in `internal/engine/tick/engine.go`;
plus the two road-base-encounter notifications and the
"resolved without engaging" alert in
`internal/engine/tick/roadbaseencounter.go`; plus one notification each
in `internal/engine/agent/agent.go` (agent deactivation, agent
auto-upgrade) and `internal/engine/starvation/starvation.go` (starvation
desertion). Since `internal/engine/*` packages can't import
`internal/bot/handlers` (would create an import cycle / breaks the
package-per-feature convention), added local `htmlEscapeAgent`/
`htmlBoldAgent`/`htmlCodeAgent` and `htmlEscapeStarvation`/
`htmlBoldStarvation`/`htmlCodeStarvation` helpers (each package's own
`render.go`, mirroring the existing `internal/engine/tick/render.go`
pattern) rather than reusing `internal/bot/handlers`' or each other's.
Escaped every user-authored encampment name embedded in these
notifications (`ex.defenderName`, `alert.attackerName`, `baseName`,
`m.attackerName`, `a.CampName`, `c.name`). Confirmed the
`starvation.go` Ghost Mode `world_news` headline does NOT need the same
treatment: `internal/bot/handlers/world.go` already `htmlEscape()`s the
entire aggregated news-feed text at render time, so a raw name in that
headline is safe by the time a player sees it - only the direct
notifications insert needed fixing. Verified with a full `go build
./... && go vet ./... && go test ./...` pass (wave 7's GOPROXY=direct
method); `gofmt -l` also caught and fixed a pre-existing missing
trailing newline in `starvation.go` unrelated to this session's edits.

`internal/engine/tick/engine.go` is 3459 lines with 49 total
notification call sites; roughly a dozen were already polished by an
earlier session (boss-slain, clan-war end/victory/defeat, espionage
breach, arena win/loss, campaign-engagement-activated - all already
using `htmlBoldTick`/`htmlEscapeTick`) before this session added the 12
listed above. **~25 sites in `engine.go` remain plain text** - recommend
wave 10 continue there directly (line-by-line, the file is too large
for one more full pass) rather than picking a new file.

Wave 10 finished the remaining plain-text sites in `engine.go`: tax
payout, exploration-dispatch-success, automatic-scan-sweep report,
boss-already-defeated, survivors-returned-home (both branches),
co-op-lobby cancelled/departed, idle-miner alert, espionage downlink
report, arena-queue-timeout, construction-complete, route-contact
discovery (both directions), the road-contact battle notification plus
its resolved/timeout variants, route-incident onset, conditions-
cleared, and all three supply-convoy outcomes (missed-contact,
ambushed, arrived) - about 20 more sites. Escaped every user-authored
name found along the way: encampment names (`ex.defenderName`,
`contactName`, `c.campName`, `t.campName`, `s.spyName` (already done),
`targetName` in the espionage downlink report) and the scanned rival's
Telegram first name (`targetOwnerName` in the automatic-scan-sweep
report - confirmed via `internal/db/schema/schema.go` that Telegram
`first_name` is free text the platform lets a user set, same treatment
as an encampment name). Confirmed several sites needed NO escaping
because the interpolated value is fixed game content rather than
player text: World Boss names (seeded via `schema.go`, not player-set),
arena bracket labels ("2v2"/"3v3"), and route-incident type labels.
Confirmed a few sites were already effectively safe despite not going
through the `htmlBoldTick`/`htmlCodeTick` helper functions by name -
they already had literal `<b>`/`<code>` tags with no interpolated
names, or reused an already-escaped `battlereport.Render()` /
already-built formatted-string result from a few lines up.

**`internal/engine/tick/engine.go`'s notification strings are now
believed complete** - every "INSERT INTO notifications" /
"notifications.Queue" call site in the file was individually checked
this wave. Verified with a full `go build ./... && go vet ./... && go
test ./...` pass, confirmed with an md5sum checksum diff that
`go.mod`/`go.sum` were byte-identical to their committed state before
and after the verification build (a wave-10-internal mistake earlier
in the session left the temporary telebot.v3 replace-directive
uncommitted-but-unrestored between two checkpoint builds; caught before
commit, not shipped - see the matching PROJECT_MASTER_PLAN.md entry for
the full account so this exact mistake isn't repeated).

Wave 11 did both of the above. `world.go`'s two main panels
(`HandleWorldFeed`, `HandleSectorMap`) were already fully rich-formatted
by an earlier session; `HandleSectorBroadcast` (`/broadcast`) was not -
rich-formatted it and escaped `campName`/`broadcastMsg`/`sender.Username`
in the player-facing notification (the `world_news` headline itself
still doesn't need escaping - same reasoning as `starvation.go`'s Ghost
Mode headline, since `world.go` escapes the whole aggregated feed at
render time). Also cleaned up that headline's use of Go's `%q` verb,
which added stray backslash-escaping artifacts to a player-facing
bulletin line - switched to a plain quoted `%s`.

The repo-wide grep for every `INSERT INTO notifications`/
`notifications.Queue`/`notifications.QueueToRegion`/
`notifications.QueueToAllPlayers` call site turned up real, previously-
unaudited work in five more handler files: `exchange.go` (market-sale
alert), `hero.go` (commander level-up - this one was built via raw SQL
string concatenation rather than the usual Go-side `fmt.Sprintf`,
converted to the standard pattern), `onboarding.go` (referral bonus +
milestone alerts, escaping `sender.FirstName`), `profile.go` (the
`/msg` player-to-player direct message), `clan.go` (new clan
application alert), and `combat.go` (four co-op-lobby notifications).
`silo.go` was already fully polished, nothing to do there.

**Worth flagging specifically:** `profile.go`'s `/msg` command was
inserting a player's raw, completely unescaped free-text message body
directly into another player's notification. Unlike most of this
polish work (cosmetic/consistency), this one was a real, if minor,
bug - since the notification dispatcher auto-detects HTML per message,
a sender who typed a stray `<` or `>` (or deliberately tried to inject
`<b>`/other tags to spoof formatting in the recipient's inbox) could
have broken the send or altered how their message displayed. Now
escaped like every other free-text field in the codebase.

**New architectural note:** `internal/engine/world/weather.go`
(added by the parallel session's "World-event broadcasts" commit)
shares one `eventHeadline()`/`eventLabel()` string between the plain
`world_news` feed (safe - escaped downstream) and a new direct-push
notification via `notifications.QueueToRegion` (sent as-is, no
wrapping escape). Deliberately left unformatted rather than adding
`<b>` tags: doing so would leak literal `&lt;b&gt;` into the news feed
display, since `world.go` escapes the *entire* aggregated feed text
including any tags already embedded in a headline. No player data is
interpolated into either string (only fixed event-type/continent
enums), so there's no escaping *bug* here, just an intentional
formatting trade-off - noting it so a future session doesn't "fix" it
into a display bug. If this is ever revisited, the correct fix is
building two separate strings (one plain for `world_news`, one
HTML-formatted for the direct push) rather than sharing one.

Verified with a full `go build ./... && go vet ./... && go test ./...`
pass; confirmed with an md5sum checksum that `go.mod`/`go.sum` were
byte-identical before and after (twice - once before the gofmt pass,
once after). `gofmt -l` also caught and fixed two more pre-existing
(not introduced this session) formatting issues (`world.go`, `hero.go`
- stray trailing whitespace and a missing final newline).

---

**Following session (project owner's direct request, not part of the UI
polish sequence above): onboarding overhaul + real starting-resource
and teleport/Ghost-Protocol bugs.**

1. **Starting resources for non-referred players fixed to 25,000 flat
   across all 9 resources** (was a lopsided 1,000 Scrap / 50 Rations /
   250 Electricity / 200 Metal / 20 Crystal / 40 Hydrogen / 300 Dollars
   / 5 Ether / 50 Neuro Cores split). The small per-faction bonus
   (+500 Electricity for Metal Vanguard, +1,500 Scrap for Rust Nomads)
   stays as a flavor differentiator layered on top of the shared
   25,000 baseline, not a replacement for it.

2. **Referral bonus reworked into a genuine 2x, not the old mismatched
   flat top-up.** The referral bonus was a flat +50,000 Metal / +500
   Crystal / +50,000 Neuro Cores regardless of the base pack - once the
   base pack is 25,000 flat, that would have made a referred player's
   Metal/Neuro end up at 3x (75,000) while Crystal barely moved past 1x
   (25,020) and the other 6 resources got nothing extra at all. Replaced
   with a flat +25,000 top-up to *all 9* resources for both the new
   player and their referrer (new helper `topUpAllResources`), so a
   referred player now cleanly ends up at 50,000 of everything - an
   honest 2x. Milestone bonuses (5/10/25 referral count) were untouched,
   since the request was specifically about the base per-referral
   reward. Updated the two other places that quoted the old flat
   figures: `gettingStartedGuideText` (onboarding.go) and `/refer`'s
   panel text (profile.go).

3. **Naming-first onboarding.** Previously, a new player's outpost was
   silently auto-named `Outpost-<telegramID%1000>` and they only saw
   their real dashboard afterward. Now, the very first thing a player
   sees after picking a faction is a mandatory "name your outpost"
   prompt - before resources, location, or anything else - via a new
   `users.state = 'naming'` gate and a free-text capture handler
   (`HandleOnboardingPendingInput`, registered in `cmd/bot/main.go`'s
   `OnText` chain ahead of `admin.HandleAdminPendingInput`, same pattern
   admin.go already established for guided multi-step input). The camp
   is created with a throwaway placeholder name
   (`Unnamed-Outpost-<telegramID>`) immediately overwritten the moment
   naming completes; `HandleStart` re-shows the naming prompt instead of
   the normal dashboard if a player abandons onboarding mid-naming and
   comes back. Name validation (3-20 chars, letters/numbers/spaces/
   hyphens, uniqueness) reuses the exact same rule as the existing paid
   `/name` rename command (`outpostNameRegex`), just free and one-time
   during onboarding.

4. **New deterministic town/country flavor-naming system**
   (`spawnlocation.go`): every screen that shows a player's base location
   (onboarding completion, the returning-player `/start` dashboard,
   `/newjobteleport`, `/ghostprotocol`) now describes it as e.g. "Ashford
   Hollow, the Kalahari Reaches (Africa Territory)" instead of raw
   `[X, Y]` coordinates or a bare continent name. Deliberately NOT a new
   DB column: `flavorLocation(x, y, continent)` is a pure, seeded
   function of the coordinate itself, so the same base always describes
   itself the same way on every screen with zero migration and zero risk
   of the display drifting from the stored coordinate. Town/country
   names are fictional (no real-world country names), matching the
   post-collapse setting.

5. **Real bug fixed: `/newjobteleport` and `/ghostprotocol` were
   assigning the literal region string `"Unknown Sector"`** to every
   relocated base - which matches none of the four names in
   `internal/engine/world.Continents`, the canonical list every
   per-continent system (weather events, and anything built on it later)
   keys off of. A player who ever teleported or used Ghost Protocol had
   their base silently and permanently excluded from weather events
   afterward, with no error or indication anything was wrong - exactly
   the kind of silent, hard-to-notice gameplay bug schema-first
   discipline exists to catch. Both commands now roll a real continent
   (`randomContinent`, uniform across all four - a genuine reroll, unlike
   onboarding's per-player deterministic spread) and generate coordinates
   in that continent's correct quadrant via a new shared
   `allocateCoordinate` helper, which also fixes a second, smaller
   pre-existing issue: the old teleport/Ghost-Protocol coordinate
   allocation used `ON CONFLICT (x,y) DO UPDATE SET x = EXCLUDED.x
   RETURNING id`, a trick that always returns a row even on collision -
   meaning two players could, at rare-but-nonzero odds, end up sharing
   the literal same coordinate row after teleporting. `allocateCoordinate`
   reuses onboarding's safer retry-on-collision loop instead (both
   onboarding and teleport/Ghost Protocol now share one implementation).

6. **Bonus find while touching this code: the faction-choice screen's
   displayed starting bonus was 10x wrong** ("+50.0 Electricity Cells" /
   "+150.0 Scrap" shown, but the code has always granted +500 / +1,500)
   - a real, pre-existing display bug, unrelated to anything above but
   directly in the text being edited. Fixed to match what the code
   actually grants.

Added `internal/bot/handlers/spawnlocation_test.go` (7 tests): continent/
quadrant sign correctness, `randomContinent` always returns a valid
`world.Continents` entry, `flavorLocation` determinism and its fallback
for an unrecognized continent, every continent having at least one
flavor country defined, the shared name-validation regex, and a guard
that `locationDescriptor`'s output never contains escaped-entity
artifacts (documenting why it's safe to skip `htmlEscape` there - every
input is fixed content, never player text). Verified with a full
`go build ./... && go vet ./... && go test ./...` pass and a checksum-
confirmed `go.mod`/`go.sum` restore.

No existing test asserted the old resource amounts, camp-name scheme, or
"Unknown Sector" string, so nothing needed updating for the changed
behavior itself - `onboarding_referral_test.go`'s existing coverage
(referral code generation, milestone ordering) was untouched by any of
this and still passes.

## Bug-hunt session (2026-08-01) — user-reported issues, confirmed and fixed

Four issues reported by the user, all confirmed against the actual code
(not assumed) before fixing.

1. **Real bug fixed: AI Developer Console responses (Balance Report,
   Weekly Report) were getting cut off mid-JSON.** Root cause: the
   `openaicompat` provider (used for Qwen/DashScope) never set
   `enable_thinking`, and Qwen3 models default that to `true`. The
   resulting reasoning tokens draw from the *same* `max_tokens` budget
   as the visible completion, so the model could burn its whole budget
   on hidden reasoning before writing any of the actual JSON — this is
   why the cutoff looked arbitrary and mid-string rather than at a
   suspiciously round token count. (Project memory claimed this was
   already fixed via `enable_thinking:false` in `ExtraFields`; a repo-
   wide grep found no trace of either — the field didn't exist on
   `CompletionRequest` at all. Whatever session made that claim either
   never actually landed it, or it was lost in a rebase from the
   parallel dev session. Either way, the code as of today's `main` did
   not have it.) Fixed in
   `internal/ai/providers/openaicompat/provider.go`: added an
   `EnableThinking *bool` field to the wire request struct (a pointer
   so it's omitted via `omitempty` for OpenAI/DeepSeek/Grok, which
   don't recognize it), and the provider now force-sets it to `false`
   whenever `ProviderName == "qwen"`. Also doubled `MaxTokens` (2048 →
   4096) specifically for `devconsole/balance.go` and
   `devconsole/console.go` (the balance and weekly/activity reports),
   since those two produce the most narrative-heavy JSON (multiple
   free-text fields) of any AI feature and were the two the user hit.
   The other seven AI-advisor packages stay at 2048 — no evidence they
   need more, and inflating budgets that aren't broken just costs money
   per ADR guidance. Added `TestProvider_Complete_QwenDisablesThinking`
   and `TestProvider_Complete_NonQwenOmitsThinkingField` to
   `provider_test.go` to lock this in both directions.

2. **Real bug fixed: long inline "toast" notifications (e.g. "Hangar
   Full: 2445/210 capacity used, only room for 0 mor...") were silently
   truncated by the Telegram client.** `c.Respond(&CallbackResponse{Text:
   ...})` without `ShowAlert: true` renders as a single-line banner at
   the top of the chat, which Telegram's client clips hard for anything
   longer than a short phrase — there is no wrapping or "read more."
   Grepped every `CallbackResponse{Text: fmt.Sprintf(...)}` call site
   (44 total) for ones whose format string is long enough to be at real
   risk, and set `ShowAlert: true` on the 10 that were: hangar-full and
   Doomsday-Rig-cap in `factory.go`, drone-intercept success/failure and
   insufficient-campaign-supplies in `combat.go`, insufficient-materiel
   and insufficient-crystal in `combat_road_encounters.go`, miner-cap
   and admin-override in `camp.go`, and the boss strike-force dispatch
   confirmation in `boss.go`. `ShowAlert: true` makes Telegram show
   these as a full modal dialog instead, which the player has to
   dismiss but which shows the complete message.

3. **Real bug fixed: the Attack/Continue buttons on a road encounter
   (raid passing another raid, or passing a base) didn't respond to
   taps.** Confirmed root cause, not guessed: Telegram caps inline
   button `callback_data` at 64 bytes, and exceeding it makes Telegram
   silently refuse to attach that button — the message still sends, the
   button is visibly there, but tapping it does nothing (no error, no
   spinner, nothing), which matches the report exactly. Two separate
   overflows found:
   - `road_encounter` (raid-vs-raid) packed `\f` + `"road_encounter"` +
     `|attack|` (or `|continue|`) + a 36-byte encounter UUID + `|` + a
     36-byte raid UUID = **~95 bytes**, way over. The raid ID was only
     ever used to know which of the two raids in the encounter belonged
     to the tapping player — information already derivable server-side
     from the encounter row's `raid_a_id`/`raid_b_id` plus the caller's
     own encampment, so it never needed to travel over the wire at all.
     `HandleRoadEncounterCallback` in `combat_road_encounters.go` now
     derives it that way instead of trusting a client-supplied ID (a
     minor security improvement as a side effect), and the button-
     building code in `combat.go` no longer passes it. New length:
     59-61 bytes.
   - `road_base_encounter` (raid vs. a passive base) was `\f` +
     `"road_base_encounter"` + `|attack|` + a 36-byte UUID = exactly
     **64 bytes** for "attack" (right at the edge — fragile, one
     character away from breaking) and **66 bytes** for "continue"
     (already broken). Shortened the callback prefix to `"rbe"`
     everywhere (registration in `cmd/bot/main.go`, button construction
     in `combat.go`) — new length: 48-50 bytes, comfortable margin.
   Added `callback_data_length_test.go` which builds the actual buttons
   via `telebot.ReplyMarkup.Data()` and asserts the resulting
   `callback_data` (prefix + separators + a representative 36-byte
   UUID) stays under 64 bytes for both encounter types and both
   actions, plus a guard that `road_encounter` never regresses back to
   carrying two UUIDs.
   - Swept the rest of `internal/bot/handlers` for the same pattern
     (any button passing two ID-shaped arguments). Found one other
     borderline case, `clan_app_accept`/`clan_app_reject` in `clan.go`
     (Telegram user ID + clan UUID), which computes to 63 bytes with a
     10-digit user ID — currently safe but with only 1 byte of margin.
     Left as-is since it isn't broken today and touching it wasn't
     requested, but flagging here since Telegram user IDs are trending
     longer over time and this is worth revisiting before it silently
     breaks the same way.

4. **Not a bug: a screenshot of a Chinese-language "your account has
   been flagged, recover within 12 hours or be banned" message with
   red/green action buttons was shared as a UI style reference.**
   Confirmed this is a phishing message impersonating Telegram's
   account-suspension flow (a known scam pattern), not a real Telegram
   bot design — declined to replicate it and flagged it as such to the
   user rather than treating it as a legitimate design request.

Verified with a full `go build ./... && go vet ./... && go test ./...`
pass and a checksum-confirmed `go.mod`/`go.sum` restore after using the
`telebot.v3` replace directive locally to resolve the module.

## Follow-up bug-hunt session (2026-08-01, later same day) — AI Advisor truncation confirmed still happening after the fix above

User reported the truncation bug persisted specifically for the AI
Advisors menu (Battle Analyst, Economy Advisor, Galaxy Advisor, Guild
Assistant, Research Planner, Fleet Commander, NPC Intel, Governor):
tapping one advisor after another, the second reply would come back
cut off. Investigated rather than assuming the earlier fix (see
section above / ADR-023) was incomplete without checking — it was.

1. **Real bug, confirmed by re-reading every advisor's completion
   call: the earlier fix only raised `MaxTokens` for the two Dev
   Console report types (Weekly, Balance), not for any of the eight
   actual AI Advisors, which were all still requesting 2048.** The
   `enable_thinking:false` fix (ADR-023) stopped Qwen's reasoning
   tokens from eating the budget, but 2048 visible tokens is still a
   tight ceiling for an advisor writing a full reasoning + risk-
   assessment + suggestion JSON object — genuinely not enough some of
   the time, matching the user's "one works, the next doesn't" pattern
   (whichever advisor's response happened to run longer that time hit
   the wall). Raised all eight to 4096, matching the two report types
   that were already fixed. Files: `fleetcommander/commander.go`,
   `governor/governor.go`, `npcintel/intel.go`,
   `researchplanner/planner.go`, `battleanalyst/analyst.go`,
   `guildassistant/assistant.go`, `galaxyadvisor/advisor.go`,
   `econadvisor/advisor.go`.

2. **Real bug, found while investigating #1: `Service.Complete`
   (`internal/ai/service.go`) cached every completion unconditionally,
   including ones that got cut off.** With the default 120s cache TTL,
   a truncated response would get served back verbatim to anything
   that repeated the same request within that window — including the
   player tapping "Refresh" on the exact report that had just
   truncated, so refreshing looked like it did nothing. Now skips the
   cache write whenever the provider's own stop/finish reason
   indicates the completion was cut short (new `ai.
   IsTruncatedStopReason`, checked against each provider's real wire
   value — `"length"` for OpenAI-compatible/Ollama, `"max_tokens"` for
   Anthropic, `"MAX_TOKENS"` for Gemini — see `CompletionResponse.
   StopReason`, which every provider already populated but nothing
   downstream previously read).

3. **Real gap, not a bug exactly: truncation detection only ever
   looked at the text itself (`ai.WasTruncated`'s brace-balance scan),
   which structurally cannot see a response truncated before any `{`
   was written at all** — confirmed by an existing test,
   `TestParseRecommendation_FallsBackOnGarbage` in
   `battleanalyst/prompt_test.go`, which explicitly asserts
   `Truncated=false` for exactly that shape of input. Threaded each
   provider's real stop reason through to every
   `ParseRecommendation`/`ParseAnswer`/`ParseClassification` function
   (now `(text string, stopReason string)`) across all eight advisors
   plus the three Dev Console entry points (Weekly Report, Balance
   Report, NL Query's classify+answer calls), OR'd with the existing
   brace scan. New test
   `TestParseRecommendation_StopReasonCatchesTruncationTextScanMisses`
   demonstrates the previously-blind case is now caught.

Full detail and the design rationale (why stop reason lives
per-provider rather than a generic flag, why MaxTokens/cache fixes are
repo-wide rather than per-package) is in ADR-025, PROJECT_MASTER_PLAN.md.

Verified with a full `go build ./... && go vet ./... && go test ./...`
pass (temporary `telebot.v3` replace directive used locally to resolve
the module against the sandbox's network allowlist, then reverted —
`git diff go.mod go.sum` empty before commit).

### Follow-up: closed out the `clan_app_accept`/`clan_app_reject` margin item

Item 3's flagged-but-not-fixed case (63-64 bytes, 1 byte of margin) is
now fixed too. Renamed the prefixes to `cl_acc`/`cl_rej` (registration
in `cmd/bot/main.go`, button construction in `clan.go`) — new length
is 55-56 bytes with a 10-digit Telegram user ID, comfortable margin
even if user IDs grow a couple of digits over time. Added
`TestClanApplicationCallbackData_HasMargin`, which asserts real
headroom (budget of 60 bytes, not just "under the 64-byte cap") using
a 12-digit test user ID specifically so a future regression shows up
as a test failure well before it would actually break in production.
Verified with the same build/vet/test/`go.mod`-restore process, then
pushed to `main` (this was after the rebase onto `a17eeab`'s colored
buttons had already landed, so no further conflict here).

## Investigation session (2026-08-01, later) — user-reported "climate always nominal" and "AI never raids"

Player reported (Telegram screenshot, Wasteland Radio panel): climate has
shown "Nominal - no active debuffs" on all four continents every single
time they've checked, and no AI faction has raided them or anyone else for
1-3 days, despite the README/plan docs describing both as active,
constantly-running systems. Investigated by reading the actual engine
code end to end (not assumed), same standard as every other entry in this
file.

1. **Weather/climate: code is correct and wired into the tick loop -
   root cause is NOT in this repo's game logic and needs a production/
   deployment check, not a code fix.** `RunWeatherPass`
   (`internal/engine/world/weather.go`) rolls a 10% chance *per
   continent, per tick* to start one of 7 event types, is registered as
   the very first phase in `ProcessTick`'s phase list
   (`internal/engine/tick/engine.go`), and correctly persists an event
   for 2 hours once rolled. `GAME_TICK_SECONDS` defaults to 60s
   (`cmd/bot/main.go`). At that cadence the probability of *all four*
   continents staying clear for even one hour is under 1%, so "nominal
   everywhere for 1-3 days straight" is not explainable by the roll
   itself being unlucky - something is preventing `ProcessTick` (or at
   least this phase of it) from actually running against the live
   database. Candidates that need checking against the **running
   process**, which this sandbox has no access to: (a) the deployed
   bot process might not be built from current `main` / might be
   stale; (b) `GAME_TICK_SECONDS` might be misconfigured to something
   huge in the live `.env`; (c) the tick loop might be crashing or
   silently stuck on a phase that panics without recovery before
   reaching a later tick (the phase list runs sequentially inside one
   pass, but a hang/crash on *any* prior tick's phase would stop
   `weather` from ever re-running on schedule); (d) the live DB's
   `world_events`/`world_news` tables might not match
   migration 025 (`spacehunt_phase7_regional_world_events.sql`) if a
   migration didn't fully apply. **Action needed from whoever has
   production log/console access**: check the bot process's recent
   logs for `"World Event Pass:"` lines (weather.go's own success log)
   or a `ProcessTick` panic/error - that will say definitively whether
   this phase is executing at all. Filed here rather than "fixed"
   because there's nothing to fix in the code as written; don't let a
   future session re-derive this same conclusion from scratch.

2. **AI raids: real, confirmed root cause - not a "never runs" bug, but
   a tuning/gating combination that can leave a raid genuinely rare for
   a small player base.** `decideAIFactionActions`
   (`internal/engine/tick/aidecisions.go`) *is* fully implemented,
   wired into the tick phase list as `"ai_civilization_decisions"`, and
   tested - `AI_FACTION_DECISION_LOOP_PLAN.md` had been left saying
   "Status: not started" this whole time, which was simply stale and
   has been corrected in that file. The actual behavior, read directly
   from the constants at the top of `aidecisions.go`:
   - Exactly **8 AI factions total, fixed at boot, never grows**
     (`seedAICivilizations` in `cmd/bot/main.go`: 2 per continent,
     hardcoded `ai_faction_key`s, idempotent seed so it can never
     insert more than these 8). There is no code path anywhere that
     spawns a new AI faction after server start.
   - Each of those 8 factions only gets to make **one decision every
     20 minutes** (`aiDecisionCadence`), and a "decision" is scout-or-
     raid-or-idle, not "raid".
   - A faction can only raid a target it has *already discovered*
     through its own `aiScout` passes (one new discovery per faction
     per 20-minute cycle, mirroring the no-omniscience rule human
     exploration follows) - so on a fresh or sparsely-populated world,
     several 20-minute cycles pass before a faction even has a
     candidate target, before probability enters into it at all.
   - Even with a discovered target, the target must be within
     `aiMaxLevelsBelowSelfForFairTarget` (2 levels) of the faction's
     own level - fine for a populated server with a level spread, but
     on a server with very few real players (the screenshot's own "20
     monthly users") it's easy for zero currently-discovered targets to
     fall in-band for any given faction at any given moment.
   - Then, only a **40%** roll (12% if the fair target happens to be
     another AI faction, since AI-vs-AI is deliberately kept as rare
     background texture per `AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md`
     section 2) actually launches the raid that cycle even once a fair
     target exists.
   Net effect worldwide: 8 factions × one gated decision per 20 minutes
   is already a low ceiling, and most of that ceiling is consumed by
   slow scouting and the fairness band before the 40% roll is even
   reached - which fully explains a real player going multiple days
   without ever being raided, especially at low level / low population.
   This is a **design/balance question, not an engineering bug** - the
   system does exactly what `AI_FACTION_DECISION_LOOP_PLAN.md`
   specified, deliberately conservative "start low, loosen only after
   observing AI factions are too passive in practice" per that doc's
   own notes on `aiMaxLevelsBelowSelfForFairTarget`. Recommended
   tuning changes (raid cadence, fairness band width, probability, and
   the request for continuous new-AI-faction spawning + AI-vs-AI
   raiding twice daily + constant AI scouting) are written up as a new
   plan doc rather than changed silently here - see
   `AI_AND_SCOUTING_EXPANSION_PLAN.md` (new file, this session).

3. **Scouting: confirmed by design, not a bug - only one concurrent
   scout mission per encampment is allowed today.**
   `doDispatchScoutMission`
   (`internal/bot/handlers/scoutmissions.go`) explicitly blocks a new
   dispatch with *"You already have a scout party out"* whenever a
   `scout_missions` row exists in `phase IN ('searching', 'returning')`
   for that encampment - by design, one mission at a time, one
   destination at a time. (What already *does* work today: a single
   mission can commit any number of available Scout Walkers together -
   the quick-dispatch buttons offer 1/5/10/25 and there's no upper cap
   in the handler itself - so "up to 3 scouts on the same mission" is
   already possible; what's missing is 3 *independent, simultaneously-
   running* missions to different destinations with independent return
   times, which the current one-row-per-encampment gate above
   prevents.) Requested expansion (raise the concurrent-mission cap to
   3, independent destinations/ETAs per mission) written up alongside
   the AI expansion items in `AI_AND_SCOUTING_EXPANSION_PLAN.md` rather
   than implemented ad hoc in this session, since it touches the same
   `scout_missions` schema `AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md`
   section 3 already extended for long-range scouting, and needs to be
   built as one coherent change, not two colliding ones from different
   sessions.

4. **Not a bug, already correctly implemented: world-event push
   notifications.** The player also asked for climate/big-event
   notifications to reach every player in the affected continent ASAP.
   This is already live: `RunWeatherPass` calls
   `notifications.QueueToRegion` on both the event-start and event-
   clear branches (see `AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md`
   section 5, marked complete and merged), which queues a direct,
   non-mutable notification to every real player in that continent
   through the existing 3-second-poll notification engine. If this
   genuinely isn't arriving in practice, it's downstream of finding
   #1 above (no events are triggering at all right now to notify
   about) rather than a separate notification-path bug.

## Follow-up bug-hunt session (2026-08-01, later still) — "pq: unnamed prepared statement does not exist (26000)" on Dev Console reports

User confirmed truncation is fixed (see section above) but reported a
new, intermittent error surfacing on Balance/Weekly reports:
`devconsole: balance stats for destroyers_mobilized: pq: unnamed
prepared statement does not exist (26000)` and `devconsole: count
active users: pq: unnamed prepared statement does not exist (26000)`.
Already handled non-fatally (the bot showed a "temporarily
unavailable" message rather than crashing, via an existing error
wrapper) but a graceful failure that could be avoided entirely is
still a bug worth fixing rather than living with.

1. **Real bug, root-caused rather than patched at the query level:
   this is SQLSTATE 26000, a known Supabase PgBouncer (transaction
   pooling mode) + lib/pq incompatibility, not anything wrong with the
   specific queries that happened to hit it.** lib/pq runs every
   parameterized query through the Postgres extended query protocol
   with an unnamed prepared statement — a Parse followed by a later
   Bind/Execute. PgBouncer's transaction pooling can route that second
   half to a different backend server than the one that saw the
   Parse, and that backend has genuinely never heard of the statement.
   It's a connection-pooler artifact, not a real query problem, and
   it's inherently intermittent — explaining why Dev Console reports
   (which run several queries back-to-back) were where it got noticed
   first, without anything being specifically wrong with `devconsole`
   itself. Fixed with a new `internal/dbdriver` package wrapping
   lib/pq's `postgres` driver: on SQLSTATE 26000 specifically, it
   returns `driver.ErrBadConn`, which triggers database/sql's own
   built-in retry-on-a-fresh-connection behavior. Registered once in
   `cmd/bot/main.go` (`sql.Open("postgres-retry", dbURL)` instead of
   `"postgres"`), so it covers every `*sql.DB` call site in the
   codebase — not just devconsole — without touching any of them.
   Verified the fix doesn't swallow real errors: a genuinely different
   Postgres error code still passes straight through unchanged (new
   test `TestQueryContext_PassesThroughUnrelatedPqErrors`), so a real
   bug can't get masked behind a silent retry.

Full rationale in ADR-026, PROJECT_MASTER_PLAN.md.

Verified with a full `go build ./... && go vet ./... && go test ./...`
pass, including new unit tests in `internal/dbdriver` that exercise
the retry-translation logic against a fake `driver.Conn` (no live
Postgres/PgBouncer needed to prove the behavior). go.mod/go.sum
reverted before commit — git diff go.mod go.sum is empty.

## Telegram UI design pass (2026-08-01) — expandable blockquotes + real tables

User asked for a research-first pass on Telegram bot UI design patterns
beyond inline-button coloring (explicitly a parallel session's
territory this round) — search first, note everything down, then
implement where it actually helps. Full design rationale in ADR-027,
PROJECT_MASTER_PLAN.md.

**Researched, not assumed:** confirmed via search that the codebase
already covers most of what Telegram supports (bold/italic/underline/
code/pre/blockquote/spoiler all already had helpers). The two genuinely
new, current features found: `<blockquote expandable>` (Bot API 7.3+,
a real collapsible-quote entity, not a hack) and the fact that
Telegram has no native table entity whatsoever — any aligned columns
require hand-padded monospace text inside a `<pre>` block, with mobile
clients wrapping around 34-40 characters, which is why a naive
markdown-table dump looks broken on a phone (a documented, common
complaint independent of this project).

**Implemented:**
- Added `HTMLExpandableQuote`, `HTMLTable`, `HTMLQuote`, `HTMLStrike`
  to `internal/ai/render.go` — chosen over `bot/handlers/render.go`
  specifically because every AI feature package already imports
  `internal/ai` and none of them can import `bot/handlers` (standing
  import-cycle rule), making this the one place reachable from all
  nine AI packages at once.
- Found a real, previously undocumented gap while looking for a good
  first place to apply these: **the Balance Report
  (`devconsole/balance.go`) was still being sent as fully plain text**
  — confirmed by reading the actual `c.Send(...)` calls in
  `dev_console.go`, neither of which passed `telebot.ModeHTML`. This
  is the one AI Developer Console output the original HTML polish
  campaign (waves 1-11) never reached. Rewrote it: real HTML
  formatting, `telebot.ModeHTML` added to both call sites, and a unit
  usage table built from `BalanceSnapshot`'s actual numbers (not the
  model's `UnitNotes` prose) with the AI's per-unit caution shown in
  an expandable blockquote alongside it.
- The Weekly/Activity Report (`devconsole/prompt.go`) gained a Top
  Players table from its own real `Snapshot.TopPlayers` data, and its
  three narrative fields (new players, top performers, admin
  recommendations) — each free text that can genuinely run several
  sentences — now render as expandable blockquotes instead of dense
  inline prose.
- Both `BalanceRecommendation` and `Recommendation` gained a
  `Snapshot *…Snapshot \`json:"-"\`` field, set by the caller
  (`RecommendBalance`/`Recommend`) after parsing the model's response,
  never unmarshaled from the model's own JSON — deliberately, so a
  table showing real numbers can never accidentally be populated by
  something the AI wrote instead of the game's own data.

**Bug caught and fixed before it shipped, not after:** the first draft
of the Top Players table escaped each player name (`ai.HTMLEscape`)
and then let `HTMLTable`'s own length-clipping truncate the result.
If a name contained `&`, the escaped `&amp;` could get sliced mid-
entity (e.g. cut to `&am…`), which Telegram would reject the entire
message for — a real, if rare, way this feature could have broken a
live report. Fixed by clipping the raw name to a safe rune length
first and escaping second; the hazard itself is now called out
directly in `HTMLTable`'s doc comment so a future caller building
another table doesn't repeat it.

**Explicitly out of scope this round, by the user's own instruction:**
inline-button coloring (a parallel session already shipped real
colored buttons via Bot API 9.4 — see `a17eeab`). Also out of scope:
extending these same helpers to player-facing panels (referral
leaderboard, clan stats, warehouse stocks) — good candidates for a
future pass, not touched here to keep this session's diff reviewable
and focused on the two reports that most needed it.

New tests: `internal/ai/render_test.go` (table column alignment,
malformed-row handling, long-cell clipping, quote/strike wrapping) and
targeted regression tests in `devconsole/balance_test.go` and
`devconsole/prompt_test.go` covering the new table/expandable-quote
wiring, including a nil-`Snapshot` safety case for both reports (since
every existing hand-built test `*Recommendation` predates the field
and doesn't set it).

Verified with a full `go build ./... && go vet ./... && go test ./...`
pass, plus a manual render of a realistic Balance Report through a
throwaway `cmd/renderpreview` binary (removed before commit) to
visually confirm the table lines up and the expandable quote renders
as expected — not just asserted via `strings.Contains`. go.mod/go.sum
reverted before commit — checksums confirmed to match the pre-session
values.

### Follow-up: extended to the `/refer` panel — another confirmed plain-text gap

Picked up one of the "future pass" candidates flagged above. Reading
`HandleRefer` (`profile.go`) found it was the same situation as the
Balance Report: `return c.Send(panelText)` with no `telebot.ModeHTML`
at all, hand-built with `+=` string concatenation, and — worse than
the Balance Report — **rendering real Telegram first names
(`ref.first_name`) completely unescaped** into a plain-text message.
Plain-text mode means literal `<`/`>`/`&` in a name just show up as
themselves (no HTML injection risk there), but it's still the kind of
raw-user-input handling this codebase generally does carefully
everywhere else it touches HTML mode.

Rewrote with `internal/ai`'s shared helpers directly (this package
already imports `internal/ai` elsewhere, in `ai_status.go`, so no new
dependency and no need to duplicate `HTMLTable`/`HTMLExpandableQuote`
locally the way `bot/handlers/render.go`'s older helpers do — that
duplication predates `internal/ai/render.go` existing at all).
Milestone bonuses and the Top Referrers leaderboard are both now real
`ai.HTMLTable`s instead of one-line-per-entry prose, `telebot.ModeHTML`
added to the `Send` call, and referral names go through the same
"clip the raw rune length first, escape second" order established in
ADR-027 — reusing that exact fix rather than risking the same
entity-slicing bug a second time in a different file. Verified with a
manual render through a throwaway `cmd/renderpreview2` binary (removed
before commit), plus the same full build/vet/test/`go.mod`-restore
process. No dedicated unit test added for `HandleRefer` itself, since
no handler in this package currently has one (it needs a live `*sql.DB`
and this codebase doesn't use a SQL-mocking library) — the table logic
this change actually depends on (`ai.HTMLTable`) already has full unit
coverage from the change above.

### Follow-up: Warehouse Reserves, Admin Metrics, and Alliance/Clan Stats

Finished the sweep the user asked for ("do the rest too, the clan
stats and others"). Went through every stat-dump-style panel in
`internal/bot/handlers` (grepped for files with a high density of
`%d`/`%.Nf` format placeholders, then read each candidate rather than
converting on pattern-match alone) and converted the three that were
genuinely a good fit for a table:

- **Warehouse Reserves** (`HandleWarehouseReserves`, `economy.go`) —
  confirmed via reading the code, not assumed: this was **fully plain
  text**, no `telebot.ModeHTML` at all, nine resources each on their
  own hand-written line grouped under four category headers. This is
  the closest match in the whole game to the numeric-stats-table style
  originally referenced. Now a single real `ai.HTMLTable` (Resource |
  Amount), `telebot.ModeHTML` added to the `Send` call.
- **Admin Metrics** (`HandleAdminMetrics`, `admin.go`) — already HTML
  mode, but line-by-line prose under two headers. This is the single
  closest match to the original screenshot's design (a database-
  telemetry dashboard: user counts, goroutines, memory, GC cycles).
  Converted to one `ai.HTMLTable` (Metric | Value).
- **Alliance Stats** (`HandleAllianceStatsCallback`, `clan.go`) — the
  "clan stats" the user asked for by name. Already HTML mode, four
  numbers spread across two labeled lines. Converted to an
  `ai.HTMLTable` (Stat | Value) for consistency with the other two,
  even though four rows is a modest table - the value here is mostly
  matching the house style now that it exists elsewhere.

**Deliberately left as prose, and why (checked each, not skipped by
default):**
- `HandleGuildMissions` (clan raid/transfer history, `clan.go`) — rows
  contain two free-form outpost/commander names of unbounded length
  plus a combined multi-resource loot string; forcing that into fixed
  columns would either clip real player-chosen names constantly or
  blow out the mobile-safe width every time. Prose reads better here.
- `HandleManageMembersCallback` (clan roster, `clan.go`) and the Silo
  panel's target list (`silo.go`) — both pair each row with its own
  inline buttons (Kick/Promote, Detonate/Pierce). A `<pre>` table is
  plain text with no buttons inside it, so putting the row data in a
  table would visually separate it from the button that acts on it -
  worse, not better, despite being numeric-adjacent.

Verified with the same full `go build ./... && go vet ./... && go test
./...` pass, a manual visual render of all three new tables through a
throwaway preview binary (removed before commit), and a checksum-
confirmed `go.mod`/`go.sum` restore.

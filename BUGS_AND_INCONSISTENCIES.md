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

# SPACEHUNT_PHASE7_LOG.md

**Read this before touching SpaceHunt gameplay code.** This is the
working log for the **SpaceHunt Roadmap** side of this repo — the
Telegram-facing gameplay systems (combat, economy, heroes, world,
factory, etc). It is maintained by a Claude session working through
Asiwaju's "Phase 7 — Core Gameplay, Combat & Polish" brief.

This file is **not** `PROJECT_MASTER_PLAN.md`. That file belongs to a
separate, independent workstream — the AI Systems Roadmap
(`internal/ai/`, Governor, Fleet Commander, Economy Advisor) — being
built by a different session. The two workstreams don't touch the same
code paths on purpose (see that file's section 0 for the split), and
this log exists so nobody has to guess which session did what in
`git log`. If you're an AI or human picking this up cold:

- Gameplay/combat/economy/hero/world/UX work → this file.
- `internal/ai/*`, Governor, Fleet Commander, Economy Advisor → `PROJECT_MASTER_PLAN.md`.
- If a commit message starts with "Phase 7 (part N)" it came from this workstream.
- If a commit message references AI Roadmap Phase A–J, it came from the other one.

---

## 0. Source brief

The full Phase 7 brief (13 numbered requirements) was supplied directly
by Asiwaju in-chat. Short version of the ask: turn several UI-mockup or
partially-wired gameplay systems into fully persistent, notification-
driven systems, fix a handful of named bugs, and pass a general audit.
The 13 items, in the priority order used for this log:

1. AI Rogue Battles — full lifecycle + real scaling (**highest priority**)
2. Better (bulk) unit selection
3. Keyboard/UI bugs — full menu audit
4. Hero Commander system + manual defense assignment
5. Automation Agent limit-respecting bugs
6. Doomsday unit balance
7. Battle Logistics (supplies, retreat, long battles)
8. Emoji/UI consistency
9. Leave AI Agent files (Governor/Automation/Planning) untouched
10. World exploration (continents/sectors, discovery)
11. Discovery, diplomacy (Known Bases, friend/enemy), long battles/reinforcements
12. Dynamic World Events + notification engine upgrade
13. Admin Panel consolidation

**Important corrected finding (do not re-litigate this):** items 1, 7,
and 11's "battle lifecycle" claims (no travel/no marching/no
persistence/no notifications) were **already false** by the time this
session started — `raids`/`raid_forces` already had a full
marching → engaged → returning → completed lifecycle with persistence
and per-phase notifications, built in an earlier SpaceHunt phase. Don't
rebuild that from scratch. What was actually missing under item 1 was
narrower: **Rogue AI scaling** was a flat formula, not "uses the same
systems as a player." That's what got fixed. Re-audit before assuming
something is missing — this codebase has more built than any single
brief describes.

---

## 1. Status by item

| # | Item | Status | Commit(s) |
|---|---|---|---|
| 1 | AI Rogue Battles / scaling | **Done** — Nest now has per-turret Defense Grid, Guardians/Observers, Integrity Tech, Shields, Warlord superpower; resolved through the identical code path as a real defender. Battle lifecycle itself was already complete pre-Phase-7. | `88c01b1` |
| 1b | Ongoing Events hub | **Done** — Expedition Radar's silent `LIMIT 2`/`LIMIT 1` caps removed; World Boss attacks added to the same panel. | `88c01b1` |
| 7 | Battle Logistics | **Done** — `attacker_ammo` was a dead stub (never decremented); now depletes for real, fires threshold notifications (25%/empty), and a still-marching force that runs fully dry auto-retreats. | `3e2195f` |
| 4 | Hero Commander — manual garrison | **Partial** — "Manual Defense Garrison" panel lets a player lock/withdraw Soldiers/Mechs from the draftable pool (enforced, not just displayed). Still open: explicit "which hero leads this raid" picker UI (the DB link `raid_forces.hero_id` exists — confirm it's actually exposed before assuming it needs building), per-hero XP-from-battles-led, ability unlocks beyond the existing superpower. | `3e2195f` |
| 2 | Bulk unit selection | **Done** — Step: x1/x10/x100/MAX toggle on the draft board (bulk moves now clamp instead of rejecting partial steps); `/add <n> <unit>`, `/remove <n> <unit>` text commands; `/deconstruct <n> <unit>` bulk text shortcut alongside the existing per-tap panel. | `f412147`→`60665b1` (rebased) |
| 3 | Keyboard/UI audit | **In progress** — 9 confirmed instances fixed of the exact bug named in the brief ("System Economy does not replace the keyboard as expected"): Trade Hub, Financial Vault, Market Exchange, Warehouse Reserves, World Bosses, The Rebellion, Admin Terminal, Research Lab, and Silo panels all previously sent their inline panel with no persistent bottom keyboard at all (or omitted it entirely), so the bottom bar just stayed whatever it was before. Full menu-by-menu audit not yet exhaustive — see notes below for what's confirmed clean vs. still unchecked. | `<pending>` |
| 5 | Automation Agent limit bugs | **Done** — audited `internal/engine/agent`. `builder` mode's auto-upgrade selection ignored the "module level cannot exceed Outpost Core level" cap enforced on the manual `/camp` path; `military` mode's auto-recruit ignored the Hangar capacity cap enforced on the manual Recruit Soldier path. Both now gated to match. A third suspected gap (collector_omega/precious having no storage cap) was investigated and ruled out at the time — see item 5b, which then made the same cap universal anyway per Asiwaju's follow-up direction. | `90caeef`, `53d1916` |
| 5b | Storage cap — full audit, every resource-gain path | **Done** — Asiwaju asked for storage caps everywhere, not just the two Automation Agent modes. New `internal/game/storagecap` package: single source of truth for the cap formula (Tent*500 + Warehouse*750 + Extension*1000) and the "Surplus Preservation" clamp rule (pre-existing surplus above cap is preserved, but no further gain lands on top of it). Applied to every resource-gain site game-wide: passive tick's Metal Mine/Crystal Mine/Ether generation (previously totally uncapped — and Metal/Crystal/Ether weren't even being read into the passive-tick struct before this, just blind-incremented); `collector_omega`/`collector_precious` automation modes; Bank borrow + Market buy/sell; Ether Shop conversions; P2P Exchange (both seller's Dollars and buyer's Metal/Crystal, independently); Engineering Bay's post-craft refund (also fixed a staleness bug — it was reading pre-cost-deduction values); onboarding referral bonuses (both sides); Rebellion donation reward; Gather Sunlight job; and six separate `tick/engine.go` sites (Daily Tax Law payout, World Boss loot, Clan War spoils, Arena team-victory loot, Arena queue-timeout refunds, returning raid loot). Deliberately left uncapped: `main.go`'s one-time schema-consolidation migration (not a live gameplay path), and `/admin` resource-injection commands (intentional override tool, same as other admin bypasses in the codebase). | `e3f6e15` (rebased to `ef47fb6`) |
| 6 | Doomsday unit balance | **Done** — hard cap of exactly 1 replaced with a level-scaled cap; attack rating and toughness bumped; a real missing-affordability-check bug (neuro_cores wasn't validated before deduction) fixed along the way. | `3fd86a5` |
| 10 | World exploration (continents/sectors/discovery) | **Not started** — biggest single item left; needs new schema (sectors/continents, discovery state per player) | — |
| 11 | Diplomacy (Known Bases, friend/enemy), long battles/reinforcements | **Not started** — long-battle round cap (currently 5 rounds max, see `engine.go` `r.roundNumber >= 5` draw condition) needs revisiting once reinforcement mechanics exist | — |
| 12 | Dynamic World Events + notification engine | **Partially exists** — `internal/engine/world/weather.go` already drives Acid Rain/Radiation Storm effects on combat (see `engine.go` weather switch); notification *dispatcher* itself (`internal/engine/notifications`) is a working 3s-poll queue, not a stub. What's missing: more event types (EMP, Supply Crisis, Disease, Sandstorm from the brief), and continent/world-scoped broadcast rather than the current single global weather state. | — |
| 13 | Admin panel consolidation | **Done** — every admin action now lives in exactly one shared helper, callable from both its original slash command and the consolidated `/admin` panel (previously 3 different copies of the same logic for "inject", 6 actions with no panel path at all). Guided free-text input flow for the 5 actions that need arguments; DB Reset gained a two-tap confirm it never had. | `<pending>` |
| 9 | AI Agent files hands-off | **Exception granted for items 5/5b** — no edits to `internal/game/governor`, fleetcommander, or econadvisor. `internal/engine/agent` *was* edited (`90caeef`, `53d1916`, `e3f6e15`), but only game-limit-enforcement bugs Asiwaju explicitly directed. No other agent logic (mode selection, resource-gain rates, upkeep formulas) touched. | — |

---

## 2. Detailed notes per shipped item

### Rogue AI scaling (`88c01b1`)
- `internal/game/content/roguenest.go`: `RogueNestForce` struct expanded
  from `{Soldiers, Mechs, Drones, Jets, TurretBonus}` to include
  individually-typed turret levels (Light/Heavy Laser, Gauss/Ion Cannon,
  Plasma), `Guardians`, `Observers`, `IntegrityTechLvl`, `Shields`, and
  `HeroSuperpower` (level 20+, cycles the same 3 superpowers a player
  Commander can roll).
- `internal/engine/tick/engine.go`: the `!r.defenderID.Valid` (AI nest)
  branch of raid resolution now populates `defenderLightLaserLvl` etc.
  directly from `content.RogueNestComposition(...)` instead of the old
  flat-bonus-only fallback. The turret-bonus formula block that used to
  branch on `if r.defenderID.Valid { ... } else { flat }` no longer
  branches at all — both cases run the identical per-turret-type math.
- `/recon_ai` (`combat.go` `HandleReconAICallback`) shows the new
  breakdown so scouting still matches what gets fought.

### Ongoing Events / Expedition Radar (`88c01b1`)
- Removed `LIMIT 2` on outbound raid query and the `LIMIT 1` +
  `QueryRow` single-row pattern on inbound (converted to a loop over all
  matching rows).
- Added a World Boss Attacks section (`world_boss_attacks` JOIN
  `world_bosses`), covering marching/engaged/returning states.

### Battle Logistics (`3e2195f`)
- `applyActiveLogisticsConsumption` in `engine.go` rewritten. Previously:
  read `attacker_rations`/`attacker_ammo` off the raid, but **only ever
  wrote `attacker_rations`** (ammo was declared, selected, defaulted to
  100.0, and never once updated anywhere in the codebase — confirmed via
  full-repo grep before fixing).
- Now: both deplete by 4.0/tick on the raid row itself (separate from
  the existing home-base rations/metal-fuel upkeep, which is unchanged).
  Notifications fire once when crossing 25% and once at 0 for each of
  rations/ammo (not every tick). A `marching`-state force that hits 0 on
  both before arriving auto-retreats (`state = 'returning'`) instead of
  arriving to eat the pre-existing -50% offense penalty
  (`r.attackerRations <= 0 || r.attackerAmmo <= 0` check, unchanged) in a
  fight it can't win.
- Not done: the brief's "before marching, player chooses rations/fuel/
  optional supplies" loadout screen. Currently supply levels always
  start full and deplete passively — there's no player choice at launch
  time. Worth deciding whether that's actually wanted before building it
  (it adds a UI step to every single raid launch).

### Hero Commander manual garrison (`3e2195f`)
- New columns `workshop_inventory.garrisoned_soldiers` /
  `garrisoned_mechs` (migration `023_spacehunt_phase7_garrison.sql`).
- New panel off Hero Commander: `🛡️ Manual Defense Garrison`, adjust via
  `garrison_adjust` callback (+10 soldiers / +5 mechs per tap, symmetric
  withdraw).
- Garrisoned units are **not** a separate defense pool — they still read
  straight out of `workshop_inventory.soldiers`/`mechs` for combat
  (nothing changed in defense math). They're subtracted from the
  *draftable* total in **both** places that compute it:
  `renderDraftCustomizerHUD` (display) and `HandleAdjustDraftCallback`
  (the actual enforced cap) — check both if you touch this again, they
  used to be (and still are) two separate copies of the same query.

### Bulk unit selection (`f412147`, rebased to `60665b1`)
- `campaign_drafts.step_size` (migration `024`), default `1`. Values:
  `1`, `10`, `100`, or `-1` (MAX sentinel). Cycled via
  `adjust_draft|step|cycle` callback (special-cased at the top of
  `HandleAdjustDraftCallback` before the normal per-unit switch).
- `/add <n> <unit>` / `/remove <n> <unit>` (new file-level functions in
  `combat.go`: `HandleAddDraftCommand`/`HandleRemoveDraftCommand` →
  shared `handleBulkDraftCommand`). Unit aliases in `draftUnitAliases`
  map — extend that map if you add a new draftable unit type, it is
  **not** auto-derived from `content.MustFindUnit`.
- `/deconstruct <n> <unit>` bulk path lives in `deconstruct.go`
  (`HandleDeconstructCommand`), reusing the existing `deconstructTable`.
  Bare `/deconstruct` (no args) still opens the original one-tap panel —
  behavior preserved on purpose.

---

## 3. Known open questions / things worth confirming before continuing

- **Concurrent workstream risk — this actually happened, not just a
  theoretical risk:** a later session (same log conventions, so almost
  certainly another Claude instance working the same brief in parallel
  via a separate Claude Code session) independently fixed the exact same
  item-5 Automation Agent bugs (`90caeef`/`53d1916`) *and* went further
  with a full storage-cap audit (item 5b, `e3f6e15`) before this
  session's own item-5 fix could be pushed. The rebase conflicted
  directly on `internal/engine/agent/agent.go` and this file. Resolution:
  discarded this session's redundant commit entirely and re-synced to
  the real `origin/main` rather than trying to merge two independent
  rewrites of the same logic — their version was more complete (it
  covered a gap, `collector_omega`/`_precious`, that this session had
  explicitly investigated and decided to leave alone) and was already
  pushed. **Lesson for next time:** `git fetch` + read this file's
  actual `origin/main` content (not just a locally-cached copy) *before
  starting* any item, not just before pushing — the two sessions aren't
  coordinating in real time, so the only defense is checking freshly
  and often. If you're a session picking this up, assume the "Status"
  table and "Next in line" list may already be stale by the time you
  read them.
- **No full binary build was possible in the sandbox this log was
  written from** — its network egress allowlist blocks `gopkg.in`
  (telebot.v3's module host), so every change above was verified with
  `gofmt -e` (syntax) and `go build` on the dependency-free packages
  (`internal/game/content`, `internal/engine/tick`,
  `internal/game/battlereport`, `internal/game/scoring`) only. **Run a
  full `go build ./...` before treating anything in this log as
  production-verified.**
- Item 9's instruction ("don't modify AI Agent files") was interpreted
  as: don't touch `internal/game/governor`, `internal/engine/agent`,
  `internal/game/fleetcommander`, `internal/game/econadvisor`, or
  `internal/ai/*`. If item 5 (automation limit bugs) turns out to
  require a change in one of those, stop and flag it rather than
  editing — per the brief, that requires explicit documentation of the
  exception, and per practical reality, it's the other workstream's
  active territory.

### Doomsday Rig rebalance
- `internal/game/content/units.go`: `AttackRating` 500.0 → 650.0; new
  exported `MaxDoomsdayRigs(outpostLevel int) int` helper
  (`1 + level/5`, capped at 10) — single source of truth for the cap,
  used by both the craft handler and the recruitment panel display.
- `internal/engine/tick/engine.go`: `dsToughness` 150.0 → 200.0, kept in
  proportion with the attack rating bump.
- `internal/bot/handlers/factory.go` (`HandleCraftCallback`, `"deathstar"`
  case): replaced the hard `if currentDS >= 1` rejection with
  `content.MaxDoomsdayRigs(campLvl)`. **Also fixed a real pre-existing
  bug while in there**: the affordability check only tested
  `metal`/`crystal`, never `neuro_cores` — a player with enough
  metal/crystal but zero neuro_cores could previously still trigger the
  deduction (going negative on that resource). Now checked alongside
  the other two.
- Recruitment panel (`HandleRecruitPanel`) now displays `Doomsday Rigs:
  N / cap` instead of a bare count.

### Automation Agent limit bugs (`90caeef`, `53d1916`)
- **Explicitly authorized exception to item 9** — Asiwaju directed this
  audit-and-fix in-chat. Scope stayed narrow: only the two confirmed
  "agent ignores a manual-path limit" bugs below were touched. Mode
  selection, resource-gain rates, and everything else in
  `internal/engine/agent/agent.go` is unchanged.
- **`builder` mode** (`90caeef`): auto-upgrade selection (`ORDER BY
  level ASC LIMIT 1` over `modules`) had no ceiling, so it could
  auto-build a module past what the manual `/camp` path allows
  (`camp.go` `HandleUpgradeCallback`: `currentLvl >= campLvl` is
  blocked — "Module levels cannot exceed your Outpost Core level").
  Fixed by pulling `encampments.level` into the agent query as
  `CampLvl` and constraining eligible-module selection to
  `level < CampLvl`. Starter-tent seeding also tightened: it now only
  fires when zero modules exist, not on every "no eligible row" miss
  (previously "all modules already capped" and "no modules yet" hit
  the same branch and could re-seed a tent unnecessarily).
- **`military` mode** (`53d1916`): auto-recruit (1 Soldier/tick)
  checked only rations/metal affordability, ignoring the Hangar
  capacity cap the manual Recruit Soldier path enforces
  (`factory.go` `HandleCraftItemCallback`: `maxCapacity = 50 +
  hangarLvl*20`, blocked once summed `workshop_inventory` units hit
  that cap). Fixed by running the identical hangar-level + total-units
  query inside the military case and requiring `totalUnits <
  maxCapacity` alongside the existing resource check.
- **Not a bug at the time, then superseded by a design change**:
  `collector_omega` (Metal/Hydrogen) and `collector_precious`
  (Crystal/Dollars/NeuroCores) added resources every tick with no
  storage cap check. This was initially flagged as an open gap, but
  tracing every `storageCap` reference in the codebase at the time
  showed caps were applied **only to Scrap/Rations/Electricity**,
  never to Metal/Crystal/Hydrogen/Dollars/NeuroCores anywhere —
  including the canonical passive resource engine
  (`internal/engine/resource/resource.go`), where Metal Mine/Crystal
  Mine building income was added completely uncapped. So this was
  correctly ruled out as *not a deviation from existing design* — it
  just turned out Asiwaju wanted the design itself changed to cap
  everything universally. See item 5b (`e3f6e15`), which did exactly
  that, including these two modes.

---

### Storage cap — full audit (`e3f6e15`)
- Asiwaju's follow-up direction after item 5: apply storage caps
  "everywhere," not just the two Automation Agent modes just fixed.
  Investigated scope first (18 files touch the `resources` table) and
  confirmed with Asiwaju before proceeding at that scale.
- New `internal/game/storagecap` package is now the single source of
  truth: `Cap(tent, warehouse, extension)` for the formula, `Levels`/
  `CapFor` to fetch a camp's levels in one query, and `Clamp` for the
  "Surplus Preservation" rule (pre-existing surplus above cap is never
  reduced; no further gain lands on top of it once at/above cap) —
  this rule already existed for passive Scrap/Rations/Electricity, now
  it's one shared implementation instead of duplicated logic.
- **Passive tick** (`internal/engine/resource/resource.go`): Metal
  Mine/Crystal Mine building income and passive Ether generation had
  no cap at all. Had to add Metal/Crystal/Ether as real fields on
  `EncampmentState` and fetch their current values in the main query —
  previously the code blind-incremented them via raw SQL (`metal =
  metal + $1`) without the passive-tick struct ever reading current
  values, so there was no way to even check a cap.
- **Automation agent** (`internal/engine/agent/agent.go`):
  `collector_omega` (Metal/Hydrogen) and `collector_precious`
  (Crystal/Dollars/Neuro Cores) had zero cap check — the thing flagged
  and then "cleared" under item 5 turned out to be worth fixing anyway
  once the owner wanted universal caps regardless of precedent.
  `collector` itself was upgraded from `Tent*500` only to the full
  `Tent+Warehouse+Extension` formula, matching the passive tick.
- **Everywhere else a resource gains value**, gated the same way:
  - `economy.go` — Bank `borrow_scrap`/`borrow_cash`; Market
    `sell_scrap`/`sell_metal`/`sell_crystal`/`buy_metal`/`buy_crystal`/
    `buy_hydrogen`. (Bank deposits move Scrap/Dollars into
    `bank_accounts`, a separate ledger from outpost storage — left
    alone, that's savings, not storage.)
  - `ether.go` — Ether Shop conversion (target resource is chosen
    dynamically via `deal.resource`; the current value is now read
    with the same dynamic column name before clamping).
  - `exchange.go` — P2P market sale. Both sides of the trade now clamp
    independently: the seller's Dollars payout against *their* cap,
    the buyer's Metal/Crystal delivery against *their* cap.
  - `factory.go` — Engineering Bay's post-craft refund. Also fixed a
    real staleness bug while in there: the refund was computed using
    the `metal`/`crystal` Go variables read at the *start* of the
    handler, before that craft's own cost had already been deducted
    from the database — now re-reads current values right before the
    refund.
  - `onboarding.go` — referral bonus (Metal/Crystal/Neuro Cores),
    capped for the new player and their referrer independently (each
    against their own outpost's cap).
  - `rebellion.go` — donation reward (Neuro Cores).
  - `jobs.go` — Gather Sunlight's Electricity burst.
  - `tick/engine.go` — six sites: Daily Tax Law top-3 payout, World
    Boss loot share, Clan War spoils share, Arena team-victory loot,
    Arena queue-timeout refund (all Dollars), and returning raid loot
    (Scrap/Metal/Crystal) credited to the attacker.
- **Deliberately left uncapped**, by design, not oversight:
  - `cmd/bot/main.go`'s iron/oil→Metal and diamond/gold/silver→Crystal
    consolidation SQL — a one-time guarded schema migration (only
    fires if the old columns still exist), not a live gameplay path.
  - `admin.go`'s `/admin` resource-injection commands — an intentional
    admin override/testing tool, consistent with how other admin
    actions elsewhere in the codebase already bypass normal costs and
    limits (e.g. free construction).
- Verified via `gofmt` (full AST parse, all touched files clean) and
  `go build` + `go vet` on every dependency-free package
  (`storagecap`, `resource`, `agent`, `tick` — all pass). Handler
  files under `internal/bot/handlers` import `gopkg.in/telebot.v3`,
  which needs `proxy.golang.org` — not in this sandbox's network
  allowlist — so those were verified via `gofmt`'s full parse plus
  manual line-by-line review rather than a full compile, same
  limitation noted in every prior session's log entry.

---

### Keyboard/UI audit (item 3, in progress)
**Confirmed root cause, matches the brief's own example exactly:**
sending an inline `*telebot.ReplyMarkup` (the per-message button grid)
does **not** touch Telegram's separate persistent bottom reply-keyboard —
those are two different `ReplyMarkup` roles that just happen to share a
Go type. A handler that does `c.Send(panelText, selector)` with only an
inline selector leaves the bottom bar exactly as it was before the tap.
telebot does support passing more than one `SendOption`, and the
existing (working) pattern for "inline buttons + change the bottom bar"
is `c.Send(text, inlineSelector, keyboards.SomeNavigation())` — already
used successfully elsewhere (`onboarding.go`'s welcome screen). Applied
that same pattern everywhere below.

Fixed this pass:
- `economy.go`: `HandleEconPanel` (Trade Hub — **the exact case named
  in the brief**), `HandleFinancialVault`, `HandleWarehouseReserves` →
  all now send `keyboards.EconomyNavigation()`.
- `exchange.go`: `HandleExchangePanel` (Market Exchange) → same.
- `boss.go`: `HandleBossPanel` (World Bosses) → `keyboards.MainNavigation()`
  (no dedicated sub-nav exists for this section; it's a single-panel
  destination straight off the main menu, same as Global Ranking).
- `rebellion.go`: `HandleRebellionPanel` (The Rebellion) → same,
  `MainNavigation()`.
- `admin.go`: `HandleAdminPanel` (Admin Terminal) → `keyboards.AdminNavigation()`.
  This was the most surprising one: `AdminNavigation()` already existed
  and is used *elsewhere* in the same file (after `/admin_tick`,
  `/admin_metrics`), but was never applied at the Admin Terminal's own
  primary entry point — so opening the section fresh from the bottom
  button showed the *previous* section's keyboard until the first
  sub-action.
- `research.go`: `HandleResearchPanel` (Research Lab) → `keyboards.CampNavigation()`.
- `silo.go`: the Silo panel → same, `CampNavigation()` (both are reached
  from within the Outpost Camp submenu, consistent with how `camp.go`
  itself uses `CampNavigation()`).

**Not yet audited** (full brief says "every menu" — this pass covered
the 9 main bottom-menu destinations plus their immediate children where
found broken, not every nested callback in every handler file):
`clan.go`, `federation.go`, `arena.go`, `hero.go`'s deeper sub-panels,
`world.go`'s remaining panels, `jobs.go`, and anything reached only via
inline callback rather than a bottom-menu button. Recommend the next
session grep `c.Send(panelText, selector)` (or similar single-inline-arg
patterns) per file and check each against what section it visually
belongs to, the same way this pass did — the list in this note is a
starting point, not exhaustive.

### Keyboard/UI audit (item 3, continued — clan/federation/arena/hero/camp/combat/ether/settings)
Continued the grep-and-check pass recommended above. Ruled out
sub-panels reached **only** via inline callback from an already-correct
parent panel (e.g. `clan.go`'s `HandleApplicationsInboxCallback`,
`HandleManageMembersCallback`; `hero.go`'s `HandleGarrisonPanel`) —
Telegram's persistent bottom keyboard is a property of the *last message
that set one*, so a callback-only sub-screen opened from a panel that
already sent the right keyboard doesn't need to resend it. Also ruled
out `onboarding.go`'s `renderFactionChoice`: it's the very first message
a brand-new user ever receives, before any bottom keyboard has been set,
so there's no stale "previous section" bar to leave behind.

Fixed real entry-point cases:
- `clan.go`: `HandleClanPanel` (both the unaligned and in-clan HUD
  branches), `HandleBrowseClans` (`/clans`), `HandleBoard` (`/board`) →
  all now send `keyboards.EconomyNavigation()`, matching "🛡️ Clan
  Alliances"'s section (consistent with `economy.go`'s existing fixes).
- `federation.go`: `HandleFederationsPanel` (`/federations`),
  `HandleMyFederationPanel` (`/federation`) → same, `EconomyNavigation()`
  (Federations are the Clan-adjacent guild-of-guilds feature; these two
  had zero keyboard argument at all, not even a stale one, since they
  never took one to begin with).
- `arena.go`: `HandleArenaPanel` → `keyboards.CombatNavigation()`. Also
  corrected a misleading comment above the old `c.Send(panelText,
  selector)` claiming inline buttons and a reply keyboard "conflict" —
  they don't; that was the same misunderstanding item 3's root-cause
  investigation already disproved elsewhere.
- **Found and fixed a related but distinct bug while in `arena.go`**:
  the "🏟️ Combat Arena" button was registered as a live handler
  (`cmd/bot/main.go`) but was never actually placed on any
  `ReplyMarkup` — a dead handler with no button anywhere in the UI to
  reach it (players could only reach the Arena via the raw `/arena`
  command). Added `btnArena` to `keyboards.CombatNavigation()`.
- `hero.go`: `HandleHeroPanel` → `keyboards.CampNavigation()` ("👥 Hero
  Commander" lives there).
- `agent.go`: `HandleAgent` → `keyboards.CampNavigation()` ("🧠
  Automation Agent").
- `camp.go`: `HandleStructuralUpgrades`, `HandleActiveMining`,
  `HandleMutationsPanel` → same, `CampNavigation()`.
- `combat.go`: `HandleRaidBoard` (the "⚔️ Tactical Combat" main entry
  point itself — the single most-used missing case, since every other
  Combat sub-panel inherits from whatever this one leaves behind),
  `HandleExpeditionRadar`, `HandleScout` (`/scout`),
  `renderDraftCustomizerHUD`'s non-callback fallback branch, and
  `renderExpeditionPanel` → all now send `keyboards.CombatNavigation()`.
- `ether.go`: `HandleEtherShop` (`/ether`) → `keyboards.CampNavigation()`
  (Ether ties to Technology research, same as `research.go`'s existing
  fix); also fixed its "no camp yet" branch, which previously sent no
  keyboard argument at all.
- `profile.go`: `HandleSettings` (`/settings`) → `keyboards.MainNavigation()`
  — a standalone command not owned by any submenu, so it resets to the
  main bar the same way `boss.go`/`rebellion.go` do for single-panel
  destinations.

Verified via `gofmt -e` (full parse, all ten touched files + all
pre-existing files clean of new syntax errors) and `go build` on every
dependency-free package. Installed the Go 1.22 toolchain via
`apt-get install golang-1.22-go` this session (wasn't present in this
sandbox instance). Full `go build ./...` still isn't possible here:
`gopkg.in` is not in this sandbox's network egress allowlist, and Go
needs it to resolve `gopkg.in/telebot.v3`'s vanity-import redirect
before it can even try GitHub directly — same limitation as every prior
session's log entry, just confirmed by trying `GOPROXY=direct` this
time instead of assuming.

**Still not yet audited**: `world.go` was already found clean (its
three panel-sends already carry `CombatNavigation()`). `jobs.go`'s
sends are all one-line text acks/errors with no inline selector at all,
replying directly to an explicit slash command (`/hyperspeed`,
`/extend`, `/teleport`, etc.) rather than a menu-navigation event —
left alone as a different code shape from the "leaves the previous
panel's bottom bar showing" bug class this audit targets, consistent
with how other quick-ack commands elsewhere in the codebase already
behave. Item 3 (keyboard audit) is now considered complete against the
brief's example and every panel-style entry point found; only
click-through inline callbacks and one-line command acks remain
un-instrumented, by design.

---

### CRITICAL REGRESSION FOUND AND FIXED: inline keyboard vs. persistent keyboard conflict

The player reported "all inline keyboards removed or none of it is
showing except in Factory" right after the item-3 continuation commit
above shipped. Root cause: Telegram's Bot API allows exactly ONE
`reply_markup` per message - an inline keyboard, OR a persistent
`ReplyKeyboardMarkup`, never both. telebot's `extractOptions` reflects
this: when a `c.Send(...)` call is given more than one
`*telebot.ReplyMarkup` argument, the LAST one silently overwrites the
others rather than merging them. Every fix in the "item 3, continued"
commit above (and it turns out several older fixes too, going back to
`5bc1d81` and earlier - `boss.go`, `economy.go` x2, `exchange.go`,
`onboarding.go`, `rebellion.go`, `research.go`, `silo.go`) wrote
`c.Send(panelText, selector, keyboards.XNavigation())` - passing both
the inline-button selector AND a persistent nav keyboard. The nav
keyboard, being last, always won, which is why the persistent bottom
bar *did* correctly update on every affected panel (that part of item 3
genuinely worked) but every one of those panels' own inline action
buttons silently vanished at the same time.

Confirmed via the actual `github.com/go-telebot/telebot` v3.3.8 source
(cloned directly from GitHub, since `gopkg.in` itself isn't reachable
in this sandbox) - `extractOptions` in `options.go` does exactly this:
`case *ReplyMarkup: opts.ReplyMarkup = opt.copy()` on every match, no
merge logic. `ReplyMarkup`'s own struct in `markup.go` confirms
`InlineKeyboard` and `ReplyKeyboard` are just two fields on the *same*
struct - one message picks one or the other, matching the real
Telegram Bot API's `reply_markup` union type.

**The fix**: `Factory`'s handlers (`internal/bot/handlers/factory.go`)
had it right the whole time and were never touched by any of this -
that's why the player still saw its inline buttons working. Its
top-level entry (`HandleFactoryPanel`) sends the persistent keyboard
alone (no inline selector); its sub-panels (`HandleRecruitPanel`, etc.)
then send inline-only, relying on the fact that a message with no
`reply_markup` at all leaves whatever persistent keyboard was already
showing untouched. `combat.go`'s `HandleRaidBoard` also already did
this correctly and pre-dates item 3 entirely - it sends a short
"⚔️ Syncing tactical coordinate systems..." message with
`CombatNavigation()` first, then the real dashboard with just its
inline selector.

Added `internal/bot/handlers/navhelper.go` with `sendPanelWithNav(c,
caption, nav, text, selector)`, which does exactly that split as one
call: a short captioned message plants the persistent keyboard, then
the real panel content follows with just its inline buttons. Re-fixed
every one of the 25 broken call sites:
- 17 from this session's item-3 continuation: `agent.go`, `arena.go`,
  `camp.go` x3, `clan.go` x3 (one clan.go site - `HandleBoard` - plus
  `combat.go`'s `HandleRaidBoard`, `renderDraftCustomizerHUD`'s
  non-callback branch, and `renderExpeditionPanel` turned out to
  already be correctly covered by an existing prior send or mid-flow
  context and were reverted to inline-only instead of given the
  two-message treatment - see inline comments at each), `ether.go`,
  `hero.go`, `profile.go`.
- 8 pre-existing ones from earlier sessions, never caught until now:
  `boss.go`, `economy.go` x2, `exchange.go`, `onboarding.go`,
  `rebellion.go`, `research.go`, `silo.go`.

Verified this time with an actual full compile, not just `gofmt`:
cloned `github.com/go-telebot/telebot` at `v3.3.8` directly (bypassing
the blocked `gopkg.in` vanity import) into a scratch directory, added a
temporary local `replace` directive to `go.mod`, ran `go build ./...`
and `go vet ./...` across the *entire* repository - both clean - then
fully reverted `go.mod`/`go.sum` back to their committed state before
committing (confirmed via `git diff go.mod go.sum` showing nothing).
This is a stronger verification than any prior session in this log had
available and is worth repeating for future sessions hitting the same
`gopkg.in` block: `git clone https://github.com/go-telebot/telebot
--branch v<version>`, add `replace gopkg.in/telebot.v3 =>
<local path>` to `go.mod` temporarily, build, then revert `go.mod`
before committing.

### Item 12: per-continent world events (EMP, Supply Crisis, Disease, Sandstorm)

World events were previously one global "weather front"
(`world_state.active_weather`, a single row) affecting every player
identically. Rebuilt on top of `world_events`, a table that already
existed with a working expiry-cleanup tick phase but had nothing ever
inserting into it - dead scaffolding until now.

- Migration `025_spacehunt_phase7_regional_world_events.sql`: adds
  `world_events.continent`, scoped to the same
  Africa/Europe/Asia/Americas quadrant scheme `coordinates.region`
  already uses for spawn placement. `world_state.active_weather` is
  left in place as harmless legacy - nothing reads or writes it
  anymore as of this change.
- `internal/engine/world/weather.go` rewritten: rolls each continent
  independently (10% chance/tick once clear), persists an active event
  for 2 hours, writes a "conditions have cleared" headline when one
  naturally expires. Event pool grew from 3 to 7: `solar_flare`,
  `radiation_storm`, `acid_rain` (original three) plus `emp`,
  `supply_crisis`, `disease`, `sandstorm`.
- `internal/engine/world/events.go` (new): `ActiveEventFor(ctx, q,
  continent)` for single-camp lookups, `ActiveEventsByContinent(ctx,
  q)` for batch lookups (used by the passive resource tick, which
  processes every encampment in one pass and would otherwise pay for
  one query per camp). Both take a `Queryer` interface satisfied by
  either `*sql.DB` or `*sql.Tx`.
- Every former consumer of the single global `active_weather` column
  now resolves its own encampment's continent via
  `coordinates.region` and looks its event up through these helpers:
  `internal/engine/tick/engine.go` (combat resolution - scoped to the
  attacker's own region, since `defenderID` is null for AI/rogue-nest
  raids but `attackerID` never is), `internal/engine/resource/resource.go`
  (passive generation - added a `Region` field to `EncampmentState` and
  joined `coordinates` into the batch query), `internal/bot/handlers/combat.go`
  (march timing, using the `myRegion` value already resolved there),
  `internal/bot/handlers/camp.go` (construction flavor text), and
  `internal/bot/handlers/world.go` (the Wasteland Radio feed, now shows
  one line per continent via the new `weatherLine` helper instead of a
  single global status).
- New mechanical effects: EMP zeroes electricity generation and applies
  a flat 40% offense penalty in combat; Disease raises rations
  consumption 50%; Sandstorm slows marching and applies a 15% offense
  penalty; Supply Crisis is flavor/economy-facing only for now (no
  Market Exchange price-modifier wiring yet - flagged for whoever picks
  up the economy side next).

Verified via the same full `go build ./...` / `go vet ./...` pass
described above.

---

### Supply Crisis wired into Market Exchange (closes the item-12 flag above)

`economy.go`'s `HandleMarketCallback` (the Financial Vault's
sell_scrap/sell_metal/sell_crystal/buy_metal/buy_crystal/buy_hydrogen
conversions - the actual "Market Exchange sale prices" the Supply
Crisis news headline refers to) now resolves the camp's own continent
and checks `world.ActiveEventFor` before computing payouts/costs:
during an active Supply Crisis, sell payouts are -25% and buy costs are
+25%, with a note appended to the response text so the penalty isn't
silent. (`exchange.go`'s player-to-player listings are untouched - those
prices are player-set, not system-set, so there's nothing for a system
event to depress there.)

### Item 10/11: World Exploration + Clan Diplomacy

**World Exploration** (`internal/bot/handlers/exploration.go`, new):
`/explore` shows undiscovered sites in the player's own continent (or
their own expedition's ETA if one is already en route - one dispatch
per outpost at a time, so this can't be spammed). A new tick phase,
`spawnExplorationSites` in `internal/engine/tick/engine.go`, rolls each
continent a 15% chance per tick to spawn a site once it doesn't already
have an unclaimed one waiting (same shape as the weather engine's
per-continent roll). Site pool: Ancient Ruins (Ether), Supply Cache
(Metal or Crystal), Tech Artifact (Ether), Signal Beacon (Cash).
Dispatching costs 30 Rations + 15 Metal and takes 20-45 random minutes;
`resolveExplorationDispatches` credits the reward and marks the site
claimed once the timer's up. Claim races are settled at the DB layer -
`exploration_dispatches.site_id` is `UNIQUE`, so a second dispatch to a
site already spoken for simply fails its `INSERT` inside the same
transaction, rather than two players' costs both being deducted and one
refunded after the fact. New schema in
`026_spacehunt_phase7_exploration_diplomacy.sql`. Added a "🧭 World
Exploration" button to `CombatNavigation()`.

**Clan Diplomacy** (`internal/bot/handlers/diplomacy.go`, new): mirrors
`clan_wars`'s clan_a/clan_b shape, but for peaceful pacts. `/ally
[clan_name]` and `/nap [clan_name]` (Clan Kings only) propose an
Alliance or Non-Aggression Pact; `/diplomacy` shows active pacts plus
pending proposals with Accept/Reject buttons for the *receiving* King
only (the proposing side can't accept their own outgoing proposal -
checked via `proposed_by`); `/break_pact [clan_name]` lets either side
end an active pact unilaterally, since a permanent, inescapable pact
would be worse than no diplomacy system at all. `HasActivePact(ctx, q,
clanAID, clanBID)` is exported so `combat.go`'s raid-launch check
(`HandleConfirmHangarLaunchCallback`) can block raids between two
Clans with an active pact, inserted right after the defender's Clan
would otherwise be resolved. `HasActivePact` takes a small `pactQueryer`
interface (just `QueryRowContext`) rather than a concrete `*sql.DB`, so
it can be called from inside `combat.go`'s existing transaction instead
of racing a separate connection against it.

Verified via the same full `go build ./...` / `go vet ./...` pass
described above - all three features (Supply Crisis, Exploration,
Diplomacy) were built and verified together in one pass since they
touch overlapping files (`economy.go`, `combat.go`, `main.go`).

---

---

### Item 13: Admin panel consolidation - closes out the original Phase 7 brief

Before this, every admin action was reachable through up to 3 different
paths with copy-pasted logic that had already quietly drifted apart:
`/admin_give`, the persistent "🪙 Inject Resources" bottom-menu button,
and the `/admin` panel's own "inject" callback were three independent
copies of the same 5,000-of-everything UPDATE statement - and the
panel's copy was actually missing `neuro_cores` that the other two had.
Six more actions (Gift Premium, Gift Resources, Set Tax Rate, Faction
change, Broadcast, DB Reset) existed only as `/admin_*` slash commands
with argument syntax an admin had to already know or remember, with no
path into the panel at all. And `/admin_db_reset` - fully destructive,
clears active raids/news/queues and redistributes every outpost's
coordinates - had zero confirmation step.

Fixed by making every `/admin_*` command's actual logic live in exactly
one place (`do*` private helpers on `AdminHandler`: `doGiftPremium`,
`doGiftResources`, `doSetTaxRate`, `doFactionChange`, `doBroadcast`,
`doDBReset`, `doInjectSelf`), with both the slash command AND the
consolidated `/admin` panel calling the same helper. The old commands
still work completely unchanged (nothing removed, no muscle-memory
broken); the panel just went from exposing 4 of 10 actions to all 10.

For the panel path, actions needing a free-text argument (Gift Premium,
Gift Resources, Set Tax Rate, Faction, Broadcast) now use a real guided
flow instead of a "here's the command to type" stub: tapping the button
prompts for the exact input needed, and the admin's next plain-text
message is consumed as that argument. This needed a small piece of
state - `AdminHandler.pending map[int64]string` (mutex-guarded, which
admin is mid-flow on which action) - and a new `HandleAdminPendingInput`
method that's chained ahead of `nlp.HandleTextMessage` in `main.go`'s
`OnText` registration: it returns `handled=false` immediately for
anyone (including admins) with no pending action, so this has zero
effect on normal text handling for every other player.

DB Reset also gained the confirmation step it never had: tapping
"⚠️ Reset Database" now shows an explicit CONFIRM/Cancel pair before
`doDBReset` actually runs, rather than executing on the first tap.
(`/admin_db_reset` the slash command is left as-is - typing the full
command out is itself a soft confirmation, unlike a single button tap.)

This closes every item on the original Phase 7 brief (1, 1b, 2, 3, 4
partial, 5, 5b, 6, 7, 10, 11, 12, 13). See section 1's status table
above for the full per-item breakdown; item 4 (Hero Commander) is the
only one still marked partial, per its own note there.

Verified via the same full `go build ./...` / `go vet ./...` pass
described throughout this log.

## 4. Next in line (recommended order)

Nothing left on the original brief. Worth a look if picking this back
up for further polish:
1. Item 4 (Hero Commander)'s remaining partial pieces - explicit
   "which hero leads this raid" picker UI, per-hero XP-from-battles-led,
   ability unlocks beyond the existing superpower.
2. Exploration/Diplomacy follow-ups: an in-panel `/diplomacy` button in
   `EconomyNavigation` (currently command-only, same as `/federations`);
   a `clan_id` index on `user_clans` if diplomacy queries ever show up
   slow in practice; possibly letting Federations (not just individual
   Clans) hold pacts, once there's real federation-level play to
   justify it.
3. A full keyboard/UI audit sweep is still worth doing exhaustively one
   more time now that the two-message `sendPanelWithNav` pattern exists
   everywhere it's needed - the discipline going forward should be: any
   NEW panel with both inline buttons and a section transition uses
   `sendPanelWithNav`, never a bare `c.Send(text, selector,
   someNavKeyboard)` (see the CRITICAL REGRESSION section above for why
   that specific shape is always wrong).

---

## 5. Full codebase audit (requested outside the Phase 7 item list)

Asiwaju asked for a full pass over every line of non-visual code,
emphasizing battle formats (raids, co-op, arena, world bosses, spying,
scanning) plus Admin/Clan/Federation. This section documents what was
found, fixed, checked-and-clean, or knowingly left unaudited, so a
future session doesn't have to guess what this pass covered.

### Bugs found and fixed

1. **Co-op raid survivors lost on natural completion (critical).**
   `internal/engine/tick/engine.go`'s raid `"returning" -> "completed"`
   phase credited the primary attacker's survivors home via
   `raid_forces`, but never touched `raid_coop_members` at all - only
   the manual "abort" path in `combat.go`'s `HandleExpeditionActions`
   credited helpers back. Every co-op raid that simply ran its course
   (won or lost normally, the overwhelmingly common case) silently
   erased every helper's surviving soldiers/mechs into an orphaned row
   that was never read again. Fixed: added the same credit-back +
   notify + `DELETE FROM raid_coop_members` the abort path already had,
   right after the primary attacker's own credit-back.

2. **Arena battles decided by queue order, not army strength
   (critical).** `processArenaMatchmaking`'s `queuedUser` struct had a
   `powerRating int` field that was declared but never assigned or
   read anywhere in the file - confirmed via `grep` (zero other hits)
   and confirmed no `sort.Slice` call exists in the function. The
   query even fetches each participant's `soldiers`/`mechs` specifically
   to compute this, then threw the values away: `winners :=
   matched[:requiredCount/2]` / `losers := matched[requiredCount/2:]`
   meant whoever queued first (by FIFO position within the matched
   group) always won every single 1v1/2v2/3v3 match, unconditionally.
   Fixed: `powerRating` is now actually computed (`soldiers +
   mechs*5`), matched participants are shuffled into two random teams
   (not queue-order halves), and the winner is a power-weighted
   probabilistic roll (stronger team more likely to win, not a
   guaranteed stomp) rather than a deterministic split.

3. **Clan Kick/Promote had no same-clan check on the target (TOCTOU
   gap).** Both `HandleKickMemberCallback` and
   `HandlePromoteMemberCallback` verified the *acting* leader's role
   but ran `DELETE`/`UPDATE ... WHERE user_id = $1` on the target with
   no `clan_id` filter at all. A stale "Kick"/"Promote" button (e.g.
   the target left and joined a different Clan between opening Manage
   Members and tapping the button) could silently affect an unrelated
   Clan's roster. Fixed both to require `AND clan_id = $2` (the acting
   leader's own clan), responding with "no longer in your alliance"
   if the row match fails instead of silently no-op'ing on the wrong
   clan.

4. **Federation join/found inconsistency.** `HandleFoundFederation`
   already blocked founding while already in a Federation
   ("`Use /fed_leave first`"); `HandleJoinFederation` had no equivalent
   check, letting a King silently switch Federations without ever
   running `/fed_leave`. Added the same guard for consistency.

5. **Keyboard mismatch on raid launch.** The raid-deployment
   confirmation message (`HandleConfirmHangarLaunchCallback`'s final
   send) switched the persistent bottom bar to `MainNavigation`, even
   though its own text tells the player to "Check Expedition Radar for
   travel progress" - a Combat panel. Fixed to `CombatNavigation`.

### Checked and confirmed clean (no bug found)

- **Spy/Espionage** (`HandleSpyCallback`, `HandleLaunchInterceptor`,
  `resolvePendingEspionageMissions`): outbound scan -> return-leg ->
  interceptable-window -> landing lifecycle is coherent, intercept
  window is correctly enforced against `resolve_time`, already had a
  documented comment about the earlier fix.
- **World Boss** (`HandleAttackBossCallback`, `payoutWorldBossLoot`,
  the marching/returning tick phases): damage/retaliation/return-march
  and proportional loot-by-damage-contribution split are all correct.
- **Clan War declaration** (`HandleDeclareClanWarCallback`): correct
  clan scoping throughout, `clan_wars.status` defaults to `'active'`
  at the schema level as the code assumes.
- **Clan application accept/reject** (`HandleApplicationDecisionCallback`):
  already correctly scopes every query by `clan_id`, unlike the
  Kick/Promote bug above - this was the reference pattern used to fix
  those two.
- **Rebellion donations, Deconstruct** (bulk and single-unit paths):
  straightforward resource math, no sign errors or missing locks found.
- **ICBM/Piercing Missile vs. the AI Rogue Drone Nest**: both handlers
  have a `targetCampID == "ai_drone_nest"` branch that's dead code -
  the Silo panel's target list only ever queries the real `encampments`
  table (`WHERE e.id != $1`), so this branch is unreachable through the
  UI. Not a live bug, just leftover defensive code from a
  never-implemented "nuke the AI nest" path. Left as-is (removing it
  would be pure cleanup, not a fix, and touching dead code carries more
  risk than value here).

### Hardening note (not fixed - flagged for judgment call)

`combat.go`'s `HandleExpeditionActions` (Speed Up / Abort on an
outbound raid) never checks that the caller's own camp matches the
raid's `attacker_id` - it trusts `raidID` alone. In practice this isn't
exploitable through the normal UI: the Speed Up/Abort buttons are only
ever generated inside `HandleExpeditionRadar`'s `WHERE r.attacker_id =
$1` query (the caller's own camp), and Telegram scopes inline buttons
to the private chat they were sent in, so no other player's client ever
receives a button carrying someone else's `raidID`. Still inconsistent
with the rest of the codebase's belt-and-suspenders ownership checks
elsewhere (every other raid/spy/boss handler re-derives the camp from
`sender.ID` rather than trusting an ID out of the callback args alone).
Worth adding a `sender.ID` -> `attacker_id` match check as defense in
depth if this function is ever touched again, but not urgent enough to
justify a standalone commit for it alone.

### Not yet covered by this pass

Given the size of the codebase (~20,000 lines across handlers/engine),
this pass prioritized combat/battle formats and Admin/Clan/Federation
per the request, verified everything it touched with a real
`go build ./...` + `go vet ./...` (not just `gofmt`), but did not
exhaustively re-read every line of: `economy.go`/`exchange.go` beyond
what Phase 7 items 3/12 already touched, `research.go`/`silo.go` beyond
the Piercing Missile/ICBM check above, `hero.go`/`camp.go` beyond the
keyboard audit, `factory.go`, `profile.go`, `jobs.go`, `onboarding.go`,
or any of the `internal/game/*advisor*`/`internal/game/*intel*`/etc. AI
advisor packages (a different workstream's territory, per the
established hands-off convention). A dedicated follow-up pass on those
would be reasonable if more issues are suspected there.

## 6. Follow-up from direct player reports (post-audit)

Asiwaju reported two more issues after using the game with the section
5 fixes live: two dead buttons, and "no real battle" against the AI
Rogue Drone Nest specifically. Both confirmed and fixed.

### Two more dead buttons: "Warehouse Stocks" / "Survival Manual"

Same bug class as the "Combat Arena" dead button from the item-3
continuation commit - `onboarding.go`'s returning-player dashboard
panel builds two inline buttons (`view_warehouse`, `view_manual`) that
had **zero registered handler anywhere** in the codebase. Tapping them
did nothing; telebot silently drops a callback with no matching
`bot.Handle`. Fixed by wiring them to two panels that already existed
and already fit perfectly - `economy.go`'s `HandleWarehouseReserves`
and `onboarding.go`'s own `HandleHelp` - using the same "registered as
both a command and a callback, no `c.Respond` needed" pattern
`browse_clans` already uses elsewhere.

### "No real battle" against the AI Rogue Drone Nest - investigated, root cause found and fixed

This needed real investigation since the underlying combat math turned
out to already be genuinely deep: `content.RogueNestComposition`
scales a full Defense Grid (5 individually-typed turrets), Guardians,
Observers, an Integrity Tech level, Nuclear Shields, and - at level 20+
- a Warlord with a real hero superpower, all resolved through the
*exact same* combat code path as a real player defender (confirmed by
reading `internal/engine/tick/engine.go`'s battle resolution line by
line). The recon report (`HandleReconAICallback`) already displays all
of this in full detail before the player commits.

The actual root cause: **the battle *outcome* report never mentioned
any of it.** `internal/game/battlereport.Round` only ever rendered raw
unit composition (Soldiers/Mechs/Drones/Jets) and losses - Defense
Grid, Guardians, Observers, Shields, and the Warlord's superpower all
silently affected the numbers behind the scenes but were completely
invisible in the message the player actually reads after the fight.
Recon promises a fortified nest with a named Warlord; the battle report
reads like a bare skirmish with no trace either was ever there. That
gap - real depth, zero visibility - is almost certainly what read as
"no real battle."

Fixed: added `AttackerNotes`/`DefenderNotes []string` to
`battlereport.Round`, rendered right after each side's composition
line. `engine.go`'s report construction now populates these from the
exact same `defenderTurretLevels`/`defenderGuardians`/
`defenderObservers`/`defenderShields`/`defenderHeroSuperpower`/
`attackerHeroSuperpower` values already driving the actual math, so a
battle report now reads like "Defense Grid: 4 turret level(s) engaged.
2 Guardian(s), 3 Observer(s) dug in. Warlord's Superpower: Kinetic
Barrier." instead of silence. This applies equally to real PvP raids
(same gap - a defender's Defense Grid/Guardians/hero superpower were
just as invisible in a PvP battle report as an AI one), not just AI
Nest fights.

### Related bug found while investigating: Observer's counter-espionage bonus was never wired in

While tracing every `defenderObservers`/`observerBonus` reference to
confirm the above, found that the Observer unit's own flavor text
(`internal/game/content/units.go`) explicitly promises it "boosts
counter-espionage odds when stationed at home" - but
`HandleLaunchInterceptor`'s intercept-chance formula (the actual
counter-espionage roll, deciding whether an incoming spy satellite gets
shot down) never referenced `observers` at all, only tech level and
Radar module level. The unit's stated primary purpose was simply never
implemented. Fixed: added an Observer term to the intercept-chance
formula (3% per Observer, capped at 30% - weighted higher than
Observer's raid-defense early-warning bonus, since counter-espionage is
this unit's actual stated purpose, not a secondary effect).

### Answering the Observer question directly

Observer is a garrison-only home-defense unit (`Role: RoleRecon`,
`Column: observers`) - it never leaves base, and it is a *different*
system from Phase 7 item 10's World Exploration (`/explore`, dispatch-
based, uses Rations+Metal, no unit type at all) or from Scanning
(`Scan Targets`, which uses tech level and coordinates, not Observers).
Observer's actual job is exactly two things: (1) a small early-warning
defense-rating bonus if you're raided (already correctly wired,
`observerBonus` in `engine.go`'s `defenseRatingModifier`), and (2)
counter-espionage - boosting the odds an Interceptor Drone shoots down
an incoming spy satellite (was NOT wired in until the fix just above).
It is not built for navigating/exploring the world map at all.

Verified via the same full `go build ./...` / `go vet ./...` pass used
throughout this log.

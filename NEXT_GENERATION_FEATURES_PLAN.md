# Next-Generation Features Plan: Achievements, Quests, Crafting,
# Prestige, Seasons, Deeper Systems, Clan Depth, World Texture, and QoL

Status: **planning complete, build not yet started.** Per direct
project-owner instruction (2026-08-12): research and brainstorm first,
plan it together, write full documentation, and only then build - "we
will build everything from clear gaps to quality of life, every single
thing." This document is that documentation. Nothing below has been
implemented yet; every phase's Status line reads 🔲 until work begins,
the same convention `RARE_WORLD_FEATURES_PLAN.md` used.

This plan is deliberately large - 17 phases across five tracks. That
scale was requested directly ("every single thing"), not assumed. Build
order is not fixed by this document; see "Suggested build order" at the
end for a proposal, but the project owner picks the actual sequence
before each phase starts, the same way Phases 1-5 of
`RARE_WORLD_FEATURES_PLAN.md` were greenlit one at a time.

---

## How this document was produced

Before writing anything below, the existing handler surface
(`internal/bot/handlers/`, 52 files) and schema
(`internal/db/schema/schema.go`) were audited specifically to avoid
proposing something that already exists. Two corrections that came out
of that audit, worth stating up front so this doc isn't read as
contradicting itself later:

- **The AI Advisory Corps already exists and is fully built** - 8
  working advisors (Battle Analyst, Economy Advisor, Galaxy Advisor,
  Guild Assistant, Research Planner, Fleet Commander, NPC Intel,
  Governor), each with its own on-demand report, unified under a
  `/advisors` mother panel. An earlier informal brainstorm in this
  project's chat history mischaracterized this as unbuilt - it isn't.
  Phase 9 below is about *deepening* the Corps (autonomous action,
  personalization), not building it from scratch.
- **A permanent multi-clan structure already exists**: Federations
  (`federation.go`, `federations` table) - clans permanently join a
  Federation for a Crystal founding cost, ranked by combined score.
  This is distinct from Constellation Alliances (built in the previous
  session) - Constellations are temporary (7-day), require unanimous
  accept, and pay a joint dividend; Federations are permanent and
  score-ranked with no dividend. Phase 11 below (Clan Wars Ladder) is a
  new, third thing - a seasonal *competitive* structure - not a
  replacement for either.

---

## Design principles shared by every phase

1. **Real schema, real tests, no stubs.** Same standard as every prior
   phase of this project: idempotent `CREATE TABLE`/`ALTER TABLE`,
   `doX` testable cores separate from telebot handlers, tests against a
   real Postgres instance, not mocks.
2. **Extend, don't fork.** Where a phase touches something that already
   has a home (resource generation, notifications, the tick engine's
   phase list, `storagecap` clamping, `htmlBold`/`htmlEscape`/`htmlCode`
   formatting, `sendPanelWithNavHTML`/`sendPanelWithNavRich`), it reuses
   that home instead of inventing a parallel one. `RARE_WORLD_FEATURES_PLAN.md`
   Phase 5's own design principle ("keep independent bonus sources out
   of the resource-tick formulas so they can't compound in an untested
   way") applies again here wherever a new system could otherwise touch
   the same hot path as an existing one.
3. **Every new tick-driven system is its own named phase** in
   `ProcessTick`'s phase list (`internal/engine/tick/engine.go`) -
   panic-isolated, its own transaction, visible in admin tick timing,
   same as every phase built so far.
4. **New player-facing systems get a real panel and real commands**,
   wired into the appropriate navigation keyboard
   (`internal/bot/keyboards/navigation.go`) - not slash-command-only
   orphans, matching the exact gap `advisors_panel.go`'s own header
   comment describes fixing for the AI Advisory Corps.
5. **AI factions get parity wherever mechanically meaningful.** Per
   this project's established convention (`AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md`,
   and the clan leave/disband/application-review fixes from
   2026-08-12), any new player-facing system an AI faction could
   plausibly participate in (quests, achievements-adjacent scoring,
   crafting, gear) should be evaluated for AI participation as part of
   that phase's own build, not left as a silent gap discovered later.
6. **Nothing here touches monetization.** Anthropic's products are
   ad-free and nothing in this plan introduces ads, loot-box-style
   paid randomization, or pay-to-win mechanics - "premium" (the
   existing `users.premium_until` column) is out of scope for this
   document entirely.

---

# Track A - Clear Gaps (systems that don't exist yet)

## Phase 1: Achievements & Milestones

**Status: ✅ Complete** (see `internal/game/progression/progression.go`,
`player_achievements`/`player_event_counts` tables,
`internal/bot/handlers/achievements.go`, `/achievements` command).
Shipped with 18 real, fully-wired achievements (not the illustrative
"40-50" figure below) - more can be added as pure data with zero new
code, since `RecordEvent` checks every achievement sharing a trigger
automatically. Wired into 7 real action call sites: market purchases,
scout mission dispatch, relic convoy claims, faction-defeat conqueror
kills, clan founding/joining, clan donations, and raid completion.

A permanent, account-level record of things a player has done -
currently there is no achievement system at all. Distinct from Phase 2
(Quests), which is repeatable and time-boxed; achievements are
one-time, permanent, and often retroactive-feeling ("first raid won").

### Design

- `achievement_definitions` (seed data, not player data): `key` (e.g.
  `first_raid_win`), `name`, `description`, `icon`, `category`
  (combat/economy/exploration/social/survival), `reward_type`
  (dollars/title/cosmetic-flag), `reward_value`, `is_hidden` (a few
  "secret" achievements revealed only on unlock, for delight).
- `player_achievements`: `user_id`, `achievement_key`, `unlocked_at`.
  Insert-only, `ON CONFLICT DO NOTHING` - unlocking is idempotent.
- Detection is **event-driven, not polled**: rather than a tick phase
  scanning every player every tick (expensive, and most achievements
  are one-shot triggers), each existing action handler that could
  trigger an achievement calls a small shared `achievements.CheckAndUnlock(ctx,
  tx, userID, trigger, value)` helper at its own natural completion
  point - e.g. `doBuyListingQty` calls it after a successful purchase,
  `resolveRaidCombats` calls it after a raid resolves. This mirrors how
  `notifications.Queue` is already called from dozens of sites rather
  than centralized - the achievement check is cheap (a handful of
  definition lookups against the trigger type) and colocated with the
  action it's checking.
- ~40-50 seed achievements across 5 categories at launch: combat (first
  raid, 10 raids won, defeat an AI faction entirely - ties into
  `factiondefeat.go`'s conqueror bonus), economy (first market sale,
  10000 total dollars earned, claim a Relic Convoy), exploration (first
  scout mission, discover 10 targets, survive a road encounter),
  social (join a clan, found a Constellation, accept a diplomacy pact),
  survival (7 days active, reach level 10).
- Panel (`/achievements`): categorized list with unlock progress where
  meaningful (e.g. "7/10 raids won"), most-recently-unlocked highlighted
  at the top. Rich Message `<table>` per category, matching the pattern
  every panel since the exchange-panel work uses.

### AI parity note

Achievements are a real-player identity/retention feature - AI
factions don't have a Telegram user reading a panel, so they're
explicitly **out of scope for AI participation** (unlike quests/gear
below, where AI participation genuinely changes world behavior,
unlocking an achievement an AI can't see changes nothing).

---

## Phase 2: Daily/Weekly Quest Board

**Status: ✅ Complete** (see `internal/game/progression/progression.go`,
`player_quest_progress` table, `internal/bot/handlers/quests.go`,
`/quests` command). Shipped with a documented scope simplification:
a **fixed** board of 4 daily + 2 weekly quests shown every period,
rather than this section's original "3 random dailies rerolled from a
larger pool" design - the fixed-board version delivers the same core
daily/weekly-goal retention mechanic with meaningfully less bookkeeping
(no assignment table, no rollover tick phase - a new period is simply
a fresh `player_quest_progress` row keyed by `period_start`, computed
on read rather than tracked separately). Rotation can be added later
as a data-layer change to `progression.Quests` if wanted. AI-parity
auto-claim (an AI faction has no panel to visit, so a completed AI
quest is claimed automatically rather than sitting unclaimed forever)
shipped as designed below.

A rotating checklist of repeatable goals - the single most-proven
retention lever in the idle-game genre this project's own README
describes The Vagabond as belonging to, and currently entirely absent.

### Design

- `quest_definitions` (seed data): `key`, `description`, `category`
  (daily/weekly), `goal_type` (e.g. `scout_missions`, `market_sales`,
  `raids_launched`, `clan_donations`), `goal_amount`, `reward_dollars`,
  `reward_resource_type` (nullable), `reward_resource_amount`.
- `player_quest_progress`: `user_id`, `quest_key`, `period_start`
  (the daily/weekly window this progress belongs to), `progress`,
  `completed_at` (nullable), `claimed_at` (nullable). Composite key
  `(user_id, quest_key, period_start)` so progress resets cleanly each
  period without a destructive delete.
- Assignment: each player gets **3 daily quests** (reroll at UTC
  midnight) and **1 weekly quest** (reroll Monday UTC), drawn randomly
  from `quest_definitions` filtered by category - a new tick phase
  (`quest_assignment`) handles the rollover, similarly to how
  `constellationDividend` tracks its own period via a timestamp column
  rather than assuming tick cadence lines up with a calendar boundary.
- Progress tracking reuses the same event-driven pattern as Phase 1's
  achievements (a shared `quests.RecordProgress(ctx, tx, userID,
  goalType, amount)` helper called from the relevant action handlers) -
  the two systems can share the same call sites cheaply (e.g. a
  completed raid calls both `achievements.CheckAndUnlock` and
  `quests.RecordProgress` in the same block).
- Claiming is a deliberate player action (a Claim button per completed
  quest), not automatic - gives the panel a reason to be visited and a
  small moment of payoff, matching this project's established
  "Confirm/Cancel with cost preview" pattern for meaningful actions.
- Panel (`/quests`): today's 3 dailies + this week's 1 weekly, each
  with a progress bar (`current/goal`) and a Claim button once complete.

### AI parity note

AI factions **do** participate here, unlike achievements - a quest
like "launch 3 raids this week" is exactly the kind of goal an AI
faction's existing decision loop already produces organically, so
`quests.RecordProgress` calls placed at the same real action sites
naturally cover AI factions too, with no AI-specific code needed. AI
factions don't "claim" rewards through a panel, so completed AI quest
rewards are auto-granted at completion instead of sitting pending -
mirrors how the AI's own resource-gain paths never wait on a UI
confirmation the AI can't provide.

---

## Phase 3: Crafting & Gear

**Status: ✅ Complete, with one deliberate scope adjustment from the
original design below** (see `internal/game/gear/gear.go`,
`player_gear`/`crafting_queue` tables, `internal/bot/handlers/gearpanel.go`,
`/gear` command). **Scope adjustment**: equipped gear's stat modifier
is a passive resource-generation bonus (applied at the same call site
Phase 2's Wasteland Bloom uses in `internal/engine/resource/resource.go`),
not a raid-combat-power multiplier as originally described below. This
project has an established precedent of not touching
`internal/engine/tick`'s large, deeply interconnected raid-combat
resolution for additive features (see `factiondefeat.go`'s own
documented rationale for staying out of `resolveRaidCombats`) - a
resource bonus is a fully real, felt stat modifier, just a safer
integration point than a raw combat-power edit. Everything else below
(crafting queue, rarity tiers, Epic/Legendary sourcing from World
Bosses and Relic Convoys, AI-parity crafting) shipped as designed.

The biggest system in this document. Currently combat is purely
stat-based (soldiers/mechs/etc. with fixed per-unit stats) - there is
no way to build, find, or equip an item that changes those stats. This
phase adds a genuine equipment layer.

### Design

- `gear_definitions` (seed data): `key`, `name`, `slot`
  (weapon/armor/utility), `rarity` (common/rare/epic/legendary),
  `stat_modifiers` (JSONB - e.g. `{"combat_bonus_pct": 5}`), `craft_cost`
  (JSONB resource costs), `craft_time_minutes`.
- `player_gear`: `id`, `encampment_id`, `gear_key`, `equipped` (BOOLEAN),
  `crafted_at`. A player can own multiple copies of the same gear key
  (e.g. to equip across a future multi-hero roster) but only one per
  slot can be `equipped = TRUE` at a time - enforced at the application
  layer (mirroring how e.g. `doBuyListingQty` enforces its own
  invariants in Go rather than a DB constraint, for clearer error
  messages).
- Crafting: a new `crafting_queue` table (mirrors the existing
  `research_states`/upgrade-queue pattern already in this codebase -
  spend resources up front, wait `craft_time_minutes`, tick phase
  resolves completion and notifies). Reuses `storagecap`-style resource
  deduction, not a new pattern.
- Stat application: gear's `stat_modifiers` apply as multipliers at the
  same point combat math already reads unit stats -
  `internal/game/combatmath` (the package built for the Doomsday Rig
  toughness fix) is the natural integration seam, since it's already
  the single place hazard-weighted casualty allocation happens.
- Rarity drives where gear comes from: `common`/`rare` are craftable
  directly from resources; `epic`/`legendary` are **not** purchasable
  or craftable outright - they drop from World Boss kills (tiered per
  Phase 8) and Relic Convoy claims (a natural second reward tier for
  `relicconvoy.go`'s existing claim flow, alongside the cash windfall).
- Panel (`/gear`): equipped loadout per slot, owned-but-unequipped
  inventory, and a Craft sub-panel listing `gear_definitions` with
  affordability shown inline (matching the exchange panel's existing
  bulk-buy affordability pattern).

### AI parity note

AI factions get a **much simpler** crafting heuristic, not the full
player crafting queue UX: a new roll in `growAICivilizations` (small
chance per tick, gated by faction level like every other AI growth
behavior) crafts and auto-equips whatever `common`/`rare` gear the
faction can currently afford in its best-value slot - AI factions don't
need epic/legendary gear logic since they don't fight World Bosses or
claim Relic Convoys today (both are real-player-only content), so their
gear ceiling is intentionally lower. This keeps AI factions
mechanically relevant in raids against them (a raided AI faction isn't
always gearless) without building a second full crafting UI nobody
reads.

---

## Phase 4: Prestige / Rebirth Loop

**Status: ✅ Complete, with two documented adjustments below** (see
`internal/game/prestige/prestige.go`, `encampments.prestige_level`/
`prestige_bonus_pct`, `/prestige` command). **Adjustments**: (1) the
research eligibility/reset requirement covers all 7 research tracks
this build actually exposes (`research.go`'s own `fetchResearchLevels`
reads econ/production/integrity/defense/intel/speed/military), not
the plan's illustrative "four tracks"; (2) garrison (soldiers/mechs)
is explicitly kept, not reset - this section's own list of what resets
names camp level, buildings, research, and resources specifically and
never mentions troops, so that's treated as the source of truth over
the "starting values" phrasing elsewhere in this write-up.

A veteran player who has maxed out buildings/research currently has
nothing left to do except keep idling - a classic idle-genre problem
with a well-known fix: let them reset with a permanent bonus.

### Design

- `encampments.prestige_level INT NOT NULL DEFAULT 0` and
  `encampments.prestige_bonus_pct DOUBLE PRECISION NOT NULL DEFAULT 0`
  (a permanent, stacking resource-generation bonus - e.g. `+5%` per
  prestige level, applied as one more multiplier at the exact same
  call site Phase 2 (Wasteland Blooms) already extended in
  `internal/engine/resource/resource.go`, keeping every bonus source
  in that one well-tested spot).
- Prestiging (a deliberate player action, `/prestige`, gated behind a
  meaningful requirement - e.g. camp level 25+ and all four research
  tracks at level 20+) resets: camp level, building levels, research
  levels, and current resources to their starting values. It does
  **not** reset: gear (Phase 3), achievements (Phase 1), hero level
  (Phase 6), clan membership, or Legacy/Relic history - prestige is
  about the economic/building game resetting, not erasing a player's
  identity or social standing.
- A confirm dialog (matching this project's established
  Confirm/Cancel-with-cost-preview convention exactly) makes unmistakably
  clear what resets and what a player keeps before they commit -
  prestige is irreversible by design, so the confirmation has to be
  real, not a formality.
- Panel (`/prestige`): current prestige level and cumulative bonus,
  eligibility check with a clear "what's missing" breakdown if not yet
  eligible, and the confirm flow once eligible.

### AI parity note

AI factions **do not prestige** - their whole identity is "persistent
world resident, not a min-maxing veteran player," and Legacy Epitaphs
(Phase 3 of the rare-features plan) already gives them their own
permanent-arc ending. Adding prestige to AI factions would mean an AI
faction's level/history becomes non-monotonic in a way nothing else in
this codebase currently has to handle (leaderboards, discovery
permanence, Legacy Epitaphs) - explicitly out of scope.

---

## Phase 5: Battle Pass / Seasons

**Status: 🔲 Not started.**

A time-boxed progression track layered on top of the game's existing
persistent progression - a proven "give players a reason to log in
every day of a defined window" structure, and a natural home for
tying the Rare World Features (Blooms, Relic Convoys, the Wandering
Merchant) into a bigger seasonal narrative rather than being purely
ambient.

### Design

- `seasons`: `id`, `name`, `theme` (flavor text, can reference a Rare
  Feature focus, e.g. "Season of the Bloom"), `starts_at`, `ends_at`,
  `xp_per_tier` (JSONB or a simple formula constant).
- `season_xp`: `user_id`, `season_id`, `xp`, `claimed_tiers` (INT,
  highest tier whose reward has been claimed). Season XP is earned
  from the **same event-driven call sites** Phase 1/2 already
  instrumented (achievements/quests) - a season is not a third
  parallel tracking system, it's a scorekeeping layer over the same
  underlying player actions, avoiding a third set of call-site
  instrumentation across the codebase.
- `season_tier_rewards` (seed data, per season): `season_id`, `tier`,
  `reward_type`, `reward_value`. A simple, single free track at launch
  (no "premium track" - see the shared no-monetization principle
  above) - 30-50 tiers per season, each a small reward (dollars, a
  gear-craft-cost discount token, a cosmetic title), building toward a
  genuinely good capstone reward at the final tier.
- A new tick phase (`season_rollover`) closes an ended season and opens
  the next one, broadcasting server-wide (`QueueToAllPlayers`) exactly
  like Relic Convoy/Wandering Merchant arrival broadcasts already do.
- Panel (`/season`): current tier, XP progress to next tier, a
  scrollable/paginated reward track (reusing Phase 4 of the market-
  exchange work's pagination pattern - `market_page`-style Prev/Next -
  for browsing tiers beyond the current one).

### AI parity note

Same as achievements: seasons are a real-player retention feature with
no world-state consequence, so AI factions are explicitly out of
scope.

---

# Track B - Deepen Existing Systems

## Phase 6: Hero System Depth

**Status: ✅ Complete, with the same scope adjustment as Phase 3** (see
`internal/game/herodepth/herodepth.go`, `heroes.skill_points`/
`story_arc_stage`, `hero_skills_unlocked` table,
`player_gear.equipped_by_hero`, `/hero`'s new Skill Tree and Hero Gear
buttons). Skill-node effects are safe, non-combat bonuses (training
cost reduction, healing cost reduction, resource-generation bonus) for
the exact same reason gear's own bonus stayed out of
`resolveRaidCombats` - see Phase 3's status line and `gear.go`'s
package doc for the full rationale. Skill tree (18 nodes, 3 branches x
3 nodes x 2 factions), hero gear (a second, independent equip slot on
the same `player_gear` row - XP-gain bonus instead of resource-gen
bonus), and the 5-beat level-gated story arc all shipped as designed.

`hero.go` today: one hero per camp, a name/trait/injuries/superpower
flavor set, and a level/XP counter with no further branching. This
phase adds real player choice.

### Design

- **Skill tree**: `hero_skill_definitions` (seed data) organized into
  3 branches per faction archetype (matching the existing Steel
  Vanguard / Waste Phantom split already in `hero.go`'s fallback
  flavor text) - e.g. Steel Vanguard's branches might be Fortification/
  Offense/Command. `hero_skills_unlocked`: `hero_id`, `skill_key`,
  `unlocked_at`. Skill points are earned from hero level-ups (a new
  `hero_skill_points` column on `heroes`, incremented on level-up,
  decremented on unlock) - no new currency, reuses the leveling
  players already engage with.
- **Hero gear slots**: a hero can equip Phase 3's gear independently
  from the camp's own equipped-for-combat-units gear (a hero is a
  distinct combatant, not a unit) - `player_gear.equipped_by_hero_id`
  (nullable FK) alongside the existing `equipped` boolean, so the same
  gear table serves both without a parallel schema.
- **Hero story arc**: a short (5-8 beat), faction-flavored narrative
  that unlocks incrementally at specific hero levels (5/10/15/20/25) -
  static seed text per faction/level combination, delivered as a
  notification at the level-up moment rather than a separate panel to
  check, so it feels like a reward rather than homework.
- Panel (`/hero`) gains a Skill Tree sub-view (tappable branch
  selection, matching the exchange panel's `market_page`-style
  sub-navigation) and a Gear sub-view (linking into Phase 3's gear
  panel, hero-equip variant).

### AI parity note

AI factions don't have a hero today (`heroes.encampment_id` presumably
only ever gets rows for real players via onboarding - worth confirming
during Phase 6's own build) - if that's confirmed, hero depth stays
real-player-only by inheritance, no new AI gap introduced.

---

## Phase 7: Research Tree Branching

**Status: 🔲 Not started.**

`research_states` today: four independent, always-available tracks
(Production/Integrity/Intel/Speed), each just a flat level counter -
no mutual exclusivity, no specialization identity. This phase adds
real strategic choice without discarding the existing four tracks
(nothing here breaks an in-progress player's research investment).

### Design

- A new, separate **Specialization** layer on top of the four existing
  tracks, not a replacement for them: at camp level 15, a player
  chooses one of 3 Specializations (Raider/Trader/Fortress-Keeper),
  each unlocking its own small branching tree of 6-8 nodes with
  Specialization-flavored bonuses (e.g. Raider: raid loot % up;
  Trader: market listing fee reduction; Fortress-Keeper: defensive
  combat bonus). `encampments.specialization VARCHAR(20)` (nullable
  until chosen), `specialization_nodes_unlocked` (`encampment_id`,
  `node_key`, `unlocked_at`).
- Specialization choice is **not** permanently locked - a
  `/respec` action (costed in Crystal, scaled to discourage frequent
  flip-flopping but not prohibitive) resets specialization node
  unlocks and lets a player pick again, matching this project's general
  preference for costed-but-reversible over permanently punitive
  mechanics (contrast with Phase 4's prestige, which is deliberately
  irreversible because it's a much bigger reset).
- Panel: a new tab on the existing `/research` panel (not a separate
  command) - specialization choice and its node tree sit alongside the
  four existing tracks so research stays one coherent panel rather than
  fragmenting into two things a player has to remember separately.

### AI parity note

AI factions **do** pick a specialization - `growAICivilizations` gains
a one-time roll (once per faction, at level 15+) that picks a
specialization matching the faction's existing behavioral lean if one
is inferable (e.g. a faction with a high raid count leans Raider), or
randomly otherwise - this makes AI factions feel more differentiated
from each other in exactly the way Legacy Epitaphs already made their
deaths feel differentiated.

---

## Phase 8: World Boss Tiers & Gauntlet

**Status: ✅ Complete, with one structural adjustment from the original
design below** (see `world_bosses.tier`/`total_kills`/`is_gauntlet`/
`enrage_threshold_pct`, `internal/bot/handlers/boss.go`'s
`tierUnlocked`, `internal/engine/tick/engine.go`'s enrage/guaranteed-
drop logic). **Adjustment**: `world_bosses` turned out to already be a
small, fixed, always-live set that instantly respawns to full HP after
each kill - not a spawn-roll system like Relic Convoys, as this
section's original design implicitly assumed. So "tiering" means
gating a higher tier's Attack button behind enough lower-tier kills
server-wide (`total_kills`), and "Gauntlet" is a fixed 4th boss ("The
Ravager") rather than a per-attack opt-in mode. Both still deliver the
plan's actual intent - an automatic difficulty ramp and a real,
felt-risk source of guaranteed best-in-slot gear - just adapted to the
system that already existed rather than the one originally assumed.

`boss.go` today: one-shot world boss events, single tier. This phase
adds structured escalation.

### Design

- `world_bosses.tier INT NOT NULL DEFAULT 1` (new column) - Tier 1
  bosses spawn at roughly today's existing frequency/difficulty; Tier
  2/3 are rarer, tougher, and better-rewarding, unlocked to spawn only
  after enough Tier 1 bosses have been defeated server-wide (a simple
  `world_boss_kill_count` tracked via a new small table, checked at
  spawn-roll time) - a natural, automatic difficulty ramp with no
  per-player gating logic needed.
- **Gauntlet mode**: an opt-in, higher-risk/higher-reward boss variant
  - a Gauntlet boss enters an "enraged" state partway through its
  health bar (a fixed threshold), sharply increasing its damage output
  for the remainder of the fight, in exchange for a guaranteed Phase 3
  epic/legendary gear drop on a successful kill (vs. Tier 1's chance-
  based drop). This is the natural home for genuinely rare
  best-in-slot gear, giving raiding groups (via the existing co-op
  rally mechanic already in the tick engine) a real reason to
  coordinate around a single event rather than solo-attacking.
- Panel: `boss.go`'s existing panel gains a Tier/Gauntlet indicator and
  the drop-table preview for the currently-live boss, so players know
  what they're fighting for before committing forces.

### AI parity note

AI factions don't attack world bosses today (this is real-player co-op
content) - no change to that scope here; tiering only affects the
spawn/reward side, which stays player-facing.

---

## Phase 9: AI Advisory Corps - Autonomy Expansion

**Status: 🔲 Not started.**

Correction from the informal brainstorm this plan grew out of: the
Corps (8 advisors) already exists and works - see "How this document
was produced" above. This phase is specifically about the gap that
*does* exist: every advisor today only produces an on-demand **report**
(text a player reads and acts on manually) except the Governor, which
already has an opt-in Autopilot mode (`/governor_autopilot on`). This
phase extends that same opt-in-autopilot pattern to two more advisors
where it's a natural fit, rather than building anything from scratch.

### Design

- **Economy Advisor Autopilot**: opt-in (`/economy_autopilot on`,
  mirroring the Governor's exact command shape and confirmation
  language) automatic resolution of the advisor's own flagged
  bottlenecks where the fix is unambiguous and low-risk - e.g.
  auto-queuing a storage-cap upgrade when the advisor's own report
  already identifies "storage waste" with a single obvious fix,
  reusing the advisor's existing detection logic rather than
  duplicating it; never auto-spends on anything irreversible (no
  auto-raiding, no auto-market-selling) - this is a maintenance
  autopilot, not a decision-making one, drawing an explicit narrower
  line than the Governor's own opt-in scope.
- **Research Planner Autopilot**: opt-in (`/research_autopilot on`)
  auto-queues the advisor's own top-recommended research track once
  the previous one completes and resources are available - again,
  reusing the existing recommendation logic the on-demand report
  already computes, not new logic.
- Both remain **strictly opt-in and off by default**, with the exact
  same "read what this does before enabling it" framing the Governor's
  existing documentation already establishes in `advisors_panel.go`.
- No changes to the other 6 advisors (Battle Analyst, Galaxy Advisor,
  Guild Assistant, Fleet Commander, NPC Intel) - their domains
  (combat/raiding/clan coordination) are exactly the kind of
  higher-stakes, judgment-heavy decisions this phase deliberately
  keeps advisory-only, matching the Economy/Research Autopilot scope
  line drawn above.

### AI parity note

Not applicable - the AI Advisory Corps is itself a tool for real
players; AI factions have their own separate decision loop
(`aidecisions.go`) and were never advisor consumers.

---

# Track C - Social/Clan Depth

## Phase 10: Clan Hall

**Status: 🔲 Not started.**

A shared, clan-level building/upgrade path funded by member donations -
today `/clan_donate` (built in an earlier session) only does peer-to-
peer resource transfer between individual members; there's no
clan-level structure for pooled donations to build toward.

### Design

- `clan_hall`: `clan_id` (PK), `level INT DEFAULT 0`, `treasury_scrap`,
  `treasury_metal`, `treasury_crystal`, `treasury_dollars` (the pooled,
  clan-owned resource balance - distinct from any individual member's
  own resources).
- Members can donate directly to the Clan Hall treasury (a small
  addition to the existing `/clan_donate` flow - a target of "Clan
  Hall" alongside naming a specific clanmate) - reuses the existing
  peer-to-peer donation's confirm/cost-preview UX rather than building
  a parallel donation flow.
- A Leader (or Co-Leader, via `canManageClan`) spends the pooled
  treasury to upgrade the Hall's level - each level grants every
  member's encampment a small passive bonus (e.g. +2% resource
  generation per Hall level, applied at the same resource-tick call
  site Phase 2's Bloom and Phase 4's prestige bonus already share) -
  one more well-audited multiplier source in the one place that
  formula lives, not a fourth parallel one.
- Panel: a new tab on the existing `/clan` panel (treasury balance,
  current Hall level, upgrade cost/benefit preview) - not a separate
  command, keeping clan management as one coherent panel.

### AI parity note

An AI-led clan's Hall can still receive donations from real human
members (the same cross-membership scenario the leave/disband fix from
2026-08-12 already had to account for) - the AI Leader doesn't need
new logic to *receive* donations (that's just a resource transfer into
a table it owns), but does need a small addition to `growAICivilizations`:
a rare roll (mirroring the clan-application-review roll's cadence) that
spends available Hall treasury on an upgrade once it can afford one, so
an AI-led clan's Hall doesn't just accumulate forever unused.

---

## Phase 11: Clan Wars Ladder (Seasonal)

**Status: 🔲 Not started.**

Today `/declare_war` (built in an earlier session) is ad-hoc,
one-off, and has no structured competitive framing or payoff beyond
the war itself. This phase adds a seasonal ladder - explicitly a third,
new thing alongside Federations (permanent) and Constellations
(temporary/cooperative) rather than a replacement for either; see "How
this document was produced" above.

### Design

- `clan_ladder_seasons`: mirrors Phase 5's `seasons` shape closely
  (`id`, `starts_at`, `ends_at`) but scoped to clan-vs-clan competitive
  standing rather than individual player XP.
- `clan_ladder_standings`: `season_id`, `clan_id`, `wins`, `losses`,
  `rating` (a simple Elo-style adjustment on each war's resolution,
  reusing whatever win/loss determination `resolveClanWars` in
  `engine.go` already computes - this phase adds scorekeeping on top of
  existing war-resolution logic, not new combat resolution).
- A season's top clans (top 10, say) get a Rich Message `<table>`
  standings board (`/clan_ladder`, matching `ranking.go`'s Top Guilds
  table pattern exactly) and a one-time seasonal reward pool
  (dollars, scaled by final rank) distributed to every member
  encampment of the top clans at season close - the payoff this
  system is specifically meant to add over today's ad-hoc wars.
- Declaring a war during an active ladder season **optionally** flags
  it as a ladder match (a checkbox-equivalent at declare time) - wars
  outside a season, or explicitly not flagged, don't affect standings,
  so casual/personal wars aren't forced into the competitive framing.

### AI parity note

AI-led clans' wars (already possible via the existing
`growAICivilizations` declare-war roll) count toward ladder standings
identically to human-led clan wars - no AI-specific change needed here
beyond the shared scorekeeping hook.

---

## Phase 12: Mentorship System

**Status: 🔲 Not started.**

Pairing a new player with a clan veteran, specifically targeting
first-week retention - the point in a player's life this project's own
brainstorm flagged as the highest-risk drop-off window (see Phase 17,
Onboarding Polish, for the complementary first-session-specific work).

### Design

- `mentorships`: `mentor_user_id`, `mentee_user_id`, `clan_id`,
  `started_at`, `graduated_at` (nullable - a mentorship "graduates"
  automatically after 7 days or when the mentee reaches camp level 10,
  whichever comes first, tracked by a small tick-phase check
  `mentorship_graduation`).
- A clan's Applications Inbox (`clan.go`'s existing accept flow) gains
  an optional "assign a mentor" step when accepting a clearly-new
  player (account age under 48 hours, inferred from `users.registered_at`)
  - the accepting Leader/Co-Leader picks any existing clan member as
  mentor, or skips.
- Both mentor and mentee get a small one-time reward on graduation
  (dollars for both, scaled modestly - this is a relationship/retention
  feature, not a resource-farming loop, so the reward is deliberately
  small enough not to incentivize hollow mentor-mentee pairs formed
  purely for the payout).
- A lightweight panel (`/mentor`) shows a mentor their current
  mentee(s) and basic progress (camp level, days active) so the
  relationship has somewhere to check in, without building a full chat/
  DM system this project has no existing infrastructure for.

### AI parity note

Not applicable - mentorship is a real-player relationship feature with
no AI-faction equivalent action.

---

# Track D - World & Narrative

## Phase 13: Faction Storylines

**Status: 🔲 Not started.**

Today AI factions are mechanically real (per
`AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md` and this session's own
clan fixes) but narratively silent - no named rivalries, no lore beyond
a generated name. This phase gives a subset of AI factions actual
authored arcs, building on Legacy Epitaphs' proof that narrative
payoff for AI factions lands well with players.

### Design

- A small, curated set of **Named Factions** (distinct from the
  procedurally-spawned generic AI factions already in the world) -
  seeded with a name, a short backstory, and 2-3 lore beats that
  surface as flavor text at specific world-state triggers (e.g. "The
  Ashfall Remnant remembers who raided them last" - a beat that
  surfaces as extra flavor in a raid notification if the attacker has
  raided that specific Named Faction 3+ times before, reusing raid
  history already queryable the same way `factiondefeat.go`'s
  finishing-attacker lookup already reads it).
- Named Factions get a small behavioral lean seeded onto them (e.g.
  "vindictive" - raises their own raid-target priority against a
  player who has raided them repeatedly) - a narrow, explicitly-scoped
  modifier to `aidecisions.go`'s existing target-selection logic, not a
  rewrite of it.
- A `/factions` panel (or a "Notable Factions" section added to the
  existing `/world` panel) surfaces each Named Faction's current
  status, standing, and lore snippet - part atlas, part leaderboard for
  "which AI factions have a personality."

### AI parity note

This phase *is* the AI parity work - it doesn't have a separate "AI
side," the whole phase is about AI factions.

---

## Phase 14: World Map Visualization

**Status: 🔲 Not started.**

Every panel in this game is text (with the Rich Message `<table>`
upgrade from the previous session for structured data) - there's no
visual representation of the world's geography at all. This is the
single highest "wow factor" item in this document, and also the
riskiest to scope tightly.

### Design

- Rendered as an **HTML5 Canvas artifact-style image**, generated
  server-side and sent as a Telegram photo (not an interactive Telegram
  UI element - Telegram bots can't render live interactive maps inline,
  so this is a periodically-regenerated snapshot, not a live view).
- A new lightweight Go image-generation step (using a pure-Go plotting
  approach, avoiding a heavy graphics dependency) reads `coordinates`
  (region/biome/danger_level), `encampments` (real players + AI
  factions, colored by faction/clan), and any currently-active Rare
  World Feature (a Relic Convoy marker, a Blooming region's tint, a
  Wandering Merchant marker) into a single top-down map image.
- `/map` generates and sends a fresh snapshot on demand (rate-limited
  per user, since image generation is the most expensive single
  operation this project would introduce) rather than a cached
  server-wide image, so a player's own discovered/known locations
  (respecting the existing discovery-permanence system) can be
  highlighted specifically for them.
- Explicitly **not** in this phase's scope: real-time updates, pan/
  zoom, or click-to-target-from-map interactivity - Telegram's bot
  platform doesn't support that class of interaction, and pretending
  otherwise would make this phase's scope open-ended. A static,
  regenerated-on-request snapshot is the honest ceiling of what a
  Telegram bot can do here.

### AI parity note

Not applicable - this is a rendering feature for real players' own
viewing.

---

## Phase 15: More Rare World Events

**Status: 🔲 Not started.**

A direct, explicitly-scoped extension of `RARE_WORLD_FEATURES_PLAN.md`
(Phases 1-5, all shipped) - three more events in the same "rare,
sweet, and beautiful" spirit, sized and built exactly like that
document's phases (own schema, own tick phase(s), own handler, own
tests, own status line), added to that document as Phases 6-8 rather
than duplicated here.

- **Phase 6 (of that doc): The Blackout** - a rare, world-wide (not
  per-continent) event: all electricity generation server-wide is
  disabled for a short window, framed as a "the grid itself failed"
  moment - unlike Bloom (positive) or the existing negative weather
  events (per-continent), this is deliberately global and universally
  felt, a shared-hardship moment the whole playerbase experiences
  together rather than a per-region roll.
- **Phase 7 (of that doc): The Bounty Board** - a rare NPC posting
  (structurally similar to the Wandering Merchant's `merchant_offers`
  table shape) naming a specific AI faction as a bounty target - the
  first player to defeat that specific faction (hooking into Legacy
  Epitaphs' existing defeat-detection) claims an escalated reward
  beyond the normal Conqueror bonus.
- **Phase 8 (of that doc): Sanctuary Zones** - a rare, temporary
  PvP-free window for a specific region (raids into/out of that region
  blocked for the duration) - the one purely defensive/protective rare
  event in the set, complementing Bloom's purely economic-positive one.

This section is intentionally a pointer, not a full spec - each of
these three gets its own full phase write-up in
`RARE_WORLD_FEATURES_PLAN.md` itself when picked up for build, keeping
that document the single source of truth for the Rare Features
initiative rather than splitting it across two files.

---

# Track E - Quality of Life

## Phase 16: Unified Notification Digest

**Status: 🔲 Not started.**

Every system in this game (and every phase in this document) fires
individual notifications via `notifications.Queue`/`QueueToRegion`/
`QueueToAllPlayers` - for an active player during a busy session, that
can mean a real flood of individual messages. This phase adds an
opt-in digest mode without touching the existing real-time delivery
path (which stays the default, and is right for time-sensitive
notifications like "Relic Convoy sighted").

### Design

- `users.notification_mode VARCHAR(20) DEFAULT 'realtime'`
  (`realtime`/`digest`) - opt-in, matching the Governor Autopilot's own
  "off by default, explicit opt-in" convention.
- In `digest` mode, non-`"general"`-category notifications (the
  category already used to mark broadcast-tier, never-silenced
  messages per `AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md` section 5)
  are queued but **not sent** individually - instead a new tick phase
  (`notification_digest_delivery`) batches each digest-mode player's
  pending notifications into a single grouped message, sent on a fixed
  cadence (every 30 minutes, configurable), grouped by category
  (Combat/Economy/Social/World) with a count-and-summary format for
  same-category floods (e.g. "💰 5 market sales completed" instead of 5
  separate lines) rather than just concatenating every message
  verbatim.
- `"general"`-category notifications (Relic Convoys, world-boss
  spawns, faction defeats) **always** deliver in real time regardless
  of digest mode - the whole point of that category's existing
  "never silenced" guarantee is preserved, digest mode only ever
  batches the routine, non-urgent stream.
- A `/notifications digest` / `/notifications realtime` command pair
  toggles the mode; the existing notification-preferences system
  (`internal/engine/notifications/preferences.go`) already has the
  per-user opt-out infrastructure this reuses rather than duplicates.

---

## Phase 17: Onboarding Polish

**Status: ✅ Complete** (see `ONBOARDING_AUDIT_FINDINGS.md` for the
audit itself, `internal/game/progression/progression.go`'s `PeriodOnce`
and 4 `onboarding_*` quest definitions, `TriggerResearchUpgrade` wired
into `internal/bot/handlers/research.go`, and `gettingStartedGuideText`
in `onboarding.go`).

`onboarding.go` is already the single largest handler file in the
project (670 lines) - this phase is explicitly **not** a rewrite, it's
a targeted audit-and-fix pass, the same "polish wave" format
`BUGS_AND_INCONSISTENCIES.md` already established for this codebase,
scoped specifically to the first-session experience.

### Design

- A dedicated audit pass (its own short findings doc, following
  `BUGS_AND_INCONSISTENCIES.md`'s exact format) walking a brand-new
  account through onboarding end to end, flagging: any step using
  plain text instead of the Rich Message formatting standard the rest
  of this codebase has largely adopted since the "UI Polish Wave 1"
  pass; any point where a new player is shown a system (advisors,
  quests, achievements, gear) with no explanatory context; and any
  drop-off point with no clear next action.
- Concretely in scope once the audit is done: a "Getting Started"
  checklist (reusing Phase 2's quest-progress-tracking infrastructure
  for a one-time onboarding quest chain rather than a third tracking
  system) walking a new player through their first raid, first
  research upgrade, first clan join, and first market interaction,
  with small guaranteed rewards at each step - the new player's
  version of Phase 2's daily quest board, using the same underlying
  machinery.
- Explicitly **not** in scope for this phase: any change to the
  account-creation/faction-choice flow itself (steel_vanguard vs.
  waste_phantom) unless the audit specifically finds it broken -
  this phase is about the *experience* around onboarding, not
  re-litigating its existing structure.

---

## Suggested build order

Not binding - the project owner picks the actual sequence. One
reasonable proposal, front-loading the items with the clearest
standalone value and the fewest cross-phase dependencies:

1. **Phase 1 (Achievements)** and **Phase 2 (Quests)** together first -
   they share the same event-driven instrumentation work, so building
   them back-to-back means touching each action handler's call sites
   once instead of twice.
2. **Phase 17 (Onboarding Polish)** next - benefits from Phase 2's
   quest infrastructure existing already (its "Getting Started"
   checklist depends on it), and first-session retention is normally
   the highest-leverage single fix available.
3. **Phase 3 (Crafting & Gear)** - the biggest single system, but
   self-contained once started; Phase 6 (Hero Depth) and Phase 8 (World
   Boss Tiers) both depend on gear existing first, so it's a natural
   prerequisite to unlock next.
4. **Phase 6 (Hero Depth)** and **Phase 8 (World Boss Tiers)** - both
   consume Phase 3's gear system directly.
5. Everything else (Prestige, Seasons, Research Branching, Clan Hall,
   Clan Wars Ladder, Mentorship, Faction Storylines, World Map, more
   Rare Events, Notification Digest, AI Advisor Autonomy) has no hard
   dependency chain and can be sequenced by whatever the project owner
   finds most exciting to build next.

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
| 3 | Keyboard/UI audit | **Not started** | — |
| 5 | Automation Agent limit bugs | **Partial** — audited `internal/engine/agent`. `builder` mode's auto-upgrade selection ignored the "module level cannot exceed Outpost Core level" cap enforced on the manual `/camp` path; `military` mode's auto-recruit ignored the Hangar capacity cap enforced on the manual Recruit Soldier path. Both now gated to match. Asiwaju explicitly directed this session to go ahead (supersedes the item-9 hands-off default below for this specific fix). Still open: `collector`/`collector_omega`/`collector_precious` haven't been audited against their own caps yet (collector already respects `storageCap`; the two collector_omega/precious variants add Metal/Hydrogen/Crystal/Dollars/NeuroCores with no cap check at all — see item 9 note, needs a look). | `90caeef`, `53d1916` |
| 6 | Doomsday unit balance | **Done** — hard cap of exactly 1 replaced with a level-scaled cap; attack rating and toughness bumped; a real missing-affordability-check bug (neuro_cores wasn't validated before deduction) fixed along the way. | `3fd86a5` |
| 10 | World exploration (continents/sectors/discovery) | **Not started** — biggest single item left; needs new schema (sectors/continents, discovery state per player) | — |
| 11 | Diplomacy (Known Bases, friend/enemy), long battles/reinforcements | **Not started** — long-battle round cap (currently 5 rounds max, see `engine.go` `r.roundNumber >= 5` draw condition) needs revisiting once reinforcement mechanics exist | — |
| 12 | Dynamic World Events + notification engine | **Partially exists** — `internal/engine/world/weather.go` already drives Acid Rain/Radiation Storm effects on combat (see `engine.go` weather switch); notification *dispatcher* itself (`internal/engine/notifications`) is a working 3s-poll queue, not a stub. What's missing: more event types (EMP, Supply Crisis, Disease, Sandstorm from the brief), and continent/world-scoped broadcast rather than the current single global weather state. | — |
| 13 | Admin panel consolidation | **Not started** | — |
| 9 | AI Agent files hands-off | **Exception granted for item 5** — no edits to `internal/game/governor`, fleetcommander, or econadvisor. `internal/engine/agent` *was* edited this session (`90caeef`, `53d1916`), but only the game-limit-enforcement bugs under item 5 — Asiwaju explicitly directed the audit-and-fix in-chat. No other agent logic (mode selection, resource-gain formulas) touched. | — |

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

- **Concurrent workstream risk:** the AI-roadmap session pushed
  `Deep schema-drift audit: rebase onto real main + fix Fleet Commander
  gaps` (`82e34e6`) while this workstream was mid-flight. It rebased
  clean with no file overlap this time, but the two sessions are not
  coordinating in real time — if you're picking this up, `git fetch`
  and check `git log origin/main` before assuming this file's "Status"
  table is still accurate.
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
- **Not yet audited**: `collector_omega` (Metal/Hydrogen) and
  `collector_precious` (Crystal/Dollars/NeuroCores) modes add resources
  every tick with **no storage cap check at all** — unlike plain
  `collector`, which already respects `storageCap` (`TentLvl * 500`).
  This looks like the same class of bug but wasn't in the original
  two flagged cases, so it wasn't touched without separate sign-off.
  Worth flagging back to Asiwaju before fixing.

---

## 4. Next in line (recommended order)

1. Item 5 — Automation limit bugs, remainder: `collector_omega` /
   `collector_precious` resource-gain has no storage cap check at all
   (see note above) — confirm with Asiwaju this is in-scope before
   touching `internal/engine/agent` again.
2. Item 3 — Keyboard audit (needs a menu-by-menu pass across
   `internal/bot/keyboards` and every handler that sends a
   `ReplyMarkup`, checking for stale/missing keyboard replacement).
3. Item 12 — expand world events beyond weather (Acid Rain/Radiation
   Storm already exist; add EMP/Supply Crisis/Disease/Sandstorm,
   scope to continent rather than global).
4. Item 10/11 — world exploration + diplomacy (biggest remaining
   item, needs new schema; do this after the smaller items above so
   the schema design benefits from everything else being settled).
5. Item 13 — Admin panel consolidation (mechanical, do last).

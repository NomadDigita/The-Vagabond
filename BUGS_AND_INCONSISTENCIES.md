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

## Confirmed still-open items from prior sessions (re-verified today)

4. **`coordinates.danger_level` is stored but never read by any game
   system** — confirmed again: the only reference in the entire repo is
   the struct tag in `internal/models/models.go`. Nothing sets combat
   difficulty, encounter rates, or AI behavior from it. Either wire it
   up or remove the column/field to stop it looking load-bearing.

5. **Guild roles are still binary (`Leader` / `Soldier`)** — confirmed
   in `clan.go`; no officer/mid-tier role exists despite some UI copy
   implying a hierarchy. Matches ADR notes from previous sessions.

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

Wave 6 covered: research.go, ether.go, crystal_exchange.go,
rebellion.go, jobs.go, agent.go - plus the bug #4 Co-Leader permission
fix itself (see above). `jobs.go`'s two cooldown messages
(`HandleTeleport`, `HandleGatherSunlight`) were deliberately left plain
- low-value one-liners, not worth the churn.

Still plain text / not yet audited: admin.go, diplomacy.go,
federation.go, combat_road_encounters.go, starvation.go (the tick
notifications in `internal/engine/starvation`, not a handler file),
the remaining ~15 tick-engine notifications (tax collection,
exploration discovery, ETA/proximity alerts). Recommend admin.go and
combat_road_encounters.go (the two largest remaining files) as the
next batch - wave 7.

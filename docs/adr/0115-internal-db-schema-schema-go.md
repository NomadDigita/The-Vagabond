# ADR 0115: Decisions from `internal/db/schema/schema.go`

Status: Accepted

These decisions were written as comments in `internal/db/schema/schema.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package schema holds the full set of idempotent startup migration

Removed from `internal/db/schema/schema.go:1`.

~~~~text
Package schema holds the full set of idempotent startup migration
statements for The Vagabond. This used to live inline inside
cmd/bot/main.go's executeStartupMigrations, which made it untestable -
Phase 7 milestone 5 wants tests for "migration compatibility, critical
idempotency paths", and you can't unit test something that only exists
inside func main()'s call graph. Extracting it here changes nothing
about what runs at startup (main.go just calls Statements() now) but
makes it possible to run every statement twice against a real database
in a test and confirm CREATE TABLE IF NOT EXISTS / ADD COLUMN IF NOT
EXISTS / etc. actually hold up, instead of trusting that by inspection.
~~~~


## Statements returns every startup schema statement, in the exact order

Removed from `internal/db/schema/schema.go:13`.

~~~~text
Statements returns every startup schema statement, in the exact order
they must run (later ALTERs assume earlier CREATE TABLEs already ran).
Every statement here must be safe to execute an unbounded number of
times against a database that already has some or all of the prior
statements applied - that's what lets this run on every boot.
~~~~


## --- NEXT_GENERATION_FEATURES_PLAN.md Phase 4: Prestige /

Removed from `internal/db/schema/schema.go:87`.

~~~~text
--- NEXT_GENERATION_FEATURES_PLAN.md Phase 4: Prestige /
Rebirth Loop. A permanent, stacking resource-generation bonus
earned by resetting the economic game back to its starting
point - see internal/game/prestige/prestige.go's own package
doc for exactly what resets and what's kept.
~~~~


## --- SpaceHunt resource revival migration ---

Removed from `internal/db/schema/schema.go:110`.

~~~~text
--- SpaceHunt resource revival migration ---
Renames energy/steel/uranium -> electricity/metal/crystal on any
pre-existing database (no-op on a fresh install, since the CREATE
TABLE above already uses the new names directly).
~~~~


## Folds iron+oil into metal, and diamond+gold+silver into crystal,

Removed from `internal/db/schema/schema.go:130`.

~~~~text
Folds iron+oil into metal, and diamond+gold+silver into crystal,
then drops the now-redundant columns. Guarded so it only runs
once (columns won't exist on subsequent boots).
~~~~


## Bugfix (found live, 2026-07-15): CREATE TABLE IF NOT EXISTS

Removed from `internal/db/schema/schema.go:148`.

~~~~text
Bugfix (found live, 2026-07-15): CREATE TABLE IF NOT EXISTS
resources is a no-op on any database where the table already
existed before `ether`/`neuro_cores` were added to its column
list above, so those two columns silently never got created on
already-deployed databases. This broke internal/game/econadvisor
("column ether does not exist") and would have caused the same
failure in internal/game/governor. Idempotent regardless of
whether the table is brand new or years old.
~~~~


## Referral system fix (2026-07-26): referral_code was never

Removed from `internal/db/schema/schema.go:159`.

~~~~text
Referral system fix (2026-07-26): referral_code was never
enforced unique, and the old `telegramID % 1_000_000` scheme
could collide, silently misattributing referrals. Codes are
now derived deterministically from the player's Telegram ID
(base36), which is unique by construction, so this index just
protects against any stale/legacy duplicate codes.
~~~~


## Tracks the highest referral-count milestone tier (5/10/25) a

Removed from `internal/db/schema/schema.go:166`.

~~~~text
Tracks the highest referral-count milestone tier (5/10/25) a
player has already claimed, so milestone bonuses are granted
exactly once.
~~~~


## --- NEXT_GENERATION_FEATURES_PLAN.md Phase 8: World Boss

Removed from `internal/db/schema/schema.go:300`.

~~~~text
--- NEXT_GENERATION_FEATURES_PLAN.md Phase 8: World Boss
Tiers & Gauntlet. world_bosses is a small, fixed, always-live
set (not a spawn-roll system like Relic Convoys) that
respawns to full HP after each kill - so "tiering" here
means gating a higher tier's Attack button until enough
lower-tier kills have happened server-wide (total_kills),
rather than gating a spawn roll. Gauntlet is a fixed 4th
boss (seeded below) rather than a per-attack opt-in mode -
see boss.go's own comment for the full design rationale.
~~~~


## ask_type/ask_quantity generalize market_exchange beyond

Removed from `internal/db/schema/schema.go:444`.

~~~~text
ask_type/ask_quantity generalize market_exchange beyond
cash-only listings: a seller can ask for another resource
instead of dollars (barter). ask_type = 'dollars' keeps the
original behavior (ask_quantity mirrors price_dollars for
that row); any other ask_type is a tradeable resource name
and ask_quantity is how much of it is wanted. price_dollars
is kept as-is for backward compatibility with existing
readers (econadvisor, the AI faction auto-buy tick) rather
than migrated away - see doPostListing in exchange.go.
~~~~


## --- NEXT_GENERATION_FEATURES_PLAN.md Phase 6: Hero System

Removed from `internal/db/schema/schema.go:500`.

~~~~text
--- NEXT_GENERATION_FEATURES_PLAN.md Phase 6: Hero System
Depth. Skill points are earned on level-up and spent on
branch nodes (internal/game/herodepth's static Go
definitions); story_arc_stage tracks how many story beats
have already been delivered so a beat is never re-sent.
~~~~


## A hero can independently equip one item for a hero-specific

Removed from `internal/db/schema/schema.go:513`.

~~~~text
A hero can independently equip one item for a hero-specific
bonus (XP gain rate - see herodepth.go) distinct from that
same item equipped at the camp level (resource-generation
bonus - see gear.go) - a second boolean on the same
player_gear row rather than a parallel ownership table, so
ownership stays single-sourced.
~~~~


## A notification that can never actually be delivered (player

Removed from `internal/db/schema/schema.go:528`.

~~~~text
A notification that can never actually be delivered (player
blocked the bot, malformed HTML tripping Telegram's "can't
parse entities" 400, a message over Telegram's 4096-char cap,
a stale/invalid chat) used to retry forever with no limit and
no way to tell it apart from a message that's just waiting its
turn. Worse: notifications.Dispatcher.drainQueue polls a fixed
LIMIT 10 oldest-first batch every 3 seconds - if enough
permanently-failing rows accumulate at the front of that
queue, they occupy every slot in every batch forever, and
genuinely new notifications behind them - for any player, any
feature - never get a turn at all. See drainQueue's updated
give-up-after-N-attempts logic, which this column supports.
~~~~


## Phase 7 (item 12): world events are now scoped per-continent

Removed from `internal/db/schema/schema.go:566`.

~~~~text
Phase 7 (item 12): world events are now scoped per-continent
instead of one global front, matching coordinates.region's
existing Africa/Europe/Asia/Americas quadrant scheme. The
column is nullable-safe (backfilled 'Global' for any legacy
rows) so this is a pure additive migration.
~~~~


## Converges legacy (migrations/001_initial_schema.sql, which had

Removed from `internal/db/schema/schema.go:574`.

~~~~text
Converges legacy (migrations/001_initial_schema.sql, which had
title/starts_at NOT NULL with no default) and fresh
(schema.go-only, no title/starts_at at all) deployments onto
the same shape. See the 2026-08-02 note in
internal/engine/world/weather.go's RunWeatherPass for the full
story: on a legacy DB, every event-roll INSERT was silently
failing on the old NOT NULL title constraint, which is why zero
world events had ever actually been created despite the roll
logic always being correct. The DEFAULTs here are a safety net
only - RunWeatherPass populates both explicitly on every insert
going forward - but they mean this migration can't reintroduce
the same trap for anyone who skips straight to a fresh DB.
~~~~


## weather.go inserts the full formatted headline (e.g. "⚠️

Removed from `internal/db/schema/schema.go:587`.

~~~~text
weather.go inserts the full formatted headline (e.g. "⚠️
ENVIRONMENTAL ALERT: Solar Flare detected over Africa...")
into this column, not a short label - VARCHAR(150) was too
narrow and failed every world-event insert whose headline
happened to run long, with the same silent-abort effect the
comment above this table already documents for the
title-was-NULL case: RunWeatherPass returns the error,
aborting the whole tick's weather pass. Widened rather than
truncating, since nothing in this codebase currently reads
this column back (write-only today) - no reason to lose
information a future reader might want.
~~~~


## Phase 7 (item 10): World Exploration. Sites rotate in per

Removed from `internal/db/schema/schema.go:616`.

~~~~text
Phase 7 (item 10): World Exploration. Sites rotate in per
continent (same cadence/pattern as world_events above) and
are claimed first-come-first-served by whichever outpost
dispatches an expedition and survives the timer first.
~~~~


## MMO living-world Phase 2: discoveries are directional and are

Removed from `internal/db/schema/schema.go:641`.

~~~~text
MMO living-world Phase 2: discoveries are directional and are
the authoritative gate for target visibility and raid launch.
target_key supports non-encampment world targets while AI
civilizations are migrated onto persistent settlements.
~~~~


## Route snapshots make proximity-based radar notices deterministic

Removed from `internal/db/schema/schema.go:667`.

~~~~text
Route snapshots make proximity-based radar notices deterministic
and prepare active campaigns for the later road-event phases.
~~~~


## PRODUCTION OUTAGE, self-inflicted and fixed same day (2026-08-02):

Removed from `internal/db/schema/schema.go:693`.

~~~~text
PRODUCTION OUTAGE, self-inflicted and fixed same day (2026-08-02):
this DO block used to also (IF NOT EXISTS) create
raids_movement_state_valid with the narrow, pre-'awaiting_
reinforcement' CHECK list. That combined with the DROP+v2 pair
below it into an infinite crash loop: every boot dropped the
old constraint (DROP CONSTRAINT IF EXISTS, a few statements
down), which meant IF NOT EXISTS here was true again on the
*next* boot, which recreated the old (narrow) constraint, which
then immediately failed Postgres's mandatory validate-existing-
rows check against real 'awaiting_reinforcement' rows already
in the table - a fatal error (schema init treats failures as
fatal and refuses to start), confirmed via Render's deploy logs
("Fatal: Failed to execute startup database initialization
script: pq: check constraint "raids_movement_state_valid" of
relation "raids" is violated by some row (23514)"), crash-
looping the whole server on every single restart. The old
constraint's creation is deleted outright (not just left
unreferenced) so there is nothing left in this file that can
ever recreate it. See the comment further below, on the
DROP + raids_movement_state_valid_v2 pair, for the actual
content fix this was meant to ship alongside.
~~~~


## Confirmed live via Render logs (2026-08-02): applyActiveLogisticsConsumption

Removed from `internal/db/schema/schema.go:715`.

~~~~text
Confirmed live via Render logs (2026-08-02): applyActiveLogisticsConsumption
in internal/engine/tick/engine.go sets movement_state =
'awaiting_reinforcement' when a marching column runs out of
supplies or sustains a power outage - and that value has been used
throughout the app (combat.go, combat_road_encounters.go,
devconsole/queries.go) since the resupply-convoy feature shipped.
But raids_movement_state_valid above never included it, only
'supply_paused' (a name nothing in the codebase actually sets) -
so every one of those UPDATEs was rejected by the CHECK
constraint, which poisoned the transaction and made every later
statement in that same tick phase fail with "current transaction
is aborted", surfacing at commit time as "logistics_consumption:
could not complete operation in a failed transaction" on
essentially every tick. Given as its own constraint (not an ALTER
of the original) because the original is guarded by
`IF NOT EXISTS (SELECT ... conname = 'raids_movement_state_valid')`
- once that first ran on a given database, editing this list in
place would never actually reach it again.
PRODUCTION OUTAGE #2, same day (2026-08-02): the first fix
above (raids_movement_state_valid_v2) still validated every
existing row against its new CHECK list at creation time -
standard Postgres behavior for ADD CONSTRAINT, but risky here
because it means missing even one real, in-use value crashes
the boot exactly like outage #1 did. That's exactly what
happened: this list didn't include 'camped'
(internal/engine/tick/engine.go's handleRoadIncident-style
pause flow), confirmed via `grep -rnE
"movement_state[[:space:]]*[=:][[:space:]]*['\"][a-z_]+['\"]"`
across the whole repo - the only reliable way to enumerate
every value actually in use, since eyeballing the CHECK list
against memory had already missed it once. Added 'camped'
AND switched to NOT VALID, which skips validating existing
rows at creation time entirely (still enforced for every new
INSERT/UPDATE going forward) - this is the standard safe
pattern for adding a CHECK constraint to a table that already
has live data, and means a future un-enumerated legacy value
can no longer take the whole server down at boot the way both
outages today did.
~~~~


## Phase 7 (item 11): Diplomacy. Mirrors clan_wars's clan_a/clan_b

Removed from `internal/db/schema/schema.go:768`.

~~~~text
Phase 7 (item 11): Diplomacy. Mirrors clan_wars's clan_a/clan_b
shape, but for peaceful pacts instead of conflicts. A pact only
takes effect (blocks raids - see combat.go's launch check) once
status = 'active', which requires the receiving Clan King to
accept a 'pending' proposal.
~~~~


## --- AI Foundation (Phase A, independent AI roadmap branch) ---

Removed from `internal/db/schema/schema.go:820`.

~~~~text
--- AI Foundation (Phase A, independent AI roadmap branch) ---
See migrations/020_vagabond_ai_foundation.sql for the annotated
standalone copy of this schema and internal/ai for the Go layer
that reads/writes it.
~~~~


## --- AI Planet Governor (Phase B, independent AI roadmap branch) ---

Removed from `internal/db/schema/schema.go:877`.

~~~~text
--- AI Planet Governor (Phase B, independent AI roadmap branch) ---
See migrations/021_vagabond_ai_governor.sql for the annotated
standalone copy and internal/game/governor for the Go layer.
~~~~


## --- Phase 6: Engage-weapon turret differentiation + remaining

Removed from `internal/db/schema/schema.go:886`.

~~~~text
--- Phase 6: Engage-weapon turret differentiation + remaining
new units (Liberator, Observer, Wraith, Piercing Missile,
Guardian, Cargo Ship Mk I/II/III). See
migrations/022_spacehunt_phase6_weapons_and_units.sql for the
annotated standalone copy.
~~~~


## --- Phase 7: Hero Commander / manual defense garrison. Lets a

Removed from `internal/db/schema/schema.go:906`.

~~~~text
--- Phase 7: Hero Commander / manual defense garrison. Lets a
player lock a portion of their Soldiers/Mechs as a permanent
home garrison that campaign drafts can never pull from, and
withdraw them back to the general pool at any time. See
migrations/023_spacehunt_phase7_garrison.sql for the annotated
standalone copy.
~~~~


## --- Phase 7: Bulk unit selection. A single "step size" stored on

Removed from `internal/db/schema/schema.go:915`.

~~~~text
--- Phase 7: Bulk unit selection. A single "step size" stored on
the draft itself (cycled via a Step: x1/x10/x100/MAX button row)
makes every existing +/- button move that many units at once,
instead of forcing 100 taps to draft 100 Soldiers. See
migrations/024_spacehunt_phase7_bulk_selection.sql.
~~~~


## --- MMO Living World Phase 3/4: route legs + road encounters.

Removed from `internal/db/schema/schema.go:922`.

~~~~text
--- MMO Living World Phase 3/4: route legs + road encounters.
See migrations/030_mmo_route_legs_and_road_encounters.sql for
the annotated standalone copy.
~~~~


## See migrations/034_mmo_road_base_encounters.sql for the annotated

Removed from `internal/db/schema/schema.go:966`.

~~~~text
See migrations/034_mmo_road_base_encounters.sql for the annotated
version. Completes Phase 4 milestone 2 (expedition-vs-base road
encounters), the "expeditions and bases" half that road_encounters
above (expedition-vs-expedition only) didn't cover.
~~~~


## --- MMO Living World Phase 5: weather route incidents (temporary

Removed from `internal/db/schema/schema.go:998`.

~~~~text
--- MMO Living World Phase 5: weather route incidents (temporary
camps) + reinforcement convoys. See
migrations/033_mmo_route_weather_and_reinforcement_convoys.sql
for the annotated standalone copy.
~~~~


## --- MMO Living World Phase 6: persistent AI civilizations.

Removed from `internal/db/schema/schema.go:1050`.

~~~~text
--- MMO Living World Phase 6: persistent AI civilizations.
AI factions are REAL encampments (with a synthetic negative-
telegram_id "users" row satisfying the existing NOT NULL/UNIQUE
FK, seeded once by seedAICivilizations at startup) rather than
a special-cased entity, so the entire existing discovery/
targeting/raiding/looting pipeline works on them for free -
see MMO_WORLD_EVOLUTION_PLAN.md's Phase 6 completed-detail
section for why that integration point was chosen.
~~~~


## See migrations/035_phase7_notification_preferences.sql for the

Removed from `internal/db/schema/schema.go:1062`.

~~~~text
See migrations/035_phase7_notification_preferences.sql for the
annotated version. Phase 7 milestone 2: lets a player mute
exactly one high-volume, low-stakes category (peaceful road
passes, weather clears, convoy arrivals); combat/discovery/
supply-loss notifications are never tagged with a mutable
category (see notifications/preferences.go's MutableCategories),
so this mechanism structurally cannot silence them.
~~~~


## Phase 7 milestone 3's "speed-up expenditure" admin metric needs a

Removed from `internal/db/schema/schema.go:1076`.

~~~~text
Phase 7 milestone 3's "speed-up expenditure" admin metric needs a
structured event to aggregate - nothing else logs this today.
~~~~


## See migrations/036_ai_faction_decision_loop.sql for the

Removed from `internal/db/schema/schema.go:1088`.

~~~~text
See migrations/036_ai_faction_decision_loop.sql for the
annotated version. Second half of Phase 6 - see
AI_FACTION_DECISION_LOOP_PLAN.md for the full design.
~~~~


## AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 3.4: discovery

Removed from `internal/db/schema/schema.go:1104`.

~~~~text
AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 3.4: discovery
permanence and Ghost Protocol. known_locations is a NEW concept,
distinct from encampment_discoveries (a permanent boolean "have
I ever heard of this entity" relationship, which stays
untouched) - this table stores an actual coordinate SNAPSHOT,
locked at discovery time, so an attacker acts on where they
last saw a target rather than re-reading its live position
every time. Ghost Protocol (see jobs.go's HandleGhostProtocol)
is the only thing that deletes rows here.
~~~~


## AI factions get a new possible decision-loop intent, 'flee'

Removed from `internal/db/schema/schema.go:1123`.

~~~~text
AI factions get a new possible decision-loop intent, 'flee'
(chosen when a faction's resources have been ground down by
repeated raiding - see aidecisions.go's maybeFlee), which needs
its own slot in the existing CHECK constraint.
~~~~


## AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 3.2: long-range

Removed from `internal/db/schema/schema.go:1130`.

~~~~text
AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 3.2: long-range
scouting's data model. A wholly separate feature from
exploration_sites/exploration_dispatches (guaranteed-resource
personal exploration, unrelated purpose) - deliberately its own
table, per the plan's explicit instruction not to touch that
system for this.
~~~~


## AI_AND_SCOUTING_EXPANSION_PLAN.md item 1: continuous AI faction

Removed from `internal/db/schema/schema.go:1161`.

~~~~text
AI_AND_SCOUTING_EXPANSION_PLAN.md item 1: continuous AI faction
spawning needs a world-scoped (not per-faction) cadence tracker,
so it lives on the existing world_state singleton row (id=1)
rather than a new table. ai_factions_spawned_count doubles as
both "how many have spawned" and the source of each new
faction's unique ai_faction_key/telegram_id, so it must never
be reset even if last_ai_spawn_at is somehow cleared.
~~~~


## FEEDBACK_CHANGELOG_NLP_PLAN.md milestone 2: changelog home.

Removed from `internal/db/schema/schema.go:1171`.

~~~~text
FEEDBACK_CHANGELOG_NLP_PLAN.md milestone 2: changelog home.
changelog_reads is what makes "at least 5 oldest" meaningful -
see doPublishChangelog/HandleChangelogPanel in
internal/bot/handlers/changelog.go for the full reasoning: a
brand-new or returning player has a backlog of entries they've
never seen, and showing oldest-unread-first means they catch
up in chronological order instead of landing mid-story.
~~~~


## See migrations/037_scout_status_mute.sql for the annotated

Removed from `internal/db/schema/schema.go:1193`.

~~~~text
See migrations/037_scout_status_mute.sql for the annotated
version. 2026-08-06 direct request: long-range scouting's
"still searching"/"en route home"/"party returned" pings were
sharing the single "route_status" mute toggle with unrelated
road/convoy chatter, so a player couldn't silence one without
silencing the other. Splits scouting into its own mutable
category, toggleable straight from the Scout panel (see
HandleScoutMuteToggleCallback) - "CONTACT!"/"SPOTTED" discovery
events stay on "general" and are never affected either way.
~~~~


## See migrations/038_notification_effect_id.sql for the annotated

Removed from `internal/db/schema/schema.go:1204`.

~~~~text
See migrations/038_notification_effect_id.sql for the annotated
version. 2026-08-06 direct request to actually add Telegram's
message_effect_id (send-time animation) support rather than
skip it - queued notifications (raid/clan-war/arena outcomes,
which resolve async in the tick engine, not synchronously from
a bot handler) need a way to carry an optional effect through
to delivery time; see notifications.Dispatcher.drainQueue.
~~~~


## --- RARE_WORLD_FEATURES_PLAN.md Phase 1: Relic Convoys ---

Removed from `internal/db/schema/schema.go:1213`.

~~~~text
--- RARE_WORLD_FEATURES_PLAN.md Phase 1: Relic Convoys ---
A vanishingly rare, server-wide race: a pre-war relic convoy
surfaces at a random coordinate, broadcast to every player at
once, and the first to tap Claim wins a permanent title plus
a cash windfall. See internal/engine/tick/relicconvoys.go for
the spawn/expire tick phases and
internal/bot/handlers/relicconvoy.go for the panel/claim flow.
~~~~


## Partial index over exactly the "is there currently a live

Removed from `internal/db/schema/schema.go:1230`.

~~~~text
Partial index over exactly the "is there currently a live
relic" query the spawn-gate and panel both run every tick/
panel-load - keeps that check O(1) rather than a growing
table scan as claimed history accumulates.
~~~~


## A relic title is permanent and never overwritten by a later

Removed from `internal/db/schema/schema.go:1237`.

~~~~text
A relic title is permanent and never overwritten by a later
relic claim (see 1.3 of the plan doc) - only the FIRST relic
an encampment ever wins sets this.
~~~~


## --- RARE_WORLD_FEATURES_PLAN.md Phase 3: Legacy Epitaphs ---

Removed from `internal/db/schema/schema.go:1242`.

~~~~text
--- RARE_WORLD_FEATURES_PLAN.md Phase 3: Legacy Epitaphs ---
The game's first permanent-defeat state, deliberately scoped
to AI factions only (see the plan doc's design principle 3 -
real players are never wiped by anything in this feature
set). See internal/engine/tick/factiondefeat.go.
~~~~


## --- RARE_WORLD_FEATURES_PLAN.md Phase 4: The Wandering Merchant ---

Removed from `internal/db/schema/schema.go:1252`.

~~~~text
--- RARE_WORLD_FEATURES_PLAN.md Phase 4: The Wandering Merchant ---
A rare, time-limited NPC vendor - deliberately its own table
rather than a synthetic market_exchange seller (see the plan
doc's 4.1 for why). See
internal/engine/tick/wanderingmerchant.go and
internal/bot/handlers/merchant.go.
~~~~


## --- RARE_WORLD_FEATURES_PLAN.md Phase 5: Constellation Alliances ---

Removed from `internal/db/schema/schema.go:1270`.

~~~~text
--- RARE_WORLD_FEATURES_PLAN.md Phase 5: Constellation Alliances ---
A rare, temporary, N-ary cross-clan structure sitting above
clan_diplomacy's existing pairwise pacts - see the plan doc's
5.1 for why this needed its own header+membership table pair
rather than reusing clan_diplomacy's pairwise shape. See
internal/bot/handlers/constellation.go and
internal/engine/tick/constellations.go.
~~~~


## --- NEXT_GENERATION_FEATURES_PLAN.md Phase 1: Achievements &

Removed from `internal/db/schema/schema.go:1301`.

~~~~text
--- NEXT_GENERATION_FEATURES_PLAN.md Phase 1: Achievements &
Milestones, and Phase 2: Daily/Weekly Quest Board. Built
together per that document's suggested order - both are
event-driven off the same real action call sites via
internal/game/progression.RecordEvent. Definitions
(which achievements/quests exist) are static Go data in that
package, not DB tables - only per-player progress lives here.
~~~~


## --- NEXT_GENERATION_FEATURES_PLAN.md Phase 3: Crafting & Gear ---

Removed from `internal/db/schema/schema.go:1331`.

~~~~text
--- NEXT_GENERATION_FEATURES_PLAN.md Phase 3: Crafting & Gear ---
Definitions are static Go data in internal/game/gear, not DB
tables - only ownership/equip state and the crafting queue
live here. See gear.go's own package doc for the "why
resource-generation bonus, not raw combat power" scope note.
~~~~

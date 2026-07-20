# 🛰️ THE VAGABOND

> A persistent, tick-based Telegram MMO — a from-scratch revival of the
> discontinued **SpaceHunt**, rebuilt under a new identity with the same
> command feel, the same emoji language, and a deeper combat/economy
> layer underneath it.

The Vagabond runs entirely inside a Telegram chat. There's no separate
app or website — you build an outpost, recruit an army, raid other
survivors (or the world's AI-controlled Rogue Drone Nests), research
technologies, run a faction, and fight world bosses, all through
buttons and slash commands in one bot conversation. A background tick
engine advances the whole world on a fixed heartbeat, independent of
whether any particular player is online.

---

## What's actually in the game right now

This project has gone through several development phases (see
`SPACEHUNT_PHASE7_LOG.md` for the currently active one), so "what
exists" has grown a lot past the original engine. As of this writing:

- **Persistent combat lifecycle** — raids and Rogue Drone Nest
  skirmishes both go through `marching → engaged → returning →
  completed`, with per-phase notifications, a full battle report
  renderer, retaliation, and toughness-weighted casualties. Nothing
  resolves instantly.
- **AI-controlled Rogue Drone Nests** that scale to the attacking
  player's level using the *same* subsystems a real player base has —
  individually-typed Defense Grid turrets, Guardians/Observers,
  research level, shields, and (at high tiers) a hero-equivalent
  Warlord superpower — not a single flat difficulty number.
- **A real economy**: Scrap/Metal/Crystal/Rations/Electricity resources,
  a Financial Vault (savings, loans, currency conversion), and a
  player-to-player Market Exchange.
- **Research tree** (Technology, Production, Integrity, Shields,
  Intelligence, Thrusters, Weapons) with mechanical effects, not just
  flavor text.
- **A full military roster** — Soldiers, Tactical Drones, Colossus
  Mechs, Nuclear Devices, Destroyers, Bombers, Scout Walkers,
  Battlecruisers, Doomsday Rigs, Liberators, Wraiths, Observers,
  Guardians, Piercing Missiles, and tiered Cargo Ships — each with a
  distinct combat role (hard counters, stealth, garrison-only, etc.),
  defined centrally in `internal/game/content`.
- **Defense Grid**: five turret types with engage-weapon mechanics
  (each turret type is actually good against specific attacker unit
  types, not a flat sum) plus a Warehouse and Anti-Missile Battery.
- **Hero Commander system** with XP/leveling, faction superpowers, and
  (new in Phase 7) a manual "lock units as permanent home garrison"
  control so a player can't accidentally draft away their entire home
  defense.
- **Battle Logistics**: raids require a real war party (20+ combat units,
  staged transport support, and travel units for walking forces), consume scaled
  launch supplies, and drain carried rations, ammunition, electricity, and
  logistics equipment in the field; running dry triggers warnings and can
  force auto-retreat before reaching the target. Active agents now consume
  2.0 base Electricity per tick before upgrades, making automation a real
  operational expense.
- **Discovery-gated world intelligence**: outposts and the Rogue Drone Nest
  are hidden until an exploration expedition or route sighting establishes
  contact. Scout Walkers improve discovery odds; target selection and launch
  checks enforce the same rule server-side. Incoming-raid warnings are issued
  once at a defender's radar-derived proximity threshold, not at launch.
- **Bulk unit selection**: a cycling `x1 → x10 → x100 → MAX` step
  control on the raid draft board, plus `/add`, `/remove`, and bulk
  `/deconstruct` text commands, so building a 500-Soldier army doesn't
  take 500 taps.
- **Clans**, Clan Wars scored from real raid outcomes, **Federations**
  (guilds of guilds), a **Rebellion** faction hub, and a **Global
  Ranking** board.
- **World Bosses** with real marching/retaliation/return mechanics, and
  dynamic **weather events** (Acid Rain, Radiation Storms) that
  actually modify combat and construction, not just decorate the
  broadcast feed.
- **Three-tier scanning** (Manual/Auto/Advanced) and AI/exploration
  combat against level-scaled rogue nests.
- A **notification engine** (Postgres-queue-backed, polling every 3s)
  that drives every phase of the above instead of the bot only ever
  replying synchronously to the message that triggered something.
- A parallel, **independent AI infrastructure workstream**
  (`internal/ai/`) — provider-agnostic (Anthropic/OpenAI/DeepSeek/Qwen/
  Grok/Gemini/Ollama) completion plumbing with caching, budget/cost
  tracking, and permission gating, feeding an in-progress Governor /
  Fleet Commander / Economy Advisor layer. This is a **separate
  workstream from the gameplay work above** — see
  `PROJECT_MASTER_PLAN.md`.

If you want the full unit stat block, turret mechanics, and build
costs, they're rendered live in-bot via the Heavy Workshop panel
(`/factory`) — that's the source of truth, not this file.

---

## Two roadmaps, two logs

This repo is being actively developed by two independent AI-assisted
workstreams that deliberately don't touch the same code:

| Workstream | Scope | Tracking file |
|---|---|---|
| **SpaceHunt gameplay** (combat, economy, heroes, world, UX) | Everything under "What's actually in the game" above | [`SPACEHUNT_PHASE7_LOG.md`](./SPACEHUNT_PHASE7_LOG.md) |
| **AI systems** (`internal/ai/`, Governor, Fleet Commander, Economy Advisor) | Provider-agnostic AI infra + AI-driven gameplay features built on it | [`PROJECT_MASTER_PLAN.md`](./PROJECT_MASTER_PLAN.md) |
| **Living-world continuation** (discovery, routes, encounters, camps, AI civilizations) | The MMO extension of the gameplay workstream | [`MMO_WORLD_EVOLUTION_PLAN.md`](./MMO_WORLD_EVOLUTION_PLAN.md) |

If you're picking this repo up cold — human or AI — read whichever of
those two matches what you're about to touch **before** writing code.
Each one documents what's done, what's in progress, and what's
deliberately left alone so the two workstreams don't collide.

---

## Tech stack

- **Language:** Go 1.22+
- **Database:** PostgreSQL (Supabase)
- **Bot framework:** [`telebot.v3`](https://github.com/tucnak/telebot)
- **Realtime:** Postgres `LISTEN`/`NOTIFY` for push-style updates,
  plus a fixed-interval tick loop (`GAME_TICK_SECONDS`) for world
  simulation (resource production, starvation/desertion, raid
  resolution, logistics consumption, world events, and more)
- **Config:** `godotenv` (`.env` file, see below)
- **Optional AI layer:** Anthropic, OpenAI, DeepSeek, Qwen, Grok,
  Gemini, or a local Ollama instance — entirely optional; the bot runs
  fine with none of these configured (a deterministic mock provider is
  always the last fallback).

---

## Architecture

```text
The-Vagabond/
├── cmd/bot/main.go              # Bootstrap: DB connect, migrations, every
│                                  bot.Handle(...) registration, tick loop start
├── internal/
│   ├── bot/
│   │   ├── handlers/            # One file per feature area — combat.go,
│   │   │                          camp.go, factory.go, hero.go, economy.go,
│   │   │                          clan.go, federation.go, rebellion.go,
│   │   │                          boss.go, research.go, deconstruct.go,
│   │   │                          admin.go, world.go, nlp.go, and more
│   │   └── keyboards/            # Persistent bottom-menu keyboard layouts
│   ├── engine/
│   │   ├── tick/engine.go        # The master heartbeat: resource ticks,
│   │   │                          raid/combat resolution, logistics
│   │   │                          consumption, starvation, world events —
│   │   │                          each phase in its own transaction
│   │   ├── resource/              # Production, upkeep, storage caps
│   │   ├── starvation/            # Morale decay & desertions
│   │   ├── agent/                 # Automation Agent execution
│   │   ├── world/                 # Weather/world-event effects
│   │   ├── notifications/         # Postgres-queue notification dispatcher
│   │   └── realtime/              # Postgres LISTEN/NOTIFY bridge
│   ├── game/
│   │   ├── content/                # Canonical unit registry (stats, costs,
│   │   │                            counters, deconstruct refunds) + Rogue
│   │   │                            Nest composition — the single source of
│   │   │                            truth other packages read from
│   │   ├── battlereport/           # SpaceHunt-style battle report renderer
│   │   ├── scoring/                # Ranking/leaderboard formulas
│   │   ├── governor/               # (AI workstream — see PROJECT_MASTER_PLAN.md)
│   │   ├── fleetcommander/         # (AI workstream)
│   │   └── econadvisor/            # (AI workstream)
│   ├── ai/                        # Provider-agnostic AI infrastructure
│   │                                (AI workstream — zero imports from
│   │                                bot/engine/game on purpose)
│   └── models/                    # Shared domain structs
└── migrations/                    # Every schema change, numbered and
                                     annotated — the statements themselves
                                     are also embedded directly in main.go
                                     and run automatically on startup
```

---

## Environment configuration

Create a `.env` file at the project root:

```bash
# Runtime environment
APP_ENV=development

# Telegram bot token (from @BotFather)
TELEGRAM_TOKEN=1234567890:YOUR_TELEGRAM_BOT_TOKEN

# Supabase/Postgres connection string
DATABASE_URL=postgres://user:password@host:port/dbname?sslmode=require

# Master world-tick interval, in seconds
GAME_TICK_SECONDS=60

# Comma-separated Telegram user IDs allowed to use /admin* commands
ADMIN_IDS=123456789,987654321

# --- Optional: only needed if you want the AI workstream's features live ---
# ANTHROPIC_API_KEY=
# OPENAI_API_KEY=
# DEEPSEEK_API_KEY=
# QWEN_API_KEY=
# GROK_API_KEY=
# GEMINI_API_KEY=
# OLLAMA_BASE_URL=
```

See `.env.example` for the up-to-date canonical template.

---

## Running it

```bash
go mod tidy
go fmt ./...
go build ./...
go run cmd/bot/main.go
```

On startup the bot connects to Postgres, runs every migration statement
embedded in `main.go` (idempotent — safe to run repeatedly), registers
every command/callback handler, and starts the tick loop and
notification dispatcher as background goroutines.

---

## Command surface

The bot responds to persistent bottom-menu buttons (see
`internal/bot/keyboards`) and free-text intent matching (`nlp.go`), but
every feature also has a direct slash command. A non-exhaustive sample
by area:

| Area | Commands |
|---|---|
| Core | `/start`, `/camp`, `/help`, `/inventory`, `/name` |
| Combat | `/raid`, `/scout`, `/autoscan`, `/add`, `/remove`, `/deconstruct` |
| Economy | `/econ`, `/factory`, `/mine`, `/research`, `/mutations` |
| Military/Defense | `/defense`, `/infrastructure`, `/silo`, `/hero` |
| Social | `/clan`, `/clans`, `/federation`, `/federations`, `/rebellion`, `/board`, `/refer`, `/msg` |
| World | `/world`, `/map`, `/broadcast`, `/bosses`, `/ranking` |
| Jobs | `/newjobhyperspeed`, `/newjobteleport`, `/newjoborbitalmaneuver`, `/newjobgathersunlight`, and others |
| Admin | `/admin`, `/admin_tick`, `/admin_metrics`, `/admin_broadcast`, `/admin_gift_resources` |
| AI (optional) | `/ai_status`, `/ai_settings`, `/governor`, `/fleet_commander`, `/economy_advisor` |

Run `/help` in-bot for the live, complete list — this table is a map,
not the territory.

---

## Contributing to this repo

1. Figure out which workstream your change belongs to (see the two-logs
   table above) and read that file first.
2. Keep changes scoped to that workstream's territory unless you're
   fixing a genuine cross-cutting bug — and if you do cross the line,
   document it explicitly in both logs.
3. Add a numbered, annotated file under `migrations/` for any schema
   change, *and* add the same statement(s) to the `migrations := []string{...}`
   slice in `cmd/bot/main.go` — the slice is what actually runs; the
   `.sql` file is the readable reference copy.
4. `go build ./...` and `go fmt ./...` clean before committing.
5. Update the relevant log file with what you did and what's next in
   line, the same way the existing entries are written — the next
   person (or AI) reading it shouldn't have to reverse-engineer intent
   from a diff.


- **Battle Logistics**: raids require 20+ combat units, transport support, travel units for walking forces, launch supplies, and carried rations/ammunition/electricity/logistics that drain during expeditions.

# 📡 THE VAGABOND — MMO SERVER ENGINE
> Persistent Multiplayer Telegram Survival • Strategy • Social MMO

The Vagabond is a tick-based, world-driven multiplayer survival game designed to run asynchronously. The engine simulates resource growth, facility construction upgrades, starvation-based desertions, automated agents, and player-vs-player combat calculations in real time.

---

## 🛠️ Tech Stack & Requirements

- **Language:** Go 1.22+
- **Database:** Supabase PostgreSQL
- **Realtime Dispatcher:** PostgreSQL `LISTEN` / `NOTIFY` Channel Interface
- **Bot Engine:** `telebot/v3`
- **Configuration:** `godotenv`

---

## 📂 Architecture Structure

```text
The-Vagabond/
├── cmd/
│   └── bot/
│       └── main.go                 # Engine Bootstrapper & Shutdown Router
├── internal/
│   ├── bot/
│   │   ├── handlers/
│   │   │   ├── onboarding.go       # Registration & Authentication 
│   │   │   ├── camp.go             # Facility Upgrades UI Handler
│   │   │   ├── combat.go           # Tactical PVP Raid Matchmaking
│   │   │   └── agent.go            # Automation Agent Configuration
│   │   └── keyboards/
│   │       └── navigation.go       # Multi-layered Bottom Context Keyboards
│   ├── models/
│   │   └── models.go               # Shared Entity Domain Structures
│   └── engine/
│       ├── tick/
│       │   └── engine.go           # Master Game Heartbeat Tick Loop
│       ├── resource/
│       │   └── resource.go         # Production, Upkeep, and Storage Caps
│       ├── starvation/
│       │   └── starvation.go       # Morale Decay & Survivor Desertions
│       ├── agent/
│       │   └── agent.go            # Autonomous Task Execution Engine
│       └── realtime/
│           └── listener.go         # Sub-millisecond Postgres LISTEN/NOTIFY Dispatcher
└── migrations/
    ├── 001_initial_schema.sql      # Core Relational DB Tables
    └── 002_realtime_triggers.sql   # Postgres Realtime Push Notify Triggers

    ⚙️ Environment Configuration
Create a .env file at the root level of your project directory using the parameters below:
# Runtime Environment
APP_ENV=development

# Core Game Heartbeat Rate (seconds)
GAME_TICK_SECONDS=60

# Telegram Credentials (From @BotFather)
TELEGRAM_TOKEN=YOUR_TELEGRAM_BOT_TOKEN

# Supabase DSN Key-Value String (Supports passwords with special characters)
DATABASE_URL=host=db.zzmbufopddomyhwcmilg.supabase.co port=5432 user=postgres password='YOUR_SUPABASE_PASSWORD' dbname=postgres sslmode=require

# Authorized Administrator Telegram IDs (Comma-separated)
ADMIN_IDS=6582793388,YOUR_OTHER_ADMIN_ID

🚀 Execution & Command Suite
You can compile, format, and run the server program directly using the native Go CLI inside your terminal:
# Format the entire codebase structure cleanly
go fmt ./...

# Tidy, resolve, and lock dependencies
go mod tidy

# Run the live bot application
go run cmd/bot/main.go

🧪 Administrative Controls
The system provides developers with safe backend commands to inspect and update execution:
/admin_metrics — Returns total player summaries, memory allocations, and active thread states.
/admin_tick — Instantly triggers and calculates a master simulation iteration.
/admin_broadcast [msg] — Dispatches a global push notification to every survivor.

---

### D. Compile Verification

Confirm that everything compiles cleanly without any errors by running:

```powershell
go fmt ./...
go mod tidy
go run cmd/bot/main.go

The application will start, subscribe to the database channels, and initialize successfully.

E. Commit and Synchronize Codebase

With the codebase and documentation complete, run these commands in your PowerShell console to save your progress and sync it to your remote GitHub repository:
# Stage all files
git add .

# Create the final commit
git commit -m "feat: complete Phase 11 production launch preparations with recovery middleware, optimized loggers, and system README manuals"

# Push to GitHub main branch
git push origin main

package main

import (
	"database/sql"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/handlers"
	"github.com/NomadDigita/The-Vagabond/internal/engine/realtime"
	"github.com/NomadDigita/The-Vagabond/internal/engine/tick"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"gopkg.in/telebot.v3"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("🔥 CRITICAL RUNTIME PANIC RECOVERED!")
			log.Printf("Panic Details: %v", r)
			log.Printf("Stack Trace:\n%s", string(debug.Stack()))
			log.Println("Panic contained successfully. Engine online.")
		}
	}()

	log.Println("Starting The Vagabond server initialization sequence...")

	if err := godotenv.Load(); err != nil {
		log.Println("Note: .env file not detected. Loading configuration from system environment variables.")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("Fatal: DATABASE_URL environment parameter not set.")
	}

	botToken := os.Getenv("TELEGRAM_TOKEN")
	if botToken == "" {
		log.Fatal("Fatal: TELEGRAM_TOKEN environment parameter not set.")
	}

	adminIDs := os.Getenv("ADMIN_IDS")
	if adminIDs == "" {
		log.Println("Warning: ADMIN_IDS is empty. Admin overrides will be inaccessible.")
	}

	tickSeconds := 60
	if intervalStr := os.Getenv("GAME_TICK_SECONDS"); intervalStr != "" {
		if val, err := strconv.Atoi(intervalStr); err == nil {
			tickSeconds = val
		}
	}

	log.Println("Connecting to Supabase Database...")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Fatal: Database driver initialization failure: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatalf("Fatal: Database network connection check failed: %v", err)
	}
	log.Println("Database connection pool established successfully.")

	pref := telebot.Settings{
		Token:  botToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := telebot.NewBot(pref)
	if err != nil {
		log.Fatalf("Fatal: Telegram API initialization failure: %v", err)
	}
	log.Printf("Telegram credentials accepted. Bot logged in as: @%s", bot.Me.Username)

	tickEngine := tick.NewEngine(db, time.Duration(tickSeconds)*time.Second)
	tickEngine.Start()

	realtimeListener := realtime.NewListener(dbURL, db, bot)
	realtimeListener.Start()

	onboarding := handlers.NewOnboardingHandler(db)
	camp := handlers.NewCampHandler(db)
	combat := handlers.NewCombatHandler(db)
	agentH := handlers.NewAgentHandler(db)
	admin := handlers.NewAdminHandler(db, tickEngine, adminIDs)
	hero := handlers.NewHeroHandler(db)
	world := handlers.NewWorldHandler(db)

	bot.Handle("/start", onboarding.HandleStart)
	bot.Handle("/camp", camp.HandleCamp)
	bot.Handle("/raid", combat.HandleRaidBoard)
	bot.Handle("/agent", agentH.HandleAgent)
	bot.Handle("/hero", hero.HandleHeroPanel)
	bot.Handle("/world", world.HandleWorldFeed)

	bot.Handle("/admin_tick", admin.HandleAdminTick)
	bot.Handle("/admin_broadcast", admin.HandleAdminBroadcast)
	bot.Handle("/admin_metrics", admin.HandleAdminMetrics)

	// Bottom-Dock Multi-layered Navigation Handlers
	bot.Handle("📡 Terminal HQ", onboarding.HandleStart)
	bot.Handle("⛺ Outpost Camp", camp.HandleCamp)
	bot.Handle("⚔️ Raid Board", combat.HandleRaidBoard)
	bot.Handle("🧠 Automation Agent", agentH.HandleAgent)

	// Submenu Layer Handlers
	bot.Handle("🔨 Structural Upgrades", camp.HandleCamp)
	bot.Handle("👥 Hero Commander", hero.HandleHeroPanel)
	bot.Handle("🛰️ Scan Targets", combat.HandleRaidBoard)
	bot.Handle("📻 Wasteland Radio", world.HandleWorldFeed)
	bot.Handle("⬅️ Back to HQ", onboarding.HandleStart)

	// Button Callbacks
	bot.Handle("\fupgrade_mod", camp.HandleUpgradeCallback)
	bot.Handle("\flaunch_raid", combat.HandleLaunchRaidCallback)
	bot.Handle("\ftoggle_agent", agentH.HandleToggleAgentCallback)
	bot.Handle("\fset_agent_mode", agentH.HandleSetModeCallback)

	go func() {
		log.Println("Active long-polling loop engaged. System operational.")
		bot.Start()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-quit

	log.Println("Termination request received. Initiating graceful shutdown protocol...")

	tickEngine.Stop()
	realtimeListener.Stop()
	db.Close()

	log.Println("System components cleanly dismantled. Server offline.")
}

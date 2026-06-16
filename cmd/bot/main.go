package main

import (
	"database/sql"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/handlers"
	"github.com/NomadDigita/The-Vagabond/internal/engine/tick"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"gopkg.in/telebot.v3"
)

func main() {
	log.Println("Starting The Vagabond server initialization sequence...")

	// 1. Load local env file if present
	if err := godotenv.Load(); err != nil {
		log.Println("Note: .env file not detected. Loading configuration from system environment variables.")
	}

	// 2. Fetch required environment values
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("Fatal: DATABASE_URL environment parameter not set.")
	}

	botToken := os.Getenv("TELEGRAM_TOKEN")
	if botToken == "" {
		log.Fatal("Fatal: TELEGRAM_TOKEN environment parameter not set.")
	}

	// Parse tick interval with fallback of 60 seconds
	tickSeconds := 60
	if intervalStr := os.Getenv("GAME_TICK_SECONDS"); intervalStr != "" {
		if val, err := strconv.Atoi(intervalStr); err == nil {
			tickSeconds = val
		}
	}

	// 3. Connect to Supabase PostgreSQL using connection pool limits
	log.Println("Connecting to Supabase Database...")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Fatal: Database driver initialization failure: %v", err)
	}
	defer db.Close()

	// Configure pool parameters for performance and scaling safety
	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	// Verify live connectivity immediately
	if err := db.Ping(); err != nil {
		log.Fatalf("Fatal: Database network connection check failed: %v", err)
	}
	log.Println("Database connection pool established successfully.")

	// 4. Initialize Telegram Bot Listener Client
	pref := telebot.Settings{
		Token:  botToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := telebot.NewBot(pref)
	if err != nil {
		log.Fatalf("Fatal: Telegram API initialization failure: %v", err)
	}
	log.Printf("Telegram credentials accepted. Bot logged in as: @%s", bot.Me.Username)

	// 5. Initialize and Boot Tick Engine background routine
	tickEngine := tick.NewEngine(db, time.Duration(tickSeconds)*time.Second)
	tickEngine.Start()

	// 6. Dependency Injection & Handler Routines Setup
	onboarding := handlers.NewOnboardingHandler(db)
	camp := handlers.NewCampHandler(db)

	// Define command mappings
	bot.Handle("/start", onboarding.HandleStart)
	bot.Handle("/camp", camp.HandleCamp)

	// Register Inline Button Callbacks
	bot.Handle("\fupgrade_mod", camp.HandleUpgradeCallback)

	// 7. Support Graceful Shutdown Intercepts
	go func() {
		log.Println("Active long-polling loop engaged. System operational.")
		bot.Start()
	}()

	// System blocks here until receiving OS termination signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-quit

	log.Println("Termination request received. Initiating graceful shutdown protocol...")

	// Stop background services gracefully
	tickEngine.Stop()
	bot.Stop()

	log.Println("System components cleanly dismantled. Server offline.")
}

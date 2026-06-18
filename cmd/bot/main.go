package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
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

	// 5. Initialize and Boot background system engines
	tickEngine := tick.NewEngine(db, time.Duration(tickSeconds)*time.Second)
	tickEngine.Start()

	realtimeListener := realtime.NewListener(dbURL, db, bot)
	realtimeListener.Start()

	// 6. Dependency Injection & Handler Routines Setup with Dynamic Admin IDs forwarding
	onboarding := handlers.NewOnboardingHandler(db)
	camp := handlers.NewCampHandler(db, adminIDs)
	combat := handlers.NewCombatHandler(db, adminIDs)
	agentH := handlers.NewAgentHandler(db, adminIDs)
	admin := handlers.NewAdminHandler(db, tickEngine, adminIDs)
	hero := handlers.NewHeroHandler(db)
	world := handlers.NewWorldHandler(db)
	econ := handlers.NewEconomyHandler(db)
	clan := handlers.NewClanHandler(db)
	factory := handlers.NewFactoryHandler(db)
	arena := handlers.NewArenaHandler(db)
	silo := handlers.NewSiloHandler(db)
	nlp := handlers.NewNLPHandler(onboarding, camp, combat, econ, clan)

	bot.Handle("/start", onboarding.HandleStart)
	bot.Handle("/camp", camp.HandleCamp)
	bot.Handle("/raid", combat.HandleRaidBoard)
	bot.Handle("/agent", agentH.HandleAgent)
	bot.Handle("/hero", hero.HandleHeroPanel)
	bot.Handle("/world", world.HandleWorldFeed)
	bot.Handle("/econ", econ.HandleEconPanel)
	bot.Handle("/clan", clan.HandleClanPanel)
	bot.Handle("/scout", combat.HandleScout)
	bot.Handle("/factory", factory.HandleFactoryPanel)
	bot.Handle("/map", world.HandleSectorMap)
	bot.Handle("/help", onboarding.HandleHelp)
	bot.Handle("/inventory", econ.HandleWarehouseReserves)
	bot.Handle("/admin", admin.HandleAdminPanel)
	bot.Handle("/arena", arena.HandleArenaPanel)
	bot.Handle("/broadcast", world.HandleSectorBroadcast)
	bot.Handle("/mutations", camp.HandleMutationsPanel)
	bot.Handle("/silo", silo.HandleSiloPanel)
	bot.Handle("/mine", camp.HandleActiveMining)

	// Admin Override commands
	bot.Handle("/admin_tick", admin.HandleAdminTick)
	bot.Handle("/admin_broadcast", admin.HandleAdminBroadcast)
	bot.Handle("/admin_metrics", admin.HandleAdminMetrics)
	bot.Handle("/admin_give", admin.HandleAdminGive)
	bot.Handle("/admin_faction", admin.HandleAdminFaction)
	bot.Handle("/admin_gift_premium", admin.HandleAdminGiftPremium)
	bot.Handle("/admin_gift_resources", admin.HandleAdminGiftResources)

	// Bottom-Dock Multi-layered Navigation Handlers (Checks Admin status dynamically)
	bot.Handle("📡 Terminal HQ", onboarding.HandleStart)
	bot.Handle("⛺ Outpost Camp", camp.HandleCamp)
	bot.Handle("⚔️ Tactical Combat", combat.HandleRaidBoard)
	bot.Handle("🏦 System Economy", econ.HandleEconPanel)

	// Admin sub-navigation panel routing
	bot.Handle("🏛️ Admin Terminal", admin.HandleAdminPanel)
	bot.Handle("⚡ Force Master Tick", admin.HandleAdminTick)
	bot.Handle("🪙 Inject Resources", admin.HandleAdminGive)
	bot.Handle("🛰️ Server Metrics", admin.HandleAdminMetrics)

	// Submenu Layer Handlers
	bot.Handle("🔨 Structural Upgrades", camp.HandleStructuralUpgrades)
	bot.Handle("👥 Hero Commander", hero.HandleHeroPanel)
	bot.Handle("🧠 Automation Agent", agentH.HandleAgent)
	bot.Handle("🧪 Research Lab", camp.HandleCamp)
	bot.Handle("🧬 Mutation Core", camp.HandleMutationsPanel)
	bot.Handle("⛏️ Active Mining", camp.HandleActiveMining) // Mapped manual mining keyboard trigger
	bot.Handle("🛰️ Scan Targets", combat.HandleTargetMatrix)
	bot.Handle("📻 Wasteland Radio", world.HandleWorldFeed)
	bot.Handle("📦 Warehouse Reserves", econ.HandleWarehouseReserves)
	bot.Handle("🪙 Financial Vault", econ.HandleFinancialVault)
	bot.Handle("🛡️ Clan Alliances", clan.HandleClanPanel)
	bot.Handle("🏭 Heavy Workshop", factory.HandleFactoryPanel)
	bot.Handle("🏟️ Combat Arena", arena.HandleArenaPanel)
	bot.Handle("☢️ Strategic Silo", silo.HandleSiloPanel)
	bot.Handle("⬅️ Back to HQ", onboarding.HandleStart)

	// Map all plain text inputs to our Natural Language intent router
	bot.Handle(telebot.OnText, nlp.HandleTextMessage)

	// Button Callbacks
	bot.Handle("\fupgrade_mod", camp.HandleUpgradeCallback)
	bot.Handle("\flaunch_raid", combat.HandleLaunchRaidCallback)
	bot.Handle("\ftoggle_agent", agentH.HandleToggleAgentCallback)
	bot.Handle("\fset_agent_mode", agentH.HandleSetModeCallback)
	bot.Handle("\fjoin_faction", onboarding.HandleFactionCallback)
	bot.Handle("\fbank_action", econ.HandleBankCallback)
	bot.Handle("\fmarket_buy", econ.HandleMarketCallback)
	bot.Handle("\fcreate_clan", clan.HandleCreateClanCallback)
	bot.Handle("\fleave_clan", clan.HandleLeaveClanCallback)
	bot.Handle("\fdeclare_clan_war", clan.HandleDeclareClanWarCallback)
	bot.Handle("\fexp_action", combat.HandleExpeditionActions)
	bot.Handle("\fcraft_item", factory.HandleCraftCallback)
	bot.Handle("\fspy_action", combat.HandleSpyCallback)
	bot.Handle("\fupgrade_tech", camp.HandleUpgradeCallback)
	bot.Handle("\fpost_listing", econ.HandleMarketCallback)
	bot.Handle("\fbuy_listing", econ.HandleMarketCallback)
	bot.Handle("\fmutate_mod", camp.HandleMutationCallback)
	bot.Handle("\fjoin_queue", arena.HandleJoinQueueCallback)
	bot.Handle("\flaunch_icbm", silo.HandleLaunchICBMCallback)
	bot.Handle("\fmine_action", camp.HandleMineCallback)
	bot.Handle("\fhero_action", hero.HandleHeroCallback) // Mapped Hero training & healing callback

	// --- 7. BIND LIGHTWEIGHT HTTP PORT FOR RENDER DEPLOYMENTS ---
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("SYSTEM OPERATIONAL"))
	})

	go func() {
		log.Printf("Inbound HTTP listener bound to port :%s for health telemetry checks.", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Printf("Warning: HTTP Server closed: %v", err)
		}
	}()

	// --- 8. AUTONOMOUS KEEP-ALIVE SELF-PINGER ---
	selfPingURL := os.Getenv("SELF_PING_URL")
	if selfPingURL != "" {
		go func() {
			log.Printf("Autonomous Keep-Alive Pinger active. target: %s", selfPingURL)
			ticker := time.NewTicker(10 * time.Minute)
			for range ticker.C {
				resp, err := http.Get(selfPingURL)
				if err != nil {
					log.Printf("Keep-Alive Pinger connection warning: %v", err)
					continue
				}
				_ = resp.Body.Close()
				log.Println("⚡ Keep-Alive Pinger succeeded. Instance held awake.")
			}
		}()
	} else {
		log.Println("Note: SELF_PING_URL parameters not set. Keep-Alive pinger is idle.")
	}

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
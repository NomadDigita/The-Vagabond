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

func executeStartupMigrations(db *sql.DB) {
	log.Println("Executing database initialization check...")

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			telegram_id BIGINT PRIMARY KEY,
			username VARCHAR(255) DEFAULT '',
			first_name VARCHAR(255) DEFAULT '',
			state VARCHAR(50) DEFAULT 'onboarding',
			faction VARCHAR(50) DEFAULT '',
			premium_until TIMESTAMP WITH TIME ZONE,
			registered_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			last_active TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS coordinates (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			x INT NOT NULL,
			y INT NOT NULL,
			biome VARCHAR(50) NOT NULL,
			danger_level INT DEFAULT 1,
			region VARCHAR(50) NOT NULL,
			terrain VARCHAR(50) NOT NULL,
			CONSTRAINT unique_coordinates UNIQUE (x, y)
		);`,

		`CREATE INDEX IF NOT EXISTS idx_coordinates_xy ON coordinates(x, y);`,

		`CREATE TABLE IF NOT EXISTS encampments (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id BIGINT UNIQUE NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			coordinate_id UUID NOT NULL REFERENCES coordinates(id),
			level INT DEFAULT 1,
			established_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS resources (
			encampment_id UUID PRIMARY KEY REFERENCES encampments(id) ON DELETE CASCADE,
			scrap DOUBLE PRECISION DEFAULT 0.00,
			rations DOUBLE PRECISION DEFAULT 0.00,
			energy DOUBLE PRECISION DEFAULT 0.00,
			neuro_cores DOUBLE PRECISION DEFAULT 0.00,
			steel DOUBLE PRECISION DEFAULT 0.00,
			uranium DOUBLE PRECISION DEFAULT 0.00,
			hydrogen DOUBLE PRECISION DEFAULT 0.00,
			iron DOUBLE PRECISION DEFAULT 0.00,
			oil DOUBLE PRECISION DEFAULT 0.00,
			gold DOUBLE PRECISION DEFAULT 0.00,
			silver DOUBLE PRECISION DEFAULT 0.00,
			diamond DOUBLE PRECISION DEFAULT 0.00,
			dollars DOUBLE PRECISION DEFAULT 0.00,
			last_mined_at TIMESTAMP WITH TIME ZONE,
			last_ticked_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS modules (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			type VARCHAR(50) NOT NULL,
			level INT DEFAULT 1,
			is_upgrading BOOLEAN DEFAULT FALSE,
			upgrade_ready_at TIMESTAMP WITH TIME ZONE,
			CONSTRAINT unique_camp_module UNIQUE (encampment_id, type)
		);`,

		`CREATE TABLE IF NOT EXISTS workshop_inventory (
			encampment_id UUID PRIMARY KEY REFERENCES encampments(id) ON DELETE CASCADE,
			fusion_tanks INT DEFAULT 0,
			nuclear_shields INT DEFAULT 0,
			soldiers INT DEFAULT 0,
			drones INT DEFAULT 0,
			jets INT DEFAULT 0,
			mechs INT DEFAULT 0,
			nukes INT DEFAULT 0,
			buggies INT DEFAULT 0,
			ships INT DEFAULT 0,
			haulers INT DEFAULT 0,
			tankers INT DEFAULT 0,
			rigs INT DEFAULT 0
		);`,

		`ALTER TABLE workshop_inventory ADD COLUMN IF NOT EXISTS miners INT DEFAULT 1;`,

		`CREATE TABLE IF NOT EXISTS active_mining_queues (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			resource_type VARCHAR(50) NOT NULL,
			miners_assigned INT DEFAULT 1,
			started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			ready_at TIMESTAMP WITH TIME ZONE NOT NULL,
			is_completed BOOLEAN DEFAULT FALSE
		);`,

		`CREATE TABLE IF NOT EXISTS raids (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			attacker_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			defender_id UUID REFERENCES encampments(id) ON DELETE CASCADE,
			state VARCHAR(50) NOT NULL,
			resolve_time TIMESTAMP WITH TIME ZONE NOT NULL,
			round_number INT DEFAULT 0,
			attacker_rations DOUBLE PRECISION DEFAULT 100.0,
			attacker_ammo DOUBLE PRECISION DEFAULT 100.0,
			attacker_losses INT DEFAULT 0,
			defender_losses INT DEFAULT 0
		);`,

		`ALTER TABLE raids ADD COLUMN IF NOT EXISTS stolen_scrap DOUBLE PRECISION DEFAULT 0.00;`,

		`CREATE TABLE IF NOT EXISTS raid_forces (
			raid_id UUID PRIMARY KEY REFERENCES raids(id) ON DELETE CASCADE,
			hero_id UUID,
			soldiers_mobilized INT DEFAULT 0,
			mechs_mobilized INT DEFAULT 0,
			buggies_mobilized INT DEFAULT 0,
			route_type VARCHAR(50) DEFAULT 'direct'
		);`,

		`CREATE TABLE IF NOT EXISTS raid_coop_members (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			raid_id UUID NOT NULL REFERENCES raids(id) ON DELETE CASCADE,
			encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			soldiers_contributed INT DEFAULT 0,
			mechs_contributed INT DEFAULT 0,
			CONSTRAINT unique_raid_coop_member UNIQUE (raid_id, encampment_id)
		);`,

		`CREATE TABLE IF NOT EXISTS agent_tasks (
			user_id BIGINT PRIMARY KEY REFERENCES users(telegram_id) ON DELETE CASCADE,
			mode VARCHAR(50) DEFAULT 'collector',
			is_active BOOLEAN DEFAULT FALSE
		);`,

		`CREATE TABLE IF NOT EXISTS mutation_states (
			encampment_id UUID PRIMARY KEY REFERENCES encampments(id) ON DELETE CASCADE,
			synaptic_lvl INT DEFAULT 1,
			salvage_lvl INT DEFAULT 1,
			bio_lvl INT DEFAULT 1
		);`,

		`CREATE TABLE IF NOT EXISTS research_states (
			encampment_id UUID PRIMARY KEY REFERENCES encampments(id) ON DELETE CASCADE,
			econ_tech_lvl INT DEFAULT 1,
			defense_tech_lvl INT DEFAULT 1,
			military_tech_lvl INT DEFAULT 1
		);`,

		`CREATE TABLE IF NOT EXISTS bank_accounts (
			encampment_id UUID PRIMARY KEY REFERENCES encampments(id) ON DELETE CASCADE,
			balance DOUBLE PRECISION DEFAULT 0.00,
			balance_cash DOUBLE PRECISION DEFAULT 0.00,
			loan_amount DOUBLE PRECISION DEFAULT 0.00,
			loan_cash DOUBLE PRECISION DEFAULT 0.00
		);`,

		`ALTER TABLE bank_accounts ADD COLUMN IF NOT EXISTS balance_cash DOUBLE PRECISION DEFAULT 0.00;`,
		`ALTER TABLE bank_accounts ADD COLUMN IF NOT EXISTS loan_cash DOUBLE PRECISION DEFAULT 0.00;`,

		`CREATE TABLE IF NOT EXISTS clans (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) UNIQUE NOT NULL,
			leader_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE
		);`,

		`CREATE TABLE IF NOT EXISTS user_clans (
			user_id BIGINT PRIMARY KEY REFERENCES users(telegram_id) ON DELETE CASCADE,
			clan_id UUID NOT NULL REFERENCES clans(id) ON DELETE CASCADE,
			role VARCHAR(50) NOT NULL
		);`,

		`CREATE TABLE IF NOT EXISTS market_exchange (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			seller_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			item_type VARCHAR(50) NOT NULL,
			quantity INT NOT NULL,
			price_dollars DOUBLE PRECISION NOT NULL,
			is_sold BOOLEAN DEFAULT FALSE
		);`,

		`CREATE TABLE IF NOT EXISTS spy_missions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			spy_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			target_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			is_intercepted BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`ALTER TABLE spy_missions ADD COLUMN IF NOT EXISTS resolved BOOLEAN DEFAULT FALSE;`,

		`CREATE TABLE IF NOT EXISTS arena_queue (
			user_id BIGINT PRIMARY KEY REFERENCES users(telegram_id) ON DELETE CASCADE,
			bracket VARCHAR(50) NOT NULL,
			entered_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`ALTER TABLE arena_queue ADD COLUMN IF NOT EXISTS entered_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;`,

		`CREATE TABLE IF NOT EXISTS arena_battles (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			bracket VARCHAR(50) NOT NULL,
			winner_username VARCHAR(255) NOT NULL,
			loser_username VARCHAR(255) NOT NULL,
			winner_loot DOUBLE PRECISION DEFAULT 0.00,
			battle_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS heroes (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			encampment_id UUID UNIQUE NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			trait VARCHAR(255) NOT NULL,
			injuries VARCHAR(255) NOT NULL,
			battles_survived INT DEFAULT 0,
			superpower VARCHAR(255) NOT NULL,
			level INT DEFAULT 1,
			xp INT DEFAULT 0
		);`,

		`ALTER TABLE heroes ADD COLUMN IF NOT EXISTS level INT DEFAULT 1;`,
		`ALTER TABLE heroes ADD COLUMN IF NOT EXISTS xp INT DEFAULT 0;`,

		`CREATE TABLE IF NOT EXISTS notifications (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id BIGINT NOT NULL REFERENCES users(telegram_id) ON DELETE CASCADE,
			message TEXT NOT NULL,
			is_sent BOOLEAN DEFAULT FALSE,
			queued_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS world_state (
			id INT PRIMARY KEY,
			active_weather VARCHAR(50) DEFAULT 'nominal',
			last_changed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`INSERT INTO world_state (id, active_weather, last_changed_at)
		VALUES (1, 'nominal', CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO NOTHING;`,

		`CREATE TABLE IF NOT EXISTS world_news (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			headline TEXT NOT NULL,
			logged_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS world_events (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			event_type VARCHAR(50) NOT NULL,
			expires_at TIMESTAMP WITH TIME ZONE NOT NULL
		);`,

		`CREATE INDEX IF NOT EXISTS idx_world_events_expires ON world_events(expires_at);`,
	}

	for _, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil {
			log.Fatalf("Fatal: Failed to execute startup database initialization script: %v", err)
		}
	}
	log.Println("All schema initialization verifications complete.")
}

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

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(20)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatalf("Fatal: Database network connection check failed: %v", err)
	}
	log.Println("Database connection pool established successfully.")

	executeStartupMigrations(db)

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
	research := handlers.NewResearchHandler(db)
	exchange := handlers.NewExchangeHandler(db)
	nlp := handlers.NewNLPHandler(onboarding, camp, combat, econ, clan, hero, agentH, factory, silo, research, exchange, world)

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
	bot.Handle("/research", research.HandleResearchPanel)

	bot.Handle("/admin_tick", admin.HandleAdminTick)
	bot.Handle("/admin_broadcast", admin.HandleAdminBroadcast)
	bot.Handle("/admin_metrics", admin.HandleAdminMetrics)
	bot.Handle("/admin_give", admin.HandleAdminGive)
	bot.Handle("/admin_faction", admin.HandleAdminFaction)
	bot.Handle("/admin_gift_premium", admin.HandleAdminGiftPremium)
	bot.Handle("/admin_gift_resources", admin.HandleAdminGiftResources)

	bot.Handle("📡 Terminal HQ", onboarding.HandleStart)
	bot.Handle("⛺ Outpost Camp", camp.HandleCamp)
	bot.Handle("⚔️ Tactical Combat", combat.HandleRaidBoard)
	bot.Handle("🏦 System Economy", econ.HandleEconPanel)
	bot.Handle("🏭 Heavy Workshop", factory.HandleFactoryPanel)

	bot.Handle("🏛️ Admin Terminal", admin.HandleAdminPanel)
	bot.Handle("⚡ Force Master Tick", admin.HandleAdminTick)
	bot.Handle("🪙 Inject Resources", admin.HandleAdminGive)
	bot.Handle("🛰️ Server Metrics", admin.HandleAdminMetrics)

	bot.Handle("🔨 Structural Upgrades", camp.HandleStructuralUpgrades)
	bot.Handle("👥 Hero Commander", hero.HandleHeroPanel)
	bot.Handle("🧠 Automation Agent", agentH.HandleAgent)
	bot.Handle("🧪 Research Lab", research.HandleResearchPanel)
	bot.Handle("🧬 Mutation Core", camp.HandleMutationsPanel)
	bot.Handle("⛏️ Active Mining", camp.HandleActiveMining)
	bot.Handle("🛰️ Scan Targets", combat.HandleTargetMatrix)
	bot.Handle("🛸 Expedition Radar", combat.HandleExpeditionRadar)
	bot.Handle("📻 Wasteland Radio", world.HandleWorldFeed)
	bot.Handle("📦 Warehouse Reserves", econ.HandleWarehouseReserves)
	bot.Handle("🪙 Financial Vault", econ.HandleFinancialVault)
	bot.Handle("🛡️ Clan Alliances", clan.HandleClanPanel)
	bot.Handle("🏟️ Combat Arena", arena.HandleArenaPanel)
	bot.Handle("☢️ Strategic Silo", silo.HandleSiloPanel)
	bot.Handle("💱 Market Exchange", exchange.HandleExchangePanel)
	bot.Handle("🪖 Recruit Troops", factory.HandleRecruitPanel)
	bot.Handle("🚗 Logistics Vehicles", factory.HandleVehiclesPanel)
	bot.Handle("⬅️ Back to HQ", onboarding.HandleStart)

	bot.Handle(telebot.OnText, nlp.HandleTextMessage)

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
	bot.Handle("\fupgrade_tech", research.HandleUpgradeTechCallback)
	bot.Handle("\fpost_listing", exchange.HandlePostListingCallback)
	bot.Handle("\fbuy_listing", exchange.HandleBuyListingCallback)
	bot.Handle("\fmutate_mod", camp.HandleMutationCallback)
	bot.Handle("\fjoin_queue", arena.HandleJoinQueueCallback)
	bot.Handle("\flaunch_icbm", silo.HandleLaunchICBMCallback)
	bot.Handle("\fmine_action", camp.HandleMineCallback)
	bot.Handle("\fhero_action", hero.HandleHeroCallback)
	bot.Handle("\flaunch_interceptor", combat.HandleLaunchInterceptor)
	bot.Handle("\fadmin_action", admin.HandleAdminActionCallback)
	bot.Handle("\fstage_coop", combat.HandleStageCoopCallback)
	bot.Handle("\fjoin_coop", combat.HandleJoinCoopCallback)

	bot.Handle("\fclan_manage", clan.HandleManageMembersCallback)
	bot.Handle("\fclan_stats", clan.HandleAllianceStatsCallback)
	bot.Handle("\fclan_kick", clan.HandleKickMemberCallback)
	bot.Handle("\fclan_promote", clan.HandlePromoteMemberCallback)

	bot.Handle("\fconfirm_launch", combat.HandleConfirmHangarLaunchCallback)

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
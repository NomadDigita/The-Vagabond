package main

import (
	"context"
	"database/sql"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
	"github.com/NomadDigita/The-Vagabond/internal/ai/providers/anthropic"
	"github.com/NomadDigita/The-Vagabond/internal/ai/providers/gemini"
	"github.com/NomadDigita/The-Vagabond/internal/ai/providers/mock"
	"github.com/NomadDigita/The-Vagabond/internal/ai/providers/ollama"
	"github.com/NomadDigita/The-Vagabond/internal/ai/providers/openaicompat"
	"github.com/NomadDigita/The-Vagabond/internal/bot/handlers"
	"github.com/NomadDigita/The-Vagabond/internal/db/schema"
	"github.com/NomadDigita/The-Vagabond/internal/dbdriver"
	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications" // Added missing package import
	"github.com/NomadDigita/The-Vagabond/internal/engine/realtime"
	"github.com/NomadDigita/The-Vagabond/internal/engine/tick"
	"github.com/NomadDigita/The-Vagabond/internal/game/battleanalyst"
	"github.com/NomadDigita/The-Vagabond/internal/game/devconsole"
	"github.com/NomadDigita/The-Vagabond/internal/game/econadvisor"
	"github.com/NomadDigita/The-Vagabond/internal/game/fleetcommander"
	"github.com/NomadDigita/The-Vagabond/internal/game/galaxyadvisor"
	"github.com/NomadDigita/The-Vagabond/internal/game/governor"
	"github.com/NomadDigita/The-Vagabond/internal/game/guildassistant"
	"github.com/NomadDigita/The-Vagabond/internal/game/nlpcommand"
	"github.com/NomadDigita/The-Vagabond/internal/game/npcintel"
	"github.com/NomadDigita/The-Vagabond/internal/game/researchplanner"
	"github.com/joho/godotenv"
	"gopkg.in/telebot.v3"
	"gopkg.in/telebot.v3/middleware"
)

// handleRecoveredPanic is wired into middleware.Recover (see main()) to
// stop a single panicking handler from crashing the entire bot process
// for every connected player. A named function instead of an inline
// closure specifically so it's unit-testable (see main_test.go) -
// nothing here should ever itself be able to panic, since a panic inside
// the panic recoverer has nothing left to catch it.
func handleRecoveredPanic(err error, c telebot.Context) {
	var senderID int64
	if sender := c.Sender(); sender != nil {
		senderID = sender.ID
	}
	callbackUnique := ""
	if cb := c.Callback(); cb != nil {
		callbackUnique = cb.Unique
	}
	log.Printf("PANIC RECOVERED: %v | sender=%d | callback=%q | text=%q", err, senderID, callbackUnique, c.Text())
	if c.Callback() != nil {
		_ = c.Respond(&telebot.CallbackResponse{Text: "⚠️ Something went wrong processing that. Please try again."})
	} else {
		_ = c.Send("⚠️ Something went wrong processing that command. Please try again.")
	}
}

func executeStartupMigrations(db *sql.DB) {
	log.Println("Executing database initialization check...")

	migrations := schema.Statements()

	for _, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil {
			log.Fatalf("Fatal: Failed to execute startup database initialization script: %v", err)
		}
	}
	log.Println("All schema initialization verifications complete.")
}

func relocateZeroCoordinates(db *sql.DB) {
	log.Println("Geographical Spawning Self-Healing relocator pass active...")
	ctx := context.Background()

	queryZeroAndDuplicated := `
		SELECT DISTINCT c.id, c.region 
		FROM coordinates c
		JOIN encampments e ON e.coordinate_id = c.id
		WHERE (c.x = 0 AND c.y = 0) 
		   OR (c.x = 913 AND c.y = -843)
		   OR c.id IN (
		       SELECT coordinate_id 
		       FROM encampments 
		       GROUP BY coordinate_id 
		       HAVING COUNT(*) > 1
		   )`

	rows, err := db.QueryContext(ctx, queryZeroAndDuplicated)
	if err != nil {
		log.Printf("Spawning relocator sweep skipped: %v", err)
		return
	}
	defer rows.Close()

	type zeroCoord struct {
		id     string
		region string
	}
	var coords []zeroCoord
	for rows.Next() {
		var z zeroCoord
		if err := rows.Scan(&z.id, &z.region); err == nil {
			coords = append(coords, z)
		}
	}
	rows.Close()

	rSource := rand.NewSource(time.Now().UnixNano())
	rGen := rand.New(rSource)

	for _, c := range coords {
		success := false
		var x, y int
		for attempt := 0; attempt < 100; attempt++ {
			switch c.region {
			case "Africa":
				x = rGen.Intn(991) + 10
				y = rGen.Intn(991) + 10
			case "Europe":
				x = -(rGen.Intn(991) + 10)
				y = rGen.Intn(991) + 10
			case "Asia":
				x = rGen.Intn(991) + 10
				y = -(rGen.Intn(991) + 10)
			default:
				x = -(rGen.Intn(991) + 10)
				y = -(rGen.Intn(991) + 10)
			}

			_, err := db.ExecContext(ctx, "UPDATE coordinates SET x = $1, y = $2 WHERE id = $3 AND NOT EXISTS(SELECT 1 FROM coordinates WHERE x = $1 AND y = $2)", x, y, c.id)
			if err == nil {
				success = true
				break
			}
		}
		if success {
			log.Printf("Database Healing: Stuck coordinate [%s] redistributed to [%s quadrant: %d, %d]", c.id, c.region, x, y)
		}
	}
}

// aiFactionSeed describes one persistent AI civilization to seed at
// startup (Phase 6, MMO_WORLD_EVOLUTION_PLAN.md). Deliberately modeled as
// a real encampments row (see migrations/032_mmo_ai_civilizations.sql for
// why) rather than a special-cased entity, so the entire existing
// discovery/targeting/raiding/looting pipeline handles it with zero
// changes.
type aiFactionSeed struct {
	telegramID int64
	key        string
	name       string
	region     string
	level      int
}

// seedAICivilizations creates the persistent AI factions described in
// MMO_WORLD_EVOLUTION_PLAN.md's Phase 6 if they don't already exist
// (idempotent via ai_faction_key). Two per continent, at distinct starting
// levels, so the world has real discoverable, raidable, growing AI-run
// outposts instead of the single difficulty-matched "Rogue Drone Nest"
// dummy target Phase 0-5 relied on. The Rogue Drone Nest itself is left
// completely untouched as a fallback for a continent with no other
// outpost at all (see resolveExplorationDiscovery in internal/engine/tick
// /engine.go).
func seedAICivilizations(db *sql.DB) {
	ctx := context.Background()

	seeds := []aiFactionSeed{
		{-900001, "ai_ironclad_directive", "Ironclad Directive", "Africa", 6},
		{-900002, "ai_sandrunner_clan", "Sandrunner Clan", "Africa", 3},
		{-900003, "ai_veridian_compact", "Veridian Compact", "Europe", 6},
		{-900004, "ai_ashfall_remnant", "Ashfall Remnant", "Europe", 3},
		{-900005, "ai_lotus_dominion", "Lotus Dominion", "Asia", 6},
		{-900006, "ai_crimson_tide_syndicate", "Crimson Tide Syndicate", "Asia", 3},
		{-900007, "ai_frontier_collective", "Frontier Collective", "Americas", 6},
		{-900008, "ai_dustbowl_militia", "Dustbowl Militia", "Americas", 3},
	}

	rGen := rand.New(rand.NewSource(time.Now().UnixNano()))

	for _, s := range seeds {
		var existing string
		err := db.QueryRowContext(ctx, "SELECT id FROM encampments WHERE ai_faction_key = $1", s.key).Scan(&existing)
		if err == nil {
			continue // already seeded
		}
		if err != sql.ErrNoRows {
			log.Printf("AI civilization seed check failed for %s: %v", s.key, err)
			continue
		}

		if _, err := db.ExecContext(ctx, `
			INSERT INTO users (telegram_id, username, first_name, state, faction)
			VALUES ($1, $2, $2, 'ai_faction', 'ai')
			ON CONFLICT (telegram_id) DO NOTHING`, s.telegramID, s.name); err != nil {
			log.Printf("AI civilization user seed failed for %s: %v", s.key, err)
			continue
		}

		var x, y int
		var coordID string
		placed := false
		for attempt := 0; attempt < 100; attempt++ {
			switch s.region {
			case "Africa":
				x = rGen.Intn(991) + 10
				y = rGen.Intn(991) + 10
			case "Europe":
				x = -(rGen.Intn(991) + 10)
				y = rGen.Intn(991) + 10
			case "Asia":
				x = rGen.Intn(991) + 10
				y = -(rGen.Intn(991) + 10)
			default:
				x = -(rGen.Intn(991) + 10)
				y = -(rGen.Intn(991) + 10)
			}
			err := db.QueryRowContext(ctx, `
				INSERT INTO coordinates (x, y, biome, danger_level, region, terrain)
				VALUES ($1, $2, 'wasteland', $3, $4, 'plains')
				ON CONFLICT (x, y) DO NOTHING
				RETURNING id`, x, y, s.level, s.region).Scan(&coordID)
			if err == nil {
				placed = true
				break
			}
		}
		if !placed {
			log.Printf("AI civilization %s could not find a free coordinate after 100 attempts", s.key)
			continue
		}

		var campID string
		if err := db.QueryRowContext(ctx, `
			INSERT INTO encampments (user_id, name, coordinate_id, level, is_ai_faction, ai_faction_key)
			VALUES ($1, $2, $3, $4, TRUE, $5)
			RETURNING id`, s.telegramID, s.name, coordID, s.level, s.key).Scan(&campID); err != nil {
			log.Printf("AI civilization encampment seed failed for %s: %v", s.key, err)
			continue
		}

		startScrap := float64(s.level) * 400.0
		startMetal := float64(s.level) * 200.0
		startRations := float64(s.level) * 150.0
		startElectricity := float64(s.level) * 100.0
		_, _ = db.ExecContext(ctx, `
			INSERT INTO resources (encampment_id, scrap, metal, rations, electricity, crystal)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			campID, startScrap, startMetal, startRations, startElectricity, float64(s.level)*0.5)

		startSoldiers := s.level * 15
		startMechs := s.level * 2
		_, _ = db.ExecContext(ctx, `
			INSERT INTO workshop_inventory (encampment_id, soldiers, mechs, buggies)
			VALUES ($1, $2, $3, 1)`, campID, startSoldiers, startMechs)

		log.Printf("Seeded AI civilization [%s] (%s, level %d) at [%d,%d]", s.name, s.region, s.level, x, y)
	}
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
	// Registered once at startup rather than via lib/pq's package-level
	// init(), because it wraps lib/pq's own "postgres" driver — see
	// internal/dbdriver for why (Supabase's PgBouncer transaction-mode
	// pooler occasionally hands the second half of a parameterized
	// query to a different backend than the one that parsed it; this
	// driver retries transparently instead of surfacing that as a
	// player-facing "temporarily unavailable" error).
	const dbDriverName = "postgres-retry"
	dbdriver.Register(dbDriverName)
	db, err := sql.Open(dbDriverName, dbURL)
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
	relocateZeroCoordinates(db)
	seedAICivilizations(db)

	pref := telebot.Settings{
		Token:  botToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := telebot.NewBot(pref)
	if err != nil {
		log.Fatalf("Fatal: Telegram API initialization failure: %v", err)
	}
	log.Printf("Telegram credentials accepted. Bot logged in as: @%s", bot.Me.Username)

	// Global panic recovery. Without this, any single unhandled panic in
	// any handler - an index-out-of-range on c.Args() (many call sites
	// across this codebase index into it without a preceding length
	// check, relying on the button that triggers them always supplying
	// enough args - true for normal play, but not a guarantee telebot's
	// dispatch loop itself enforces), a nil pointer dereference, etc. -
	// crashes the ENTIRE bot process, disconnecting every player, not
	// just failing the one interaction that triggered it. telebot has no
	// built-in recovery of its own; this wraps every handler invocation.
	// The custom callback both logs with real context (not just the bare
	// error middleware.Recover's default OnError would produce) and
	// tries to leave the player with a clear response instead of a
	// button that spins forever or a message that silently never arrives.
	bot.Use(middleware.Recover(handleRecoveredPanic))

	tickEngine := tick.NewEngine(db, time.Duration(tickSeconds)*time.Second)
	tickEngine.Start()

	realtimeListener := realtime.NewListener(dbURL, db, bot)
	realtimeListener.Start()

	// INSTANTIATE AND START THE BACKUP NOTIFICATION DISPATCHER
	notificationDispatcher := notifications.NewDispatcher(db, bot)
	notificationDispatcher.Start()

	onboarding := handlers.NewOnboardingHandler(db)
	camp := handlers.NewCampHandler(db, adminIDs)
	combat := handlers.NewCombatHandler(db, adminIDs)
	agentH := handlers.NewAgentHandler(db, adminIDs)
	admin := handlers.NewAdminHandler(db, tickEngine, adminIDs)
	hero := handlers.NewHeroHandler(db)
	world := handlers.NewWorldHandler(db)
	exchange := handlers.NewExchangeHandler(db)
	econ := handlers.NewEconomyHandler(db, exchange)
	clan := handlers.NewClanHandler(db)
	factory := handlers.NewFactoryHandler(db)
	arena := handlers.NewArenaHandler(db)
	silo := handlers.NewSiloHandler(db)
	research := handlers.NewResearchHandler(db)
	deconstruct := handlers.NewDeconstructHandler(db)
	ranking := handlers.NewRankingHandler(db)
	boss := handlers.NewBossHandler(db)
	rebellion := handlers.NewRebellionHandler(db)
	federation := handlers.NewFederationHandler(db)
	profile := handlers.NewProfileHandler(db, admin.AdminIDs)
	changelog := handlers.NewChangelogHandler(db, admin.AdminIDs)
	ether := handlers.NewEtherHandler(db)
	jobs := handlers.NewJobsHandler(db)
	exploration := handlers.NewExplorationHandler(db)
	scoutMissions := handlers.NewScoutMissionsHandler(db)
	diplomacy := handlers.NewDiplomacyHandler(db)

	// --- AI Foundation wiring (Phase A, independent AI roadmap branch) ---
	// Provider-agnostic by design: register additional providers here
	// (OpenAI, Gemini, Qwen, Grok, DeepSeek, Ollama, ...) without
	// touching internal/ai itself. Mock is always registered last so
	// the bot degrades gracefully instead of failing when no real
	// provider key is configured.
	aiConfig := ai.LoadConfig()
	aiRegistry := ai.NewRegistry()
	aiRegistry.Register(anthropic.New(aiConfig.AnthropicAPIKey, aiConfig.AnthropicModel))
	aiRegistry.Register(openaicompat.New("openai", "https://api.openai.com/v1", aiConfig.OpenAIAPIKey, aiConfig.OpenAIModel, true))
	aiRegistry.Register(openaicompat.New("deepseek", "https://api.deepseek.com/v1", aiConfig.DeepSeekAPIKey, aiConfig.DeepSeekModel, true))
	aiRegistry.Register(openaicompat.New("qwen", aiConfig.QwenBaseURL, aiConfig.QwenAPIKey, aiConfig.QwenModel, true))
	aiRegistry.Register(openaicompat.New("grok", "https://api.x.ai/v1", aiConfig.GrokAPIKey, aiConfig.GrokModel, true))
	aiRegistry.Register(gemini.New(aiConfig.GeminiAPIKey, aiConfig.GeminiModel, aiConfig.GeminiModelFallbacks))
	aiRegistry.Register(ollama.New(aiConfig.OllamaBaseURL, aiConfig.OllamaModel))
	aiRegistry.Register(mock.New())
	aiCostTracker := ai.NewPostgresCostTracker(db)
	aiPermissions := ai.NewPermissionManager(db)
	aiMemory := ai.NewPostgresMemoryStore(db)
	aiService := ai.NewService(aiConfig, aiRegistry, aiCostTracker, aiPermissions, aiMemory)
	aiStatus := handlers.NewAIStatusHandler(aiService, adminIDs)
	log.Printf("AI Foundation initialized. Default provider: %s | Fallback order: %v | Enabled: %v",
		aiConfig.DefaultProvider, aiConfig.FallbackOrder, aiConfig.Enabled)

	// --- AI Planet Governor wiring (Phase B, independent AI roadmap branch) ---
	aiGovernor := governor.New(db, aiService)
	governorHandler := handlers.NewGovernorHandler(aiGovernor)

	// --- AI Fleet Commander wiring (Phase C, independent AI roadmap branch) ---
	aiFleetCommander := fleetcommander.New(db, aiService)
	fleetCommanderHandler := handlers.NewFleetCommanderHandler(aiFleetCommander)

	// --- AI Economy Advisor wiring (Phase D, independent AI roadmap branch) ---
	aiEconAdvisor := econadvisor.New(db, aiService)
	econAdvisorHandler := handlers.NewEconomyAdvisorHandler(aiEconAdvisor)

	// --- AI Research Planner wiring (Phase E, independent AI roadmap branch) ---
	aiResearchPlanner := researchplanner.New(db, aiService)
	researchPlannerHandler := handlers.NewResearchPlannerHandler(aiResearchPlanner)

	// --- AI Battle Analyst wiring (Phase F, independent AI roadmap branch) ---
	aiBattleAnalyst := battleanalyst.New(db, aiService)
	battleAnalystHandler := handlers.NewBattleAnalystHandler(aiBattleAnalyst)

	// --- AI Guild Assistant wiring (Phase G, independent AI roadmap branch) ---
	aiGuildAssistant := guildassistant.New(db, aiService)
	guildAssistantHandler := handlers.NewGuildAssistantHandler(aiGuildAssistant)

	// --- AI Dynamic Galaxy wiring (Phase H, independent AI roadmap branch) ---
	aiGalaxyAdvisor := galaxyadvisor.New(db, aiService)
	galaxyAdvisorHandler := handlers.NewGalaxyAdvisorHandler(aiGalaxyAdvisor)

	// --- AI NPC Intelligence wiring (Phase I, independent AI roadmap branch) ---
	aiNPCIntel := npcintel.New(db, aiService)
	npcIntelHandler := handlers.NewNPCIntelHandler(aiNPCIntel)
	advisorsHandler := handlers.NewAdvisorsHandler(db)

	// --- AI Developer Console wiring (Phase J, independent AI roadmap branch) ---
	// Reuses admin.AdminIDs (already parsed above) rather than re-parsing
	// ADMIN_IDS a second time.
	aiDevConsole := devconsole.New(db, aiService)
	devConsoleHandler := handlers.NewDevConsoleHandler(aiDevConsole, admin.AdminIDs)

	// --- AI Command Interpreter wiring (Milestone 3, FEEDBACK_CHANGELOG_NLP_PLAN.md) ---
	// Constructed after aiService (unlike every handler above, nlp
	// used to be built before the AI Foundation section existed) so
	// it can be handed a real interpreter. See internal/game/nlpcommand
	// for the safety design - this is the only AI feature in the
	// codebase whose parsed output can directly trigger a game action.
	aiCommandInterpreter := nlpcommand.New(aiService)
	nlp := handlers.NewNLPHandler(onboarding, camp, combat, econ, clan, hero, agentH, factory, silo, research, exchange, world, scoutMissions, aiCommandInterpreter)

	bot.Handle("/start", onboarding.HandleStart)
	bot.Handle("/name", onboarding.HandleRenameOutpost)
	bot.Handle("/camp", camp.HandleCamp)
	bot.Handle("/warehouse", econ.HandleWarehouseReserves)
	bot.Handle("/raid", combat.HandleRaidBoard)
	bot.Handle("/agent", agentH.HandleAgent)
	bot.Handle("/hero", hero.HandleHeroPanel)
	bot.Handle("/world", world.HandleWorldFeed)
	bot.Handle("/econ", econ.HandleEconPanel)
	bot.Handle("/clan", clan.HandleClanPanel)
	bot.Handle("/clans", clan.HandleBrowseClans)
	bot.Handle("/clan_create", clan.HandleCreateClanCommand)
	bot.Handle("/clan_rename", clan.HandleRenameClanCommand)
	bot.Handle("/guild_missions", clan.HandleGuildMissions)
	bot.Handle("/guildmsg", clan.HandleGuildMsg)
	bot.Handle("/guild_icon", clan.HandleGuildIcon)
	bot.Handle("/guild_description", clan.HandleGuildDescription)
	bot.Handle("/board", clan.HandleBoard)
	bot.Handle("📋 Clan Board", clan.HandleBoard)
	bot.Handle("/scout", combat.HandleScout)
	bot.Handle("/factory", factory.HandleFactoryPanel)
	bot.Handle("/map", world.HandleSectorMap)
	bot.Handle("🗺️ Sector Map", world.HandleSectorMap)
	bot.Handle("/help", onboarding.HandleHelp)
	bot.Handle("/guide", onboarding.HandleGuide)
	bot.Handle("/inventory", econ.HandleWarehouseReserves)
	bot.Handle("/admin", admin.HandleAdminPanel)
	bot.Handle("/arena", arena.HandleArenaPanel)
	bot.Handle("/broadcast", world.HandleSectorBroadcast)
	bot.Handle("/mutations", camp.HandleMutationsPanel)
	bot.Handle("/silo", silo.HandleSiloPanel)
	bot.Handle("/mine", camp.HandleActiveMining)
	bot.Handle("/research", research.HandleResearchPanel)
	bot.Handle("/deconstruct", deconstruct.HandleDeconstructCommand)
	bot.Handle("/add", combat.HandleAddDraftCommand)
	bot.Handle("/remove", combat.HandleRemoveDraftCommand)
	bot.Handle("/defense", camp.HandleDefenseGridPanel)
	bot.Handle("/infrastructure", camp.HandleInfrastructureGridPanel)
	bot.Handle("/ranking", ranking.HandleRankingPanel)
	bot.Handle("/bosses", boss.HandleBossPanel)
	bot.Handle("/autoscan", combat.HandleAutoScanToggle)
	bot.Handle("🔄 Toggle Auto-Scan", combat.HandleAutoScanToggle)
	bot.Handle("/rebellion", rebellion.HandleRebellionPanel)
	bot.Handle("/federations", federation.HandleFederationsPanel)
	bot.Handle("\fopen_federations_browse", federation.HandleFederationsPanel)
	bot.Handle("/federation", federation.HandleMyFederationPanel)
	bot.Handle("\fopen_federation", federation.HandleMyFederationPanel)
	bot.Handle("/fed_found", federation.HandleFoundFederation)
	bot.Handle("/fed_join", federation.HandleJoinFederation)
	bot.Handle("/fed_leave", federation.HandleLeaveFederation)
	bot.Handle("\fleave_federation", federation.HandleLeaveFederation)
	bot.Handle("/explore", exploration.HandleExplorePanel)
	bot.Handle("/scoutmission", scoutMissions.HandleDispatchScoutMission)
	bot.Handle("/scoutstatus", scoutMissions.HandleScoutStatus)
	bot.Handle("/diplomacy", diplomacy.HandleDiplomacyPanel)
	bot.Handle("\fopen_diplomacy", diplomacy.HandleDiplomacyPanel)
	bot.Handle("/ally", diplomacy.HandleProposeAlliance)
	bot.Handle("/nap", diplomacy.HandleProposeNAP)
	bot.Handle("/break_pact", diplomacy.HandleBreakPact)
	bot.Handle("/description", profile.HandleDescription)
	bot.Handle("/settings", profile.HandleSettings)
	bot.Handle("/refer", profile.HandleRefer)
	bot.Handle("/feedback", profile.HandleFeedback)
	bot.Handle("💬 Send Feedback", profile.HandleFeedbackButton)
	bot.Handle("/feedback_inbox", profile.HandleFeedbackInbox)
	bot.Handle("/changelog", changelog.HandleChangelogPanel)
	bot.Handle("🗞️ Changelog", changelog.HandleChangelogPanel)
	bot.Handle("\fchangelog_more", changelog.HandleChangelogMoreCallback)
	bot.Handle("/msg", profile.HandleMsg)
	bot.Handle("/mute", profile.HandleMute)
	bot.Handle("/unmute", profile.HandleUnmute)
	bot.Handle("/mutes", profile.HandleMutesList)
	bot.Handle("🔇 Muted Players", profile.HandleMutesList)
	bot.Handle("/log", profile.HandleLog)
	bot.Handle("/stats", profile.HandleStats)
	bot.Handle("/units", profile.HandleUnits)
	bot.Handle("/ether", ether.HandleEtherShop)
	bot.Handle("🛒 Ether Shop", ether.HandleEtherShop)
	bot.Handle("/missions", profile.HandleMissions)
	bot.Handle("/destinations", profile.HandleDestinations)
	bot.Handle("/newjobhyperspeed", jobs.HandleHyperSpeed)
	bot.Handle("/newjobextendplanet", jobs.HandleExtendPlanet)
	bot.Handle("/newjobteleport", jobs.HandleTeleport)
	bot.Handle("/ghostprotocol", jobs.HandleGhostProtocol)
	bot.Handle("/newjoborbitalmaneuver", jobs.HandleOrbitalManeuver)
	bot.Handle("/newjobrepairunits", jobs.HandleRepairUnits)
	bot.Handle("/newjobrepairbuildings", jobs.HandleRepairBuildings)
	bot.Handle("/newjobgathersunlight", jobs.HandleGatherSunlight)
	bot.Handle("/newjobmanualscan", jobs.HandleManualScanAlias)
	bot.Handle("/newjobautoscan", jobs.HandleAutoScanAlias)
	bot.Handle("/newjobadvancedscan", jobs.HandleAdvancedScanAlias)
	bot.Handle("/newjobpublishtrade", jobs.HandlePublishTradeAlias)

	// Jobs section buttons (mother/child keyboard pair, see
	// internal/bot/keyboards/navigation.go's JobsNavigation doc comment -
	// this is purely additive discoverability on top of the slash
	// commands above, which keep working unchanged).
	bot.Handle("🛠️ Odd Jobs", jobs.HandleJobsPanel)
	bot.Handle("🚀 HyperSpeed Mission", jobs.HandleHyperSpeed)
	bot.Handle("🌍 Extend Planet", jobs.HandleExtendPlanet)
	bot.Handle("🌀 Teleport Outpost", jobs.HandleTeleport)
	bot.Handle("👻 Ghost Protocol", jobs.HandleGhostProtocol)
	bot.Handle("🛰️ Orbital Maneuver", jobs.HandleOrbitalManeuver)
	bot.Handle("🔧 Repair Units", jobs.HandleRepairUnits)
	bot.Handle("🏚️ Repair Buildings", jobs.HandleRepairBuildings)
	bot.Handle("☀️ Gather Sunlight", jobs.HandleGatherSunlight)

	// AI Advisors section buttons (mother/child keyboard pair, matching
	// the Jobs pattern above) - purely additive discoverability on top of
	// the slash commands already registered elsewhere for these same
	// handlers, which keep working unchanged.
	bot.Handle("🎓 AI Advisors", advisorsHandler.HandleAdvisorsPanel)
	bot.Handle("⚔️ Battle Analyst", battleAnalystHandler.HandleBattleAnalyst)
	bot.Handle("💹 Economy Advisor", econAdvisorHandler.HandleEconomyAdvisor)
	bot.Handle("🌌 Galaxy Advisor", galaxyAdvisorHandler.HandleGalaxyAdvisor)
	bot.Handle("🤝 Guild Assistant", guildAssistantHandler.HandleGuildAssistant)
	bot.Handle("🔬 Research Planner", researchPlannerHandler.HandleResearchPlanner)
	bot.Handle("🚀 Fleet Commander", fleetCommanderHandler.HandleFleetCommander)
	bot.Handle("🛰️ NPC Intel", npcIntelHandler.HandleNPCIntel)
	bot.Handle("🏛️ Governor", governorHandler.HandleGovernor)

	// Player Profile section buttons (mother/child keyboard pair) -
	// notably gives /settings its first button access ever, despite it
	// being where the route_status notification mute toggle actually lives.
	bot.Handle("📊 Player Profile", profile.HandleProfilePanel)
	bot.Handle("📈 Server Stats", profile.HandleStats)
	bot.Handle("🪖 My Units", profile.HandleUnits)
	bot.Handle("📜 My Missions", profile.HandleMissions)
	bot.Handle("🗺️ My Destinations", profile.HandleDestinations)
	bot.Handle("📰 Event Log", profile.HandleLog)
	bot.Handle("⚙️ Settings", profile.HandleSettings)
	bot.Handle("📖 Player Guide", onboarding.HandleGuide)

	bot.Handle("/ai_status", aiStatus.HandleAIStatus)
	bot.Handle("🤖 AI Status", aiStatus.HandleAIStatus)
	bot.Handle("/ai_status_toggle", aiStatus.HandleAIStatusToggle)
	bot.Handle("/ai_probe", aiStatus.HandleAIProbe)
	bot.Handle("/ai_settings", aiStatus.HandleAISettings)
	bot.Handle("🤖 AI Settings", aiStatus.HandleAISettings)
	bot.Handle("/governor", governorHandler.HandleGovernor)
	bot.Handle("/governor_autopilot", governorHandler.HandleGovernorAutopilot)
	bot.Handle("\fgov_refresh", governorHandler.HandleGovernorRefreshCallback)
	bot.Handle("\fgov_toggle_autopilot", governorHandler.HandleGovernorToggleCallback)
	bot.Handle("/fleet_commander", fleetCommanderHandler.HandleFleetCommander)
	bot.Handle("\ffleet_refresh", fleetCommanderHandler.HandleFleetCommanderRefreshCallback)
	bot.Handle("/economy_advisor", econAdvisorHandler.HandleEconomyAdvisor)
	bot.Handle("\fecon_refresh", econAdvisorHandler.HandleEconomyAdvisorRefreshCallback)
	bot.Handle("/research_planner", researchPlannerHandler.HandleResearchPlanner)
	bot.Handle("\fresearch_refresh", researchPlannerHandler.HandleResearchPlannerRefreshCallback)
	bot.Handle("\fresearch_goal", researchPlannerHandler.HandleResearchPlannerGoalCallback)
	bot.Handle("/battle_analyst", battleAnalystHandler.HandleBattleAnalyst)
	bot.Handle("\fbattle_analyst_refresh", battleAnalystHandler.HandleBattleAnalystRefreshCallback)
	bot.Handle("/guild_assistant", guildAssistantHandler.HandleGuildAssistant)
	bot.Handle("\fguild_assistant_refresh", guildAssistantHandler.HandleGuildAssistantRefreshCallback)
	bot.Handle("/galaxy_advisor", galaxyAdvisorHandler.HandleGalaxyAdvisor)
	bot.Handle("\fgalaxy_advisor_refresh", galaxyAdvisorHandler.HandleGalaxyAdvisorRefreshCallback)
	bot.Handle("/npc_intel", npcIntelHandler.HandleNPCIntel)
	bot.Handle("\fnpc_intel_refresh", npcIntelHandler.HandleNPCIntelRefreshCallback)
	bot.Handle("\fnlp_c", nlp.HandleNLPConfirmCallback)
	bot.Handle("\fnlp_x", nlp.HandleNLPCancelCallback)
	bot.Handle("/weekly_report", devConsoleHandler.HandleWeeklyReport)
	bot.Handle("📅 Weekly Report", devConsoleHandler.HandleWeeklyReport)
	bot.Handle("\fdev_console_refresh", devConsoleHandler.HandleDevConsoleRefreshCallback)
	bot.Handle("/admin_ask", devConsoleHandler.HandleAdminAsk)
	bot.Handle("/balance_report", devConsoleHandler.HandleBalanceReport)
	bot.Handle("⚖️ Balance Report", devConsoleHandler.HandleBalanceReport)
	bot.Handle("\fbalance_report_refresh", devConsoleHandler.HandleBalanceReportRefreshCallback)
	bot.Handle("👹 World Bosses", boss.HandleBossPanel)
	bot.Handle("✊ The Rebellion", rebellion.HandleRebellionPanel)
	bot.Handle("/settaxrate", admin.HandleAdminSetTaxRate)
	bot.Handle("🏆 Global Ranking", ranking.HandleRankingPanel)

	bot.Handle("/admin_tick", admin.HandleAdminTick)
	bot.Handle("/admin_db_reset", admin.HandleAdminDBReset)
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
	bot.Handle("🧭 World Exploration", exploration.HandleExplorePanel)
	bot.Handle("🔭 Long-Range Scouting", scoutMissions.HandleScoutPanel)
	bot.Handle("☢️ Strategic Silo", silo.HandleSiloPanel)
	bot.Handle("💱 Market Exchange", exchange.HandleExchangePanel)
	bot.Handle("🪖 Recruit Troops", factory.HandleRecruitPanel)
	bot.Handle("🚗 Logistics Vehicles", factory.HandleVehiclesPanel)
	bot.Handle("♻️ Deconstruct Units", deconstruct.HandleDeconstructPanel)
	bot.Handle("🛡️ Defense Grid", camp.HandleDefenseGridPanel)
	bot.Handle("🏗️ Infrastructure Grid", camp.HandleInfrastructureGridPanel)
	bot.Handle("⬅️ Back to HQ", onboarding.HandleStart)

	// Phase 7 (item 13): admin panel consolidation's guided-input flow.
	// If the sender is an admin mid-flow on a button that needs a
	// free-text argument (e.g. "Gift Premium"), consume this message as
	// that argument instead of falling through to normal NLP parsing.
	// Everyone else (and any admin with no pending action) is completely
	// unaffected - HandleAdminPendingInput returns handled=false
	// immediately for them.
	bot.Handle(telebot.OnText, func(c telebot.Context) error {
		if handled, err := onboarding.HandleOnboardingPendingInput(c); handled {
			return err
		}
		if handled, err := profile.HandleFeedbackPendingInput(c); handled {
			return err
		}
		if handled, err := admin.HandleAdminPendingInput(c); handled {
			return err
		}
		return nlp.HandleTextMessage(c)
	})

	bot.Handle("\fupgrade_mod", camp.HandleUpgradeCallback)
	bot.Handle("\flaunch_raid", combat.HandleLaunchRaidCallback)
	bot.Handle("\ftoggle_agent", agentH.HandleToggleAgentCallback)
	bot.Handle("\fset_agent_mode", agentH.HandleSetModeCallback)
	bot.Handle("\fjoin_faction", onboarding.HandleFactionCallback)

	// New-survivor welcome message quick-action buttons. view_warehouse
	// and view_manual are already registered further down (pre-existing
	// dashboard buttons); only open_agent/open_refer are new here.
	bot.Handle("\fopen_agent", func(c telebot.Context) error {
		_ = c.Respond(&telebot.CallbackResponse{})
		return agentH.HandleAgent(c)
	})
	bot.Handle("\fopen_refer", func(c telebot.Context) error {
		_ = c.Respond(&telebot.CallbackResponse{})
		return profile.HandleRefer(c)
	})

	bot.Handle("\fbank_action", econ.HandleBankCallback)
	bot.Handle("\fmarket_buy", econ.HandleMarketCallback)
	bot.Handle("\fbrowse_clans", clan.HandleBrowseClans)
	// BUGFIX: these two dashboard buttons (onboarding.go's returning-user
	// panel) had no registered handler at all - tapping them did
	// nothing, since telebot silently drops a callback with no matching
	// bot.Handle. Reusing the existing HandleWarehouseReserves/HandleHelp
	// panels, same "registered as both a command and a callback, no
	// c.Respond needed" pattern browse_clans above already uses.
	bot.Handle("\fview_warehouse", econ.HandleWarehouseReserves)
	bot.Handle("\fview_manual", onboarding.HandleHelp)
	bot.Handle("\fexplore_dispatch", exploration.HandleDispatchExpeditionCallback)
	bot.Handle("\fscout_dispatch", scoutMissions.HandleScoutDispatchCallback)
	bot.Handle("\fdiplo_respond", diplomacy.HandleDiplomacyRespondCallback)
	bot.Handle("\fclan_apply", clan.HandleApplyToClanCallback)
	bot.Handle("\fclan_apps", clan.HandleApplicationsInboxCallback)
	bot.Handle("\fcl_acc", clan.HandleAcceptApplicationCallback)
	bot.Handle("\fcl_rej", clan.HandleRejectApplicationCallback)
	bot.Handle("\fleave_clan", clan.HandleLeaveClanCallback)
	bot.Handle("\fdeclare_clan_war", clan.HandleDeclareClanWarCallback)
	bot.Handle("\fexp_action", combat.HandleExpeditionActions)
	bot.Handle("\froad_encounter", combat.HandleRoadEncounterCallback)
	bot.Handle("\frbe", combat.HandleRoadBaseEncounterCallback)
	bot.Handle("\fdispatch_convoy", combat.HandleDispatchConvoy)
	bot.Handle("\fconvoy_cfg", combat.HandleConvoyConfigPanel)
	bot.Handle("\fconvoy_cancel", combat.HandleConvoyCancel)
	bot.Handle("\fcraft_item", factory.HandleCraftCallback)
	bot.Handle("\fcraft_qty", factory.HandleCraftQuantityCallback)
	bot.Handle("\fdeconstruct_item", deconstruct.HandleDeconstructCallback)
	bot.Handle("\fattack_boss", boss.HandleAttackBossCallback)
	bot.Handle("\frebellion_donate", rebellion.HandleRebellionDonateCallback)
	bot.Handle("\ftrade_hub_nav", econ.HandleTradeHubNavCallback)
	bot.Handle("\fcrystal_exchange", econ.HandleCrystalExchangeCallback)
	bot.Handle("\frecon_ai", combat.HandleReconAICallback)
	bot.Handle("\fsettings_toggle", profile.HandleSettingsToggleCallback)
	bot.Handle("\fether_convert", ether.HandleEtherConvertCallback)
	bot.Handle("\fspy_action", combat.HandleSpyCallback)
	bot.Handle("\fupgrade_tech", research.HandleUpgradeTechCallback)
	bot.Handle("\fpost_listing", exchange.HandlePostListingCallback)
	bot.Handle("\fbuy_listing", exchange.HandleBuyListingCallback)
	bot.Handle("\fmutate_mod", camp.HandleMutationCallback)
	bot.Handle("\fjoin_queue", arena.HandleJoinQueueCallback)
	bot.Handle("\flaunch_icbm", silo.HandleLaunchICBMCallback)
	bot.Handle("\flaunch_piercing", silo.HandleLaunchPiercingMissileCallback)
	bot.Handle("\fmine_action", camp.HandleMineCallback)
	bot.Handle("\fhero_action", hero.HandleHeroCallback)
	bot.Handle("\fgarrison_adjust", hero.HandleGarrisonAdjustCallback)
	bot.Handle("\flaunch_interceptor", combat.HandleLaunchInterceptor)
	bot.Handle("\fadmin_action", admin.HandleAdminActionCallback)
	bot.Handle("\fstage_coop", combat.HandleStageCoopCallback)
	bot.Handle("\fjoin_coop", combat.HandleJoinCoopCallback)

	bot.Handle("\fclan_manage", clan.HandleManageMembersCallback)
	bot.Handle("\fclan_stats", clan.HandleAllianceStatsCallback)
	bot.Handle("\fclan_kick", clan.HandleKickMemberCallback)
	bot.Handle("\fclan_promote", clan.HandlePromoteMemberCallback)

	bot.Handle("\fconfirm_launch", combat.HandleConfirmHangarLaunchCallback)
	bot.Handle("\fadjust_draft", combat.HandleAdjustDraftCallback)

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
	notificationDispatcher.Stop() // Terminate dispatcher cleanly on shutdown
	db.Close()

	log.Println("System components cleanly dismantled. Server offline.")
}

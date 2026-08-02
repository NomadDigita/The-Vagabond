package keyboards

import (
	"gopkg.in/telebot.v3"
)

// MainNavigation builds the primary bottom layout.
func MainNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnHQ := menu.Text("📡 Terminal HQ")
	btnCamp := menu.Text("⛺ Outpost Camp")
	btnCombat := menu.Text("⚔️ Tactical Combat")
	btnEcon := menu.Text("🏦 System Economy")
	btnFactory := menu.Text("🏭 Heavy Workshop")
	btnRanking := menu.Text("🏆 Global Ranking")
	btnBosses := menu.Text("👹 World Bosses")
	btnRebellion := menu.Text("✊ The Rebellion")
	btnJobs := menu.Text("🛠️ Odd Jobs")
	btnAdvisors := menu.Text("🎓 AI Advisors")
	btnProfile := menu.Text("📊 Player Profile")
	btnAdmin := menu.Text("🏛️ Admin Terminal")

	menu.Reply(
		menu.Row(btnHQ, btnCamp),
		menu.Row(btnCombat, btnEcon),
		menu.Row(btnFactory, btnRanking),
		menu.Row(btnBosses, btnRebellion),
		menu.Row(btnJobs, btnAdvisors),
		menu.Row(btnProfile, btnAdmin),
	)

	return menu
}

// CampNavigation builds the custom contextual submenu for Encampments.
func CampNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnUpgrade := menu.Text("🔨 Structural Upgrades")
	btnHero := menu.Text("👥 Hero Commander")
	btnAgent := menu.Text("🧠 Automation Agent")
	btnMutation := menu.Text("🧬 Mutation Core")
	btnMine := menu.Text("⛏️ Active Mining")
	btnResearch := menu.Text("🧪 Research Lab") // Added for Research Lab access
	btnDefense := menu.Text("🛡️ Defense Grid")
	btnInfra := menu.Text("🏗️ Infrastructure Grid")
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnUpgrade, btnHero),
		menu.Row(btnAgent, btnMutation),
		menu.Row(btnMine, btnResearch),
		menu.Row(btnDefense, btnInfra),
		menu.Row(btnBack),
	)

	return menu
}

// CombatNavigation builds the custom submenu for raids and wasteland feeds.
func CombatNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnScan := menu.Text("🛰️ Scan Targets")
	btnRadar := menu.Text("🛸 Expedition Radar")
	btnAutoScan := menu.Text("🔄 Toggle Auto-Scan")
	btnNews := menu.Text("📻 Wasteland Radio")
	btnArena := menu.Text("🏟️ Combat Arena")
	btnExplore := menu.Text("🧭 World Exploration")
	btnScout := menu.Text("🔭 Long-Range Scouting")
	btnMap := menu.Text("🗺️ Sector Map")
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnScan, btnRadar),
		menu.Row(btnAutoScan, btnNews),
		menu.Row(btnArena, btnExplore),
		menu.Row(btnScout, btnMap),
		menu.Row(btnBack),
	)

	return menu
}

// EconomyNavigation builds the custom submenu for vault and clan alliances.
func EconomyNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnVault := menu.Text("🪙 Financial Vault")
	btnClan := menu.Text("🛡️ Clan Alliances")
	btnBoard := menu.Text("📋 Clan Board")
	btnExchange := menu.Text("💱 Market Exchange")
	btnEther := menu.Text("🛒 Ether Shop")
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnVault, btnClan),
		menu.Row(btnBoard, btnExchange),
		menu.Row(btnEther),
		menu.Row(btnBack),
	)

	return menu
}

// WorkshopNavigation builds the custom submenu for vehicles and troop forging.
func WorkshopNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnRecruit := menu.Text("🪖 Recruit Troops")
	btnVehicles := menu.Text("🚗 Logistics Vehicles")
	btnDeconstruct := menu.Text("♻️ Deconstruct Units")
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnRecruit, btnVehicles),
		menu.Row(btnDeconstruct),
		menu.Row(btnBack),
	)

	return menu
}

// JobsNavigation builds the custom submenu for one-off "Odd Jobs" utility
// actions (previously slash-command-only, with no button discoverability
// at all - every other section in this game has had a mother/child
// keyboard pair since the original navigation.go; Jobs was the one
// exception). Every button here fires its job directly rather than
// opening a further sub-panel, matching how a leaf action inside any
// other child menu (e.g. Admin's "⚡ Force Master Tick") behaves - no
// extra reply-keyboard replant happens on tap, so this same keyboard
// just stays on screen between jobs.
//
// Deliberately excludes jobs.go's four *Alias handlers
// (HandleManualScanAlias/HandleAutoScanAlias/HandleAdvancedScanAlias/
// HandlePublishTradeAlias) - those aren't real actions, just text
// pointers telling the player which *other* panel to use instead, and
// would read as broken buttons here ("tap this button to be told to go
// tap a different button elsewhere").
func JobsNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnHyperSpeed := menu.Text("🚀 HyperSpeed Mission")
	btnExtendPlanet := menu.Text("🌍 Extend Planet")
	btnTeleport := menu.Text("🌀 Teleport Outpost")
	btnGhostProtocol := menu.Text("👻 Ghost Protocol")
	btnOrbitalManeuver := menu.Text("🛰️ Orbital Maneuver")
	btnRepairUnits := menu.Text("🔧 Repair Units")
	btnRepairBuildings := menu.Text("🏚️ Repair Buildings")
	btnGatherSunlight := menu.Text("☀️ Gather Sunlight")
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnHyperSpeed, btnExtendPlanet),
		menu.Row(btnTeleport, btnGhostProtocol),
		menu.Row(btnOrbitalManeuver, btnRepairUnits),
		menu.Row(btnRepairBuildings, btnGatherSunlight),
		menu.Row(btnBack),
	)

	return menu
}

// AdvisorsNavigation builds the custom submenu for the AI Advisory Corps
// (PROJECT_MASTER_PLAN.md's Phase E-J roadmap - 8 distinct AI advisor
// personas that were entirely slash-command-only before this, with zero
// button discoverability anywhere in the game, not even from the
// thematically-adjacent "🧠 Automation Agent" panel). Same "leaf action,
// no further sub-panel" shape as JobsNavigation - every button here
// renders its advisor's full report directly.
func AdvisorsNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnBattleAnalyst := menu.Text("⚔️ Battle Analyst")
	btnEconomyAdvisor := menu.Text("💹 Economy Advisor")
	btnGalaxyAdvisor := menu.Text("🌌 Galaxy Advisor")
	btnGuildAssistant := menu.Text("🤝 Guild Assistant")
	btnResearchPlanner := menu.Text("🔬 Research Planner")
	btnFleetCommander := menu.Text("🚀 Fleet Commander")
	btnNPCIntel := menu.Text("🛰️ NPC Intel")
	btnGovernor := menu.Text("🏛️ Governor")
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnBattleAnalyst, btnEconomyAdvisor),
		menu.Row(btnGalaxyAdvisor, btnGuildAssistant),
		menu.Row(btnResearchPlanner, btnFleetCommander),
		menu.Row(btnNPCIntel, btnGovernor),
		menu.Row(btnBack),
	)

	return menu
}

// ProfileNavigation builds the custom submenu for player-facing info
// panels and settings. Every button here was slash-command-only before
// this - notably /settings itself, which had zero button access despite
// being where the route_status notification mute toggle actually lives.
func ProfileNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnStats := menu.Text("📈 Server Stats")
	btnUnits := menu.Text("🪖 My Units")
	btnMissions := menu.Text("📜 My Missions")
	btnDestinations := menu.Text("🗺️ My Destinations")
	btnLog := menu.Text("📰 Event Log")
	btnSettings := menu.Text("⚙️ Settings")
	btnAISettings := menu.Text("🤖 AI Settings")
	btnMutes := menu.Text("🔇 Muted Players")
	btnGuide := menu.Text("📖 Player Guide")
	btnFeedback := menu.Text("💬 Send Feedback")
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnStats, btnUnits),
		menu.Row(btnMissions, btnDestinations),
		menu.Row(btnLog, btnSettings),
		menu.Row(btnAISettings, btnMutes),
		menu.Row(btnGuide, btnFeedback),
		menu.Row(btnBack),
	)

	return menu
}

// AdminNavigation builds the custom submenu for administrator actions.
func AdminNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnTick := menu.Text("⚡ Force Master Tick")
	btnResources := menu.Text("🪙 Inject Resources")
	btnMetrics := menu.Text("🛰️ Server Metrics")
	btnBalance := menu.Text("⚖️ Balance Report")
	btnWeekly := menu.Text("📅 Weekly Report")
	btnAIStatus := menu.Text("🤖 AI Status")
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnTick, btnResources),
		menu.Row(btnMetrics, btnBalance),
		menu.Row(btnWeekly, btnAIStatus),
		menu.Row(btnBack),
	)

	return menu
}

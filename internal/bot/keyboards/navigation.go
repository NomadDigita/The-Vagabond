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
	btnAdmin := menu.Text("🏛️ Admin Terminal")

	menu.Reply(
		menu.Row(btnHQ, btnCamp),
		menu.Row(btnCombat, btnEcon),
		menu.Row(btnFactory, btnAdmin),
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
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnUpgrade, btnHero),
		menu.Row(btnAgent, btnMutation),
		menu.Row(btnMine, btnResearch),
		menu.Row(btnDefense),
		menu.Row(btnBack),
	)

	return menu
}

// CombatNavigation builds the custom submenu for raids and wasteland feeds.
func CombatNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnScan := menu.Text("🛰️ Scan Targets")
	btnRadar := menu.Text("🛸 Expedition Radar")
	btnNews := menu.Text("📻 Wasteland Radio")
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnScan, btnRadar),
		menu.Row(btnNews, btnBack),
	)

	return menu
}

// EconomyNavigation builds the custom submenu for vault and clan alliances.
func EconomyNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnVault := menu.Text("🪙 Financial Vault")
	btnClan := menu.Text("🛡️ Clan Alliances")
	btnExchange := menu.Text("💱 Market Exchange")
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnVault, btnClan),
		menu.Row(btnExchange, btnBack),
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

// AdminNavigation builds the custom submenu for administrator actions.
func AdminNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnTick := menu.Text("⚡ Force Master Tick")
	btnResources := menu.Text("🪙 Inject Resources")
	btnMetrics := menu.Text("🛰️ Server Metrics")
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnTick, btnResources),
		menu.Row(btnMetrics, btnBack),
	)

	return menu
}
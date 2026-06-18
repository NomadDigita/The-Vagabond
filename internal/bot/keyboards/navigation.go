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
	btnAdmin := menu.Text("🏛️ Admin Terminal")

	menu.Reply(
		menu.Row(btnHQ, btnCamp),
		menu.Row(btnCombat, btnEcon),
		menu.Row(btnAdmin),
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
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnUpgrade, btnHero),
		menu.Row(btnAgent, btnMutation),
		menu.Row(btnMine, btnBack),
	)

	return menu
}

// CombatNavigation builds the custom submenu for raids and wasteland feeds.
func CombatNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnScan := menu.Text("🛰️ Scan Targets")
	btnNews := menu.Text("📻 Wasteland Radio")
	btnRadar := menu.Text("🛸 Expedition Radar") // Integrated Radar HUD
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnScan, btnNews),
		menu.Row(btnRadar, btnBack),
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
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnRecruit, btnVehicles),
		menu.Row(btnBack),
	)

	return menu
}

// AdminNavigation builds the restricted console submenu for developers.
func AdminNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnTick := menu.Text("⚡ Force Master Tick")
	btnGive := menu.Text("🪙 Inject Resources")
	btnMetrics := menu.Text("🛰️ Server Metrics")
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnTick, btnGive),
		menu.Row(btnMetrics, btnBack),
	)

	return menu
}
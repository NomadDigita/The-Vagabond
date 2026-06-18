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
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnUpgrade, btnHero),
		menu.Row(btnAgent, btnMutation),
		menu.Row(btnBack),
	)

	return menu
}

// CombatNavigation builds the custom submenu for raids and wasteland feeds.
func CombatNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnScan := menu.Text("🛰️ Scan Targets")
	btnNews := menu.Text("📻 Wasteland Radio")
	btnSilo := menu.Text("☢️ Strategic Silo")
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnScan, btnNews),
		menu.Row(btnSilo, btnBack),
	)

	return menu
}

// EconomyNavigation builds the custom submenu for vault and clan alliances.
func EconomyNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnVault := menu.Text("🪙 Financial Vault")
	btnClan := menu.Text("🛡️ Clan Alliances")
	btnFactory := menu.Text("🏭 Heavy Workshop")
	btnExchange := menu.Text("💱 Market Exchange")
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnVault, btnClan),
		menu.Row(btnFactory, btnExchange),
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
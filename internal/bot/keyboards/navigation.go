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

	menu.Reply(
		menu.Row(btnHQ, btnCamp),
		menu.Row(btnCombat, btnEcon),
	)

	return menu
}

// CampNavigation builds the custom contextual submenu for Encampments.
func CampNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnUpgrade := menu.Text("🔨 Structural Upgrades")
	btnHero := menu.Text("👥 Hero Commander")
	btnAgent := menu.Text("🧠 Automation Agent")
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnUpgrade, btnHero),
		menu.Row(btnAgent, btnBack),
	)

	return menu
}

// CombatNavigation builds the custom submenu for raids and wasteland feeds.
func CombatNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnScan := menu.Text("🛰️ Scan Targets")
	btnNews := menu.Text("📻 Wasteland Radio")
	btnFactory := menu.Text("🏭 Heavy Workshop")
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnScan, btnNews),
		menu.Row(btnFactory, btnBack),
	)

	return menu
}

// EconomyNavigation builds the custom submenu for vault and clan alliances.
func EconomyNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnVault := menu.Text("🪙 Financial Vault")
	btnClan := menu.Text("🛡️ Clan Alliances")
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnVault, btnClan),
		menu.Row(btnBack),
	)

	return menu
}

package keyboards

import (
	"gopkg.in/telebot.v3"
)

// MainNavigation builds the primary bottom layout.
func MainNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnHQ := menu.Text("📡 Terminal HQ")
	btnCamp := menu.Text("⛺ Outpost Camp")
	btnRaid := menu.Text("⚔️ Raid Board")
	btnAgent := menu.Text("🧠 Automation Agent")

	menu.Reply(
		menu.Row(btnHQ, btnCamp),
		menu.Row(btnRaid, btnAgent),
	)

	return menu
}

// CampNavigation builds the custom contextual submenu for Encampments.
func CampNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{ResizeKeyboard: true}

	btnUpgrade := menu.Text("🔨 Structural Upgrades")
	btnBack := menu.Text("⬅️ Back to HQ")

	menu.Reply(
		menu.Row(btnUpgrade),
		menu.Row(btnBack),
	)

	return menu
}

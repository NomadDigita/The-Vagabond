package keyboards

import (
	"gopkg.in/telebot.v3"
)

// MainNavigation builds the persistent application layout dashboard buttons.
func MainNavigation() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{
		ResizeKeyboard: true,
	}

	btnHQ := menu.Text("📡 Terminal HQ")
	btnCamp := menu.Text("⛺ Outpost Camp")
	btnRaid := menu.Text("⚔️ Raid Missions")

	// Set layout grid rows
	menu.Reply(
		menu.Row(btnHQ, btnCamp),
		menu.Row(btnRaid),
	)

	return menu
}

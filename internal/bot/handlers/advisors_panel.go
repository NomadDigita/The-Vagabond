package handlers

import (
	"database/sql"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

type AdvisorsHandler struct {
	DB *sql.DB
}

func NewAdvisorsHandler(db *sql.DB) *AdvisorsHandler {
	return &AdvisorsHandler{DB: db}
}

func (h *AdvisorsHandler) HandleAdvisorsPanel(c telebot.Context) error {
	panelText := "🎓━━━━━━━━━━━━━━━━━━━━━━🎓\n" +
		htmlBold("AI ADVISORY CORPS") + "\n" +
		"🎓━━━━━━━━━━━━━━━━━━━━━━🎓\n\n" +
		"Strategic AI reports on demand - pick an advisor below.\n\n" +
		"⚔️ " + htmlBold("Battle Analyst") + " - reviews your recent raids, spots patterns in your losses/wins.\n" +
		"💹 " + htmlBold("Economy Advisor") + " - flags production bottlenecks and storage waste.\n" +
		"🌌 " + htmlBold("Galaxy Advisor") + " - surveys the wider map for opportunity/threat.\n" +
		"🤝 " + htmlBold("Guild Assistant") + " - clan-level coordination suggestions.\n" +
		"🔬 " + htmlBold("Research Planner") + " - what to research next, and why.\n" +
		"🚀 " + htmlBold("Fleet Commander") + " - fleet composition and readiness review.\n" +
		"🛰️ " + htmlBold("NPC Intel") + " - what's known about nearby AI factions.\n" +
		"🏛️ " + htmlBold("Governor") + " - big-picture outpost strategy. Autopilot mode is opt-in only via " + htmlCode("/governor_autopilot on") + " or " + htmlCode("/governor_autopilot off") + " - see the Governor report for what it does before enabling it.\n\n" +
		"🎓━━━━━━━━━━━━━━━━━━━━━━🎓"

	return sendPanelWithNavHTML(c, "🎓 Connecting to Advisory Corps...", keyboards.AdvisorsNavigation(), panelText, &telebot.ReplyMarkup{})
}

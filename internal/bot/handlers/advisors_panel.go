package handlers

import (
	"database/sql"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

// AdvisorsHandler is the "🎓 AI Advisors" mother-keyboard entry point.
// Every one of the 8 AI advisor personas (Battle Analyst, Economy
// Advisor, Galaxy Advisor, Guild Assistant, Research Planner, Fleet
// Commander, NPC Intel, Governor - PROJECT_MASTER_PLAN.md's Phase E-J
// roadmap) was slash-command-only before this, exactly the gap Jobs had
// before it got its own mother/child keyboard pair earlier this session -
// nothing in the game linked to any of them, not even the "🧠 Automation
// Agent" panel, which is a different feature (offline tick automation,
// not an AI advisor) that a player could easily mistake for covering
// this too.
//
// DB is unused today but kept on the struct (matching every other
// handler in this package) so a future "which advisors are premium-
// gated for you" summary doesn't need a signature change to add it.
type AdvisorsHandler struct {
	DB *sql.DB
}

func NewAdvisorsHandler(db *sql.DB) *AdvisorsHandler {
	return &AdvisorsHandler{DB: db}
}

// HandleAdvisorsPanel plants the AdvisorsNavigation child keyboard and
// shows a one-line summary of each advisor so a player can pick the
// right one without guessing from the button label alone. Every button
// on AdvisorsNavigation fires its advisor's report directly (same "leaf
// action, no further sub-panel" shape as JobsNavigation) - each advisor
// handler already renders its own full report with its own inline
// refresh/detail buttons.
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

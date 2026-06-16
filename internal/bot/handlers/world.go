package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

type WorldHandler struct {
	DB *sql.DB
}

func NewWorldHandler(db *sql.DB) *WorldHandler {
	return &WorldHandler{DB: db}
}

// HandleWorldFeed displays world news, coordinates biome multipliers, and weather telemetry
func (h *WorldHandler) HandleWorldFeed(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)

	ctx := context.Background()

	// Query player count
	var activeCamps int
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM encampments").Scan(&activeCamps)

	// Fetch dynamic clock variables
	currentTime := time.Now().UTC().Format("15:04:05")

	dashboard := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"📻 WASTELAND BROADCAST RADIO\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Universal Coordinate Time: [%s]\n"+
			"Sensors tracking: %d active encampment lines\n\n"+
			"DYNAMIC WEATHER FEED:\n"+
			"☣️ Core Zone: [Acid Dust Storms]\n"+
			"🌡️ Ambient Temperature: 38°C\n"+
			"📡 System Interference: Low\n\n"+
			"REGIONAL WORLD LOGS:\n"+
			"☠️ Sector West-14: Scavenger lines collapsed.\n"+
			"🛰️ Unknown frequency signals intercepted near grid [1,1].\n"+
			"⚙️ Scrap value spikes recorded in central markets.\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Listen to the static. The wastes are breathing.",
		currentTime, activeCamps,
	)

	return c.Send(dashboard, keyboards.CombatNavigation())
}

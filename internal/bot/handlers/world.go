package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
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

// HandleWorldFeed displays real-time database world news, coordinates biome multipliers, and weather telemetry
func (h *WorldHandler) HandleWorldFeed(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)

	ctx := context.Background()

	// Query player count
	var activeCamps int
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM encampments").Scan(&activeCamps)

	// Fetch dynamic clock variables
	currentTime := time.Now().UTC().Format("15:04:05")

	// Query most recent 5 world news headlines
	queryNews := `
		SELECT headline, logged_at 
		FROM world_news 
		ORDER BY logged_at DESC 
		LIMIT 5`

	rows, err := h.DB.QueryContext(ctx, queryNews)
	var newsLogText string
	if err != nil {
		log.Printf("Failed querying world news: %v", err)
		newsLogText = "📡 Static Interference: News feed currently offline."
	} else {
		defer rows.Close()
		for rows.Next() {
			var headline string
			var loggedAt time.Time
			if err := rows.Scan(&headline, &loggedAt); err == nil {
				newsLogText += fmt.Sprintf("[%s] %s\n", loggedAt.UTC().Format("15:04"), headline)
			}
		}
		if newsLogText == "" {
			newsLogText = "📡 Sensors Clean: No major sector events recorded yet."
		}
	}

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
			"LIVE BROADCAST NEWS FEED:\n"+
			"%s\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Listen to the static. The wastes are breathing.",
		currentTime, activeCamps, newsLogText,
	)

	return c.Send(dashboard, keyboards.CombatNavigation())
}

// HandleSectorMap displays close coordinates sector maps
func (h *WorldHandler) HandleSectorMap(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)

	sender := c.Sender()
	ctx := context.Background()

	var myX, myY int
	err := h.DB.QueryRowContext(ctx, "SELECT c.x, c.y FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.user_id = $1", sender.ID).Scan(&myX, &myY)
	if err != nil {
		return c.Send("⚠️ Access Denied: Establish your camp first using /start.")
	}

	mapHUD := "━━━━━━━━━━━━━━━━━━━━━━\n" +
		"🧭 SPATIAL CARTOGRAPHY SECTOR MAP\n" +
		"━━━━━━━━━━━━━━━━━━━━━━\n" +
		"Your outpost radar scans the coordinates nearby:\n\n"

	for y := myY + 1; y >= myY-1; y-- {
		rowText := "  "
		for x := myX - 1; x <= myX+1; x++ {
			var biome string
			err := h.DB.QueryRowContext(ctx, "SELECT biome FROM coordinates WHERE x = $1 AND y = $2", x, y).Scan(&biome)
			if err != nil {
				rowText += "░░ "
			} else {
				switch biome {
				case "ruins":
					rowText += "🏢 "
				case "wasteland":
					rowText += "💀 "
				default:
					rowText += "⛺ "
				}
			}
		}
		mapHUD += rowText + "\n"
	}

	mapHUD += fmt.Sprintf("\nCURRENT LOCATION: Sector [%d, %d]\n", myX, myY)
	mapHUD += "LEGEND:  ⛺ Outpost Base | 🏢 Ruins | 💀 Wasteland | ░░ Fog\n"
	mapHUD += "━━━━━━━━━━━━━━━━━━━━━━"

	return c.Send(mapHUD, keyboards.CombatNavigation())
}

package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/engine/world"
	"gopkg.in/telebot.v3"
)

type WorldHandler struct {
	DB *sql.DB
}

func NewWorldHandler(db *sql.DB) *WorldHandler {
	return &WorldHandler{DB: db}
}

// HandleWorldFeed displays real-time weather alerts and news headlines
func (h *WorldHandler) HandleWorldFeed(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)

	ctx := context.Background()

	var totalSurvivors int
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&totalSurvivors)

	// Phase 7 (item 12): world events are per-continent now, so this
	// feed shows a line per continent instead of one global status.
	continentWeather := world.ActiveEventsByContinent(ctx, h.DB)

	var weatherText string
	for _, continent := range world.Continents {
		weatherText += fmt.Sprintf("%s: %s\n", continent, weatherLine(continentWeather[continent]))
	}

	currentTime := time.Now().UTC().Format("15:04:05")

	queryNews := `
		SELECT headline, logged_at 
		FROM world_news 
		ORDER BY logged_at DESC 
		LIMIT 5`
	
	rows, err := h.DB.QueryContext(ctx, queryNews)
	var newsText string
	if err != nil {
		log.Printf("Failed querying world news: %v", err)
		newsText = "📡 Static Interference: News feed offline.\n\n"
	} else {
		defer rows.Close()
		hasNews := false
		for rows.Next() {
			var headline string
			var loggedAt time.Time
			if err := rows.Scan(&headline, &loggedAt); err == nil {
				if !hasNews {
					newsText += "📰 LATEST SECTOR BROADCAST REPORTS:\n"
					hasNews = true
				}
				newsText += fmt.Sprintf("[%s] %s\n", loggedAt.UTC().Format("15:04"), headline)
			}
		}
		if hasNews {
			newsText += "\n"
		} else {
			newsText = "📡 Radio Static: No active tactical bulletins registered on regional frequencies.\n\n"
		}
	}

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"📻 WASTELAND BROADCAST RADIO\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Universal Coordinate Time: [%s]\n"+
			"Active commanders on local frequencies: %d lines\n\n"+
			"🌍 DYNAMIC CLIMATE FORECAST:\n"+
			"%s\n\n"+
			"%s"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Listen to the static. The wastes are breathing.",
		currentTime, totalSurvivors, weatherText, newsText,
	)

	return c.Send(panelText, keyboards.CombatNavigation())
}

// HandleSectorMap displays close coordinates sector maps and neighboring settlements
func (h *WorldHandler) HandleSectorMap(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)

	sender := c.Sender()
	ctx := context.Background()

	var campID string
	var myX, myY int
	var myRegion string
	err := h.DB.QueryRowContext(ctx, "SELECT e.id, c.x, c.y, c.region FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.user_id = $1", sender.ID).Scan(&campID, &myX, &myY, &myRegion)
	if err != nil {
		return c.Send("⚠️ Access Denied: Establish your camp first using /start.")
	}

	mapHUD := "━━━━━━━━━━━━━━━━━━━━━━\n" +
		"🧭 SPATIAL CARTOGRAPHY SECTOR MAP\n" +
		"━━━━━━━━━━━━━━━━━━━━━━\n" +
		"Your outpost radar scans the coordinates nearby:\n\n"

	// Render the immediate 3x3 grid around player coordinates
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

	mapHUD += fmt.Sprintf("\nCURRENT LOCATION: Sector [%d, %d] (%s Territory Quadrant)\n", myX, myY, myRegion)
	mapHUD += "LEGEND:  ⛺ Outpost Base | 🏢 Ruins | 💀 Wasteland | ░░ Fog\n\n"
	mapHUD += "📡 NEIGHBORING SECTOR DISCOVERIES:\n"

	// Fetch regional quadrant neighbor outposts
	rows, err := h.DB.QueryContext(ctx, "SELECT e.name, u.first_name, c.x, c.y, c.region FROM encampments e JOIN users u ON u.telegram_id = e.user_id JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id != $1 LIMIT 3", campID)
	if err == nil {
		defer rows.Close()
		index := 1
		for rows.Next() {
			var name, owner, region string
			var x, y int
			if err := rows.Scan(&name, &owner, &x, &y, &region); err == nil {
				mapHUD += fmt.Sprintf("[%d] Outpost: %s (%s Quadrant)\n    Commander: %s | Location: Sector [%d, %d]\n\n", index, name, region, owner, x, y)
				index++
			}
		}
	}

	mapHUD += "━━━━━━━━━━━━━━━━━━━━━━"

	return c.Send(mapHUD, keyboards.CombatNavigation())
}

// HandleSectorBroadcast modulates a high-power wireless signal across neighboring coordinates and log bulletins
func (h *WorldHandler) HandleSectorBroadcast(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	broadcastMsg := c.Message().Payload
	if broadcastMsg == "" {
		return c.Send("⚠️ Broadcast Failed: Payload empty. Syntax: `/broadcast [message]`")
	}

	ctx := context.Background()

	var campID, campName string
	var campLvl int
	var myX, myY int
	err := h.DB.QueryRowContext(ctx, "SELECT e.id, e.name, e.level, c.x, c.y FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.user_id = $1", sender.ID).Scan(&campID, &campName, &campLvl, &myX, &myY)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	// 1. Enforce level requirement (Core level 10+)
	if campLvl < 10 {
		return c.Send("❌ Frequency Jammed: You must reach Outpost Core Level 10 to modulate long-range broadcasts.")
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Send("⚠️ Broadcast transaction error.")
	}
	defer tx.Rollback()

	// 2. Spend 50 Electricity Cells as fuel
	var electricity float64
	_ = tx.QueryRowContext(ctx, "SELECT electricity FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&electricity)

	if electricity < 50.0 {
		return c.Send("❌ Insufficient Electricity: Sector broadcasts require 50.0 Electricity Cells.")
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET electricity = electricity - 50.0 WHERE encampment_id = $1", campID)

	// 3. Write dynamic dispatch to global world_news radio bulletin feed
	headline := fmt.Sprintf("📻 OUTPOST BROADCAST: Encampment [%s] dispatched message: %q", campName, broadcastMsg)
	_, err = tx.ExecContext(ctx, "INSERT INTO world_news (headline) VALUES ($1)", headline)
	if err != nil {
		return c.Send("⚠️ Failed to modulate regional frequencies.")
	}

	// 4. Find target users within immediate coordinate sectors (3x3 grid)
	queryTargets := `
		SELECT e.user_id 
		FROM encampments e
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE c.x BETWEEN $1 AND $2 AND c.y BETWEEN $3 AND $4`
	
	rows, err := tx.QueryContext(ctx, queryTargets, myX-1, myX+1, myY-1, myY+1)
	if err != nil {
		log.Printf("Failed querying sector targets: %v", err)
		return c.Send("⚠️ Error reading coordinate database.")
	}
	defer rows.Close()

	var targets []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			targets = append(targets, id)
		}
	}
	rows.Close()

	// 5. Queue inbox notifications for nearby outposts
	formattedMsg := fmt.Sprintf(
		"📡 SECTOR BROADCAST (CO-ORDINATOR @%s from [%s]):\n\n\"%s\"",
		sender.Username, campName, broadcastMsg,
	)

	for _, targetID := range targets {
		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", targetID, formattedMsg)
	}

	_ = tx.Commit()
	return c.Send(fmt.Sprintf("📡 Broadcast successfully modulated over sector [%d, %d]. Logged to news feeds and dispatched to %d active proximity lines.", myX, myY, len(targets)), keyboards.CombatNavigation())
}
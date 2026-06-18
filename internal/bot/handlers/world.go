package handlers

import (
	"context"
	"database/sql"
	"errors"
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

// HandleWorldFeed displays real-time weather alerts and news headlines
func (h *WorldHandler) HandleWorldFeed(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)

	ctx := context.Background()

	var activeCamps int
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM encampments").Scan(&activeCamps)

	var activeWeather string
	_ = h.DB.QueryRowContext(ctx, "SELECT active_weather FROM world_state WHERE id = 1").Scan(&activeWeather)

	weatherLabel := "☀️ NOMINAL"
	weatherDebuff := "All systems are operating within baseline limits."
	switch activeWeather {
	case "solar_flare":
		weatherLabel = "⚡ SOLAR FLARE (ACTIVE)"
		weatherDebuff = "🔋 Solar panels generate 2.0x Energy. 🤖 Agents disabled."
	case "radiation_storm":
		weatherLabel = "☢️ RADIATION STORM (FALLOUT)"
		weatherDebuff = "💀 Morale decay doubled. 🔋 Solar panels output 50% power."
	case "acid_rain":
		weatherLabel = "🌧️ ACID RAIN (CORROSIVE)"
		weatherDebuff = "🏗️ Structural construction takes 2.0x longer."
	}

	currentTime := time.Now().UTC().Format("15:04:05")

	queryNews := `
		SELECT headline, logged_at 
		FROM world_news 
		ORDER BY logged_at DESC 
		LIMIT 5`
	
	rows, err := h.DB.QueryContext(ctx, queryNews)
	var newsLogText string
	if err != nil {
		log.Printf("Failed querying world news: %v", err)
		newsLogText = "📡 Static Interference: News feed offline."
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
			newsLogText = "📡 Sensors Clean: No major events recorded."
		}
	}

	dashboard := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"📻 WASTELAND BROADCAST RADIO\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Universal Coordinate Time: [%s]\n"+
			"Active Survivors: %d lines\n\n"+
			"TACTICAL WEATHER FORECAST:\n"+
			"🌍 Current Front: %s\n"+
			"⚠️ Modifiers: %s\n\n"+
			"LIVE BROADCAST NEWS FEED:\n"+
			"%s\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Listen to the static. The wastes are breathing.",
		currentTime, activeCamps, weatherLabel, weatherDebuff, newsLogText,
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

// HandleSectorBroadcast modulates a high-power wireless signal across neighboring coordinates
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

	var campID string
	var campLvl int
	var myX, myY int
	err := h.DB.QueryRowContext(ctx, "SELECT e.id, e.level, c.x, c.y FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.user_id = $1", sender.ID).Scan(&campID, &campLvl, &myX, &myY)
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

	// 2. Spend 50 Energy Cells as fuel
	var energy float64
	_ = tx.QueryRowContext(ctx, "SELECT energy FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&energy)

	if energy < 50.0 {
		return c.Send("❌ Insufficient Energy: Sector broadcasts require 50.0 Energy Cells.")
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET energy = energy - 50.0 WHERE encampment_id = $1", campID)

	// 3. Find target users within immediate coordinate sectors (3x3 grid)
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

	// 4. Queue real-time alerts
	formattedMsg := fmt.Sprintf(
		"📡 SECTOR BROADCAST (CO-ORDINATOR %s):\n\n\"%s\"",
		sender.FirstName, broadcastMsg,
	)

	for _, targetID := range targets {
		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", targetID, formattedMsg)
	}

	_ = tx.Commit()
	return c.Send(fmt.Sprintf("📡 Broadcast successfully modulated over sector [%d, %d]. Dispatched to %d active lines.", myX, myY, len(targets)), keyboards.MainNavigation())
}
package world

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
)

type WeatherEngine struct {
	DB *sql.DB
}

func NewWeatherEngine(db *sql.DB) *WeatherEngine {
	return &WeatherEngine{DB: db}
}

// RunWeatherPass rolls a 10% chance to cycle the world's active weather front
func (w *WeatherEngine) RunWeatherPass(ctx context.Context, tx *sql.Tx) error {
	if rand.Float64() >= 0.10 {
		// Weather remains stable
		return nil
	}

	weatherFronts := []string{"nominal", "solar_flare", "radiation_storm", "acid_rain"}
	newWeather := weatherFronts[rand.Intn(len(weatherFronts))]

	// Update global world state
	updateQuery := `
		UPDATE world_state 
		SET active_weather = $1, last_changed_at = CURRENT_TIMESTAMP 
		WHERE id = 1`
	
	_, err := tx.ExecContext(ctx, updateQuery, newWeather)
	if err != nil {
		return fmt.Errorf("failed updating global weather state: %w", err)
	}

	// Write environmental transition announcement to Wasteland Radio news ticker
	var alertHeadline string
	switch newWeather {
	case "nominal":
		alertHeadline = "☀️ ENVIRONMENTAL REPORT: Toxic storms have cleared. Regional sectors have returned to nominal conditions."
	case "solar_flare":
		alertHeadline = "⚡ SOLAR FLARE DETECTED: Intense electromagnetic wave spikes registered. Outpost solar generators are operating at 200% efficiency. Agent automation stand by."
	case "radiation_storm":
		alertHeadline = "☢️ RADIATION STORM WARNING: High-altitude radioactive fallout sweeping central sectors. Morale decay rates doubled."
	case "acid_rain":
		alertHeadline = "🌧️ ACID RAIN ALERT: Highly corrosive precipitation is slowing down regional logistics. All active construction projects are running at 50% speed."
	}

	insertNews := `INSERT INTO world_news (headline) VALUES ($1)`
	_, err = tx.ExecContext(ctx, insertNews, alertHeadline)
	if err != nil {
		log.Printf("Failed writing weather news headline: %v", err)
	}

	log.Printf("Weather Cycle Pass resolved: Global weather transitioned to [%s].", newWeather)
	return nil
}
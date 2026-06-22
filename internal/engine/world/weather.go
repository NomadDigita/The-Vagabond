package world

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"time"
)

type WeatherEngine struct {
	DB *sql.DB
}

func NewWeatherEngine(db *sql.DB) *WeatherEngine {
	return &WeatherEngine{DB: db}
}

func (w *WeatherEngine) RunWeatherPass(ctx context.Context, tx *sql.Tx) error {
	var lastChanged time.Time
	err := tx.QueryRowContext(ctx, "SELECT last_changed_at FROM world_state WHERE id = 1").Scan(&lastChanged)
	
	// Enforced UTC timezone comparisons to prevent skewing
	if err == nil && time.Now().UTC().Sub(lastChanged.UTC()) < 2*time.Hour {
		return nil 
	}

	if rand.Float64() >= 0.10 {
		return nil 
	}

	weatherFronts := []string{"nominal", "solar_flare", "radiation_storm", "acid_rain"}
	newWeather := weatherFronts[rand.Intn(len(weatherFronts))]

	updateQuery := `
		UPDATE world_state 
		SET active_weather = $1, last_changed_at = CURRENT_TIMESTAMP 
		WHERE id = 1`
	
	_, err = tx.ExecContext(ctx, updateQuery, newWeather)
	if err != nil {
		return fmt.Errorf("failed updating global weather state: %w", err)
	}

	continents := []string{"Africa", "Europe", "Asia", "Americas"}
	targetContinent := continents[rand.Intn(len(continents))]

	var alertHeadline string
	switch newWeather {
	case "nominal":
		alertHeadline = fmt.Sprintf("☀️ ENVIRONMENTAL REPORT: Toxic storms over %s have cleared. Regional sectors have returned to nominal baseline conditions.", targetContinent)
	case "solar_flare":
		alertHeadline = fmt.Sprintf("⚡ SOLAR FLARE DETECTED: Intense electromagnetic wave spikes registered over %s. Outpost solar generators operating at 200%%. Agent automation stand by.", targetContinent)
	case "radiation_storm":
		alertHeadline = fmt.Sprintf("☢️ RADIATION STORM WARNING: High-altitude radioactive fallout sweeping %s sectors. Morale decay rates doubled.", targetContinent)
	case "acid_rain":
		alertHeadline = fmt.Sprintf("🌧️ ACID RAIN ALERT: Highly corrosive precipitation over %s is slowing down logistics. Active construction projects running at 50%% speed.", targetContinent)
	}

	insertNews := `INSERT INTO world_news (headline) VALUES ($1)`
	_, err = tx.ExecContext(ctx, insertNews, alertHeadline)
	if err != nil {
		log.Printf("Failed writing weather news headline: %v", err)
	}

	log.Printf("Weather Cycle Pass resolved: Global weather transitioned to [%s] over %s.", newWeather, targetContinent)
	return nil
}
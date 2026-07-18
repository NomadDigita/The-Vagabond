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

// Continents mirrors coordinates.region's existing quadrant scheme
// (see relocateZeroCoordinates in cmd/bot/main.go) so that world events
// line up with where players actually spawn instead of an unrelated
// separate zone list.
var Continents = []string{"Africa", "Europe", "Asia", "Americas"}

// eventPool is every non-nominal world event that can currently roll.
// Solar Flare/Radiation Storm/Acid Rain are the original three; EMP,
// Supply Crisis, Disease, and Sandstorm are Phase 7 item 12's additions.
var eventPool = []string{
	"solar_flare", "radiation_storm", "acid_rain", "emp", "supply_crisis", "disease", "sandstorm",
}

const eventDuration = 2 * time.Hour
const eventRollChance = 0.10

// RunWeatherPass rolls world events independently per continent, instead
// of one global front affecting the whole game. Each continent persists
// its current event for 2 hours (via expires_at), then rolls a 10%
// chance per tick to start a new one once clear. A continent with an
// event still running is left alone (persistence); a continent whose
// event just expired gets a "conditions have cleared" headline before
// it's eligible to roll again.
func (w *WeatherEngine) RunWeatherPass(ctx context.Context, tx *sql.Tx) error {
	for _, continent := range Continents {
		var eventID, eventType string
		var expiresAt time.Time
		err := tx.QueryRowContext(ctx,
			`SELECT id, event_type, expires_at FROM world_events
			 WHERE continent = $1
			 ORDER BY expires_at DESC LIMIT 1`, continent).Scan(&eventID, &eventType, &expiresAt)

		hasRow := err == nil
		stillActive := hasRow && time.Now().UTC().Before(expiresAt.UTC())

		if stillActive {
			continue // persistence barrier: this continent's event holds stable
		}

		if hasRow {
			// The most recent event for this continent exists but has
			// expired - clear it out and announce the all-clear before
			// considering a fresh roll this same pass.
			if _, delErr := tx.ExecContext(ctx, "DELETE FROM world_events WHERE id = $1", eventID); delErr != nil {
				log.Printf("Failed clearing expired world event %s (%s/%s): %v", eventID, continent, eventType, delErr)
			} else {
				headline := fmt.Sprintf("☀️ ENVIRONMENTAL REPORT: %s over %s has cleared. Regional sectors have returned to nominal baseline conditions.", eventLabel(eventType), continent)
				if _, err := tx.ExecContext(ctx, "INSERT INTO world_news (headline) VALUES ($1)", headline); err != nil {
					log.Printf("Failed writing world-event clear headline: %v", err)
				}
			}
		}

		if rand.Float64() >= eventRollChance {
			continue // this continent stays clear this pass
		}

		newEvent := eventPool[rand.Intn(len(eventPool))]
		expiresAt = time.Now().UTC().Add(eventDuration)

		_, err = tx.ExecContext(ctx,
			"INSERT INTO world_events (event_type, continent, expires_at) VALUES ($1, $2, $3)",
			newEvent, continent, expiresAt)
		if err != nil {
			return fmt.Errorf("failed inserting world event for %s: %w", continent, err)
		}

		headline := eventHeadline(newEvent, continent)
		if _, err := tx.ExecContext(ctx, "INSERT INTO world_news (headline) VALUES ($1)", headline); err != nil {
			log.Printf("Failed writing world-event news headline: %v", err)
		}

		log.Printf("World Event Pass: [%s] triggered over %s (expires %s).", newEvent, continent, expiresAt.Format(time.RFC3339))
	}
	return nil
}

// eventLabel gives a short human-readable name for a stored event_type,
// used in "conditions have cleared" headlines.
func eventLabel(eventType string) string {
	switch eventType {
	case "solar_flare":
		return "Solar Flare interference"
	case "radiation_storm":
		return "Radiation Storm fallout"
	case "acid_rain":
		return "Acid Rain corrosion"
	case "emp":
		return "EMP disruption"
	case "supply_crisis":
		return "the Supply Crisis"
	case "disease":
		return "the Disease outbreak"
	case "sandstorm":
		return "the Sandstorm"
	default:
		return "the anomaly"
	}
}

// eventHeadline builds the full news-feed announcement for a freshly
// triggered event, matching the original three's tone and mechanical
// callouts. See internal/bot/handlers/world.go for the matching
// in-panel description text, and each mechanical consumer
// (internal/engine/tick/engine.go, internal/engine/resource/resource.go,
// internal/bot/handlers/combat.go, internal/bot/handlers/camp.go) for
// where these actually apply their effects.
func eventHeadline(eventType, continent string) string {
	switch eventType {
	case "solar_flare":
		return fmt.Sprintf("⚡ SOLAR FLARE DETECTED: Intense electromagnetic wave spikes registered over %s. Outpost solar generators operating at 200%%. Agent automation stand by.", continent)
	case "radiation_storm":
		return fmt.Sprintf("☢️ RADIATION STORM WARNING: High-altitude radioactive fallout sweeping %s sectors. Morale decay rates doubled.", continent)
	case "acid_rain":
		return fmt.Sprintf("🌧️ ACID RAIN ALERT: Highly corrosive precipitation over %s is slowing down logistics. Active construction projects running at reduced speed.", continent)
	case "emp":
		return fmt.Sprintf("🌩️ EMP BURST WARNING: A regional electromagnetic pulse over %s has knocked out unshielded electronics. Automation Agents standing down; Electricity generation offline.", continent)
	case "supply_crisis":
		return fmt.Sprintf("📉 SUPPLY CRISIS: Logistics networks across %s are in disarray. Market Exchange sale prices are depressed until conditions improve.", continent)
	case "disease":
		return fmt.Sprintf("🦠 DISEASE OUTBREAK: An unidentified pathogen is spreading through %s outposts. Rations consumption elevated as commanders divert stock to treatment.", continent)
	case "sandstorm":
		return fmt.Sprintf("🌪️ SANDSTORM WARNING: Visibility across %s sectors has collapsed. Scan and Scout operations report degraded intel accuracy.", continent)
	default:
		return fmt.Sprintf("🌍 Unusual atmospheric readings detected over %s.", continent)
	}
}

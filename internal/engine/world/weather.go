package world

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
)

type WeatherEngine struct {
	DB *sql.DB
}

func NewWeatherEngine(db *sql.DB) *WeatherEngine {
	return &WeatherEngine{DB: db}
}

var Continents = []string{"Africa", "Europe", "Asia", "Americas"}

var eventPool = []string{
	"solar_flare", "radiation_storm", "acid_rain", "emp", "supply_crisis", "disease", "sandstorm", "bloom",
}

type weatherPoolEntry struct {
	eventType string
	weight    int
}

var weatherPool = func() []weatherPoolEntry {
	pool := make([]weatherPoolEntry, 0, len(eventPool))
	for _, e := range eventPool {
		weight := 4
		if e == "bloom" {
			weight = 1
		}
		pool = append(pool, weatherPoolEntry{eventType: e, weight: weight})
	}
	return pool
}()

func pickWeightedEvent() string {
	total := 0
	for _, e := range weatherPool {
		total += e.weight
	}
	roll := rand.Intn(total)
	for _, e := range weatherPool {
		if roll < e.weight {
			return e.eventType
		}
		roll -= e.weight
	}
	return weatherPool[len(weatherPool)-1].eventType
}

const eventDuration = 2 * time.Hour
const eventRollChance = 0.10

func (w *WeatherEngine) RunWeatherPass(ctx context.Context, tx *sql.Tx) error {
	summary := make([]string, 0, len(Continents))

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
			summary = append(summary, fmt.Sprintf("%s=%s(active,%s left)", continent, eventType, time.Until(expiresAt.UTC()).Round(time.Minute)))
			continue
		}

		if hasRow {
			if _, delErr := tx.ExecContext(ctx, "DELETE FROM world_events WHERE id = $1", eventID); delErr != nil {
				log.Printf("Failed clearing expired world event %s (%s/%s): %v", eventID, continent, eventType, delErr)
			} else {
				headline := fmt.Sprintf("☀️ ENVIRONMENTAL REPORT: %s over %s has cleared. Regional sectors have returned to nominal baseline conditions.", eventLabel(eventType), continent)
				if _, err := tx.ExecContext(ctx, "INSERT INTO world_news (headline) VALUES ($1)", headline); err != nil {
					log.Printf("Failed writing world-event clear headline: %v", err)
				}
				if err := notifications.QueueToRegion(ctx, tx, continent, headline, "general"); err != nil {
					log.Printf("Failed broadcasting world-event clear notification for %s: %v", continent, err)
				}
			}
		}

		if rand.Float64() >= eventRollChance {
			summary = append(summary, fmt.Sprintf("%s=nominal(rolled,miss)", continent))
			continue
		}

		newEvent := pickWeightedEvent()
		startsAt := time.Now().UTC()
		expiresAt = startsAt.Add(eventDuration)
		headline := eventHeadline(newEvent, continent)

		_, err = tx.ExecContext(ctx,
			`INSERT INTO world_events (title, event_type, continent, starts_at, expires_at)
			 VALUES ($1, $2, $3, $4, $5)`,
			headline, newEvent, continent, startsAt, expiresAt)
		if err != nil {
			return fmt.Errorf("failed inserting world event for %s: %w", continent, err)
		}

		if _, err := tx.ExecContext(ctx, "INSERT INTO world_news (headline) VALUES ($1)", headline); err != nil {
			log.Printf("Failed writing world-event news headline: %v", err)
		}
		if err := notifications.QueueToRegion(ctx, tx, continent, headline, "general"); err != nil {
			log.Printf("Failed broadcasting world-event notification for %s: %v", continent, err)
		}

		summary = append(summary, fmt.Sprintf("%s=%s(NEW HIT)", continent, newEvent))
		log.Printf("World Event Pass: [%s] triggered over %s (expires %s).", newEvent, continent, expiresAt.Format(time.RFC3339))
	}

	log.Printf("Weather heartbeat: %s", strings.Join(summary, ", "))
	return nil
}

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
	case "bloom":
		return "the Wasteland Bloom"
	default:
		return "the anomaly"
	}
}

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
	case "bloom":
		return fmt.Sprintf("🌸 WASTELAND BLOOM: Bioluminescent flora has erupted across %s's ruins overnight. Outpost harvesters are reporting significantly elevated yields across every resource line.", continent)
	default:
		return fmt.Sprintf("🌍 Unusual atmospheric readings detected over %s.", continent)
	}
}

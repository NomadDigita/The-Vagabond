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
	// Per-continent outcome for this pass, logged once at the end
	// regardless of whether anything happened. Previously this
	// function only logged on an actual trigger, so a live deployment
	// that only ever rolled misses produced zero log output from this
	// phase - indistinguishable from the phase not running at all when
	// diagnosing "no world events have ever fired" reports. See
	// LastTickStatus in internal/engine/tick/engine.go for the
	// companion admin-visible confirmation that ticks are executing.
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
				// Direct push, not just the passive world_news feed - see
				// AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 5.3.
				// Non-mutable ("general") for now; see that doc's open
				// question 2 on whether this should get its own mutable
				// category instead.
				if err := notifications.QueueToRegion(ctx, tx, continent, headline, "general"); err != nil {
					log.Printf("Failed broadcasting world-event clear notification for %s: %v", continent, err)
				}
			}
		}

		if rand.Float64() >= eventRollChance {
			summary = append(summary, fmt.Sprintf("%s=nominal(rolled,miss)", continent))
			continue // this continent stays clear this pass
		}

		newEvent := eventPool[rand.Intn(len(eventPool))]
		startsAt := time.Now().UTC()
		expiresAt = startsAt.Add(eventDuration)
		headline := eventHeadline(newEvent, continent)

		// NOTE (2026-08-02, confirmed live via Render logs): world_events
		// was originally created by migrations/001_initial_schema.sql with
		// title VARCHAR(150) NOT NULL and starts_at TIMESTAMPTZ NOT NULL
		// (plus description/multiplier, both nullable/defaulted). This
		// package's simplified 4-column model (id/event_type/continent/
		// expires_at) was added later via schema.go's `CREATE TABLE IF NOT
		// EXISTS world_events`, which is a no-op against a table that
		// already exists - so on any deployment whose DB predates that
		// change, the live table still carries the old NOT NULL columns
		// this INSERT never populated. Every single roll-hit was failing
		// at the DB layer with "null value in column title violates
		// not-null constraint", silently eating the whole weather pass
		// (RunWeatherPass returns the error, aborting the tx) - this is
		// the actual root cause of "zero world events have ever fired"
		// despite the roll logic itself always having been correct.
		// Supplying title/starts_at here fixes it directly; see schema.go
		// for the accompanying idempotent ALTER TABLE that also relaxes
		// the constraint with a DEFAULT, so a *fresh* database (no legacy
		// columns at all) and this codebase's own migration order can't
		// reintroduce the same trap.
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
		// Same headline text pushed directly, so the world_news feed and
		// the direct notification never say different things (section 5.3).
		if err := notifications.QueueToRegion(ctx, tx, continent, headline, "general"); err != nil {
			log.Printf("Failed broadcasting world-event notification for %s: %v", continent, err)
		}

		summary = append(summary, fmt.Sprintf("%s=%s(NEW HIT)", continent, newEvent))
		log.Printf("World Event Pass: [%s] triggered over %s (expires %s).", newEvent, continent, expiresAt.Format(time.RFC3339))
	}

	log.Printf("Weather heartbeat: %s", strings.Join(summary, ", "))
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

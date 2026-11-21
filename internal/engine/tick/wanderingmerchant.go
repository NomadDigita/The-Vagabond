package tick

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"

	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
)

var merchantSpawnChance = 0.004

const merchantVisitWindowMinutes = 90.0

const (
	merchantMinOffers = 2
	merchantMaxOffers = 4
)

const merchantDiscountFactor = 0.6

var merchantOfferResources = []string{"metal", "crystal", "scrap"}

var merchantBaseUnitPrice = map[string]float64{
	"metal":   3.0,
	"crystal": 15.0,
	"scrap":   2.0,
}

const (
	merchantMinQty = 40
	merchantMaxQty = 200
)

func (e *Engine) wanderingMerchantSpawn(ctx context.Context, tx *sql.Tx) error {
	var activeExists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM merchant_offers WHERE purchased_by IS NULL AND expires_at > CURRENT_TIMESTAMP)
	`).Scan(&activeExists); err != nil {
		return fmt.Errorf("checking for active merchant offers: %w", err)
	}
	if activeExists {
		return nil
	}
	if rand.Float64() >= merchantSpawnChance {
		return nil
	}

	offerCount := merchantMinOffers + rand.Intn(merchantMaxOffers-merchantMinOffers+1)
	for i := 0; i < offerCount; i++ {
		resource := merchantOfferResources[rand.Intn(len(merchantOfferResources))]
		qty := merchantMinQty + rand.Intn(merchantMaxQty-merchantMinQty+1)
		price := merchantBaseUnitPrice[resource] * float64(qty) * merchantDiscountFactor

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO merchant_offers (item_type, quantity, price_dollars, expires_at)
			VALUES ($1, $2, $3, CURRENT_TIMESTAMP + ($4 * INTERVAL '1 minute'))`,
			resource, qty, price, merchantVisitWindowMinutes); err != nil {
			return fmt.Errorf("spawning merchant offer: %w", err)
		}
	}

	broadcast := fmt.Sprintf(
		"🛒 THE WANDERING MERCHANT HAS ARRIVED: A rare trader has set up shop with %d steeply discounted lots, gone in %d minutes or as soon as they're bought out. Check /merchant before it's too late!",
		offerCount, int(merchantVisitWindowMinutes))
	if err := notifications.QueueToAllPlayers(ctx, tx, broadcast, "general"); err != nil {
		return fmt.Errorf("broadcasting merchant arrival: %w", err)
	}
	return nil
}

func (e *Engine) wanderingMerchantExpire(ctx context.Context, tx *sql.Tx) error {
	result, err := tx.ExecContext(ctx, `
		DELETE FROM merchant_offers WHERE purchased_by IS NULL AND expires_at <= CURRENT_TIMESTAMP`)
	if err != nil {
		return fmt.Errorf("clearing expired merchant offers: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil
	}
	broadcast := "🛒 The Wandering Merchant has packed up and moved on. Any unclaimed bargains are gone until their next visit."
	if err := notifications.QueueToAllPlayers(ctx, tx, broadcast, "general"); err != nil {
		return fmt.Errorf("broadcasting merchant departure: %w", err)
	}
	return nil
}

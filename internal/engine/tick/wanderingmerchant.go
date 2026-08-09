package tick

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"

	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
)

// RARE_WORLD_FEATURES_PLAN.md Phase 4: The Wandering Merchant.
//
// A rare, time-limited NPC vendor - not tied to any player's own
// listing, not competing with the player market exchange - offering a
// short rotating batch of steeply discounted resource lots. Gone after
// being bought out or after its window closes. See the plan doc's
// section 4.1 for why this is its own dedicated table rather than a
// synthetic market_exchange seller.

// merchantSpawnChance is the per-tick probability of the Wandering
// Merchant arriving with a fresh batch of offers, checked only when
// there are currently no active (unpurchased, unexpired) offers - a
// fresh visit is a clean batch, not an ever-growing backlog. Rarer
// than a normal weather roll, commoner than a relic convoy - averaging
// roughly one visit every ~4 hours of continuous uptime at the tick
// engine's ~60s cadence.
//
// A var, not a const, so wanderingmerchant_test.go can deterministically
// force a spawn instead of looping thousands of real-Postgres round
// trips waiting for a legitimate 0.4% roll to land.
var merchantSpawnChance = 0.004

// merchantVisitWindowMinutes is how long an unpurchased offer batch
// stays live - shorter than a relic convoy's window since this is a
// recurring, lighter event rather than a once-in-many-hours moment.
const merchantVisitWindowMinutes = 90.0

// merchantMinOffers/merchantMaxOffers bound how many offers spawn per
// visit.
const (
	merchantMinOffers = 2
	merchantMaxOffers = 4
)

// merchantDiscountFactor is what fraction of a "typical" market rate
// the Merchant charges - 0.6 means roughly 40% cheaper than a normal
// player listing, a genuine bargain rather than a reskinned market
// post.
const merchantDiscountFactor = 0.6

// merchantOfferResources are the resources the Merchant can offer -
// the same three tradeable resources the player market exchange
// supports (see exchange.go's validMarketResources).
var merchantOfferResources = []string{"metal", "crystal", "scrap"}

// merchantBaseUnitPrice is a rough "typical market rate" per unit for
// each resource, used only to derive a plausible discounted Merchant
// price - not read from live market_exchange listings (which may be
// sparse or absent early in a world's life), so the Merchant always
// has a sensible price to offer regardless of current player market
// activity. Deliberately in the same ballpark as the exchange panel's
// own quick-list presets (e.g. "List 50 Metal ($150)" implies $3/unit).
var merchantBaseUnitPrice = map[string]float64{
	"metal":   3.0,
	"crystal": 15.0,
	"scrap":   2.0,
}

// merchantMinQty/merchantMaxQty bound a single offer's quantity.
const (
	merchantMinQty = 40
	merchantMaxQty = 200
)

// wanderingMerchantSpawn is the "wandering_merchant_spawn" tick phase -
// see engine.go's ProcessTick phase list. Rolls a fresh batch of offers
// into existence only when none are currently active.
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

// wanderingMerchantExpire is the "wandering_merchant_expire" tick
// phase - clears offers nobody bought in time, with a "moved on"
// broadcast, same pattern as Phase 1/2's expiry precedent. Only
// broadcasts once per expiry sweep (not once per leftover offer) since
// a whole batch expiring together is one event, not several.
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

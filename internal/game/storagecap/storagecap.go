// Package storagecap is the single source of truth for outpost storage
// caps. Every resource in the `resources` table (Scrap, Rations,
// Electricity, Metal, Crystal, Hydrogen, Dollars, Neuro Cores, Ether) is
// subject to the same cap, derived from Tent + Warehouse + Extension
// levels. This was previously only enforced for Scrap/Rations/Electricity
// in the passive resource tick and the automation agent's `collector`
// mode; Phase 7 (full audit, see SPACEHUNT_PHASE7_LOG.md item 5) extends
// it to every resource-gain site in the game.
package storagecap

import (
	"context"
	"database/sql"
	"math"
)

// Queryer is satisfied by both *sql.DB and *sql.Tx, so callers already
// holding an open transaction can reuse it for the levels lookup instead
// of opening a second connection.
type Queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// Cap returns the total storage cap for an outpost at the given
// Tent/Warehouse/Extension levels. This is the formula already
// established in internal/engine/resource/resource.go:
// Tent*500 + Warehouse*750 + Extension*1000.
func Cap(tentLvl, warehouseLvl, extensionLvl int) float64 {
	return float64(tentLvl)*500.0 + float64(warehouseLvl)*750.0 + float64(extensionLvl)*1000.0
}

// Levels fetches the Tent/Warehouse module levels and the Outpost's
// Extension level for a camp in a single query, for use with Cap.
// Missing modules/extension default to their un-upgraded value (Tent 1,
// Warehouse 0, Extension 0) rather than erroring.
func Levels(ctx context.Context, q Queryer, campID string) (tentLvl, warehouseLvl, extensionLvl int) {
	tentLvl = 1
	_ = q.QueryRowContext(ctx, `
		SELECT
			COALESCE((SELECT level FROM modules WHERE encampment_id = $1 AND type = 'tent'), 1),
			COALESCE((SELECT level FROM modules WHERE encampment_id = $1 AND type = 'warehouse'), 0),
			COALESCE((SELECT extension_lvl FROM encampments WHERE id = $1), 0)`,
		campID,
	).Scan(&tentLvl, &warehouseLvl, &extensionLvl)
	return
}

// CapFor is a convenience wrapper combining Levels + Cap for the common
// case of "give me this camp's current storage cap."
func CapFor(ctx context.Context, q Queryer, campID string) float64 {
	tentLvl, warehouseLvl, extensionLvl := Levels(ctx, q, campID)
	return Cap(tentLvl, warehouseLvl, extensionLvl)
}

// Clamp applies the "Surplus Preservation System" rule already used for
// passive Scrap/Rations/Electricity generation: a pre-existing balance
// already at or above the cap is never reduced, but no further gain is
// added on top of it. Below the cap, gains are allowed up to (but not
// past) it. The discarded remainder (if any) is returned alongside the
// new balance so callers can surface it in a notification if useful.
func Clamp(current, gain, cap float64) (newValue float64, discarded float64) {
	if current >= cap {
		return current, gain
	}
	newValue = math.Min(current+gain, cap)
	discarded = (current + gain) - newValue
	if discarded < 0 {
		discarded = 0
	}
	return newValue, discarded
}

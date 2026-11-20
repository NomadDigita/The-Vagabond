package storagecap

import (
	"context"
	"database/sql"
	"math"
)

type Queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

func Cap(tentLvl, warehouseLvl, extensionLvl int) float64 {
	return float64(tentLvl)*500.0 + float64(warehouseLvl)*750.0 + float64(extensionLvl)*1000.0
}

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

func CapFor(ctx context.Context, q Queryer, campID string) float64 {
	tentLvl, warehouseLvl, extensionLvl := Levels(ctx, q, campID)
	return Cap(tentLvl, warehouseLvl, extensionLvl)
}

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

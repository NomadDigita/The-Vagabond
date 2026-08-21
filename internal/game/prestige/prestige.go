// Package prestige is NEXT_GENERATION_FEATURES_PLAN.md Phase 4:
// Prestige / Rebirth Loop - a deliberate, irreversible reset of the
// economic game back to its starting point, in exchange for a
// permanent, stacking resource-generation bonus. The classic idle-
// genre answer to "a maxed-out veteran has nothing left to do."
//
// What resets: camp level, every building (modules table), every
// research track (all 7 columns on research_states - this build's
// actual research surface, wider than the plan doc's illustrative
// "four tracks" description), and current resources (reset to the
// same starting baseline onboarding.go grants a brand-new account).
//
// What is deliberately kept: gear (Phase 3), achievements (Phase 1),
// hero level/skills/story progress (Phase 6), clan membership, and any
// Legacy/Relic history - prestige resets the economic/building game,
// not a player's identity or social standing. Garrison (soldiers/
// mechs in workshop_inventory) is also kept - the plan doc's own list
// of what resets names camp level, buildings, research, and resources
// specifically, and does not mention troops.
package prestige

import (
	"context"
	"database/sql"
	"fmt"
)

// Querier is satisfied by both *sql.DB and *sql.Tx, matching the same
// convention gear.Querier/progression.Querier/herodepth.Querier
// already establish.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// campLevelRequirement/researchLevelRequirement are Prestige's
// eligibility gate - deliberately steep (matching the plan doc's own
// "meaningful requirement" framing), since resetting the economic game
// should only be available to someone who has genuinely finished it,
// not someone looking for a shortcut.
const (
	campLevelRequirement     = 25
	researchLevelRequirement = 20
	prestigeBonusPctPerLevel = 5.0
	startingResourceBaseline = 25000.0
)

// researchColumns is every column on research_states this build
// actually exposes (research.go's own fetchResearchLevels reads all
// seven) - Prestige requires and resets every one of them, not just
// the plan doc's illustrative four.
var researchColumns = []string{
	"econ_tech_lvl", "production_tech_lvl", "integrity_tech_lvl",
	"defense_tech_lvl", "intel_tech_lvl", "speed_tech_lvl", "military_tech_lvl",
}

// EligibilityResult is CheckEligibility's return shape - Eligible is
// the single yes/no the panel gates the Prestige button on; Missing is
// a human-readable breakdown of exactly what's still short, so a
// not-yet-eligible player sees a clear "what's missing" list rather
// than a bare refusal.
type EligibilityResult struct {
	Eligible bool
	Missing  []string
}

// CheckEligibility reports whether campID currently qualifies to
// Prestige: camp level >= campLevelRequirement AND every research
// track >= researchLevelRequirement.
func CheckEligibility(ctx context.Context, q Querier, campID string) (EligibilityResult, error) {
	var result EligibilityResult

	var campLevel int
	if err := q.QueryRowContext(ctx, "SELECT level FROM encampments WHERE id = $1", campID).Scan(&campLevel); err != nil {
		return result, fmt.Errorf("reading camp level: %w", err)
	}
	if campLevel < campLevelRequirement {
		result.Missing = append(result.Missing, fmt.Sprintf("Outpost level %d/%d", campLevel, campLevelRequirement))
	}

	levels := make([]int, len(researchColumns))
	scanArgs := make([]interface{}, len(researchColumns))
	for i := range levels {
		scanArgs[i] = &levels[i]
	}
	query := "SELECT " + joinColumns(researchColumns) + " FROM research_states WHERE encampment_id = $1"
	err := q.QueryRowContext(ctx, query, campID).Scan(scanArgs...)
	if err == sql.ErrNoRows {
		// No research_states row yet at all - every track is
		// effectively level 1, well under the requirement.
		for _, col := range researchColumns {
			result.Missing = append(result.Missing, fmt.Sprintf("%s 1/%d", col, researchLevelRequirement))
		}
	} else if err != nil {
		return result, fmt.Errorf("reading research levels: %w", err)
	} else {
		for i, lvl := range levels {
			if lvl < researchLevelRequirement {
				result.Missing = append(result.Missing, fmt.Sprintf("%s %d/%d", researchColumns[i], lvl, researchLevelRequirement))
			}
		}
	}

	result.Eligible = len(result.Missing) == 0
	return result, nil
}

func joinColumns(cols []string) string {
	out := ""
	for i, c := range cols {
		if i > 0 {
			out += ", "
		}
		out += c
	}
	return out
}

// DoPrestige performs the actual reset - deliberately NOT wrapped in
// its own transaction (the caller, HandlePrestigeConfirmCallback,
// already runs this inside one alongside the confirmation flow, the
// same "caller owns the transaction" convention doPostListing/
// doBuyListingQty already establish).
func DoPrestige(ctx context.Context, q Querier, campID string) error {
	eligibility, err := CheckEligibility(ctx, q, campID)
	if err != nil {
		return err
	}
	if !eligibility.Eligible {
		return fmt.Errorf("not eligible to prestige: %v", eligibility.Missing)
	}

	if _, err := q.ExecContext(ctx, "UPDATE encampments SET level = 1, extension_lvl = 0, prestige_level = prestige_level + 1, prestige_bonus_pct = prestige_bonus_pct + $1 WHERE id = $2", prestigeBonusPctPerLevel, campID); err != nil {
		return fmt.Errorf("resetting camp level: %w", err)
	}
	if _, err := q.ExecContext(ctx, "DELETE FROM modules WHERE encampment_id = $1", campID); err != nil {
		return fmt.Errorf("resetting buildings: %w", err)
	}
	resetResearch := "UPDATE research_states SET "
	for i, col := range researchColumns {
		if i > 0 {
			resetResearch += ", "
		}
		resetResearch += col + " = 1"
	}
	resetResearch += " WHERE encampment_id = $1"
	if _, err := q.ExecContext(ctx, resetResearch, campID); err != nil {
		return fmt.Errorf("resetting research: %w", err)
	}
	if _, err := q.ExecContext(ctx, `
		UPDATE resources SET scrap = $1, rations = $1, electricity = $1, neuro_cores = $1,
			metal = $1, crystal = $1, hydrogen = $1, dollars = $1, ether = $1
		WHERE encampment_id = $2`, startingResourceBaseline, campID); err != nil {
		return fmt.Errorf("resetting resources: %w", err)
	}
	return nil
}

// ResourceBonusPctFor returns campID's current cumulative Prestige
// bonus - resource.go reads this at the exact same call site
// gear.ResourceBonusPctFor/herodepth.ResourceBonusPctFor/Phase 2's
// Bloom multiplier already use, stacking with (not replacing) any of
// them.
func ResourceBonusPctFor(ctx context.Context, q Querier, campID string) float64 {
	var bonus float64
	_ = q.QueryRowContext(ctx, "SELECT prestige_bonus_pct FROM encampments WHERE id = $1", campID).Scan(&bonus)
	return bonus
}

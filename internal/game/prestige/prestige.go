package prestige

import (
	"context"
	"database/sql"
	"fmt"
)

type Querier interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

const (
	campLevelRequirement     = 25
	researchLevelRequirement = 20
	prestigeBonusPctPerLevel = 5.0
	startingResourceBaseline = 25000.0
)

var researchColumns = []string{
	"econ_tech_lvl", "production_tech_lvl", "integrity_tech_lvl",
	"defense_tech_lvl", "intel_tech_lvl", "speed_tech_lvl", "military_tech_lvl",
}

type EligibilityResult struct {
	Eligible bool
	Missing  []string
}

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

func ResourceBonusPctFor(ctx context.Context, q Querier, campID string) float64 {
	var bonus float64
	_ = q.QueryRowContext(ctx, "SELECT prestige_bonus_pct FROM encampments WHERE id = $1", campID).Scan(&bonus)
	return bonus
}

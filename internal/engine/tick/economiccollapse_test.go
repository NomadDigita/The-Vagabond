package tick

import (
	"strings"
	"testing"
)

// TestEconomicCollapseWarning verifies AI_PARITY_AND_WORLD_NOTIFICATIONS_
// PLAN.md section 1.5's "communicate this is possible" addition: a
// defeated defender left with a critically low resource total gets an
// explicit warning appended to their battle report, while one still
// holding a meaningful amount doesn't.
func TestEconomicCollapseWarning(t *testing.T) {
	cases := []struct {
		name           string
		remainingTotal float64
		wantWarning    bool
	}{
		{"well above threshold", 500.0, false},
		{"exactly at threshold", economicCollapseWarningThreshold, false},
		{"just below threshold", economicCollapseWarningThreshold - 0.01, true},
		{"effectively zero", 0.0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := economicCollapseWarning(c.remainingTotal)
			hasWarning := strings.Contains(got, "ECONOMIC COLLAPSE")
			if hasWarning != c.wantWarning {
				t.Errorf("remainingTotal=%.2f: expected warning=%v, got warning=%v (text: %q)", c.remainingTotal, c.wantWarning, hasWarning, got)
			}
		})
	}
}

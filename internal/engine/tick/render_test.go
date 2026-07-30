package tick

import (
	"testing"
	"time"
)

// TestFormatDurationTick_PicksTheRightUnitTier mirrors
// bot/handlers/render_test.go's coverage for the tick package's own
// duration formatter, used in notification text (road contact deadlines,
// espionage intercept windows, scout mission ETAs).
func TestFormatDurationTick_PicksTheRightUnitTier(t *testing.T) {
	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "0s"},
		{"negative treated as zero", -5 * time.Second, "0s"},
		{"pure seconds", 45 * time.Second, "45s"},
		{"minutes and seconds", 2*time.Minute + 30*time.Second, "2m 30s"},
		{"hours and minutes", 3*time.Hour + 15*time.Minute, "3h 15m"},
		{"days and hours", 2*24*time.Hour + 6*time.Hour, "2d 6h"},
		{"weeks and days", 9 * 24 * time.Hour, "1w 2d"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := formatDurationTick(c.d); got != c.want {
				t.Errorf("formatDurationTick(%v) = %q, want %q", c.d, got, c.want)
			}
		})
	}
}

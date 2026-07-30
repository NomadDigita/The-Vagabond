package handlers

import (
	"testing"
	"time"
)

// TestFormatDuration_PicksTheRightUnitTier verifies the human-friendly
// duration formatter used throughout the game's countdowns (raid ETAs,
// road-contact deadlines, spy timers, scout mission ETAs, etc.) instead
// of a raw second count like "310955s remaining".
func TestFormatDuration_PicksTheRightUnitTier(t *testing.T) {
	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "0s"},
		{"negative treated as zero", -5 * time.Second, "0s"},
		{"pure seconds", 45 * time.Second, "45s"},
		{"minutes and seconds", 2*time.Minute + 30*time.Second, "2m 30s"},
		{"exactly one minute", 60 * time.Second, "1m 0s"},
		{"hours and minutes", 3*time.Hour + 15*time.Minute, "3h 15m"},
		{"exactly one hour", 60 * time.Minute, "1h 0m"},
		{"days and hours", 2*24*time.Hour + 6*time.Hour, "2d 6h"},
		{"the reported bug's actual value (~3.6 days)", 310955 * time.Second, "3d 14h"},
		{"weeks and days", 9 * 24 * time.Hour, "1w 2d"},
		{"exactly one week", 7 * 24 * time.Hour, "1w 0d"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := formatDuration(c.d); got != c.want {
				t.Errorf("formatDuration(%v) = %q, want %q", c.d, got, c.want)
			}
		})
	}
}

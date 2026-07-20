package worldintel

import "testing"

func TestExplorationDiscoveryChance(t *testing.T) {
	tests := []struct {
		scouts int
		want   float64
	}{
		{-4, 0.35},
		{0, 0.35},
		{3, 0.50},
		{99, 0.75},
	}
	for _, tt := range tests {
		if got := ExplorationDiscoveryChance(tt.scouts); got != tt.want {
			t.Fatalf("ExplorationDiscoveryChance(%d) = %.2f, want %.2f", tt.scouts, got, tt.want)
		}
	}
}

func TestRadarWarningMinutes(t *testing.T) {
	if got := RadarWarningMinutes(0, 4, 5, 5, "direct"); got != 0 {
		t.Fatalf("zero-length march warning = %.2f, want 0", got)
	}
	base := RadarWarningMinutes(120, 0, 0, 0, "direct")
	strong := RadarWarningMinutes(120, 3, 2, 1, "direct")
	stealth := RadarWarningMinutes(120, 3, 2, 1, "stealth")
	if base <= 0 {
		t.Fatalf("basic radar warning = %.2f, want positive", base)
	}
	if strong <= base {
		t.Fatalf("strong radar warning = %.2f, want greater than basic %.2f", strong, base)
	}
	if stealth <= 0 || stealth >= strong {
		t.Fatalf("stealth warning = %.2f, want positive and below direct %.2f", stealth, strong)
	}
	if got := RadarWarningMinutes(20, 99, 99, 99, "direct"); got > 10 {
		t.Fatalf("warning = %.2f, must not exceed half the march", got)
	}
}

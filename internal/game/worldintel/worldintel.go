package worldintel

import "math"

const RogueDroneNestKey = "ai_drone_nest"

func ExplorationDiscoveryChance(scouts int) float64 {
	if scouts < 0 {
		scouts = 0
	}
	return math.Min(0.45+float64(scouts)*0.05, 0.80)
}

func RadarWarningMinutes(baseMarchMinutes float64, radarLevel, scouts, observers int, routeType string) float64 {
	if baseMarchMinutes <= 0 {
		return 0
	}
	if radarLevel < 0 {
		radarLevel = 0
	}
	if scouts < 0 {
		scouts = 0
	}
	if observers < 0 {
		observers = 0
	}

	warning := 8.0 + float64(radarLevel)*6.0 + float64(scouts)*2.0 + float64(observers)*3.0
	if routeType == "stealth" {
		warning *= 0.40
	}

	warning = math.Min(warning, baseMarchMinutes*0.50)
	if routeType != "stealth" {
		warning = math.Max(warning, math.Min(5.0, baseMarchMinutes*0.50))
	}
	return warning
}

// Package worldintel owns the small, deterministic rules that govern what an
// outpost is allowed to know about the wider world. Database reads/writes stay
// in handlers and tick passes; keeping balance math here makes it testable
// without Telegram or Postgres.
package worldintel

import "math"

const RogueDroneNestKey = "ai_drone_nest"

// ExplorationDiscoveryChance returns the probability that a completed
// exploration dispatch reveals a previously unknown target. Scout Walkers are
// the expedition's survey capability, so each one helps, but the probability
// has a hard cap to keep discovery meaningful rather than guaranteed.
func ExplorationDiscoveryChance(scouts int) float64 {
	if scouts < 0 {
		scouts = 0
	}
	return math.Min(0.35+float64(scouts)*0.05, 0.75)
}

// RadarWarningMinutes returns how much remaining travel time a defender gets
// when its sensors first identify an incoming column. Stronger radar and home
// recon units improve warning distance. Stealth routes reduce, rather than
// erase, detection: capable defenses should always retain a narrow chance to
// see a hostile force before impact.
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

	// Never reveal before half of a journey has elapsed, and ensure a
	// non-stealth march has at least a small actionable warning window.
	warning = math.Min(warning, baseMarchMinutes*0.50)
	if routeType != "stealth" {
		warning = math.Max(warning, math.Min(5.0, baseMarchMinutes*0.50))
	}
	return warning
}

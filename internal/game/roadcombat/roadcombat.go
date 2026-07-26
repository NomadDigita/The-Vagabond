// Package roadcombat owns the deterministic, testable rules for Phase 3/4 of
// MMO_WORLD_EVOLUTION_PLAN.md: turning a timer-only march into a route-aware
// journey, and resolving a field battle when two expeditions meet on the
// road. Like internal/game/worldintel, this package holds pure policy only -
// no database or Telegram access - so balance changes can be tested without
// a live bot.
package roadcombat

import (
	"math"
	"time"
)

// EncounterRadius is how close (in world coordinate units) two moving
// expeditions must be before a road encounter can trigger.
const EncounterRadius = 3.0

// EncounterRollChance is the per-tick probability that two expeditions
// within EncounterRadius of each other actually notice one another, once
// eligible. Kept below 1.0 so a slow-moving convoy doesn't guarantee an
// encounter the instant it enters range, and so parties that choose
// "Continue" aren't immediately re-rolled into another encounter next tick.
const EncounterRollChance = 0.35

// EncounterCooldown is how long two expeditions that just resolved an
// encounter (by fighting or passing) are exempt from re-rolling against each
// other, so a "Continue" choice doesn't just get re-asked next tick.
const EncounterCooldown = 4 * time.Minute

// ResponseWindow is how long both commanders have to choose Attack/Continue
// before the encounter auto-resolves as a peaceful pass.
const ResponseWindow = 3 * time.Minute

// Position is a world coordinate.
type Position struct {
	X float64
	Y float64
}

// RouteProgress returns how far through the CURRENT leg (outbound march or
// return march) an expedition has travelled, as a 0..1 fraction. It is
// deliberately based on the leg's own start time and planned duration
// rather than the mutable resolve_time, so a later delay (fuel-depletion
// pause, encounter freeze) cannot make an army's position snap backward -
// progress simply holds at whatever fraction it had reached and does not
// exceed 1.0.
func RouteProgress(legStartedAt time.Time, legTotalMinutes float64, now time.Time) float64 {
	if legTotalMinutes <= 0 {
		return 1.0
	}
	elapsedMinutes := now.Sub(legStartedAt).Minutes()
	if elapsedMinutes <= 0 {
		return 0.0
	}
	progress := elapsedMinutes / legTotalMinutes
	if progress > 1.0 {
		return 1.0
	}
	return progress
}

// CurrentPosition interpolates an expedition's current world position along
// its stored route snapshot. During "marching" the army moves origin ->
// destination; during "returning" it moves destination -> origin (heading
// home along the same corridor). Any other state is treated as stationary
// at the destination (matches "engaged", where the force has arrived).
func CurrentPosition(state string, originX, originY, destX, destY int, progress float64) Position {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	ox, oy, dx, dy := float64(originX), float64(originY), float64(destX), float64(destY)

	switch state {
	case "marching":
		return Position{X: ox + (dx-ox)*progress, Y: oy + (dy-oy)*progress}
	case "returning":
		return Position{X: dx + (ox-dx)*progress, Y: dy + (oy-dy)*progress}
	default:
		return Position{X: dx, Y: dy}
	}
}

// Distance returns the straight-line distance between two positions.
func Distance(a, b Position) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return math.Sqrt(dx*dx + dy*dy)
}

// InEncounterRange reports whether two moving expeditions are close enough
// to be eligible for a road encounter roll.
func InEncounterRange(a, b Position) bool {
	return Distance(a, b) <= EncounterRadius
}

// FieldForce is the subset of a raid_forces row relevant to a mobile
// field battle: no base, no defense grid, no turrets - just what each
// commander actually marched out with, plus the tech/supply modifiers that
// travel with the column itself.
type FieldForce struct {
	Soldiers       int
	Mechs          int
	Destroyers     int
	Bombers        int
	Battlecruisers int
	Deathstars     int
	Liberators     int
	Wraiths        int

	MilitaryTechLvl int  // attacker-side mech multiplier input, floor 1
	SuppliesOut     bool // rations/ammo both depleted -> offense penalty, mirrors base-raid rule
	HighTechOffline bool // electricity/logistics depleted -> mech/capital tech multiplier disabled (Phase 5 milestone 4), distinct from SuppliesOut's blanket penalty
}

// TotalUnits returns the combined headcount of every combat-relevant unit
// type in the force (used for both the 20-unit road-battle minimum and
// casualty-pool splitting).
func (f FieldForce) TotalUnits() int {
	return f.Soldiers + f.Mechs + f.Destroyers + f.Bombers + f.Battlecruisers + f.Deathstars + f.Liberators + f.Wraiths
}

// unitAttackRatings mirrors the flat per-unit attack contributions used by
// the base-raid combat resolver in internal/engine/tick/engine.go
// (resolveRaidCombats), so a field battle "feels" consistent with a raid
// against a real base rather than introducing a disconnected second power
// curve. Soldiers/Mechs keep their existing 15.0 base multiplier; capital
// units use content.Units' canonical AttackRating values, duplicated here
// as plain constants (this package intentionally has zero dependencies, so
// callers pass in only the numbers, not a content.Unit) to keep it testable
// without importing the content package.
const (
	baseUnitRating      = 15.0
	destroyerRating     = 20.0
	bomberRating        = 18.0
	liberatorRating     = 35.0
	wraithRating        = 25.0
	battlecruiserRating = 60.0
	deathstarRating     = 650.0
)

// Power computes a force's raw combat rating for a field battle. Unlike a
// base raid there is no defense grid, turret counter-play, or biome/weather
// term here - those are base-specific; two marching columns fight on equal
// terrain. Weather is still applied by the caller as part of offenseMod,
// since it affects a field battle exactly as it affects a march.
func Power(f FieldForce, offenseMod float64) float64 {
	if offenseMod <= 0 {
		offenseMod = 1.0
	}
	if f.SuppliesOut {
		offenseMod *= 0.50
	}

	techLvl := f.MilitaryTechLvl
	if techLvl < 1 {
		techLvl = 1
	}
	mechMultiplier := 1.50 * (1.0 + float64(techLvl-1)*0.25)
	capitalTechMultiplier := 1.0
	if f.HighTechOffline {
		// High-tech contributions offline: mechs fight as unenhanced
		// infantry (no multiplier) and capital units lose their tech
		// edge, but nobody gets a blanket offense penalty the way
		// SuppliesOut applies - this is "disables high-tech
		// contributions," not "the whole column can't fight."
		mechMultiplier = 1.0
		capitalTechMultiplier = 0.70
	}

	rating := float64(f.Soldiers) * baseUnitRating * offenseMod
	rating += float64(f.Mechs) * baseUnitRating * offenseMod * mechMultiplier
	rating += float64(f.Destroyers) * destroyerRating * offenseMod * capitalTechMultiplier
	rating += float64(f.Bombers) * bomberRating * offenseMod * capitalTechMultiplier
	rating += float64(f.Liberators) * liberatorRating * offenseMod * capitalTechMultiplier
	rating += float64(f.Wraiths) * wraithRating * offenseMod * capitalTechMultiplier
	rating += float64(f.Battlecruisers) * battlecruiserRating * offenseMod * capitalTechMultiplier
	rating += float64(f.Deathstars) * deathstarRating * offenseMod * capitalTechMultiplier
	return rating
}

// BattleResult is the outcome of a symmetric field battle between two
// mobile forces.
type BattleResult struct {
	AWon              bool
	Draw              bool
	APowerRating      float64
	BPowerRating      float64
	ACasualtyFraction float64 // fraction of A's TotalUnits() destroyed
	BCasualtyFraction float64 // fraction of B's TotalUnits() destroyed
}

// ResolveBattle applies the same win/lose casualty shape used for base
// raids (winner ~10-12% losses, loser ~30-35% losses) to a symmetric
// mobile-vs-mobile fight. A draw (power within 5% of each other) applies
// moderate losses to both sides and awards no cargo.
func ResolveBattle(aPower, bPower float64) BattleResult {
	res := BattleResult{APowerRating: aPower, BPowerRating: bPower}

	if aPower <= 0 && bPower <= 0 {
		res.Draw = true
		return res
	}

	higher, lower := aPower, bPower
	if lower > higher {
		higher, lower = lower, higher
	}
	if higher > 0 && (higher-lower)/higher < 0.05 {
		res.Draw = true
		res.ACasualtyFraction = 0.18
		res.BCasualtyFraction = 0.18
		return res
	}

	if aPower > bPower {
		res.AWon = true
		res.ACasualtyFraction = 0.12
		res.BCasualtyFraction = 0.35
	} else {
		res.AWon = false
		res.ACasualtyFraction = 0.35
		res.BCasualtyFraction = 0.12
	}
	return res
}

// CasualtiesFor splits a fractional casualty rate across a force's unit
// composition, proportional to headcount within each pool (soldier/mech
// pool vs. specialist pool), mirroring the base-raid casualty split so a
// road loss "feels" the same shape as a raid loss. Returns per-type losses,
// each capped at the force's actual holdings.
func CasualtiesFor(f FieldForce, casualtyFraction float64) FieldForce {
	if casualtyFraction < 0 {
		casualtyFraction = 0
	}
	if casualtyFraction > 1 {
		casualtyFraction = 1
	}

	lost := FieldForce{}
	lost.Soldiers = int(float64(f.Soldiers) * casualtyFraction)
	lost.Mechs = int(float64(f.Mechs) * casualtyFraction)
	lost.Destroyers = int(float64(f.Destroyers) * casualtyFraction)
	lost.Bombers = int(float64(f.Bombers) * casualtyFraction)
	lost.Liberators = int(float64(f.Liberators) * casualtyFraction)
	lost.Wraiths = int(float64(f.Wraiths) * casualtyFraction)
	lost.Battlecruisers = int(float64(f.Battlecruisers) * casualtyFraction)
	lost.Deathstars = int(float64(f.Deathstars) * casualtyFraction)

	// If there were any units at all in a pool and the fraction is
	// meaningfully non-zero, guarantee at least one casualty so a clear
	// loss never rounds down to "nobody died".
	if casualtyFraction >= 0.10 {
		if f.Soldiers > 0 && lost.Soldiers == 0 {
			lost.Soldiers = 1
		}
	}

	return lost
}

// Survivors returns f minus lost, floored at zero per field.
func Survivors(f, lost FieldForce) FieldForce {
	clampSub := func(a, b int) int {
		v := a - b
		if v < 0 {
			return 0
		}
		return v
	}
	return FieldForce{
		Soldiers:        clampSub(f.Soldiers, lost.Soldiers),
		Mechs:           clampSub(f.Mechs, lost.Mechs),
		Destroyers:      clampSub(f.Destroyers, lost.Destroyers),
		Bombers:         clampSub(f.Bombers, lost.Bombers),
		Liberators:      clampSub(f.Liberators, lost.Liberators),
		Wraiths:         clampSub(f.Wraiths, lost.Wraiths),
		Battlecruisers:  clampSub(f.Battlecruisers, lost.Battlecruisers),
		Deathstars:      clampSub(f.Deathstars, lost.Deathstars),
		MilitaryTechLvl: f.MilitaryTechLvl,
		HighTechOffline: f.HighTechOffline,
	}
}

// CargoShare is capacity-limited cargo a road-battle winner can capture
// from the loser's carried loot. Real warfare doesn't let a small patrol
// walk off with a fully loaded convoy's entire hold, so capture is capped
// as a fraction of what the loser is carrying, further capped by the
// winner's own free carrying capacity if provided (capacityLimit <= 0 means
// "no additional cap beyond the capture fraction").
const CargoCaptureFraction = 0.40

func CargoShare(loserCarried, capacityLimit float64) float64 {
	if loserCarried <= 0 {
		return 0
	}
	share := loserCarried * CargoCaptureFraction
	if capacityLimit > 0 && share > capacityLimit {
		share = capacityLimit
	}
	return share
}

// --- Phase 5: route weather incidents and reinforcement convoys ---
//
// These live in the same package as the route/field-battle policy above
// because they are governed by the same physical model (a column at a
// point on the route, blocked by something external, resumed via the
// paused_at/leg_started_at shift) rather than because they are
// thematically identical to combat.

// IncidentBaseRollChance is the per-tick chance of a local weather
// incident on a column travelling through a continent with no matching
// active regional event - freak weather can still happen on a clear day.
const IncidentBaseRollChance = 0.02

// IncidentElevatedRollChance is the per-tick chance once the column's
// current continent has a matching active world_events entry (e.g. an
// acid_rain event raising flood odds).
const IncidentElevatedRollChance = 0.12

// IncidentMatchesActiveWeather reports whether an active continent-wide
// world event (from internal/engine/world) makes a given local route
// incident type more likely, so Phase 5 reads the existing weather engine
// as an input signal instead of rolling a second, disconnected one.
func IncidentMatchesActiveWeather(activeEventType, incidentType string) bool {
	switch activeEventType {
	case "acid_rain":
		return incidentType == "flood"
	case "radiation_storm":
		return incidentType == "radiation" || incidentType == "storm"
	case "emp":
		return incidentType == "emp" || incidentType == "storm"
	case "sandstorm":
		return incidentType == "sandstorm" || incidentType == "heatwave"
	default:
		return false
	}
}

// IncidentDuration maps a rolled severity (1..3) to how long the temporary
// camp must hold before conditions clear, matching "which might take a day
// or even longer" - a minor incident clears in about half a day, a severe
// one can run a day and a half.
func IncidentDuration(severity int) time.Duration {
	switch severity {
	case 3:
		return 36 * time.Hour
	case 2:
		return 24 * time.Hour
	default:
		return 12 * time.Hour
	}
}

// ConvoySpeedCoordinateUnitsPerMinute is how fast a dedicated resupply
// convoy covers ground - deliberately slower than a combat column's
// implicit speed (a marching column's minutes are balance-tuned per raid,
// not distance-derived) so a convoy is a real logistics commitment, not a
// courier that outruns the very expedition it's chasing.
const ConvoySpeedCoordinateUnitsPerMinute = 0.5

// MinConvoyTravelMinutes floors convoy travel time so a resupply run to an
// adjacent tile still takes a meaningful, non-instant amount of time.
const MinConvoyTravelMinutes = 10.0

// ConvoyTravelMinutes converts a straight-line distance into travel time
// for a dedicated supply convoy.
func ConvoyTravelMinutes(distance float64) float64 {
	minutes := distance / ConvoySpeedCoordinateUnitsPerMinute
	if minutes < MinConvoyTravelMinutes {
		return MinConvoyTravelMinutes
	}
	return minutes
}

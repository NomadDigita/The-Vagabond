package roadcombat

import (
	"math"
	"time"
)

const EncounterRadius = 3.0

const EncounterRollChance = 0.35

const EncounterCooldown = 4 * time.Minute

const ResponseWindow = 3 * time.Minute

type Position struct {
	X float64
	Y float64
}

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

func Distance(a, b Position) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return math.Sqrt(dx*dx + dy*dy)
}

func InEncounterRange(a, b Position) bool {
	return Distance(a, b) <= EncounterRadius
}

type FieldForce struct {
	Soldiers       int
	Mechs          int
	Destroyers     int
	Bombers        int
	Battlecruisers int
	Deathstars     int
	Liberators     int
	Wraiths        int

	MilitaryTechLvl int
	SuppliesOut     bool
	HighTechOffline bool
}

func (f FieldForce) TotalUnits() int {
	return f.Soldiers + f.Mechs + f.Destroyers + f.Bombers + f.Battlecruisers + f.Deathstars + f.Liberators + f.Wraiths
}

const (
	baseUnitRating      = 15.0
	destroyerRating     = 20.0
	bomberRating        = 18.0
	liberatorRating     = 35.0
	wraithRating        = 25.0
	battlecruiserRating = 60.0
	deathstarRating     = 650.0
)

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

type BattleResult struct {
	AWon              bool
	Draw              bool
	APowerRating      float64
	BPowerRating      float64
	ACasualtyFraction float64
	BCasualtyFraction float64
}

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

	if casualtyFraction >= 0.10 {
		if f.Soldiers > 0 && lost.Soldiers == 0 {
			lost.Soldiers = 1
		}
	}

	return lost
}

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

const IncidentBaseRollChance = 0.02

const IncidentElevatedRollChance = 0.12

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

const ConvoySpeedCoordinateUnitsPerMinute = 0.5

const MinConvoyTravelMinutes = 10.0

func ConvoyTravelMinutes(distance float64) float64 {
	minutes := distance / ConvoySpeedCoordinateUnitsPerMinute
	if minutes < MinConvoyTravelMinutes {
		return MinConvoyTravelMinutes
	}
	return minutes
}

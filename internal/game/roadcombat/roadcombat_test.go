package roadcombat

import (
	"testing"
	"time"
)

func TestRouteProgressClampsAndAdvances(t *testing.T) {
	start := time.Now().Add(-5 * time.Minute)
	p := RouteProgress(start, 10.0, time.Now())
	if p < 0.45 || p > 0.55 {
		t.Fatalf("expected ~0.5 progress, got %.2f", p)
	}

	pDone := RouteProgress(start, 4.0, time.Now())
	if pDone != 1.0 {
		t.Fatalf("expected progress to clamp at 1.0, got %.2f", pDone)
	}

	pNotStarted := RouteProgress(time.Now().Add(1*time.Minute), 10.0, time.Now())
	if pNotStarted != 0.0 {
		t.Fatalf("expected 0 progress before leg start, got %.2f", pNotStarted)
	}
}

func TestCurrentPositionMarchingAndReturning(t *testing.T) {
	origin := Position{X: 0, Y: 0}
	dest := Position{X: 10, Y: 0}

	half := CurrentPosition("marching", int(origin.X), int(origin.Y), int(dest.X), int(dest.Y), 0.5)
	if half.X != 5 {
		t.Fatalf("expected marching midpoint x=5, got %.2f", half.X)
	}

	// Returning at the same 0.5 progress should be symmetric (also midpoint),
	// but returning progress=0 should sit AT the destination (just turned
	// around) and progress=1 should be back at origin.
	justTurned := CurrentPosition("returning", int(origin.X), int(origin.Y), int(dest.X), int(dest.Y), 0.0)
	if justTurned.X != 10 {
		t.Fatalf("expected returning start at destination x=10, got %.2f", justTurned.X)
	}
	arrived := CurrentPosition("returning", int(origin.X), int(origin.Y), int(dest.X), int(dest.Y), 1.0)
	if arrived.X != 0 {
		t.Fatalf("expected returning end at origin x=0, got %.2f", arrived.X)
	}
}

func TestInEncounterRange(t *testing.T) {
	a := Position{X: 0, Y: 0}
	near := Position{X: 2, Y: 0}
	far := Position{X: 50, Y: 0}

	if !InEncounterRange(a, near) {
		t.Fatalf("expected positions 2 apart to be in encounter range")
	}
	if InEncounterRange(a, far) {
		t.Fatalf("expected distant positions to be out of encounter range")
	}
}

func TestPowerScalesWithForceAndSupplyPenalty(t *testing.T) {
	strong := FieldForce{Soldiers: 100, Mechs: 20, MilitaryTechLvl: 3}
	weak := FieldForce{Soldiers: 20, Mechs: 2, MilitaryTechLvl: 1}

	if Power(strong, 1.0) <= Power(weak, 1.0) {
		t.Fatalf("expected stronger force to have higher power rating")
	}

	starved := strong
	starved.SuppliesOut = true
	if Power(starved, 1.0) >= Power(strong, 1.0) {
		t.Fatalf("expected supply-depleted force to have a lower power rating")
	}
}

func TestResolveBattleWinnerAndDraw(t *testing.T) {
	res := ResolveBattle(1000, 500)
	if !res.AWon || res.Draw {
		t.Fatalf("expected A to win a 2:1 power fight, got %+v", res)
	}
	if res.ACasualtyFraction >= res.BCasualtyFraction {
		t.Fatalf("expected winner to lose fewer proportionally, got %+v", res)
	}

	draw := ResolveBattle(1000, 990)
	if !draw.Draw {
		t.Fatalf("expected near-equal power to draw, got %+v", draw)
	}

	nobody := ResolveBattle(0, 0)
	if !nobody.Draw {
		t.Fatalf("expected zero-power fight to resolve as a draw, got %+v", nobody)
	}
}

func TestCasualtiesForAndSurvivors(t *testing.T) {
	force := FieldForce{Soldiers: 100, Mechs: 10, Battlecruisers: 2, MilitaryTechLvl: 2}
	lost := CasualtiesFor(force, 0.30)
	if lost.Soldiers != 30 {
		t.Fatalf("expected 30 soldier losses at 30%% casualties, got %d", lost.Soldiers)
	}

	survivors := Survivors(force, lost)
	if survivors.Soldiers != 70 {
		t.Fatalf("expected 70 soldier survivors, got %d", survivors.Soldiers)
	}
	if survivors.MilitaryTechLvl != 2 {
		t.Fatalf("expected MilitaryTechLvl to carry through Survivors, got %d", survivors.MilitaryTechLvl)
	}

	// Casualties can never exceed what the force actually had.
	tiny := FieldForce{Soldiers: 2}
	lostTiny := CasualtiesFor(tiny, 1.0)
	survTiny := Survivors(tiny, lostTiny)
	if survTiny.Soldiers != 0 {
		t.Fatalf("expected full wipeout at 100%% casualties, got %d survivors", survTiny.Soldiers)
	}
}

func TestCargoShareCapsAtCaptureFractionAndCapacity(t *testing.T) {
	share := CargoShare(1000, 0)
	if share != 400 {
		t.Fatalf("expected 40%% capture of 1000 = 400, got %.2f", share)
	}

	capped := CargoShare(1000, 100)
	if capped != 100 {
		t.Fatalf("expected capacity cap of 100 to apply, got %.2f", capped)
	}

	if CargoShare(0, 100) != 0 {
		t.Fatalf("expected zero cargo share when loser carries nothing")
	}
}

func TestIncidentMatchesActiveWeather(t *testing.T) {
	cases := []struct {
		active, incident string
		want             bool
	}{
		{"acid_rain", "flood", true},
		{"acid_rain", "storm", false},
		{"radiation_storm", "storm", true},
		{"emp", "storm", true},
		{"sandstorm", "heatwave", true},
		{"solar_flare", "flood", false},
		{"", "flood", false},
	}
	for _, tc := range cases {
		if got := IncidentMatchesActiveWeather(tc.active, tc.incident); got != tc.want {
			t.Fatalf("IncidentMatchesActiveWeather(%q, %q) = %v, want %v", tc.active, tc.incident, got, tc.want)
		}
	}
}

func TestIncidentDurationScalesWithSeverity(t *testing.T) {
	if IncidentDuration(1) != 12*time.Hour {
		t.Fatalf("expected severity 1 to be 12h, got %v", IncidentDuration(1))
	}
	if IncidentDuration(2) != 24*time.Hour {
		t.Fatalf("expected severity 2 to be 24h, got %v", IncidentDuration(2))
	}
	if IncidentDuration(3) != 36*time.Hour {
		t.Fatalf("expected severity 3 to be 36h, got %v", IncidentDuration(3))
	}
}

func TestConvoyTravelMinutesFloorsAtMinimum(t *testing.T) {
	if ConvoyTravelMinutes(1) != MinConvoyTravelMinutes {
		t.Fatalf("expected short distance to floor at %v minutes, got %v", MinConvoyTravelMinutes, ConvoyTravelMinutes(1))
	}
	long := ConvoyTravelMinutes(100)
	if long <= MinConvoyTravelMinutes {
		t.Fatalf("expected long distance to exceed the floor, got %v", long)
	}
}

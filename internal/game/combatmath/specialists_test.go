package combatmath_test

import (
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/game/combatmath"
)

var realToughness = combatmath.SpecialistToughness{
	Destroyers:     4.0,
	Bombers:        4.0,
	Battlecruisers: 20.0,
	DoomsdayRigs:   200.0,
	Liberators:     8.0,
	Wraiths:        3.0,
}

func TestAllocateSpecialistLosses_ReproducesReportedScenario(t *testing.T) {
	counts := combatmath.SpecialistCounts{
		Destroyers:     14,
		Bombers:        5,
		Battlecruisers: 30,
		DoomsdayRigs:   4,
		Liberators:     34,
		Wraiths:        32,
	}

	const totalCasualties = 1000
	lost := combatmath.AllocateSpecialistLosses(totalCasualties, counts, realToughness)

	oldHeadcountShare := float64(totalCasualties) * float64(counts.DoomsdayRigs) / float64(counts.Total())
	if float64(lost.DoomsdayRigs) >= oldHeadcountShare {
		t.Fatalf("expected Doomsday Rig losses (%d) to be well below the old headcount-only share (%.1f), toughness gave it no protection", lost.DoomsdayRigs, oldHeadcountShare)
	}

	if lost.DoomsdayRigs > 5 {
		t.Errorf("expected only a handful of the 1000 casualties to land on the 200-toughness Doomsday Rigs, got %d", lost.DoomsdayRigs)
	}
}

func TestAllocateSpecialistLosses_TougherUnitDiesLessOftenThanFragileOne(t *testing.T) {
	counts := combatmath.SpecialistCounts{Destroyers: 100, DoomsdayRigs: 100}
	toughness := combatmath.SpecialistToughness{Destroyers: 4.0, DoomsdayRigs: 200.0}

	lost := combatmath.AllocateSpecialistLosses(100, counts, toughness)

	if lost.DoomsdayRigs >= lost.Destroyers {
		t.Fatalf("expected the far-tougher Doomsday Rig to lose fewer units than the Destroyer at equal headcount, got Destroyers=%d DoomsdayRigs=%d", lost.Destroyers, lost.DoomsdayRigs)
	}
}

func TestAllocateSpecialistLosses_NeverExceedsFieldedCount(t *testing.T) {
	counts := combatmath.SpecialistCounts{Destroyers: 2, Bombers: 1}
	toughness := combatmath.SpecialistToughness{Destroyers: 4.0, Bombers: 4.0}

	lost := combatmath.AllocateSpecialistLosses(1000, counts, toughness)

	if lost.Destroyers > counts.Destroyers {
		t.Errorf("Destroyers: lost %d exceeds fielded %d", lost.Destroyers, counts.Destroyers)
	}
	if lost.Bombers > counts.Bombers {
		t.Errorf("Bombers: lost %d exceeds fielded %d", lost.Bombers, counts.Bombers)
	}
}

func TestAllocateSpecialistLosses_TotalIsConserved(t *testing.T) {
	counts := combatmath.SpecialistCounts{
		Destroyers:     14,
		Bombers:        5,
		Battlecruisers: 30,
		DoomsdayRigs:   4,
		Liberators:     34,
		Wraiths:        32,
	}

	for total := 0; total <= 40; total++ {
		lost := combatmath.AllocateSpecialistLosses(total, counts, realToughness)
		got := lost.Destroyers + lost.Bombers + lost.Battlecruisers + lost.DoomsdayRigs + lost.Liberators + lost.Wraiths
		if got != total {
			t.Fatalf("total=%d: expected losses to sum to %d, got %d (%+v)", total, total, got, lost)
		}
	}
}

func TestAllocateSpecialistLosses_ZeroOrNegativeInputsAreSafe(t *testing.T) {
	if lost := combatmath.AllocateSpecialistLosses(0, combatmath.SpecialistCounts{DoomsdayRigs: 4}, realToughness); lost != (combatmath.SpecialistCounts{}) {
		t.Errorf("expected zero casualties to produce zero losses, got %+v", lost)
	}
	if lost := combatmath.AllocateSpecialistLosses(10, combatmath.SpecialistCounts{}, realToughness); lost != (combatmath.SpecialistCounts{}) {
		t.Errorf("expected an empty pool to produce zero losses, got %+v", lost)
	}
	if lost := combatmath.AllocateSpecialistLosses(-5, combatmath.SpecialistCounts{DoomsdayRigs: 4}, realToughness); lost != (combatmath.SpecialistCounts{}) {
		t.Errorf("expected negative totalCasualties to produce zero losses, got %+v", lost)
	}
}

func TestAllocateSpecialistLosses_ZeroToughnessDoesNotPanic(t *testing.T) {
	counts := combatmath.SpecialistCounts{Destroyers: 10, Bombers: 10}
	toughness := combatmath.SpecialistToughness{Destroyers: 4.0, Bombers: 0}

	lost := combatmath.AllocateSpecialistLosses(5, counts, toughness)
	if lost.Bombers != 0 {
		t.Errorf("expected a zero-toughness type to receive zero hazard share, got Bombers=%d", lost.Bombers)
	}
	if lost.Destroyers != 5 {
		t.Errorf("expected all casualties to fall on the only type with valid toughness, got Destroyers=%d", lost.Destroyers)
	}
}

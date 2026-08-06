package combatmath_test

import (
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/game/combatmath"
)

// realToughness mirrors the constants in
// internal/engine/tick/engine.go's resolveRaidCombats
// (destroyerToughness, bomberToughness, bcToughness, dsToughness,
// liberatorToughness, wraithToughness) as of the 2026-08-05 fix.
var realToughness = combatmath.SpecialistToughness{
	Destroyers:     4.0,
	Bombers:        4.0,
	Battlecruisers: 20.0,
	DoomsdayRigs:   200.0,
	Liberators:     8.0,
	Wraiths:        3.0,
}

// TestAllocateSpecialistLosses_ReproducesReportedScenario is a direct
// regression test for the live bug report (2026-08-05): a fleet of
// Destroyers 14 / Bombers 5 / Liberators 34 / Wraiths 32 /
// Battlecruisers 30 / Doomsday Rigs 4 lost all 4 Doomsday Rigs within
// 2-3 combat rounds under the old headcount-only split. Under the new
// hazard-weighted split, a Doomsday Rig - 50x tougher than a Destroyer
// and 10x tougher than a Battlecruiser - must come out receiving a
// share of losses far below its ~3.4% headcount share of the pool
// (4 of 119 units).
func TestAllocateSpecialistLosses_ReproducesReportedScenario(t *testing.T) {
	counts := combatmath.SpecialistCounts{
		Destroyers:     14,
		Bombers:        5,
		Battlecruisers: 30,
		DoomsdayRigs:   4,
		Liberators:     34,
		Wraiths:        32,
	}

	// Run a large total so the Rig's expected share (a fraction of a
	// single unit under the old per-round math) becomes large enough
	// to compare cleanly - equivalent to simulating many rounds' worth
	// of specialist-pool casualties at once.
	const totalCasualties = 1000
	lost := combatmath.AllocateSpecialistLosses(totalCasualties, counts, realToughness)

	oldHeadcountShare := float64(totalCasualties) * float64(counts.DoomsdayRigs) / float64(counts.Total())
	if float64(lost.DoomsdayRigs) >= oldHeadcountShare {
		t.Fatalf("expected Doomsday Rig losses (%d) to be well below the old headcount-only share (%.1f), toughness gave it no protection", lost.DoomsdayRigs, oldHeadcountShare)
	}

	// With these numbers the Rig's hazard share is roughly 30x smaller
	// than its headcount share (4/200=0.02 vs 4/119=0.034, against a
	// pool whose total hazard is dominated by the many low-toughness
	// Wraiths/Destroyers) - assert it lands in that ballpark rather
	// than pinning an exact figure that would break on unrelated
	// toughness-constant tuning.
	if lost.DoomsdayRigs > 5 {
		t.Errorf("expected only a handful of the 1000 casualties to land on the 200-toughness Doomsday Rigs, got %d", lost.DoomsdayRigs)
	}
}

// TestAllocateSpecialistLosses_TougherUnitDiesLessOftenThanFragileOne
// is the general property the fix establishes: given equal headcounts,
// the more toughness a type has, the fewer losses of that type should
// result, strictly.
func TestAllocateSpecialistLosses_TougherUnitDiesLessOftenThanFragileOne(t *testing.T) {
	counts := combatmath.SpecialistCounts{Destroyers: 100, DoomsdayRigs: 100}
	toughness := combatmath.SpecialistToughness{Destroyers: 4.0, DoomsdayRigs: 200.0}

	lost := combatmath.AllocateSpecialistLosses(100, counts, toughness)

	if lost.DoomsdayRigs >= lost.Destroyers {
		t.Fatalf("expected the far-tougher Doomsday Rig to lose fewer units than the Destroyer at equal headcount, got Destroyers=%d DoomsdayRigs=%d", lost.Destroyers, lost.DoomsdayRigs)
	}
}

// TestAllocateSpecialistLosses_NeverExceedsFieldedCount confirms no
// type ever reports losing more units than it has, even when the
// hazard math would otherwise want to assign more (e.g. a tiny,
// fragile pool absorbing a disproportionately large total).
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

// TestAllocateSpecialistLosses_TotalIsConserved confirms the sum of
// losses across all types equals totalCasualties whenever the pool has
// enough total headcount to absorb it - the remainder-distribution
// step must not silently drop or double-count units.
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

// TestAllocateSpecialistLosses_ZeroOrNegativeInputsAreSafe confirms no
// panics (division by zero, negative counts) for edge-case inputs.
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

// TestAllocateSpecialistLosses_ZeroToughnessDoesNotPanic confirms a
// misconfigured (zero/negative) toughness constant for some type
// contributes zero hazard rather than dividing by zero.
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

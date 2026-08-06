// Package combatmath holds pure, DB-free combat math extracted out of
// internal/engine/tick's raid-resolution loop specifically so it can be
// unit-tested directly - the tick engine itself is DB-heavy throughout
// and, per this project's conventions, is exercised only via
// real-Postgres tests, which makes a narrow arithmetic bug like the one
// this file fixes hard to pin down with a fast, deterministic test.
package combatmath

// SpecialistCounts is how many of each specialist unit type a side is
// fielding (or has lost), keyed by the same names used throughout
// internal/engine/tick's raid combat resolution.
type SpecialistCounts struct {
	Destroyers     int
	Bombers        int
	Battlecruisers int
	DoomsdayRigs   int
	Liberators     int
	Wraiths        int
}

// Total sums every field.
func (c SpecialistCounts) Total() int {
	return c.Destroyers + c.Bombers + c.Battlecruisers + c.DoomsdayRigs + c.Liberators + c.Wraiths
}

// SpecialistToughness is how many "effective casualties" it takes to
// destroy one unit of each type - the constants
// destroyerToughness/bomberToughness/bcToughness/dsToughness/
// liberatorToughness/wraithToughness from
// internal/engine/tick/engine.go, unchanged by this fix; only how
// they're used to split losses changes here.
type SpecialistToughness struct {
	Destroyers     float64
	Bombers        float64
	Battlecruisers float64
	DoomsdayRigs   float64
	Liberators     float64
	Wraiths        float64
}

// AllocateSpecialistLosses splits totalCasualties (an already-computed
// count of how many specialist units die this round in total - see
// effectiveDbCas in internal/engine/tick/engine.go, unchanged by this
// fix) among the six specialist unit types, weighted by each type's
// own toughness rather than by raw headcount.
//
// BUG THIS FIXES (reported live, 2026-08-05 - a Doomsday Rig, the
// single most expensive unit in the game at 200x a Destroyer's
// toughness, died in 1-2 combat rounds instead of being "almost
// indestructible" as advertised): the previous formula computed one
// pool-wide AVERAGE toughness (a headcount-weighted mean across every
// specialist type) purely to size the total number of casualties, and
// then split that total among types by raw headcount share alone -
// e.g. 4 Doomsday Rigs out of a 119-unit specialist pool got roughly
// 4/119 (~3.4%) of losses, with the Rig's own 200-toughness constant
// never actually protecting an individual Rig's own survival odds
// beyond its small contribution to the shared average. A fleet with
// very few Rigs among many cheap units barely moved that average at
// all, so the Rigs died at essentially the same rate as everything
// else around them - the opposite of what a 50x-tougher unit should
// do.
//
// This function keeps the same overall total (totalCasualties is
// unchanged, still sized upstream via the same pool-average toughness
// logic - that scale is fine and untouched), but distributes it via a
// hazard-weighted share: each type's share of losses is proportional
// to (count / toughness), normalized across the pool, instead of count
// alone. A unit with high toughness now has its OWN survival
// protected in proportion to that toughness, not just diluted into a
// shared average - matching what "toughness" is supposed to mean.
//
// Any remainder left after integer division is assigned to whichever
// type currently has the largest fractional remainder, largest-first,
// so totalCasualties is always fully accounted for (never silently
// dropped or double-counted) and no single type is favored by
// truncation on every call. Each type's result is clamped to its own
// count, so this never reports losing more units than were fielded.
func AllocateSpecialistLosses(totalCasualties int, counts SpecialistCounts, toughness SpecialistToughness) SpecialistCounts {
	if totalCasualties <= 0 || counts.Total() <= 0 {
		return SpecialistCounts{}
	}

	type slot struct {
		count     *int
		lost      *int
		hazard    float64
		exact     float64
		remainder float64
	}

	var lost SpecialistCounts
	slots := []slot{
		{&counts.Destroyers, &lost.Destroyers, hazardOf(counts.Destroyers, toughness.Destroyers), 0, 0},
		{&counts.Bombers, &lost.Bombers, hazardOf(counts.Bombers, toughness.Bombers), 0, 0},
		{&counts.Battlecruisers, &lost.Battlecruisers, hazardOf(counts.Battlecruisers, toughness.Battlecruisers), 0, 0},
		{&counts.DoomsdayRigs, &lost.DoomsdayRigs, hazardOf(counts.DoomsdayRigs, toughness.DoomsdayRigs), 0, 0},
		{&counts.Liberators, &lost.Liberators, hazardOf(counts.Liberators, toughness.Liberators), 0, 0},
		{&counts.Wraiths, &lost.Wraiths, hazardOf(counts.Wraiths, toughness.Wraiths), 0, 0},
	}

	totalHazard := 0.0
	for _, s := range slots {
		totalHazard += s.hazard
	}
	if totalHazard <= 0 {
		return SpecialistCounts{}
	}

	assigned := 0
	for i := range slots {
		slots[i].exact = float64(totalCasualties) * slots[i].hazard / totalHazard
		whole := int(slots[i].exact)
		if whole > *slots[i].count {
			whole = *slots[i].count
		}
		*slots[i].lost = whole
		slots[i].remainder = slots[i].exact - float64(whole)
		assigned += whole
	}

	// Distribute the remainder (totalCasualties - assigned, always a
	// small non-negative number less than len(slots) from truncation)
	// to the largest fractional remainders first, skipping any type
	// already fully depleted.
	remaining := totalCasualties - assigned
	for remaining > 0 {
		bestIdx := -1
		bestRemainder := -1.0
		for i := range slots {
			if *slots[i].lost >= *slots[i].count {
				continue
			}
			if slots[i].remainder > bestRemainder {
				bestRemainder = slots[i].remainder
				bestIdx = i
			}
		}
		if bestIdx == -1 {
			break // every type already fully depleted; can't assign more
		}
		*slots[bestIdx].lost++
		slots[bestIdx].remainder = -1 // don't pick the same slot twice in a row for the same remainder unit
		remaining--
	}

	return lost
}

// hazardOf returns a unit type's share weight: more units of a type,
// or a lower toughness, both increase how much of the total casualty
// pool that type absorbs. A type with zero units or non-positive
// toughness (a misconfigured constant) contributes zero hazard rather
// than dividing by zero or going negative.
func hazardOf(count int, toughness float64) float64 {
	if count <= 0 || toughness <= 0 {
		return 0
	}
	return float64(count) / toughness
}

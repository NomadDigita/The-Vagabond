package combatmath

type SpecialistCounts struct {
	Destroyers     int
	Bombers        int
	Battlecruisers int
	DoomsdayRigs   int
	Liberators     int
	Wraiths        int
}

func (c SpecialistCounts) Total() int {
	return c.Destroyers + c.Bombers + c.Battlecruisers + c.DoomsdayRigs + c.Liberators + c.Wraiths
}

type SpecialistToughness struct {
	Destroyers     float64
	Bombers        float64
	Battlecruisers float64
	DoomsdayRigs   float64
	Liberators     float64
	Wraiths        float64
}

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
			break
		}
		*slots[bestIdx].lost++
		slots[bestIdx].remainder = -1
		remaining--
	}

	return lost
}

func hazardOf(count int, toughness float64) float64 {
	if count <= 0 || toughness <= 0 {
		return 0
	}
	return float64(count) / toughness
}

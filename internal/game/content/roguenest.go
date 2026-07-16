package content

// RogueNestForce is the composition of an AI "Rogue Drone Nest" defense,
// scaled to the attacking player's own outpost level. Higher-level players
// face a correspondingly larger, more varied, better-equipped garrison -
// not just more of the same single unit type.
//
// Phase 7: the Nest no longer fights with a single flat "TurretBonus". It
// now stands up the same subsystems a real player base would - an
// individually-typed Defense Grid, garrison-only Guardians/Observers, a
// research (Integrity Tech) level, nuclear shields, and (at high tiers) a
// hero-equivalent superpower - so it is resolved by the combat engine
// through the exact same code path as a real defender, not a bespoke
// fallback formula. TurretBonus is kept only as a legacy total for any
// display code that hasn't been updated to the per-turret breakdown.
type RogueNestForce struct {
	Soldiers int
	Mechs    int
	Drones   int
	Jets     int

	// Defense Grid: individually-typed turret levels, exactly mirroring
	// a player's `modules` rows of type light_laser/heavy_laser/etc.
	LightLaserLvl   int
	HeavyLaserLvl   int
	GaussCannonLvl  int
	IonCannonLvl    int
	PlasmaTurretLvl int

	// Garrison-only defensive units, mirroring workshop_inventory.guardians/observers.
	Guardians int
	Observers int

	IntegrityTechLvl int // mirrors research_states.integrity_tech_lvl
	Shields          int // mirrors nuclear_shields

	HeroSuperpower string // "" until the Nest is dangerous enough to field a Warlord-equivalent

	TurretBonus float64 // legacy flat total (sum of turret levels * 0.08), kept for older display code only
}

// RogueNestComposition computes the Rogue Drone Nest's defense force for a
// given attacker level. This is deterministic and shared by both the real
// combat resolution and the /recon_ai scouting preview, so what a player
// scouts is exactly what they'll actually fight - no surprises.
func RogueNestComposition(attackerLevel int) RogueNestForce {
	if attackerLevel < 1 {
		attackerLevel = 1
	}

	f := RogueNestForce{
		Soldiers: attackerLevel * 15,
	}

	if attackerLevel >= 5 {
		f.Mechs = (attackerLevel - 4) * 2
	}
	if attackerLevel >= 8 {
		f.Drones = (attackerLevel - 7) * 3
	}
	if attackerLevel >= 12 {
		f.Jets = (attackerLevel - 11) * 2
	}

	// Defense Grid: the Nest stands up turrets in the same tiered order a
	// player would build them, one type at a time, so it "feels" like a
	// real base rather than a monolithic bonus.
	if attackerLevel >= 6 {
		f.LightLaserLvl = (attackerLevel - 5) * 2
	}
	if attackerLevel >= 10 {
		f.HeavyLaserLvl = attackerLevel - 9
	}
	if attackerLevel >= 15 {
		f.GaussCannonLvl = attackerLevel - 14
	}
	if attackerLevel >= 18 {
		f.IonCannonLvl = attackerLevel - 17
	}
	if attackerLevel >= 22 {
		f.PlasmaTurretLvl = attackerLevel - 21
	}
	f.TurretBonus = float64(f.LightLaserLvl+f.HeavyLaserLvl+f.GaussCannonLvl+f.IonCannonLvl+f.PlasmaTurretLvl) * 0.08

	// Garrison: dug-in elite guard positions, mirroring Guardian/Observer.
	if attackerLevel >= 9 {
		f.Guardians = (attackerLevel - 8) * 2
	}
	if attackerLevel >= 7 {
		f.Observers = attackerLevel - 6
	}

	// Research & shields, mirroring a real base's tech investment.
	f.IntegrityTechLvl = 1 + attackerLevel/4
	if attackerLevel >= 16 {
		f.Shields = (attackerLevel - 15) * 3
	}

	// Extreme-tier Nests field a Warlord: a hero-equivalent that grants
	// one of the same superpowers a player Commander can roll, so top-end
	// players genuinely feel like they're fighting a fortified commander,
	// not a spreadsheet.
	if attackerLevel >= 20 {
		warlordPowers := []string{"Kinetic Barrier", "Overcharged Reactor", "Iron Discipline"}
		f.HeroSuperpower = warlordPowers[attackerLevel%len(warlordPowers)]
	}

	return f
}

// ThreatTier returns a human-readable danger label for a given level,
// used in the recon preview and battle reports.
func ThreatTier(attackerLevel int) string {
	switch {
	case attackerLevel >= 20:
		return "☠️ EXTREME"
	case attackerLevel >= 15:
		return "🔴 SEVERE"
	case attackerLevel >= 10:
		return "🟠 HIGH"
	case attackerLevel >= 5:
		return "🟡 MODERATE"
	default:
		return "🟢 LOW"
	}
}

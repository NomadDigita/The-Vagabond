package content

type RogueNestForce struct {
	Soldiers int
	Mechs    int
	Drones   int
	Jets     int

	LightLaserLvl   int
	HeavyLaserLvl   int
	GaussCannonLvl  int
	IonCannonLvl    int
	PlasmaTurretLvl int

	Guardians int
	Observers int

	IntegrityTechLvl int
	Shields          int

	HeroSuperpower string

	TurretBonus float64
}

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

	if attackerLevel >= 9 {
		f.Guardians = (attackerLevel - 8) * 2
	}
	if attackerLevel >= 7 {
		f.Observers = attackerLevel - 6
	}

	f.IntegrityTechLvl = 1 + attackerLevel/4
	if attackerLevel >= 16 {
		f.Shields = (attackerLevel - 15) * 3
	}

	if attackerLevel >= 20 {
		warlordPowers := []string{"Kinetic Barrier", "Overcharged Reactor", "Iron Discipline"}
		f.HeroSuperpower = warlordPowers[attackerLevel%len(warlordPowers)]
	}

	return f
}

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

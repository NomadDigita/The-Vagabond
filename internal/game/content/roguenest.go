package content

// RogueNestForce is the composition of an AI "Rogue Drone Nest" defense,
// scaled to the attacking player's own outpost level. Higher-level players
// face a correspondingly larger, more varied, better-equipped garrison -
// not just more of the same single unit type.
type RogueNestForce struct {
	Soldiers    int
	Mechs       int
	Drones      int
	Jets        int
	TurretBonus float64 // extra flat addition to defenseRatingModifier, representing dug-in elite guard positions at high tiers
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
	if attackerLevel >= 15 {
		f.TurretBonus = float64(attackerLevel-14) * 0.05
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

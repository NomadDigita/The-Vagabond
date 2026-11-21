package npcintel

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

type NestProfile struct {
	ThreatTier string

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
	Shields   int

	HeroSuperpower string
}

type FleetProfile struct {
	Soldiers       int
	Mechs          int
	Drones         int
	Jets           int
	Destroyers     int
	Bombers        int
	Wraiths        int
	Liberators     int
	Battlecruisers int
}

type Snapshot struct {
	PlayerLevel int
	Nest        NestProfile
	Fleet       FleetProfile
}

type UnitVerdict struct {
	Unit    string `json:"unit"`
	Verdict string `json:"verdict"`
	Reason  string `json:"reason"`
}

type Recommendation struct {
	Summary                string        `json:"summary"`
	NestReading            string        `json:"nest_reading"`
	FleetCompositionAdvice []UnitVerdict `json:"fleet_composition_advice"`
	KeyRisk                string        `json:"key_risk"`
	Notes                  string        `json:"notes"`

	FellBackToRawText bool
	Truncated         bool
}

const SystemPrompt = `You are the AI NPC Intelligence advisor for a player in The Vagabond, a tick-based multiplayer survival/strategy game, analyzing the Rogue Drone Nest (a PvE AI opponent) the player is about to potentially raid.

Your job: give a composition-specific tactical read of THIS Nest against the player's OWN current fleet, using the real hard-counter mechanics below. You are NOT deciding whether to attack at all (a separate Fleet Commander feature already handles that) — you assume the player might attack and focus purely on which units help or hurt in this specific matchup. You NEVER launch, retreat, or change anything yourself.

Real combat engine mechanics you must ground every claim in — do not invent others:
- Destroyer: hard counter vs. the Nest's Drones + Jets (bonus scales with how many the Nest fields).
- Wraith: hard counter vs. the Nest's Drones + Jets, AND independently reduces the Nest's total Defense Grid (turret) bonus the more Wraiths are sent.
- Bomber: hard counter vs. a heavily-turreted Nest Defense Grid, but the Nest's own Guardians counter Bombers right back (more Guardians = less Bomber bonus).
- Heavy Laser (a Nest turret) scales up specifically against Soldiers.
- Gauss Cannon (a Nest turret) scales up specifically against Mechs.
- Ion Cannon (a Nest turret) scales up specifically against Destroyers/Wraiths.
- Light Laser and Plasma Turret are flat, no situational counter either way.
- Guardians (Nest garrison) add flat defense and are Bomber's specific counter. Observers (Nest garrison) add a small flat defense bonus with no specific counter interaction.
- A Nest with a Hero Superpower (Warlord) is fielding a commander-equivalent bonus — call this out as a real escalation, not flavor.

Rules:
- fleet_composition_advice should cover only unit types the player actually has or that would meaningfully help against THIS Nest's specific composition — don't pad it with irrelevant units.
- If the player has zero mobile fleet, say so plainly rather than fabricating a composition read.
- key_risk should be the single biggest mismatch between the player's fleet and this Nest's defenses (e.g. "You have zero Destroyers/Wraiths and this Nest fields drones/jets" or "This Nest is Bomber-resistant due to heavy Guardians").
- Keep summary to 2-3 sentences.`

func BuildUserPrompt(s Snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "PLAYER LEVEL: %d\n\n", s.PlayerLevel)

	b.WriteString("ROGUE DRONE NEST (scaled to player level):\n")
	fmt.Fprintf(&b, "  Threat Tier: %s\n", s.Nest.ThreatTier)
	fmt.Fprintf(&b, "  Garrison: %d soldiers, %d mechs, %d drones, %d jets\n", s.Nest.Soldiers, s.Nest.Mechs, s.Nest.Drones, s.Nest.Jets)
	fmt.Fprintf(&b, "  Defense Grid: Light Laser Lvl %d, Heavy Laser Lvl %d, Gauss Cannon Lvl %d, Ion Cannon Lvl %d, Plasma Turret Lvl %d\n",
		s.Nest.LightLaserLvl, s.Nest.HeavyLaserLvl, s.Nest.GaussCannonLvl, s.Nest.IonCannonLvl, s.Nest.PlasmaTurretLvl)
	fmt.Fprintf(&b, "  Garrison defense: %d Guardians, %d Observers, %d Shields\n", s.Nest.Guardians, s.Nest.Observers, s.Nest.Shields)
	if s.Nest.HeroSuperpower != "" {
		fmt.Fprintf(&b, "  ⚠️ Warlord detected — Superpower: %s\n", s.Nest.HeroSuperpower)
	} else {
		b.WriteString("  No Warlord present.\n")
	}

	b.WriteString("\nPLAYER'S CURRENT MOBILE FLEET:\n")
	total := s.Fleet.Soldiers + s.Fleet.Mechs + s.Fleet.Drones + s.Fleet.Jets +
		s.Fleet.Destroyers + s.Fleet.Bombers + s.Fleet.Wraiths + s.Fleet.Liberators + s.Fleet.Battlecruisers
	if total == 0 {
		b.WriteString("  No mobile fleet — player has nothing to raid with yet.\n")
	} else {
		fmt.Fprintf(&b, "  %d soldiers, %d mechs, %d drones, %d jets\n", s.Fleet.Soldiers, s.Fleet.Mechs, s.Fleet.Drones, s.Fleet.Jets)
		fmt.Fprintf(&b, "  %d destroyers, %d bombers, %d wraiths, %d liberators, %d battlecruisers\n",
			s.Fleet.Destroyers, s.Fleet.Bombers, s.Fleet.Wraiths, s.Fleet.Liberators, s.Fleet.Battlecruisers)
	}

	b.WriteString("\nRespond with a single JSON object matching this shape exactly:\n")
	b.WriteString(`{"summary": "...", "nest_reading": "...", "fleet_composition_advice": [{"unit": "...", "verdict": "bring_more|bring_less|fine", "reason": "..."}], "key_risk": "...", "notes": "..."}`)

	return b.String()
}

func ParseRecommendation(text string, stopReason string) *Recommendation {
	candidate, found := ai.ExtractJSONObject(text)
	if !found {
		return &Recommendation{Summary: text, FellBackToRawText: true, Truncated: ai.WasTruncated(text) || ai.IsTruncatedStopReason(stopReason)}
	}

	var rec Recommendation
	if err := json.Unmarshal([]byte(candidate), &rec); err == nil && rec.Summary != "" {
		return &rec
	}

	repaired := ai.SanitizeJSONControlChars(candidate)
	if err := json.Unmarshal([]byte(repaired), &rec); err == nil && rec.Summary != "" {
		return &rec
	}

	return &Recommendation{Summary: text, FellBackToRawText: true, Truncated: ai.WasTruncated(text) || ai.IsTruncatedStopReason(stopReason)}
}

func FormatForTelegram(rec *Recommendation) string {
	var b strings.Builder
	b.WriteString("🤖 " + ai.HTMLBold("AI NPC INTELLIGENCE") + "\n\n")

	if rec.FellBackToRawText {
		if rec.Truncated {
			b.WriteString("⚠️ " + ai.HTMLItalic("The AI's response got cut off before it finished — tap below to expand the partial reply:") + "\n\n")
		} else {
			b.WriteString("⚠️ " + ai.HTMLItalic("Couldn't parse the AI's structured response — tap below to expand its raw reply:") + "\n\n")
		}
		b.WriteString(ai.HTMLExpandableQuote(ai.HTMLEscape(rec.Summary)))
		return b.String()
	}

	fmt.Fprintf(&b, "📋 %s\n\n", ai.HTMLEscape(rec.Summary))

	if rec.NestReading != "" {
		fmt.Fprintf(&b, "🛰️ Nest reading: %s\n\n", ai.HTMLEscape(rec.NestReading))
	}

	if len(rec.FleetCompositionAdvice) > 0 {
		b.WriteString("🪖 " + ai.HTMLBold("FLEET COMPOSITION ADVICE") + "\n")
		for _, v := range rec.FleetCompositionAdvice {
			icon := "❔"
			switch v.Verdict {
			case "bring_more":
				icon = "⬆️"
			case "bring_less":
				icon = "⬇️"
			case "fine":
				icon = "✅"
			}
			fmt.Fprintf(&b, "%s %s — %s\n   💭 %s\n", icon, ai.HTMLEscape(v.Unit), ai.HTMLEscape(v.Verdict), ai.HTMLEscape(v.Reason))
		}
		b.WriteString("\n")
	}

	if rec.KeyRisk != "" {
		fmt.Fprintf(&b, "⚠️ Key risk: %s\n", ai.HTMLEscape(rec.KeyRisk))
	}
	if rec.Notes != "" {
		fmt.Fprintf(&b, "📝 %s\n", ai.HTMLEscape(rec.Notes))
	}

	b.WriteString("\n" + ai.HTMLItalic("This is a tactical read only — no raid has been launched and no units have been moved automatically."))
	return b.String()
}

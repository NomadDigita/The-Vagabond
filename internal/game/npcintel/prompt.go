// Package npcintel implements Phase I of the AI Systems Roadmap: AI
// NPC Intelligence. Like every other Phase B-H package, this is the
// seam where game state meets ai.Service; internal/ai itself stays
// game-agnostic per ADR-001.
//
// This file has zero I/O so it can be unit tested directly. All
// database access lives in intel.go.
//
// Scope, decided by reading the actual NPC/combat systems before
// writing any Snapshot fields (the same discipline Phases F-H used):
// the Rogue Drone Nest (internal/game/content.RogueNestComposition) is
// the game's only NPC/hostile-AI entity today — there's no wandering
// monster system, faction NPC roster, or NPC dialogue tree. Fleet
// Commander (Phase C) already gives a binary attack/no-attack call
// against the Nest. What it doesn't do is unpack *why* a specific
// fleet composition wins or loses against a specific Nest composition
// — the combat engine (internal/engine/tick/engine.go) has real,
// mechanically-enforced hard counters (Destroyer/Wraith vs.
// drones+jets, Bomber vs. turreted Defense Grids, Guardian vs.
// Bomber, and per-turret-type scaling against specific attacker unit
// mixes) that the static /recon_ai report shows as raw numbers but
// never interprets. Phase I fills that specific, real gap: a
// composition-specific tactical read of the Nest the player is about
// to face, grounded in those actual counter mechanics — not a second
// attack/no-attack call duplicating Fleet Commander.
package npcintel

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

// NestProfile mirrors internal/game/content.RogueNestForce's shape —
// intentionally duplicated rather than importing that package's
// struct directly, keeping this package's Snapshot independent of
// content's internal layout the same way every other Phase B-H
// package keeps its own independent DB-facing types.
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

// FleetProfile is the player's own current mobile combat fleet
// (garrison-only units like Guardians/Observers never leave the base
// on a raid, so they're deliberately not part of this).
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

// Snapshot is everything fed to the model for one tactical read.
type Snapshot struct {
	PlayerLevel int
	Nest        NestProfile
	Fleet       FleetProfile
}

// UnitVerdict is the model's per-unit-type judgment about the
// player's own fleet composition against this specific Nest.
type UnitVerdict struct {
	Unit    string `json:"unit"`
	Verdict string `json:"verdict"` // "bring_more" | "bring_less" | "fine"
	Reason  string `json:"reason"`
}

// Recommendation is the NPC Intelligence's structured output.
type Recommendation struct {
	Summary                string        `json:"summary"`
	NestReading            string        `json:"nest_reading"`
	FleetCompositionAdvice []UnitVerdict `json:"fleet_composition_advice"`
	KeyRisk                string        `json:"key_risk"`
	Notes                  string        `json:"notes"`

	// FellBackToRawText is true when JSON parsing failed and Summary
	// holds the model's raw, unparsed text instead.
	FellBackToRawText bool
	// Truncated is true when the fallback happened because the
	// model's response was cut off mid-object (almost always meaning
	// MaxTokens was hit before the model finished). See ADR-016 in
	// PROJECT_MASTER_PLAN.md.
	Truncated bool
}

// SystemPrompt is the fixed instruction given to the model for every
// NPC Intelligence call. The counter mechanics listed here are the
// real, mechanically-enforced rules from internal/engine/tick/engine.go's
// combat resolution — not flavor text — so the model's advice stays
// grounded in what the combat engine will actually do.
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

// BuildUserPrompt renders a Snapshot into the data block the model
// reasons over.
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

// ParseRecommendation decodes the model's response text, tolerating a
// markdown code fence the same way every other Phase B-H package does.
func ParseRecommendation(text string) *Recommendation {
	candidate, found := ai.ExtractJSONObject(text)
	if !found {
		return &Recommendation{Summary: text, FellBackToRawText: true, Truncated: ai.WasTruncated(text)}
	}

	var rec Recommendation
	if err := json.Unmarshal([]byte(candidate), &rec); err == nil && rec.Summary != "" {
		return &rec
	}

	// See ADR-015: real providers occasionally leave a raw,
	// unescaped newline/tab inside a string value.
	repaired := ai.SanitizeJSONControlChars(candidate)
	if err := json.Unmarshal([]byte(repaired), &rec); err == nil && rec.Summary != "" {
		return &rec
	}

	return &Recommendation{Summary: text, FellBackToRawText: true, Truncated: ai.WasTruncated(text)}
}

// FormatForTelegram renders a Recommendation as a plain-text message
// suitable for a Telegram reply.
func FormatForTelegram(rec *Recommendation) string {
	var b strings.Builder
	b.WriteString("🤖 AI NPC INTELLIGENCE\n\n")

	if rec.FellBackToRawText {
		if rec.Truncated {
			b.WriteString("⚠️ The AI's response got cut off before it finished — showing the partial reply below:\n\n")
		} else {
			b.WriteString("⚠️ Couldn't parse the AI's structured response — showing its raw reply below:\n\n")
		}
		fmt.Fprintf(&b, "```\n%s\n```", rec.Summary)
		return b.String()
	}

	fmt.Fprintf(&b, "📋 %s\n\n", rec.Summary)

	if rec.NestReading != "" {
		fmt.Fprintf(&b, "🛰️ Nest reading: %s\n\n", rec.NestReading)
	}

	if len(rec.FleetCompositionAdvice) > 0 {
		b.WriteString("FLEET COMPOSITION ADVICE:\n")
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
			fmt.Fprintf(&b, "%s %s — %s\n   💭 %s\n", icon, v.Unit, v.Verdict, v.Reason)
		}
		b.WriteString("\n")
	}

	if rec.KeyRisk != "" {
		fmt.Fprintf(&b, "⚠️ Key risk: %s\n", rec.KeyRisk)
	}
	if rec.Notes != "" {
		fmt.Fprintf(&b, "📝 %s\n", rec.Notes)
	}

	b.WriteString("\nThis is a tactical read only — no raid has been launched and no units have been moved automatically.")
	return b.String()
}

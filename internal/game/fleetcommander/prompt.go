package fleetcommander

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

type FleetComposition map[string]int

type TargetProfile struct {
	Name        string
	IsPvE       bool
	ThreatTier  string
	Garrison    FleetComposition
	TurretBonus float64

	TurretGrid       FleetComposition
	IntegrityTechLvl int
	Shields          int
	HeroSuperpower   string
}

type CombatHistorySummary struct {
	RaidsAnalyzed int
	ApparentWins  int
	TotalLosses   int
	AverageLosses float64
}

type Action string

const (
	ActionAttack    Action = "attack"
	ActionRetreat   Action = "retreat"
	ActionReinforce Action = "reinforce"
	ActionScout     Action = "scout"
	ActionWait      Action = "wait"
	ActionSplit     Action = "split_fleet"
)

type Recommendation struct {
	Recommendation string `json:"recommendation"`
	Confidence     string `json:"confidence"`
	Reasoning      string `json:"reasoning"`
	RiskAssessment string `json:"risk_assessment"`
	SuggestedSplit string `json:"suggested_split"`

	FellBackToRawText bool
	Truncated         bool
}

const SystemPrompt = `You are the AI Fleet Commander for a player in The Vagabond, a tick-based multiplayer survival/strategy game.

Your job: analyze the player's fleet composition against a specific target's known or estimated garrison, plus the player's recent combat history, and recommend exactly one of these actions: attack, retreat, reinforce, scout, wait, split_fleet.

You NEVER launch anything yourself — you only recommend. The player always decides.

Rules:
- Always explain WHY, referencing actual unit counts and the target's known garrison — never generic advice.
- If the player's fleet is clearly outmatched, recommend retreat, reinforce, or scout — never attack just because the player seems to want to.
- If recent combat history shows a losing streak, factor that into your risk assessment explicitly.
- If you recommend split_fleet, describe roughly what to send vs. hold back in suggested_split.
- Be honest about uncertainty — if the target's garrison is only a rough estimate (as with a scanned rogue nest), say so in risk_assessment.
- If the target has an active hero superpower, treat it as a significant risk factor and call it out explicitly in risk_assessment — it can change the outcome regardless of raw unit counts.`

func BuildUserPrompt(own FleetComposition, target TargetProfile, history CombatHistorySummary) string {
	var b strings.Builder

	b.WriteString("YOUR FLEET:\n")
	writeComposition(&b, own)

	b.WriteString("\nTARGET: ")
	if target.IsPvE {
		fmt.Fprintf(&b, "%s (PvE, estimated garrison, threat tier: %s)\n", target.Name, target.ThreatTier)
	} else {
		fmt.Fprintf(&b, "%s (rival player base)\n", target.Name)
	}
	writeComposition(&b, target.Garrison)
	if len(target.TurretGrid) > 0 {
		b.WriteString("  Defense Grid:\n")
		writeComposition(&b, target.TurretGrid)
	}
	if target.TurretBonus > 0 {
		fmt.Fprintf(&b, "  Defensive bonus: +%.0f%% (dug-in position)\n", target.TurretBonus*100)
	}
	if target.IntegrityTechLvl > 0 {
		fmt.Fprintf(&b, "  Integrity Tech level: %d\n", target.IntegrityTechLvl)
	}
	if target.Shields > 0 {
		fmt.Fprintf(&b, "  Nuclear shields: %d\n", target.Shields)
	}
	if target.HeroSuperpower != "" {
		fmt.Fprintf(&b, "  ⚠️ Hero superpower active: %s\n", target.HeroSuperpower)
	}

	b.WriteString("\nRECENT COMBAT HISTORY:\n")
	if history.RaidsAnalyzed == 0 {
		b.WriteString("  No recorded raids yet.\n")
	} else {
		fmt.Fprintf(&b, "  %d raids analyzed, %d apparent wins, %.1f average losses per raid\n",
			history.RaidsAnalyzed, history.ApparentWins, history.AverageLosses)
	}

	b.WriteString("\nRespond with a single JSON object matching this shape exactly:\n")
	b.WriteString(`{"recommendation": "attack|retreat|reinforce|scout|wait|split_fleet", "confidence": "high|medium|low", "reasoning": "...", "risk_assessment": "...", "suggested_split": "..."}`)

	return b.String()
}

func writeComposition(b *strings.Builder, comp FleetComposition) {
	if len(comp) == 0 {
		b.WriteString("  (no units)\n")
		return
	}
	names := make([]string, 0, len(comp))
	for name := range comp {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		count := comp[name]
		if count <= 0 {
			continue
		}
		fmt.Fprintf(b, "  %s: %d\n", name, count)
	}
}

func ParseRecommendation(text string, stopReason string) *Recommendation {
	candidate, found := ai.ExtractJSONObject(text)
	if !found {
		return &Recommendation{Reasoning: text, FellBackToRawText: true, Truncated: ai.WasTruncated(text) || ai.IsTruncatedStopReason(stopReason)}
	}

	var rec Recommendation
	if err := json.Unmarshal([]byte(candidate), &rec); err == nil && rec.Recommendation != "" {
		return &rec
	}

	repaired := ai.SanitizeJSONControlChars(candidate)
	if err := json.Unmarshal([]byte(repaired), &rec); err == nil && rec.Recommendation != "" {
		return &rec
	}

	return &Recommendation{Reasoning: text, FellBackToRawText: true, Truncated: ai.WasTruncated(text) || ai.IsTruncatedStopReason(stopReason)}
}

var actionEmoji = map[string]string{
	string(ActionAttack):    "⚔️",
	string(ActionRetreat):   "🏳️",
	string(ActionReinforce): "🛡️",
	string(ActionScout):     "🔭",
	string(ActionWait):      "⏳",
	string(ActionSplit):     "🔀",
}

func FormatForTelegram(rec *Recommendation) string {
	var b strings.Builder
	b.WriteString("🎖️ " + ai.HTMLBold("AI FLEET COMMANDER") + "\n\n")

	if rec.FellBackToRawText {
		if rec.Truncated {
			b.WriteString("⚠️ " + ai.HTMLItalic("The AI's response got cut off before it finished — tap below to expand the partial reply:") + "\n\n")
		} else {
			b.WriteString("⚠️ " + ai.HTMLItalic("Couldn't parse the AI's structured response — tap below to expand its raw reply:") + "\n\n")
		}
		b.WriteString(ai.HTMLExpandableQuote(ai.HTMLEscape(rec.Reasoning)))
		return b.String()
	}

	emoji := actionEmoji[strings.ToLower(rec.Recommendation)]
	if emoji == "" {
		emoji = "🎯"
	}
	fmt.Fprintf(&b, "%s %s: %s (confidence: %s)\n\n", emoji, ai.HTMLBold("RECOMMENDATION"), ai.HTMLBold(ai.HTMLEscape(strings.ToUpper(rec.Recommendation))), ai.HTMLCode(rec.Confidence))
	fmt.Fprintf(&b, "💭 Reasoning: %s\n", ai.HTMLEscape(rec.Reasoning))
	if rec.RiskAssessment != "" {
		fmt.Fprintf(&b, "⚠️ Risk: %s\n", ai.HTMLEscape(rec.RiskAssessment))
	}
	if strings.EqualFold(rec.Recommendation, string(ActionSplit)) && rec.SuggestedSplit != "" {
		fmt.Fprintf(&b, "🔀 Suggested split: %s\n", ai.HTMLEscape(rec.SuggestedSplit))
	}
	b.WriteString("\n" + ai.HTMLItalic("This is a recommendation only — no fleet has been launched automatically."))
	return b.String()
}

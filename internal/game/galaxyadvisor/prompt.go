package galaxyadvisor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

type ContinentStatus struct {
	Continent string
	EventType string
}

type Snapshot struct {
	HomeContinent string
	Continents    []ContinentStatus
	RecentNews    []string
}

type Action struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}

type Recommendation struct {
	Summary             string   `json:"summary"`
	HomeContinentAdvice string   `json:"home_continent_advice"`
	GalaxyOutlook       string   `json:"galaxy_outlook"`
	RecommendedActions  []Action `json:"recommended_actions"`
	Notes               string   `json:"notes"`

	FellBackToRawText bool
	Truncated         bool
}

func eventEffect(eventType string) string {
	switch eventType {
	case "solar_flare":
		return "Solar generators running at 200% output; automation agents on standby."
	case "radiation_storm":
		return "Morale decay rate doubled."
	case "acid_rain":
		return "Active construction projects running at reduced speed."
	case "emp":
		return "Automation agents down; electricity generation offline."
	case "supply_crisis":
		return "Market Exchange sale prices depressed."
	case "disease":
		return "Rations consumption elevated."
	case "sandstorm":
		return "Scan and Scout operations report degraded intel accuracy."
	case "nominal", "":
		return "Conditions nominal — no active environmental effects."
	default:
		return "Unrecognized event type — no known mechanical effect on file."
	}
}

const SystemPrompt = `You are the AI Dynamic Galaxy advisor for a player in The Vagabond, a tick-based multiplayer survival/strategy game.

Your job: help the player understand the current environmental state of the galaxy — the world event (if any) active on their own continent, and the wider picture across all four continents (Africa, Europe, Asia, Americas) — and what that implies for near-term decisions. You NEVER take any action yourself — you only advise.

Rules:
- Ground every piece of advice in the specific event type and its stated mechanical effect — never generic "stay alert" advice untethered from what's actually happening.
- home_continent_advice must address the player's own continent specifically, even if it's currently nominal (say so plainly rather than skipping it).
- galaxy_outlook should look across all four continents together — e.g. call out if another continent is currently clear and might be a better target for a cross-continent raid, or if multiple continents share a similar event.
- World events expire on their own over time; don't claim to know exactly when one will end unless told so, and don't fabricate an end time.
- recommended_actions should be concrete next steps (e.g. "hold off on new construction until Acid Rain clears"), not vague platitudes. If nothing on the board warrants action right now, say so honestly rather than inventing one.
- Keep summary to 2-3 sentences.`

func BuildUserPrompt(s Snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "PLAYER'S HOME CONTINENT: %s\n\n", s.HomeContinent)

	b.WriteString("CONTINENT STATUS:\n")
	for _, cs := range s.Continents {
		home := ""
		if cs.Continent == s.HomeContinent {
			home = " (home)"
		}
		fmt.Fprintf(&b, "  %s%s: %s — %s\n", cs.Continent, home, cs.EventType, eventEffect(cs.EventType))
	}

	b.WriteString("\nRECENT SECTOR NEWS (most recent first):\n")
	if len(s.RecentNews) == 0 {
		b.WriteString("  No recent bulletins.\n")
	} else {
		for _, headline := range s.RecentNews {
			fmt.Fprintf(&b, "  - %s\n", headline)
		}
	}

	b.WriteString("\nRespond with a single JSON object matching this shape exactly:\n")
	b.WriteString(`{"summary": "...", "home_continent_advice": "...", "galaxy_outlook": "...", "recommended_actions": [{"action": "...", "reason": "..."}], "notes": "..."}`)

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
	b.WriteString("🌌 " + ai.HTMLBold("AI DYNAMIC GALAXY ADVISOR") + "\n\n")

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

	if rec.HomeContinentAdvice != "" {
		fmt.Fprintf(&b, "🏠 Home continent: %s\n\n", ai.HTMLEscape(rec.HomeContinentAdvice))
	}
	if rec.GalaxyOutlook != "" {
		fmt.Fprintf(&b, "🌍 Galaxy outlook: %s\n\n", ai.HTMLEscape(rec.GalaxyOutlook))
	}

	if len(rec.RecommendedActions) > 0 {
		b.WriteString("🛰️ " + ai.HTMLBold("RECOMMENDED ACTIONS") + "\n")
		for i, a := range rec.RecommendedActions {
			fmt.Fprintf(&b, "%d. %s\n   💡 %s\n", i+1, ai.HTMLBold(ai.HTMLEscape(a.Action)), ai.HTMLEscape(a.Reason))
		}
		b.WriteString("\n")
	}

	if rec.Notes != "" {
		fmt.Fprintf(&b, "📝 %s\n", ai.HTMLEscape(rec.Notes))
	}

	b.WriteString("\n" + ai.HTMLItalic("This is a briefing only — nothing has been moved, built, or changed automatically."))
	return b.String()
}

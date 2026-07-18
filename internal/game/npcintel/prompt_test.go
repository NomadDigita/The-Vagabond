package npcintel_test

import (
	"strings"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/game/npcintel"
)

func sampleSnapshot() npcintel.Snapshot {
	return npcintel.Snapshot{
		PlayerLevel: 12,
		Nest: npcintel.NestProfile{
			ThreatTier:     "🟠 HIGH",
			Soldiers:       180,
			Mechs:          16,
			Drones:         15,
			Jets:           2,
			HeavyLaserLvl:  3,
			GaussCannonLvl: 0,
			Guardians:      8,
			Observers:      6,
			Shields:        0,
			HeroSuperpower: "",
		},
		Fleet: npcintel.FleetProfile{
			Soldiers: 500, Mechs: 40, Bombers: 20,
		},
	}
}

func TestBuildUserPrompt_IsDeterministic(t *testing.T) {
	s := sampleSnapshot()
	p1 := npcintel.BuildUserPrompt(s)
	p2 := npcintel.BuildUserPrompt(s)
	if p1 != p2 {
		t.Fatalf("expected identical prompts for identical input")
	}
}

func TestBuildUserPrompt_ContainsKeyFacts(t *testing.T) {
	p := npcintel.BuildUserPrompt(sampleSnapshot())
	for _, want := range []string{
		"PLAYER LEVEL: 12",
		"🟠 HIGH",
		"180 soldiers, 16 mechs, 15 drones, 2 jets",
		"Heavy Laser Lvl 3",
		"8 Guardians, 6 Observers",
		"No Warlord present.",
		"500 soldiers, 40 mechs, 0 drones, 0 jets",
		"0 destroyers, 20 bombers, 0 wraiths",
		"fleet_composition_advice",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("expected prompt to contain %q, got:\n%s", want, p)
		}
	}
}

func TestBuildUserPrompt_WarlordPresent(t *testing.T) {
	s := sampleSnapshot()
	s.Nest.HeroSuperpower = "Kinetic Barrier"
	p := npcintel.BuildUserPrompt(s)
	if !strings.Contains(p, "Warlord detected") || !strings.Contains(p, "Kinetic Barrier") {
		t.Errorf("expected warlord callout, got:\n%s", p)
	}
}

func TestBuildUserPrompt_NoFleetPath(t *testing.T) {
	s := npcintel.Snapshot{PlayerLevel: 3, Nest: npcintel.NestProfile{ThreatTier: "🟢 LOW"}}
	p := npcintel.BuildUserPrompt(s)
	if !strings.Contains(p, "No mobile fleet") {
		t.Errorf("expected no-fleet message, got:\n%s", p)
	}
}

func TestParseRecommendation_ValidJSON(t *testing.T) {
	raw := `{"summary": "This Nest is Bomber-resistant.", "nest_reading": "Heavy Guardian presence blunts siege units.", "fleet_composition_advice": [{"unit": "bombers", "verdict": "bring_less", "reason": "8 Guardians counter your Bombers hard"}], "key_risk": "Your Bomber-heavy fleet underperforms here", "notes": "small nest, no Warlord"}`
	rec := npcintel.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected clean JSON parse, got fallback")
	}
	if len(rec.FleetCompositionAdvice) != 1 || rec.FleetCompositionAdvice[0].Verdict != "bring_less" {
		t.Errorf("unexpected advice: %+v", rec.FleetCompositionAdvice)
	}
	if rec.KeyRisk == "" {
		t.Errorf("expected key risk to be populated")
	}
}

func TestParseRecommendation_StripsMarkdownFence(t *testing.T) {
	raw := "```json\n" + `{"summary": "ok", "nest_reading": "", "fleet_composition_advice": [], "key_risk": "", "notes": ""}` + "\n```"
	rec := npcintel.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected fence to be stripped and JSON parsed, got fallback")
	}
}

func TestParseRecommendation_FallsBackOnGarbage(t *testing.T) {
	raw := "bring more bombers probably"
	rec := npcintel.ParseRecommendation(raw)
	if !rec.FellBackToRawText {
		t.Fatalf("expected fallback for non-JSON text")
	}
	if rec.Summary != raw {
		t.Errorf("expected raw text preserved in Summary, got %q", rec.Summary)
	}
	if rec.Truncated {
		t.Errorf("expected Truncated=false for prose with no JSON object at all")
	}
}

func TestParseRecommendation_TrailingProseAroundJSON(t *testing.T) {
	raw := `{"summary": "Fleet looks solid.", "fleet_composition_advice": []}` + "\n\nGood luck out there!"
	rec := npcintel.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected trailing prose to be discarded, not trigger fallback")
	}
}

func TestParseRecommendation_RawNewlineInsideStringValue(t *testing.T) {
	raw := "{\"summary\": \"Line one\nline two\", \"fleet_composition_advice\": []}"
	rec := npcintel.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected sanitized control chars to allow parsing, got fallback")
	}
}

// See ADR-016 in PROJECT_MASTER_PLAN.md: a response cut off mid-object
// is distinguished from one that never contained JSON at all.
func TestParseRecommendation_FallsBackOnTruncatedJSON(t *testing.T) {
	raw := `{"summary": "This Nest's heavy Guardian garrison specifically counters your Bomber-heavy fleet compos`
	rec := npcintel.ParseRecommendation(raw)
	if !rec.FellBackToRawText {
		t.Fatalf("expected fallback for truncated JSON")
	}
	if !rec.Truncated {
		t.Errorf("expected Truncated=true for a response cut off mid-object")
	}
}

func TestFormatForTelegram_FallbackPath(t *testing.T) {
	rec := &npcintel.Recommendation{Summary: "raw text", FellBackToRawText: true}
	out := npcintel.FormatForTelegram(rec)
	if !strings.Contains(out, "Couldn't parse") || !strings.Contains(out, "raw text") {
		t.Errorf("expected fallback notice and raw text, got: %s", out)
	}
}

func TestFormatForTelegram_TruncatedPath(t *testing.T) {
	rec := &npcintel.Recommendation{
		Summary:           `{"summary": "cut off mid`,
		FellBackToRawText: true,
		Truncated:         true,
	}
	out := npcintel.FormatForTelegram(rec)
	if !strings.Contains(out, "cut off before it finished") {
		t.Errorf("expected truncation-specific message, got: %s", out)
	}
}

func TestFormatForTelegram_StructuredPath(t *testing.T) {
	rec := &npcintel.Recommendation{
		Summary:     "This Nest is Bomber-resistant.",
		NestReading: "Heavy Guardian presence blunts siege units.",
		FleetCompositionAdvice: []npcintel.UnitVerdict{
			{Unit: "bombers", Verdict: "bring_less", Reason: "8 Guardians counter your Bombers hard"},
		},
		KeyRisk: "Your Bomber-heavy fleet underperforms here",
		Notes:   "small nest, no Warlord",
	}
	out := npcintel.FormatForTelegram(rec)
	for _, want := range []string{
		"AI NPC INTELLIGENCE",
		"This Nest is Bomber-resistant.",
		"Heavy Guardian presence blunts siege units.",
		"bombers — bring_less", "8 Guardians counter your Bombers hard",
		"Your Bomber-heavy fleet underperforms here",
		"small nest, no Warlord",
		"no raid has been launched and no units have been moved automatically",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

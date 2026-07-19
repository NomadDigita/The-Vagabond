package galaxyadvisor_test

import (
	"strings"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/game/galaxyadvisor"
)

func sampleSnapshot() galaxyadvisor.Snapshot {
	return galaxyadvisor.Snapshot{
		HomeContinent: "Asia",
		Continents: []galaxyadvisor.ContinentStatus{
			{Continent: "Africa", EventType: "nominal"},
			{Continent: "Europe", EventType: "acid_rain"},
			{Continent: "Asia", EventType: "emp"},
			{Continent: "Americas", EventType: "nominal"},
		},
		RecentNews: []string{
			"🌩️ EMP BURST WARNING: A regional electromagnetic pulse over Asia has knocked out unshielded electronics.",
			"🌧️ ACID RAIN ALERT: Highly corrosive precipitation over Europe.",
		},
	}
}

func TestBuildUserPrompt_IsDeterministic(t *testing.T) {
	s := sampleSnapshot()
	p1 := galaxyadvisor.BuildUserPrompt(s)
	p2 := galaxyadvisor.BuildUserPrompt(s)
	if p1 != p2 {
		t.Fatalf("expected identical prompts for identical input")
	}
}

func TestBuildUserPrompt_ContainsKeyFacts(t *testing.T) {
	p := galaxyadvisor.BuildUserPrompt(sampleSnapshot())
	for _, want := range []string{
		"PLAYER'S HOME CONTINENT: Asia",
		"Asia (home): emp", "Automation agents down",
		"Europe: acid_rain", "reduced speed",
		"Africa: nominal", "Conditions nominal",
		"EMP BURST WARNING",
		"recommended_actions",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("expected prompt to contain %q, got:\n%s", want, p)
		}
	}
}

func TestBuildUserPrompt_NoNewsPath(t *testing.T) {
	s := galaxyadvisor.Snapshot{HomeContinent: "Africa"}
	p := galaxyadvisor.BuildUserPrompt(s)
	if !strings.Contains(p, "No recent bulletins.") {
		t.Errorf("expected no-news message, got:\n%s", p)
	}
}

func TestBuildUserPrompt_UnrecognizedEventType(t *testing.T) {
	s := galaxyadvisor.Snapshot{
		HomeContinent: "Africa",
		Continents:    []galaxyadvisor.ContinentStatus{{Continent: "Africa", EventType: "mystery_event"}},
	}
	p := galaxyadvisor.BuildUserPrompt(s)
	if !strings.Contains(p, "no known mechanical effect on file") {
		t.Errorf("expected unrecognized-event fallback text, got:\n%s", p)
	}
}

func TestParseRecommendation_ValidJSON(t *testing.T) {
	raw := `{"summary": "Your continent is under EMP; galaxy otherwise calm.", "home_continent_advice": "Automation is down, act manually.", "galaxy_outlook": "Europe has Acid Rain but Africa and Americas are clear.", "recommended_actions": [{"action": "delay automation-dependent tasks", "reason": "EMP disables agents"}], "notes": "EMP will clear on its own"}`
	rec := galaxyadvisor.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected clean JSON parse, got fallback")
	}
	if len(rec.RecommendedActions) != 1 || rec.RecommendedActions[0].Action != "delay automation-dependent tasks" {
		t.Errorf("unexpected actions: %+v", rec.RecommendedActions)
	}
	if rec.GalaxyOutlook == "" {
		t.Errorf("expected galaxy outlook to be populated")
	}
}

func TestParseRecommendation_StripsMarkdownFence(t *testing.T) {
	raw := "```json\n" + `{"summary": "ok", "home_continent_advice": "", "galaxy_outlook": "", "recommended_actions": [], "notes": ""}` + "\n```"
	rec := galaxyadvisor.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected fence to be stripped and JSON parsed, got fallback")
	}
}

func TestParseRecommendation_FallsBackOnGarbage(t *testing.T) {
	raw := "looks fine out there I guess"
	rec := galaxyadvisor.ParseRecommendation(raw)
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
	raw := `{"summary": "All clear.", "recommended_actions": []}` + "\n\nStay safe out there!"
	rec := galaxyadvisor.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected trailing prose to be discarded, not trigger fallback")
	}
}

func TestParseRecommendation_RawNewlineInsideStringValue(t *testing.T) {
	raw := "{\"summary\": \"Line one\nline two\", \"recommended_actions\": []}"
	rec := galaxyadvisor.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected sanitized control chars to allow parsing, got fallback")
	}
}

// See ADR-016 in PROJECT_MASTER_PLAN.md: a response cut off mid-object
// is distinguished from one that never contained JSON at all.
func TestParseRecommendation_FallsBackOnTruncatedJSON(t *testing.T) {
	raw := `{"summary": "Your continent is currently experiencing an EMP burst which has disabled all automat`
	rec := galaxyadvisor.ParseRecommendation(raw)
	if !rec.FellBackToRawText {
		t.Fatalf("expected fallback for truncated JSON")
	}
	if !rec.Truncated {
		t.Errorf("expected Truncated=true for a response cut off mid-object")
	}
}

func TestFormatForTelegram_FallbackPath(t *testing.T) {
	rec := &galaxyadvisor.Recommendation{Summary: "raw text", FellBackToRawText: true}
	out := galaxyadvisor.FormatForTelegram(rec)
	if !strings.Contains(out, "Couldn't parse") || !strings.Contains(out, "raw text") {
		t.Errorf("expected fallback notice and raw text, got: %s", out)
	}
}

func TestFormatForTelegram_TruncatedPath(t *testing.T) {
	rec := &galaxyadvisor.Recommendation{
		Summary:           `{"summary": "cut off mid`,
		FellBackToRawText: true,
		Truncated:         true,
	}
	out := galaxyadvisor.FormatForTelegram(rec)
	if !strings.Contains(out, "cut off before it finished") {
		t.Errorf("expected truncation-specific message, got: %s", out)
	}
}

func TestFormatForTelegram_StructuredPath(t *testing.T) {
	rec := &galaxyadvisor.Recommendation{
		Summary:             "Your continent is under EMP; galaxy otherwise calm.",
		HomeContinentAdvice: "Automation is down, act manually.",
		GalaxyOutlook:       "Europe has Acid Rain but Africa and Americas are clear.",
		RecommendedActions: []galaxyadvisor.Action{
			{Action: "delay automation-dependent tasks", Reason: "EMP disables agents"},
		},
		Notes: "EMP will clear on its own",
	}
	out := galaxyadvisor.FormatForTelegram(rec)
	for _, want := range []string{
		"AI DYNAMIC GALAXY ADVISOR",
		"Your continent is under EMP; galaxy otherwise calm.",
		"Automation is down, act manually.",
		"Europe has Acid Rain but Africa and Americas are clear.",
		"delay automation-dependent tasks", "EMP disables agents",
		"EMP will clear on its own",
		"nothing has been moved, built, or changed automatically",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

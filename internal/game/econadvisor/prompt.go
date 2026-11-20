package econadvisor

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

type ModuleState struct {
	Type  string
	Level int
}

type MarketListing struct {
	ItemType     string
	Quantity     int
	PriceDollars float64
}

type MarketItemStats struct {
	ItemType     string
	ActiveCount  int
	AveragePrice float64
	MinPrice     float64
	MaxPrice     float64
}

type Snapshot struct {
	EncampmentID string
	Name         string
	Level        int

	Resources map[string]float64
	Modules   []ModuleState

	BankBalance     float64
	BankBalanceCash float64
	LoanAmount      float64
	LoanCash        float64

	OwnListings []MarketListing
	MarketStats []MarketItemStats
}

type Action struct {
	Action       string `json:"action"`
	Target       string `json:"target"`
	Reason       string `json:"reason"`
	ExpectedGain string `json:"expected_gain"`
}

type Recommendation struct {
	Summary       string   `json:"summary"`
	TopROIActions []Action `json:"top_roi_actions"`
	Bottlenecks   string   `json:"bottlenecks"`
	MarketTiming  string   `json:"market_timing"`
	TradingAdvice string   `json:"trading_advice"`

	FellBackToRawText bool
	Truncated         bool
}

const SystemPrompt = `You are the AI Economy Advisor for a player's base in The Vagabond, a tick-based multiplayer survival/strategy game.

Your job: analyze the player's resources, buildings, debt, and market activity, then recommend the highest-ROI next actions. You NEVER act yourself — you only recommend.

Rules:
- Every recommended action MUST include a quantitative expected_gain estimate (a rate, percentage, or absolute amount) — never a vague claim like "will help a lot."
- Identify production bottlenecks explicitly (e.g. a resource close to storage cap, or a resource stuck at zero production).
- If bank debt (loan_amount/loan_cash) is high relative to balance, factor that into your advice.
- For market timing: compare the player's own active listings' prices to the market-wide average/min/max for the same item type, and say whether they're priced competitively.
- Keep the summary to 2-3 sentences. List at most 4 top ROI actions, ordered most-to-least valuable.`

func BuildUserPrompt(s Snapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "BASE: %q (Level %d, ID %s)\n\n", s.Name, s.Level, s.EncampmentID)

	b.WriteString("RESOURCES:\n")
	resNames := make([]string, 0, len(s.Resources))
	for name := range s.Resources {
		resNames = append(resNames, name)
	}
	sort.Strings(resNames)
	for _, name := range resNames {
		fmt.Fprintf(&b, "  %s: %.1f\n", name, s.Resources[name])
	}

	modules := append([]ModuleState(nil), s.Modules...)
	sort.Slice(modules, func(i, j int) bool { return modules[i].Type < modules[j].Type })
	b.WriteString("\nMODULES:\n")
	if len(modules) == 0 {
		b.WriteString("  (none built yet)\n")
	}
	for _, m := range modules {
		fmt.Fprintf(&b, "  %s: level %d\n", m.Type, m.Level)
	}

	fmt.Fprintf(&b, "\nBANK: balance %.1f (+ %.1f cash) | debt %.1f (+ %.1f cash)\n",
		s.BankBalance, s.BankBalanceCash, s.LoanAmount, s.LoanCash)

	b.WriteString("\nYOUR ACTIVE MARKET LISTINGS:\n")
	if len(s.OwnListings) == 0 {
		b.WriteString("  (none)\n")
	}
	listings := append([]MarketListing(nil), s.OwnListings...)
	sort.Slice(listings, func(i, j int) bool { return listings[i].ItemType < listings[j].ItemType })
	for _, l := range listings {
		fmt.Fprintf(&b, "  %s x%d @ %.1f dollars each\n", l.ItemType, l.Quantity, l.PriceDollars)
	}

	b.WriteString("\nMARKET-WIDE STATS (active listings, all sellers):\n")
	stats := append([]MarketItemStats(nil), s.MarketStats...)
	sort.Slice(stats, func(i, j int) bool { return stats[i].ItemType < stats[j].ItemType })
	if len(stats) == 0 {
		b.WriteString("  (no active market data)\n")
	}
	for _, m := range stats {
		fmt.Fprintf(&b, "  %s: %d active listings, avg %.1f (range %.1f–%.1f)\n",
			m.ItemType, m.ActiveCount, m.AveragePrice, m.MinPrice, m.MaxPrice)
	}

	b.WriteString("\nRespond with a single JSON object matching this shape exactly:\n")
	b.WriteString(`{"summary": "...", "top_roi_actions": [{"action": "...", "target": "...", "reason": "...", "expected_gain": "..."}], "bottlenecks": "...", "market_timing": "...", "trading_advice": "..."}`)

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
	b.WriteString("💹 " + ai.HTMLBold("AI ECONOMY ADVISOR") + "\n\n")

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
	if len(rec.TopROIActions) > 0 {
		b.WriteString("💰 " + ai.HTMLBold("TOP ROI ACTIONS") + "\n")
		for i, a := range rec.TopROIActions {
			fmt.Fprintf(&b, "%d. %s — %s (%s)\n   ➜ %s\n", i+1, ai.HTMLBold(ai.HTMLEscape(a.Action)), ai.HTMLEscape(a.Target), ai.HTMLCode(ai.HTMLEscape(a.ExpectedGain)), ai.HTMLEscape(a.Reason))
		}
		b.WriteString("\n")
	}
	if rec.Bottlenecks != "" {
		fmt.Fprintf(&b, "🚧 Bottlenecks: %s\n", ai.HTMLEscape(rec.Bottlenecks))
	}
	if rec.MarketTiming != "" {
		fmt.Fprintf(&b, "📈 Market timing: %s\n", ai.HTMLEscape(rec.MarketTiming))
	}
	if rec.TradingAdvice != "" {
		fmt.Fprintf(&b, "🤝 Trading advice: %s\n", ai.HTMLEscape(rec.TradingAdvice))
	}
	b.WriteString("\n" + ai.HTMLItalic("This is a recommendation only — nothing has been bought, sold, or built automatically."))
	return b.String()
}

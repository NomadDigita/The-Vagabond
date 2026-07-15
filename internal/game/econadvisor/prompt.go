// Package econadvisor implements Phase D of the AI Systems Roadmap:
// the AI Economy Advisor. Like governor and fleetcommander, this
// package is the seam where game state meets ai.Service; internal/ai
// itself stays game-agnostic per ADR-001 in PROJECT_MASTER_PLAN.md.
//
// This file has zero I/O so it can be unit tested directly. All
// database access lives in advisor.go.
//
// Note: this package deliberately does NOT import internal/game/governor
// or internal/game/fleetcommander, even though all three read
// overlapping data (resources, modules). Each stays independently
// buildable/mergeable rather than forming a dependency chain between
// sibling AI features — a small amount of duplicated type definition
// (ModuleState) is a worthwhile trade for that isolation.
package econadvisor

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ModuleState is a minimal projection of a row in the `modules` table.
type ModuleState struct {
	Type  string
	Level int
}

// MarketListing is one active (unsold) market_exchange row.
type MarketListing struct {
	ItemType     string
	Quantity     int
	PriceDollars float64
}

// MarketItemStats summarizes market-wide pricing for one item type,
// across all sellers, used for market-timing advice.
type MarketItemStats struct {
	ItemType     string
	ActiveCount  int
	AveragePrice float64
	MinPrice     float64
	MaxPrice     float64
}

// Snapshot is everything about one player's economy fed to the model.
type Snapshot struct {
	EncampmentID string
	Name         string
	Level        int

	// Resources is a generic name→amount map (scrap, rations,
	// electricity, neuro_cores, metal, crystal, hydrogen, dollars,
	// ether, ...) so a new resource column never requires a change
	// to this struct's shape.
	Resources map[string]float64
	Modules   []ModuleState

	BankBalance     float64
	BankBalanceCash float64
	LoanAmount      float64
	LoanCash        float64

	OwnListings []MarketListing
	MarketStats []MarketItemStats
}

// Action is one recommended step with a required quantitative
// estimate, per the roadmap's "Explain expected gains quantitatively."
type Action struct {
	Action       string `json:"action"`
	Target       string `json:"target"`
	Reason       string `json:"reason"`
	ExpectedGain string `json:"expected_gain"` // e.g. "+18% scrap/tick", "~450 dollars/day"
}

// Recommendation is the Economy Advisor's structured output.
type Recommendation struct {
	Summary       string   `json:"summary"`
	TopROIActions []Action `json:"top_roi_actions"`
	Bottlenecks   string   `json:"bottlenecks"`
	MarketTiming  string   `json:"market_timing"`
	TradingAdvice string   `json:"trading_advice"`

	// FellBackToRawText is true when JSON parsing failed and Summary
	// holds the model's raw, unparsed text instead.
	FellBackToRawText bool
}

const SystemPrompt = `You are the AI Economy Advisor for a player's base in The Vagabond, a tick-based multiplayer survival/strategy game.

Your job: analyze the player's resources, buildings, debt, and market activity, then recommend the highest-ROI next actions. You NEVER act yourself — you only recommend.

Rules:
- Every recommended action MUST include a quantitative expected_gain estimate (a rate, percentage, or absolute amount) — never a vague claim like "will help a lot."
- Identify production bottlenecks explicitly (e.g. a resource close to storage cap, or a resource stuck at zero production).
- If bank debt (loan_amount/loan_cash) is high relative to balance, factor that into your advice.
- For market timing: compare the player's own active listings' prices to the market-wide average/min/max for the same item type, and say whether they're priced competitively.
- Keep the summary to 2-3 sentences. List at most 4 top ROI actions, ordered most-to-least valuable.`

// BuildUserPrompt renders a Snapshot into the data block the model
// reasons over. Sorted map/slice iteration keeps output deterministic
// for internal/ai's cache key hashing.
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

// ParseRecommendation decodes the model's response text, tolerating a
// markdown code fence the same way governor/fleetcommander do.
func ParseRecommendation(text string) *Recommendation {
	cleaned := strings.TrimSpace(text)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var rec Recommendation
	if err := json.Unmarshal([]byte(cleaned), &rec); err != nil || rec.Summary == "" {
		return &Recommendation{Summary: text, FellBackToRawText: true}
	}
	return &rec
}

// FormatForTelegram renders a Recommendation as a plain-text message.
func FormatForTelegram(rec *Recommendation) string {
	var b strings.Builder
	b.WriteString("💹 AI ECONOMY ADVISOR\n\n")

	if rec.FellBackToRawText {
		b.WriteString(rec.Summary)
		return b.String()
	}

	fmt.Fprintf(&b, "📋 %s\n\n", rec.Summary)
	if len(rec.TopROIActions) > 0 {
		b.WriteString("TOP ROI ACTIONS:\n")
		for i, a := range rec.TopROIActions {
			fmt.Fprintf(&b, "%d. %s — %s (%s)\n   → %s\n", i+1, a.Action, a.Target, a.ExpectedGain, a.Reason)
		}
		b.WriteString("\n")
	}
	if rec.Bottlenecks != "" {
		fmt.Fprintf(&b, "🚧 Bottlenecks: %s\n", rec.Bottlenecks)
	}
	if rec.MarketTiming != "" {
		fmt.Fprintf(&b, "📈 Market timing: %s\n", rec.MarketTiming)
	}
	if rec.TradingAdvice != "" {
		fmt.Fprintf(&b, "🤝 Trading advice: %s\n", rec.TradingAdvice)
	}
	b.WriteString("\nThis is a recommendation only — nothing has been bought, sold, or built automatically.")
	return b.String()
}

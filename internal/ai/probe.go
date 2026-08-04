package ai

import (
	"context"
	"time"
)

// ProbeResult is the outcome of one live, minimal-cost test call
// against a single registered provider.
type ProbeResult struct {
	Provider string
	OK       bool
	Model    string
	// Err is the exact error returned by the provider on failure
	// (e.g. "gemini: api error (RESOURCE_EXHAUSTED) on model
	// gemini-3.5-flash: ...", or "openaicompat: unexpected status 403:
	// ...AccessDenied.Unpurchased..."). Empty on success. Deliberately
	// NOT truncated or reworded here — the caller (a Telegram handler)
	// decides display length; this is a diagnostic surface, and a
	// clipped or paraphrased error is exactly what has made this class
	// of bug take multiple sessions to root-cause in the past, each
	// one only able to see "available: true/false", never the real
	// account/quota/config reason a provider fell through to mock.
	Err     string
	Latency time.Duration
}

// ProbeAllProviders issues one minimal real completion request against
// every registered, non-mock provider — including ones reporting
// Available()==false, so a missing key shows up the same way a quota
// error does, in one place — bypassing Service.Complete's cache and
// permission layers entirely (this is an admin diagnostic, not a game
// feature a player can trigger). The daily global budget is still
// respected: if it's already exhausted, every provider is reported
// skipped rather than silently spending past the cap.
//
// This closes the actual gap behind "AI console still shows
// PLACEHOLDER despite the key being set" being independently
// re-investigated across half a dozen sessions (see
// BUGS_AND_INCONSISTENCIES.md, 2026-08-02 entries): Registry.Ordered()
// mock-always-last was fixed long before those sessions and was never
// the problem again — the real, repeated problem was that nothing in
// Telegram could distinguish "provider unavailable", "provider
// available but erroring (bad key / quota / unpurchased model)", and
// "correctly falling through to mock" from each other. All three
// looked identical in the old /ai_status output. Every prior fix
// session had to get live Render log access to find the real error
// text; this makes that same error text visible from inside Telegram
// on demand, going forward, without needing dashboard access at all.
func (s *Service) ProbeAllProviders(ctx context.Context) []ProbeResult {
	budgetErr := s.checkBudget(ctx, 0)

	var results []ProbeResult
	for _, name := range s.Registry.AllNames() {
		if name == mockProviderName {
			continue // mock is never a real integration to diagnose
		}
		p, ok := s.Registry.Get(name)
		if !ok {
			continue
		}
		if !p.Available() {
			results = append(results, ProbeResult{Provider: name, OK: false, Err: "not configured — no API key set for this provider"})
			continue
		}
		if budgetErr != nil {
			results = append(results, ProbeResult{Provider: name, OK: false, Err: "skipped — " + budgetErr.Error()})
			continue
		}

		start := time.Now()
		resp, err := p.Complete(ctx, CompletionRequest{
			Feature:   "ai_status_probe",
			System:    "You are a connectivity check. Reply with exactly one word: OK",
			Messages:  []Message{{Role: RoleUser, Content: "Reply with exactly one word: OK"}},
			MaxTokens: 16,
		})
		latency := time.Since(start)

		if err != nil {
			results = append(results, ProbeResult{Provider: name, OK: false, Err: err.Error(), Latency: latency})
			continue
		}

		resp.Provider = p.Name()
		if resp.Model == "" {
			resp.Model = name
		}
		cost := EstimateCostUSD(resp.Provider, resp.Model, resp.Usage)
		if s.Cost != nil {
			_ = s.Cost.RecordUsage(ctx, 0, "ai_status_probe", resp.Provider, resp.Model, resp.Usage, cost)
		}
		results = append(results, ProbeResult{Provider: name, OK: true, Model: resp.Model, Latency: latency})
	}
	return results
}

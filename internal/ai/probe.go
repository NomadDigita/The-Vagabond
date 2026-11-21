package ai

import (
	"context"
	"time"
)

type ProbeResult struct {
	Provider string
	OK       bool
	Model    string
	Err      string
	Latency  time.Duration
}

func (s *Service) ProbeAllProviders(ctx context.Context) []ProbeResult {
	budgetErr := s.checkBudget(ctx, 0)

	var results []ProbeResult
	for _, name := range s.Registry.AllNames() {
		if name == mockProviderName {
			continue
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

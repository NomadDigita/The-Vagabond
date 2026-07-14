package ai

import (
	"context"
	"errors"
	"log"
	"time"
)

// ErrAIDisabled is returned when the master AI_ENABLED kill switch is off.
var ErrAIDisabled = errors.New("ai: disabled by configuration")

// ErrNoProviderAvailable is returned when every provider in the
// fallback chain is unavailable or failed.
var ErrNoProviderAvailable = errors.New("ai: no provider available")

// Service is the single entry point every game subsystem (Phase B-J)
// should depend on. It is intentionally the only exported type in this
// package that touches permissions, cost, cache, and memory together —
// callers should never need to reach into those pieces individually.
type Service struct {
	Config      *Config
	Registry    *Registry
	Cache       Cache
	Cost        CostTracker
	Permissions *PermissionManager
	Memory      MemoryStore
}

// NewService wires the foundation together from a loaded Config and a
// Postgres connection. Pass additional providers via Registry.Register
// before the first call to Complete.
func NewService(cfg *Config, registry *Registry, cost CostTracker, perms *PermissionManager, memory MemoryStore) *Service {
	registry.SetOrder(append([]string{cfg.DefaultProvider}, cfg.FallbackOrder...))
	return &Service{
		Config:      cfg,
		Registry:    registry,
		Cache:       NewInMemoryCache(),
		Cost:        cost,
		Permissions: perms,
		Memory:      memory,
	}
}

// Complete is the single call site every Phase B-J feature uses to
// talk to an LLM. It enforces, in order: the master kill switch,
// per-feature/per-user permissions, cache lookup, per-user and global
// daily budgets, then tries each available provider in fallback order,
// recording cost and (optionally) memory on success.
func (s *Service) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	if req.Feature == "" {
		return nil, errors.New("ai: CompletionRequest.Feature is required")
	}
	if !s.Config.Enabled {
		return nil, ErrAIDisabled
	}

	if s.Permissions != nil {
		allowed, reason, err := s.Permissions.IsAllowed(ctx, req.UserID, Feature(req.Feature))
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, errors.New(reason)
		}
	}

	cacheKey := CacheKey(req)
	if s.Cache != nil {
		if cached, ok := s.Cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	if s.Cost != nil {
		if err := s.checkBudget(ctx, req.UserID); err != nil {
			return nil, err
		}
	}

	var lastErr error
	for _, provider := range s.Registry.Ordered() {
		resp, err := provider.Complete(ctx, req)
		if err != nil {
			lastErr = err
			log.Printf("ai: provider %q failed for feature %q: %v — trying next fallback", provider.Name(), req.Feature, err)
			continue
		}

		resp.Provider = provider.Name()
		if resp.Model == "" {
			resp.Model = req.Model
		}
		resp.CostUSD = EstimateCostUSD(resp.Provider, resp.Model, resp.Usage)

		if s.Cost != nil {
			if err := s.Cost.RecordUsage(ctx, req.UserID, req.Feature, resp.Provider, resp.Model, resp.Usage, resp.CostUSD); err != nil {
				log.Printf("ai: failed to record cost usage: %v", err)
			}
		}
		if s.Cache != nil && s.Config.CacheTTLSeconds > 0 {
			s.Cache.Set(cacheKey, resp, time.Duration(s.Config.CacheTTLSeconds)*time.Second)
		}

		return resp, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ErrNoProviderAvailable
}

func (s *Service) checkBudget(ctx context.Context, userID int64) error {
	if s.Config.MaxGlobalCostPerDayUSD > 0 {
		global, err := s.Cost.GlobalSpendToday(ctx)
		if err != nil {
			return err
		}
		if global >= s.Config.MaxGlobalCostPerDayUSD {
			return ErrBudgetExceeded
		}
	}
	if userID != 0 && s.Config.MaxUserCostPerDayUSD > 0 {
		spend, err := s.Cost.UserSpendToday(ctx, userID)
		if err != nil {
			return err
		}
		if spend >= s.Config.MaxUserCostPerDayUSD {
			return ErrBudgetExceeded
		}
	}
	return nil
}

package ai

import (
	"context"
	"errors"
	"log"
	"time"
)

var ErrAIDisabled = errors.New("ai: disabled by configuration")

var ErrNoProviderAvailable = errors.New("ai: no provider available")

type Service struct {
	Config      *Config
	Registry    *Registry
	Cache       Cache
	Cost        CostTracker
	Permissions *PermissionManager
	Memory      MemoryStore
}

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
		if s.Cache != nil && s.Config.CacheTTLSeconds > 0 && !IsTruncatedStopReason(resp.StopReason) {
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

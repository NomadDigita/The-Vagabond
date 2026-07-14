package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

// Cache is the abstraction the Service uses to avoid re-paying for
// identical AI calls. The default implementation is an in-process TTL
// map; a future session may swap this for a Redis- or Postgres-backed
// implementation without touching Service.
type Cache interface {
	Get(key string) (*CompletionResponse, bool)
	Set(key string, resp *CompletionResponse, ttl time.Duration)
}

// CacheKey derives a stable cache key from the parts of a request that
// affect its output. Feature and UserID are included so one player's
// cached answer is never served to another under a different context.
func CacheKey(req CompletionRequest) string {
	h := sha256.New()
	enc := json.NewEncoder(h)
	_ = enc.Encode(struct {
		Feature     string
		UserID      int64
		System      string
		Messages    []Message
		JSONMode    bool
		Temperature float64
	}{
		Feature:     req.Feature,
		UserID:      req.UserID,
		System:      req.System,
		Messages:    req.Messages,
		JSONMode:    req.JSONMode,
		Temperature: req.Temperature,
	})
	return hex.EncodeToString(h.Sum(nil))
}

type cacheEntry struct {
	resp      *CompletionResponse
	expiresAt time.Time
}

// InMemoryCache is a simple sync.Map-backed TTL cache. It leaks
// expired-but-unread entries until they're next looked up (acceptable
// for the request volumes expected from Telegram slash commands); a
// background sweeper can be added later without changing the Cache
// interface.
type InMemoryCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{entries: make(map[string]cacheEntry)}
}

func (c *InMemoryCache) Get(key string) (*CompletionResponse, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	cp := *e.resp
	cp.Cached = true
	return &cp, true
}

func (c *InMemoryCache) Set(key string, resp *CompletionResponse, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := *resp
	c.entries[key] = cacheEntry{resp: &cp, expiresAt: time.Now().Add(ttl)}
}

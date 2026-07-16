package ai

import "sync"

// Registry holds every configured Provider and the order in which they
// should be tried. It is safe for concurrent use.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
	order     []string
}

// NewRegistry builds an empty registry. Callers register providers with
// Register and set fallback order with SetOrder (or rely on Config's
// FallbackOrder via Service).
func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

// Register adds or replaces a provider by its Name().
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// SetOrder fixes the fallback order explicitly. Names not registered
// are ignored at resolution time rather than erroring, so config typos
// degrade gracefully instead of crashing the game server.
func (r *Registry) SetOrder(names []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.order = append([]string(nil), names...)
}

// mockProviderName must match mock.Provider.Name() in
// internal/ai/providers/mock. Duplicated here as a plain string
// (rather than importing that package, which internal/ai itself must
// not depend on per this package's layering) so Ordered can enforce
// ADR-003's invariant in code.
const mockProviderName = "mock"

// Ordered returns the currently-available providers in fallback order,
// with one hard rule enforced regardless of configured order: the
// provider named "mock" is always placed last.
//
// Confirmed bug (2026-07-16), not hypothetical: Config's defaults are
// AI_DEFAULT_PROVIDER="mock" and AI_FALLBACK_PROVIDERS="mock", so
// mock previously ended up first in r.order. Since mock.Available()
// is always true and mock.Complete() never returns an error, the
// Service.Complete fallback loop returned immediately after mock every
// time — real providers registered afterward (confirmed live: Qwen
// and Gemini, both reporting Available()==true with valid API keys
// configured) were never reached at all. ADR-003 always documented
// "mock is always last" as the intent, but nothing in this function
// previously enforced it — it only happened to be true when whoever
// deployed the service also happened to list real providers before
// "mock" in AI_FALLBACK_PROVIDERS, which nobody had been told to do.
// This function now makes that invariant true unconditionally, so a
// correctly-configured API key works the moment it's set, with zero
// additional AI_DEFAULT_PROVIDER/AI_FALLBACK_PROVIDERS configuration
// required.
func (r *Registry) Ordered() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []Provider
	var mock Provider
	seen := make(map[string]bool)

	addIfAvailable := func(name string) {
		if seen[name] {
			return
		}
		p, ok := r.providers[name]
		if !ok || !p.Available() {
			return
		}
		seen[name] = true
		if name == mockProviderName {
			mock = p
			return
		}
		out = append(out, p)
	}

	for _, name := range r.order {
		addIfAvailable(name)
	}
	// Any registered provider not mentioned in order is appended next,
	// so a newly-registered provider is never silently unreachable.
	for name := range r.providers {
		addIfAvailable(name)
	}

	if mock != nil {
		out = append(out, mock)
	}
	return out
}

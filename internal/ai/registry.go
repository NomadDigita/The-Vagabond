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

// Ordered returns the currently-available providers in fallback order.
// Unregistered names and providers reporting Available() == false are
// skipped.
func (r *Registry) Ordered() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []Provider
	seen := make(map[string]bool)
	for _, name := range r.order {
		if p, ok := r.providers[name]; ok && p.Available() && !seen[name] {
			out = append(out, p)
			seen[name] = true
		}
	}
	// Any registered provider not mentioned in order is appended last,
	// so a newly-registered provider is never silently unreachable.
	for name, p := range r.providers {
		if !seen[name] && p.Available() {
			out = append(out, p)
			seen[name] = true
		}
	}
	return out
}

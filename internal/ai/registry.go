package ai

import "sync"

type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
	order     []string
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
}

func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

func (r *Registry) SetOrder(names []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.order = append([]string(nil), names...)
}

const mockProviderName = "mock"

func (r *Registry) AllNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

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
	for name := range r.providers {
		addIfAvailable(name)
	}

	if mock != nil {
		out = append(out, mock)
	}
	return out
}

package datasource

import (
	"sort"
	"sync"
)

type Registry struct {
	mu            sync.RWMutex
	registrations map[string]Registration
}

var defaultRegistry = NewRegistry()

func NewRegistry() *Registry {
	return &Registry{registrations: make(map[string]Registration)}
}

func DefaultRegistry() *Registry {
	return defaultRegistry
}

func Register(reg Registration) {
	defaultRegistry.Register(reg)
}

func (r *Registry) Register(reg Registration) {
	if r == nil || reg.Info.Type == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registrations[reg.Info.Type] = reg
}

func (r *Registry) Get(dsType string) (Registration, bool) {
	if r == nil {
		return Registration{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	reg, ok := r.registrations[dsType]
	return reg, ok
}

func (r *Registry) SupportsType(dsType string) bool {
	_, ok := r.Get(dsType)
	return ok
}

func (r *Registry) ListTypes() []AdapterInfo {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]AdapterInfo, 0, len(r.registrations))
	for _, reg := range r.registrations {
		items = append(items, reg.Info)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Type < items[j].Type
	})
	return items
}

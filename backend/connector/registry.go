package connector

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Factory func(Target) (Adapter, error)

type registration struct {
	minimumVersion int
	maximumVersion int
	factory        Factory
}

type Registry struct {
	mu      sync.RWMutex
	entries map[string][]registration
}

func NewRegistry() *Registry {
	return &Registry{entries: make(map[string][]registration)}
}

func (r *Registry) Register(kind string, minimumVersion, maximumVersion int, factory Factory) error {
	kind = strings.TrimSpace(kind)
	if kind == "" || minimumVersion <= 0 || (maximumVersion > 0 && maximumVersion < minimumVersion) || factory == nil {
		return invalid("kind, version range and factory are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, current := range r.entries[kind] {
		if rangesOverlap(minimumVersion, maximumVersion, current.minimumVersion, current.maximumVersion) {
			return fmt.Errorf("%w: connector %s has overlapping version ranges", ErrInvalid, kind)
		}
	}
	r.entries[kind] = append(r.entries[kind], registration{minimumVersion: minimumVersion, maximumVersion: maximumVersion, factory: factory})
	sort.Slice(r.entries[kind], func(i, j int) bool {
		return r.entries[kind][i].minimumVersion > r.entries[kind][j].minimumVersion
	})
	return nil
}

func (r *Registry) Resolve(target Target) (Adapter, error) {
	r.mu.RLock()
	registrations := append([]registration(nil), r.entries[target.Resource.Kind]...)
	r.mu.RUnlock()
	for _, current := range registrations {
		if target.Resource.SchemaVersion >= current.minimumVersion && (current.maximumVersion == 0 || target.Resource.SchemaVersion <= current.maximumVersion) {
			return current.factory(target)
		}
	}
	return nil, connectorError(CategoryUnsupported, "resolve connector", false, fmt.Errorf("%w: %s schema v%d", ErrUnsupported, target.Resource.Kind, target.Resource.SchemaVersion))
}

func rangesOverlap(firstMin, firstMax, secondMin, secondMax int) bool {
	if firstMax == 0 {
		firstMax = int(^uint(0) >> 1)
	}
	if secondMax == 0 {
		secondMax = int(^uint(0) >> 1)
	}
	return firstMin <= secondMax && secondMin <= firstMax
}

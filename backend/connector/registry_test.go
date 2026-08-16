package connector

import (
	"context"
	"errors"
	"testing"

	"opskeeper/backend/resource"
)

type stubAdapter struct {
	kind         string
	capabilities []Capability
	test         func(context.Context) error
}

func (a *stubAdapter) Kind() string { return a.kind }

func (a *stubAdapter) Capabilities() []Capability {
	return append([]Capability(nil), a.capabilities...)
}

func (a *stubAdapter) Test(ctx context.Context) error {
	if a.test == nil {
		return nil
	}
	return a.test(ctx)
}

func TestRegistryResolvesMatchingSchemaVersion(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("Example", 1, 2, func(Target) (Adapter, error) {
		return &stubAdapter{kind: "legacy"}, nil
	}); err != nil {
		t.Fatalf("Register(legacy) error = %v", err)
	}
	if err := registry.Register("Example", 3, 0, func(Target) (Adapter, error) {
		return &stubAdapter{kind: "current"}, nil
	}); err != nil {
		t.Fatalf("Register(current) error = %v", err)
	}

	adapter, err := registry.Resolve(Target{Resource: resource.Resource{Kind: "Example", SchemaVersion: 4}})
	if err != nil || adapter.Kind() != "current" {
		t.Fatalf("Resolve(v4) = %#v, %v", adapter, err)
	}
	if _, err := registry.Resolve(Target{Resource: resource.Resource{Kind: "Example", SchemaVersion: 0}}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Resolve(v0) error = %v, want ErrUnsupported", err)
	}
}

func TestRegistryRejectsOverlappingRanges(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("Example", 1, 3, func(Target) (Adapter, error) { return &stubAdapter{}, nil }); err != nil {
		t.Fatalf("Register(first) error = %v", err)
	}
	if err := registry.Register("Example", 3, 5, func(Target) (Adapter, error) { return &stubAdapter{}, nil }); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Register(overlap) error = %v, want ErrInvalid", err)
	}
}

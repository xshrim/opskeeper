package authorization

import (
	"context"
	"testing"
	"time"
)

func TestMemoryScopeCacheRoundTrip(t *testing.T) {
	cache := NewMemoryScopeCache()
	want := ScopeFilter{SubjectID: "user-1", Permission: Permission("resource:read"), ScopeIDs: []string{"scope-1"}}
	if err := cache.Set(context.Background(), "key", want, time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	got, ok, err := cache.Get(context.Background(), "key")
	if err != nil || !ok {
		t.Fatalf("Get() = %#v, %v, %v", got, ok, err)
	}
	if got.SubjectID != want.SubjectID || got.Permission != want.Permission || len(got.ScopeIDs) != 1 || got.ScopeIDs[0] != want.ScopeIDs[0] {
		t.Fatalf("Get() = %#v, want %#v", got, want)
	}
}

func TestMemoryScopeCacheExpires(t *testing.T) {
	cache := NewMemoryScopeCache()
	if err := cache.Set(context.Background(), "key", ScopeFilter{}, time.Millisecond); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, ok, err := cache.Get(context.Background(), "key"); err != nil || ok {
		t.Fatalf("Get() after expiry = ok %v, err %v; want miss", ok, err)
	}
}

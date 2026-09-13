package elasticsearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientUsesReadOnlyElasticsearchAPIs(t *testing.T) {
	requests := make([]string, 0, 7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		requests = append(requests, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	c, err := New(ConnectionInput{URL: server.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx := context.Background()
	for _, call := range []func(context.Context) (any, error){c.Health, c.ClusterInfo, c.Nodes, c.Indices, c.Shards, c.Settings} {
		if _, err := call(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.IndexMapping(ctx, "orders 2026"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 7 || requests[6] != "/orders%202026/_mapping" {
		t.Fatalf("unexpected requests: %#v", requests)
	}
}

func TestConnectionRequiresHTTPEndpoint(t *testing.T) {
	if _, err := New(ConnectionInput{URL: "tcp://search:9200"}); err == nil {
		t.Fatal("New() accepted non-HTTP URL")
	}
}

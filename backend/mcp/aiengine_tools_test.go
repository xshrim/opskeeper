package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"opskeeper/backend/aiengine"
)

func TestPostgreSQLAgentArgumentsUseResourceConnection(t *testing.T) {
	p := mcpContextProvider{service: &Service{}}
	resource := aiengine.ContextResource{Kind: "PostgreSQL", Subtype: "agent", Config: map[string]any{"host": "configured-db", "port": float64(5432), "database": "ops", "timeout_seconds": float64(10)}}
	got, err := p.agentArguments(context.Background(), resource, map[string]any{"host": "model-db", "database": "other"})
	if err != nil {
		t.Fatalf("agentArguments(): %v", err)
	}
	if got["host"] != "configured-db" || got["database"] != "ops" || got["port"] != float64(5432) {
		t.Fatalf("arguments = %#v, want resource connection", got)
	}
}

func TestPostgreSQLAgentSchemaHidesConnectionFields(t *testing.T) {
	raw := json.RawMessage(`{"type":"object","required":["host","schema","table"],"properties":{"host":{"type":"string"},"password":{"type":"string"},"schema":{"type":"string"},"table":{"type":"string"}}}`)
	var schema map[string]any
	if err := json.Unmarshal(postgreSQLAgentSchema(raw), &schema); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	properties := schema["properties"].(map[string]any)
	if _, ok := properties["host"]; ok {
		t.Fatal("host must not be model-facing for Agent")
	}
	if _, ok := properties["password"]; ok {
		t.Fatal("password must not be model-facing for Agent")
	}
	if _, ok := properties["schema"]; !ok {
		t.Fatal("table schema argument must remain available")
	}
	if _, ok := schema["required"]; ok {
		t.Fatal("resource-owned connection requirements must be removed")
	}
}

func TestDockerAgentArgumentsUseResourceConnection(t *testing.T) {
	p := mcpContextProvider{service: &Service{}}
	resource := aiengine.ContextResource{
		Kind:    "Docker",
		Subtype: "agent",
		Config: map[string]any{
			"host":            "tcp://configured:2376",
			"timeout":         float64(18),
			"tls_ca":          "Y2E=",
			"skip_tls_verify": true,
		},
	}
	arguments := map[string]any{"host": "tcp://model-supplied:2376", "timeout": 1, "container_id": "abc"}
	got, err := p.dockerAgentArguments(context.Background(), resource, arguments)
	if err != nil {
		t.Fatalf("dockerAgentArguments: %v", err)
	}
	want := map[string]any{"host": "tcp://configured:2376", "timeout": float64(18), "tls_ca": "Y2E=", "skip_tls_verify": true, "container_id": "abc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("arguments = %#v, want %#v", got, want)
	}
}

func TestDockerAgentArgumentsAlwaysUseConfiguredConnection(t *testing.T) {
	p := mcpContextProvider{service: &Service{}}
	resource := aiengine.ContextResource{Kind: "Docker", Subtype: "agent", Config: map[string]any{"host": "tcp://configured:2376"}}
	arguments := map[string]any{"host": "tcp://model-supplied:2376"}
	got, err := p.dockerAgentArguments(context.Background(), resource, arguments)
	if err != nil {
		t.Fatalf("dockerAgentArguments: %v", err)
	}
	if reflect.DeepEqual(got, arguments) {
		t.Fatalf("arguments = %#v, want configured connection", got)
	}
}

func TestDockerAgentArgumentsWithEmptyConfigLeaveModelArgumentsUntouched(t *testing.T) {
	p := mcpContextProvider{service: &Service{}}
	resource := aiengine.ContextResource{Kind: "Docker", Subtype: "agent", Config: map[string]any{}}
	arguments := map[string]any{"host": "tcp://model-supplied:2376"}
	got, err := p.dockerAgentArguments(context.Background(), resource, arguments)
	if err != nil || !reflect.DeepEqual(got, arguments) {
		t.Fatalf("arguments = %#v err=%v, want unchanged", got, err)
	}
}

package mcp

import (
	"context"
	"reflect"
	"testing"

	"opskeeper/backend/aiengine"
)

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

package connector

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"opskeeper/backend/aiengine"
	"opskeeper/backend/resource"
)

type dockerResourceReader struct{ item resource.Resource }

func (r dockerResourceReader) Get(context.Context, string) (resource.Resource, error) {
	return r.item, nil
}

func TestDockerDirectProviderRegistersStableToolSet(t *testing.T) {
	item := resource.Resource{ID: "docker-1", Kind: "Docker", Status: resource.StatusActive, SchemaVersion: 1, Config: map[string]any{"host": "tcp://configured:2375"}}
	service := NewService(nil, dockerResourceReader{item: item}, nil, nil, DefaultLimits())
	tools, facts, err := service.AIEngineProvider().Resolve(context.Background(), aiengine.ContextResource{ID: item.ID, Kind: item.Kind, Subtype: "Direct", Status: item.Status, Config: item.Config})
	if err != nil {
		t.Fatalf("resolve Docker tools: %v", err)
	}
	if len(facts) != 0 {
		t.Fatalf("facts = %#v, want none", facts)
	}
	want := []string{"docker_info", "docker_images", "docker_containers", "docker_container_logs", "docker_container_file", "docker_container_inspect", "docker_container_stats"}
	got := make([]string, 0, len(tools))
	for _, tool := range tools {
		got = append(got, tool.Definition().Name)
		if tool.Definition().ResourceID != item.ID {
			t.Fatalf("tool %q resource id = %q", tool.Definition().Name, tool.Definition().ResourceID)
		}
		var schema map[string]any
		if err := json.Unmarshal(tool.Definition().InputSchema, &schema); err != nil {
			t.Fatalf("tool %q schema: %v", tool.Definition().Name, err)
		}
		properties := schema["properties"].(map[string]any)
		for _, hidden := range []string{"host", "timeout", "tls_ca", "tls_cert", "tls_key", "skip_tls_verify"} {
			if _, ok := properties[hidden]; ok {
				t.Fatalf("tool %q exposes adapter-owned field %q", tool.Definition().Name, hidden)
			}
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tool names = %v, want %v", got, want)
	}
}

func TestDockerConnectionConfigWinsOverCredential(t *testing.T) {
	service := &Service{}
	credentialID := "credential-1"
	item := aiengine.ContextResource{CredentialID: &credentialID, Config: map[string]any{"host": "tcp://configured:2375", "skip_tls_verify": true}}
	service.credentials = fakeDockerCredentialReader{secret: []byte(`{"host":"tcp://credential:2375","tls_ca":"credential-ca"}`)}
	connection, err := service.dockerConnection(context.Background(), item)
	if err != nil {
		t.Fatalf("dockerConnection: %v", err)
	}
	if connection.DockerHost != "tcp://configured:2375" || connection.DockerCA != "credential-ca" || !connection.DockerSkipVerify {
		t.Fatalf("connection = %#v", connection)
	}
}

type fakeDockerCredentialReader struct{ secret []byte }

func (f fakeDockerCredentialReader) RevealLinked(context.Context, string) ([]byte, error) {
	return f.secret, nil
}

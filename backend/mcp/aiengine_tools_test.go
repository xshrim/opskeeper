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

func TestOracleAgentArgumentsAndSchemaHideConnection(t *testing.T) {
	p := mcpContextProvider{service: &Service{}}
	r := aiengine.ContextResource{Kind: "Oracle", Subtype: "agent", Config: map[string]any{"host": "configured-oracle", "port": float64(1521), "service_name": "ORCL", "timeout_seconds": float64(10)}}
	got, err := p.agentArguments(context.Background(), r, map[string]any{"host": "model-oracle"})
	if err != nil || got["host"] != "configured-oracle" || got["service_name"] != "ORCL" {
		t.Fatalf("arguments=%#v err=%v", got, err)
	}
	raw := json.RawMessage(`{"type":"object","required":["host","schema","table"],"properties":{"host":{"type":"string"},"password":{"type":"string"},"schema":{"type":"string"},"table":{"type":"string"}}}`)
	var schema map[string]any
	if err := json.Unmarshal(oracleAgentSchema(raw), &schema); err != nil {
		t.Fatal(err)
	}
	props := schema["properties"].(map[string]any)
	if _, ok := props["host"]; ok {
		t.Fatal("host exposed")
	}
	if _, ok := props["password"]; ok {
		t.Fatal("password exposed")
	}
	if _, ok := props["schema"]; !ok {
		t.Fatal("business argument removed")
	}
}

func TestRedisAgentArgumentsUseResourceConnection(t *testing.T) {
	p := mcpContextProvider{service: &Service{}}
	r := aiengine.ContextResource{Kind: "Redis", Subtype: "agent", Config: map[string]any{"host": "configured-redis", "port": float64(6379), "database": float64(2), "timeout_seconds": float64(9)}}
	got, err := p.agentArguments(context.Background(), r, map[string]any{"host": "model-redis", "database": 0})
	if err != nil || got["host"] != "configured-redis" || got["database"] != float64(2) {
		t.Fatalf("arguments=%#v err=%v", got, err)
	}
}

func TestRedisAgentSchemaHidesConnectionFields(t *testing.T) {
	raw := json.RawMessage(`{"type":"object","required":["host"],"properties":{"host":{"type":"string"},"password":{"type":"string"},"limit":{"type":"integer"}}}`)
	var schema map[string]any
	if err := json.Unmarshal(redisAgentSchema(raw), &schema); err != nil {
		t.Fatal(err)
	}
	props := schema["properties"].(map[string]any)
	if _, ok := props["host"]; ok {
		t.Fatal("host exposed")
	}
	if _, ok := props["password"]; ok {
		t.Fatal("password exposed")
	}
	if _, ok := props["limit"]; !ok {
		t.Fatal("business field removed")
	}
}

func TestNacosAgentSchemaHidesConnectionFields(t *testing.T) {
	raw := json.RawMessage(`{"type":"object","required":["host"],"properties":{"host":{"type":"string"},"password":{"type":"string"},"service_name":{"type":"string"}}}`)
	var schema map[string]any
	if err := json.Unmarshal(nacosAgentSchema(raw), &schema); err != nil {
		t.Fatal(err)
	}
	props := schema["properties"].(map[string]any)
	if _, ok := props["host"]; ok {
		t.Fatal("host exposed")
	}
	if _, ok := props["password"]; ok {
		t.Fatal("password exposed")
	}
	if _, ok := props["service_name"]; !ok {
		t.Fatal("business field removed")
	}
}

func TestRabbitMQAgentArgumentsUseResourceConnection(t *testing.T) {
	p := mcpContextProvider{service: &Service{}}
	r := aiengine.ContextResource{Kind: "RabbitMQ", Subtype: "agent", Config: map[string]any{"url": "http://configured:15672", "timeout_seconds": float64(12), "tls_insecure": true}}
	got, err := p.agentArguments(context.Background(), r, map[string]any{"url": "http://model:1"})
	if err != nil || got["url"] != "http://configured:15672" {
		t.Fatalf("arguments=%#v err=%v", got, err)
	}
}

func TestMinIOAgentSchemaHidesConnectionFields(t *testing.T) {
	raw := json.RawMessage(`{"type":"object","required":["endpoint","bucket"],"properties":{"endpoint":{"type":"string"},"access_key":{"type":"string"},"bucket":{"type":"string"}}}`)
	var schema map[string]any
	if err := json.Unmarshal(minIOAgentSchema(raw), &schema); err != nil {
		t.Fatal(err)
	}
	props := schema["properties"].(map[string]any)
	if _, ok := props["endpoint"]; ok {
		t.Fatal("endpoint exposed")
	}
	if _, ok := props["access_key"]; ok {
		t.Fatal("access key exposed")
	}
	if _, ok := props["bucket"]; !ok {
		t.Fatal("bucket argument removed")
	}
}

func TestMinIOAgentArgumentsUseResourceConnection(t *testing.T) {
	p := mcpContextProvider{service: &Service{}}
	r := aiengine.ContextResource{Kind: "MinIO", Subtype: "agent", Config: map[string]any{"endpoint": "configured:9000", "region": "us-east-1", "secure": true}}
	got, err := p.agentArguments(context.Background(), r, map[string]any{"endpoint": "model:9000"})
	if err != nil || got["endpoint"] != "configured:9000" || got["region"] != "us-east-1" || got["secure"] != true {
		t.Fatalf("arguments=%#v err=%v", got, err)
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

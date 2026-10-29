package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"opskeeper/backend/engine"
)

func TestPostgreSQLAgentArgumentsUseResourceConnection(t *testing.T) {
	p := mcpContextProvider{service: &Service{}}
	resource := engine.ContextResource{Kind: "PostgreSQL", Subtype: "agent", Config: map[string]any{"host": "configured-db", "port": float64(5432), "database": "ops", "timeout_seconds": float64(10)}}
	got, err := p.agentArguments(context.Background(), resource, map[string]any{"host": "model-db", "database": "other"})
	if err != nil {
		t.Fatalf("agentArguments(): %v", err)
	}
	if got["host"] != "configured-db" || got["database"] != "ops" || got["port"] != float64(5432) {
		t.Fatalf("arguments = %#v, want resource connection", got)
	}
}

func TestAgentToolNamesMatchSupportedResourceToolkits(t *testing.T) {
	for kind, names := range map[string][]string{
		"PostgreSQL": {"postgresql_health", "postgresql_table_columns"},
		"Kubernetes": {"kubernetes_cluster_info", "kubernetes_resource_search", "kubernetes_health"},
	} {
		for _, name := range names {
			if _, ok := agentToolNames(kind)[name]; ok {
				continue
			}
			t.Errorf("Agent tool allowlist is missing %q", name)
		}
	}
	if _, ok := agentToolNames("PostgreSQL")["drop_database"]; ok {
		t.Fatal("Agent tool allowlist must reject tools outside the Direct toolkit")
	}
	if len(agentToolNames("TongRDS")) != 0 {
		t.Fatal("resource kinds without a Direct toolkit must not expose arbitrary Agent tools")
	}
}

func TestRepositoryAgentSchemaHidesStorageConnectionFields(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(repositoryAgentSchema([]byte(`{"type":"object","required":["path","query"],"properties":{"path":{"type":"string"},"url":{"type":"string"},"branch":{"type":"string"},"timeout_seconds":{"type":"integer"},"storage_backend":{"type":"string"},"storage_key":{"type":"string"},"s3_endpoint":{"type":"string"},"s3_bucket":{"type":"string"},"s3_prefix":{"type":"string"},"query":{"type":"string"}}}`)), &schema); err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)
	for _, key := range []string{"path", "url", "timeout_seconds", "storage_backend", "storage_key", "s3_endpoint", "s3_bucket", "s3_prefix"} {
		if _, ok := properties[key]; ok {
			t.Errorf("Repository Agent schema must hide connection field %q", key)
		}
	}
	if _, ok := properties["query"]; !ok {
		t.Fatal("Repository Agent schema must keep operation arguments")
	}
	if !reflect.DeepEqual(schema["required"], []any{"query"}) {
		t.Fatalf("required=%#v; only operation fields should remain required", schema["required"])
	}
}

func TestKubernetesAgentContextArgumentOverridesResourceDefault(t *testing.T) {
	p := mcpContextProvider{service: &Service{}}
	resource := engine.ContextResource{Kind: "Kubernetes", Subtype: "agent", Config: map[string]any{"context": "resource-default", "connection_mode": "kubeconfig"}}
	got, err := p.agentArguments(context.Background(), resource, map[string]any{"context": " alternate "})
	if err != nil || got["context"] != "alternate" {
		t.Fatalf("explicit context arguments=%#v err=%v", got, err)
	}
	got, err = p.agentArguments(context.Background(), resource, map[string]any{})
	if err != nil || got["context"] != "resource-default" {
		t.Fatalf("default context arguments=%#v err=%v", got, err)
	}
}

func TestKubernetesAgentSchemaHidesConnectionAndKeepsContext(t *testing.T) {
	raw := json.RawMessage(`{"type":"object","required":["kubeconfig","context","namespace"],"properties":{"kubeconfig":{"type":"string"},"context":{"type":"string"},"server":{"type":"string"},"token":{"type":"string"},"namespace":{"type":"string"}}}`)
	var schema map[string]any
	if err := json.Unmarshal(kubernetesAgentSchema(raw), &schema); err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)
	for _, key := range []string{"kubeconfig", "server", "token"} {
		if _, ok := properties[key]; ok {
			t.Errorf("connection field %q exposed to model", key)
		}
	}
	if _, ok := properties["context"]; !ok {
		t.Fatal("context not exposed")
	}
	if _, ok := properties["namespace"]; !ok {
		t.Fatal("tool argument namespace was removed")
	}
	if !reflect.DeepEqual(schema["required"], []any{"namespace"}) {
		t.Fatalf("required=%#v", schema["required"])
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
	if !reflect.DeepEqual(schema["required"], []any{"schema", "table"}) {
		t.Fatalf("required=%#v; business arguments must remain required", schema["required"])
	}
}

func TestOracleAgentArgumentsAndSchemaHideConnection(t *testing.T) {
	p := mcpContextProvider{service: &Service{}}
	r := engine.ContextResource{Kind: "Oracle", Subtype: "agent", Config: map[string]any{"host": "configured-oracle", "port": float64(1521), "service_name": "ORCL", "timeout_seconds": float64(10)}}
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
	if !reflect.DeepEqual(schema["required"], []any{"schema", "table"}) {
		t.Fatalf("required=%#v; business arguments must remain required", schema["required"])
	}
}

func TestOceanBaseAgentArgumentsAndSchemaHideConnection(t *testing.T) {
	p := mcpContextProvider{service: &Service{}}
	r := engine.ContextResource{Kind: "OceanBase", Subtype: "agent", Config: map[string]any{"host": "configured-ob", "port": float64(2881), "mode": "oracle", "service_name": "ORCL"}}
	got, err := p.agentArguments(context.Background(), r, map[string]any{"host": "model-ob"})
	if err != nil || got["host"] != "configured-ob" || got["mode"] != "oracle" {
		t.Fatalf("arguments=%#v err=%v", got, err)
	}
	raw := json.RawMessage(`{"type":"object","required":["host","schema","table"],"properties":{"host":{"type":"string"},"password":{"type":"string"},"schema":{"type":"string"},"table":{"type":"string"}}}`)
	var schema map[string]any
	if err := json.Unmarshal(oceanBaseAgentSchema(raw), &schema); err != nil {
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
	if !reflect.DeepEqual(schema["required"], []any{"schema", "table"}) {
		t.Fatalf("required=%#v; business arguments must remain required", schema["required"])
	}
}

func TestRedisAgentArgumentsUseResourceConnection(t *testing.T) {
	p := mcpContextProvider{service: &Service{}}
	r := engine.ContextResource{Kind: "Redis", Subtype: "agent", Config: map[string]any{"host": "configured-redis", "port": float64(6379), "database": float64(2), "timeout_seconds": float64(9)}}
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
	r := engine.ContextResource{Kind: "RabbitMQ", Subtype: "agent", Config: map[string]any{"url": "http://configured:15672", "timeout_seconds": float64(12), "tls_insecure": true}}
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
	if !reflect.DeepEqual(schema["required"], []any{"bucket"}) {
		t.Fatalf("required=%#v; bucket must remain required", schema["required"])
	}
}

func TestDockerAgentSchemaHidesConnectionFieldsAndKeepsOperationRequirements(t *testing.T) {
	raw := json.RawMessage(`{"type":"object","required":["host","container_id","path"],"properties":{"host":{"type":"string"},"timeout":{"type":"integer"},"tls_ca":{"type":"string"},"tls_cert":{"type":"string"},"tls_key":{"type":"string"},"skip_tls_verify":{"type":"boolean"},"container_id":{"type":"string"},"path":{"type":"string"}}}`)
	var schema map[string]any
	if err := json.Unmarshal(dockerAgentSchema(raw), &schema); err != nil {
		t.Fatal(err)
	}
	props := schema["properties"].(map[string]any)
	for _, key := range []string{"host", "timeout", "tls_ca", "tls_cert", "tls_key", "skip_tls_verify"} {
		if _, ok := props[key]; ok {
			t.Errorf("connection field %q exposed", key)
		}
	}
	if !reflect.DeepEqual(schema["required"], []any{"container_id", "path"}) {
		t.Fatalf("required=%#v; Docker operation arguments must remain required", schema["required"])
	}
}

func TestElasticsearchAgentSchemaKeepsOperationRequirements(t *testing.T) {
	raw := json.RawMessage(`{"type":"object","required":["url","index"],"properties":{"url":{"type":"string"},"username":{"type":"string"},"password":{"type":"string"},"tls_insecure":{"type":"boolean"},"timeout_seconds":{"type":"integer"},"index":{"type":"string"}}}`)
	var schema map[string]any
	if err := json.Unmarshal(elasticsearchAgentSchema(raw), &schema); err != nil {
		t.Fatal(err)
	}
	props := schema["properties"].(map[string]any)
	for _, key := range []string{"url", "username", "password", "tls_insecure", "timeout_seconds"} {
		if _, ok := props[key]; ok {
			t.Errorf("connection field %q exposed", key)
		}
	}
	if !reflect.DeepEqual(schema["required"], []any{"index"}) {
		t.Fatalf("required=%#v; index must remain required", schema["required"])
	}
}

func TestMinIOAgentArgumentsUseResourceConnection(t *testing.T) {
	p := mcpContextProvider{service: &Service{}}
	r := engine.ContextResource{Kind: "MinIO", Subtype: "agent", Config: map[string]any{"endpoint": "configured:9000", "region": "us-east-1", "secure": true}}
	got, err := p.agentArguments(context.Background(), r, map[string]any{"endpoint": "model:9000"})
	if err != nil || got["endpoint"] != "configured:9000" || got["region"] != "us-east-1" || got["secure"] != true {
		t.Fatalf("arguments=%#v err=%v", got, err)
	}
}

func TestDockerAgentArgumentsUseResourceConnection(t *testing.T) {
	p := mcpContextProvider{service: &Service{}}
	resource := engine.ContextResource{
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
	resource := engine.ContextResource{Kind: "Docker", Subtype: "agent", Config: map[string]any{"host": "tcp://configured:2376"}}
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
	resource := engine.ContextResource{Kind: "Docker", Subtype: "agent", Config: map[string]any{}}
	arguments := map[string]any{"host": "tcp://model-supplied:2376"}
	got, err := p.dockerAgentArguments(context.Background(), resource, arguments)
	if err != nil || !reflect.DeepEqual(got, arguments) {
		t.Fatalf("arguments = %#v err=%v, want unchanged", got, err)
	}
}

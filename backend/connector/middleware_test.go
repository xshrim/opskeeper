package connector

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"opskeeper/backend/resource"
)

func TestPostgreSQLAdapterRequiresCompleteConfiguration(t *testing.T) {
	_, err := newPostgreSQLAdapter(Target{Resource: resource.Resource{Config: map[string]any{"host": "db"}}}, DefaultLimits())
	if category, _ := classify(err); category != CategoryConfiguration {
		t.Fatalf("newPostgreSQLAdapter() error = %v, category = %s", err, category)
	}
	secret, _ := json.Marshal(map[string]string{"username": "user@name", "password": "p@ss:/?word"})
	adapter, err := newPostgreSQLAdapter(Target{Resource: resource.Resource{Config: map[string]any{"host": "db.example", "database": "opskeeper"}}, Secret: secret}, DefaultLimits())
	if err != nil || adapter == nil {
		t.Fatalf("newPostgreSQLAdapter() = %v, %v", adapter, err)
	}
}

func TestKafkaPartitionFactsDetectReplicationFailures(t *testing.T) {
	topics, underReplicated, offline := kafkaPartitionFacts([]kafka.Partition{
		{Topic: "orders", ID: 0, Replicas: []kafka.Broker{{ID: 1}, {ID: 2}}, Isr: []kafka.Broker{{ID: 1}}},
		{Topic: "orders", ID: 1, Replicas: []kafka.Broker{{ID: 1}}, Isr: []kafka.Broker{{ID: 1}}, OfflineReplicas: []kafka.Broker{{ID: 2}}},
	})
	if topics != 1 || underReplicated != 1 || offline != 1 {
		t.Fatalf("kafkaPartitionFacts() = %d/%d/%d", topics, underReplicated, offline)
	}
}

func TestRedisInfoParserAndAuthenticationClassification(t *testing.T) {
	parsed := parseRedisInfo("# Memory\nused_memory:123\nrole:master\n")
	if parsed["used_memory"] != int64(123) || parsed["role"] != "master" {
		t.Fatalf("parseRedisInfo() = %#v", parsed)
	}
	if category, temporary := classify(redisError("info", errors.New("NOPERM this user has no permissions"))); category != CategoryAuthentication || temporary {
		t.Fatalf("redis authentication classification = %s/%v", category, temporary)
	}
}

func TestKafkaAdapterValidatesAuthenticationModesWithoutDroppingThem(t *testing.T) {
	secret, _ := json.Marshal(map[string]string{"username": "diagnostic", "password": "secret"})
	adapter, err := newKafkaAdapter(Target{Resource: resource.Resource{Config: map[string]any{"brokers": []any{"kafka:9092"}, "tls": true}}, Secret: secret}, DefaultLimits())
	if err != nil || adapter == nil {
		t.Fatalf("authenticated Kafka adapter = %v, %v", adapter, err)
	}
	partial, _ := json.Marshal(map[string]string{"username": "diagnostic"})
	_, err = newKafkaAdapter(Target{Resource: resource.Resource{Config: map[string]any{"brokers": []any{"kafka:9092"}}}, Secret: partial}, DefaultLimits())
	if category, _ := classify(err); category != CategoryConfiguration {
		t.Fatalf("partial Kafka credentials error = %v, category = %s", err, category)
	}
	_, err = newKafkaAdapter(Target{Resource: resource.Resource{Config: map[string]any{"brokers": []any{}}}}, DefaultLimits())
	if category, _ := classify(err); category != CategoryConfiguration {
		t.Fatalf("empty Kafka brokers error = %v, category = %s", err, category)
	}
}

func TestDiagnosticSnapshotCollectsTypedEvidence(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("PostgreSQL", 1, 1, func(Target) (Adapter, error) { return diagnosticAdapter{}, nil }); err != nil {
		t.Fatalf("register adapter: %v", err)
	}
	limits := DefaultLimits()
	limits.Retries = 0
	service := NewService(registry, stubResourceReader{item: resource.Resource{ID: "postgres-1", Kind: "PostgreSQL", SchemaVersion: 1, Status: resource.StatusActive}}, nil, nil, limits)
	service.now = func() time.Time { return time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC) }
	evidence, err := service.InspectPostgreSQL(context.Background(), "postgres-1")
	if err != nil {
		t.Fatalf("InspectPostgreSQL() error = %v", err)
	}
	if evidence.SourceResourceID != "postgres-1" || evidence.Capability != CapabilityPostgreSQLInspect || evidence.Summary["finding_count"] != 1 {
		t.Fatalf("evidence = %#v", evidence)
	}
	var snapshot DiagnosticSnapshot
	if err := json.Unmarshal(evidence.Data, &snapshot); err != nil || snapshot.Facts["active_sessions"] != float64(1) {
		t.Fatalf("evidence snapshot = %#v, %v", snapshot, err)
	}
}

type diagnosticAdapter struct{}

func (diagnosticAdapter) Kind() string { return "PostgreSQL" }
func (diagnosticAdapter) Capabilities() []Capability {
	return []Capability{CapabilityPostgreSQLInspect}
}
func (diagnosticAdapter) Test(context.Context) error { return nil }
func (diagnosticAdapter) InspectPostgreSQL(context.Context) (DiagnosticSnapshot, error) {
	return DiagnosticSnapshot{Kind: "PostgreSQL", Facts: map[string]any{"active_sessions": 1}, Findings: []Finding{{Code: "test.finding", Severity: "info", Message: "test"}}, Capabilities: []string{"sessions"}}, nil
}

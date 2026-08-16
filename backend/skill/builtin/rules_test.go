package builtin

import (
	"opskeeper/backend/connector"
	"testing"
)

func TestEvaluateUsesFactsAndDoesNotDuplicateConnectorFindings(t *testing.T) {
	items := Evaluate(connector.DiagnosticSnapshot{Kind: "Kafka", Facts: map[string]any{"offline_replicas": int64(1)}, Findings: []connector.Finding{{Code: "kafka.offline_replicas", Severity: "critical", Message: "upstream"}}})
	if len(items) != 1 || items[0].Code != "kafka.offline_replicas" {
		t.Fatalf("findings = %#v", items)
	}
}

func TestEvaluateUsesDeterministicThresholds(t *testing.T) {
	items := Evaluate(connector.DiagnosticSnapshot{Kind: "PostgreSQL", Facts: map[string]any{"long_running_queries": 1}})
	if len(items) != 1 || items[0].Code != "postgresql.long_running_queries" {
		t.Fatalf("findings = %#v", items)
	}
}

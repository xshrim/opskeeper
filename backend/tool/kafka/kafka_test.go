package kafka

import (
	"encoding/json"
	"testing"
)

func TestInputSchemaAndToolsAreStable(t *testing.T) {
	tools := ListTools()
	if len(tools) != 7 || tools[0].Name != "kafka_health" || tools[len(tools)-1].Name != "kafka_topic_partitions" {
		t.Fatalf("unexpected Kafka tools: %#v", tools)
	}
	schema := InputSchema(map[string]any{"topic": map[string]any{"type": "string"}})
	properties, ok := schema["properties"].(map[string]any)
	if !ok || properties["brokers"] == nil || properties["topic"] == nil {
		t.Fatalf("schema missing connection or tool fields: %#v", schema)
	}
}

func TestTopicInfoUsesDistinctMetricFields(t *testing.T) {
	raw, err := json.Marshal(TopicInfo{Name: "orders", Partitions: 3, ReplicationFactor: 2, UnderReplicated: 1, OfflineReplicas: 1})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	for _, key := range []string{"partitions", "replication_factor", "under_replicated_partitions", "offline_replicas"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("TopicInfo missing %q: %s", key, raw)
		}
	}
}

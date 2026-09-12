package connector

import (
	"encoding/json"
	"errors"
	"testing"

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

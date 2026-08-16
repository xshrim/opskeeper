// Package builtin contains immutable declarations for the Skills shipped with
// the application. Installation and publication remain explicit operations;
// a declaration never gives a model arbitrary protocol access.
package builtin

import (
	"encoding/json"
	"time"

	"opskeeper/backend/connector"
	"opskeeper/backend/skill"
)

type Definition struct {
	Key        string
	Manifest   skill.Manifest
	Tools      []skill.ToolSpec
	Capability connector.Capability
	Timeout    time.Duration
}

func Definitions() []Definition {
	inspectSchema := json.RawMessage(`{"type":"object","required":["target_resource_id"],"properties":{"target_resource_id":{"type":"string"}},"additionalProperties":false}`)
	return []Definition{
		{Key: "kubernetes-workload", Manifest: skill.Manifest{Name: "Kubernetes 工作负载诊断", Description: "只读检查工作负载、Pod、事件、探针与发布状态。", Instruction: "仅调用声明的只读 Kubernetes 工具。将事实、异常和待验证假设明确分离。", TargetKinds: []string{"Application", "Kubernetes"}}, Tools: []skill.ToolSpec{{Name: "connector_kubernetes_read", Description: "受限读取 Kubernetes 对象。", InputSchema: json.RawMessage(`{"type":"object","required":["target_resource_id","resource"],"properties":{"target_resource_id":{"type":"string"},"resource":{"type":"string"},"namespace":{"type":"string"},"name":{"type":"string"},"label_selector":{"type":"string"},"limit":{"type":"integer"}},"additionalProperties":false}`)}}, Capability: connector.CapabilityKubernetesRead, Timeout: 10 * time.Second},
		{Key: "postgresql-health", Manifest: skill.Manifest{Name: "PostgreSQL 健康诊断", Description: "只读检查连接、会话、慢查询、锁、复制和容量。", Instruction: "调用 PostgreSQL 固定诊断快照；不要请求或建议写 SQL。", TargetKinds: []string{"PostgreSQL"}}, Tools: []skill.ToolSpec{{Name: "connector_postgresql_inspect", Description: "读取固定 PostgreSQL 健康快照。", InputSchema: inspectSchema}}, Capability: connector.CapabilityPostgreSQLInspect, Timeout: 10 * time.Second},
		{Key: "redis-health", Manifest: skill.Manifest{Name: "Redis 健康诊断", Description: "只读检查内存、热 Key 能力、慢命令、客户端和复制。", Instruction: "调用 Redis 固定诊断快照；不要请求或建议写命令。", TargetKinds: []string{"Redis"}}, Tools: []skill.ToolSpec{{Name: "connector_redis_inspect", Description: "读取固定 Redis 健康快照。", InputSchema: inspectSchema}}, Capability: connector.CapabilityRedisInspect, Timeout: 10 * time.Second},
		{Key: "kafka-health", Manifest: skill.Manifest{Name: "Kafka 健康诊断", Description: "只读检查 Broker、Topic、分区、ISR、消费组和积压。", Instruction: "调用 Kafka 固定诊断快照；不要请求或建议管理操作。", TargetKinds: []string{"Kafka"}}, Tools: []skill.ToolSpec{{Name: "connector_kafka_inspect", Description: "读取固定 Kafka 健康快照。", InputSchema: inspectSchema}}, Capability: connector.CapabilityKafkaInspect, Timeout: 10 * time.Second},
	}
}

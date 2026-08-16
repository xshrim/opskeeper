package connector

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

type kafkaAdapter struct {
	broker    string
	dialer    *kafka.Dialer
	transport *kafka.Transport
}

func newKafkaAdapter(target Target, _ Limits) (Adapter, error) {
	secret := secretFields(target.Secret)
	username, password := strings.TrimSpace(secret["username"]), secret["password"]
	if (username == "") != (password == "") {
		return nil, connectorError(CategoryConfiguration, "configure Kafka", false, errors.New("Kafka credential must contain both username and password"))
	}
	brokers := brokerAddresses(target.Resource.Config["brokers"])
	if len(brokers) == 0 {
		return nil, connectorError(CategoryConfiguration, "configure Kafka", false, errors.New("at least one broker is required"))
	}
	broker := brokers[0]
	if strings.TrimSpace(broker) == "" {
		return nil, connectorError(CategoryConfiguration, "configure Kafka", false, errors.New("broker address is required"))
	}
	dialer := &kafka.Dialer{Timeout: 8 * time.Second, DualStack: true}
	transport := &kafka.Transport{}
	if configBool(target.Resource.Config, "tls") {
		serverName := configString(target.Resource.Config, "tls_server_name")
		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName}
		dialer.TLS, transport.TLS = tlsConfig, tlsConfig
	}
	if username != "" {
		mechanism := plain.Mechanism{Username: username, Password: password}
		dialer.SASLMechanism, transport.SASL = mechanism, mechanism
	}
	return &kafkaAdapter{broker: broker, dialer: dialer, transport: transport}, nil
}

func brokerAddresses(value any) []string {
	switch items := value.(type) {
	case []any:
		result := make([]string, 0, len(items))
		for _, item := range items {
			if broker, ok := item.(string); ok && strings.TrimSpace(broker) != "" {
				result = append(result, strings.TrimSpace(broker))
			}
		}
		return result
	case []string:
		result := make([]string, 0, len(items))
		for _, broker := range items {
			if strings.TrimSpace(broker) != "" {
				result = append(result, strings.TrimSpace(broker))
			}
		}
		return result
	default:
		return nil
	}
}

func configBool(config map[string]any, key string) bool { value, _ := config[key].(bool); return value }

func (a *kafkaAdapter) Kind() string               { return "Kafka" }
func (a *kafkaAdapter) Capabilities() []Capability { return []Capability{CapabilityKafkaInspect} }
func (a *kafkaAdapter) Test(ctx context.Context) error {
	conn, err := a.dialer.DialContext(ctx, "tcp", a.broker)
	if err != nil {
		return kafkaError("connect Kafka", err)
	}
	defer conn.Close()
	_, err = conn.Controller()
	return kafkaError("read Kafka controller", err)
}
func (a *kafkaAdapter) InspectKafka(ctx context.Context) (DiagnosticSnapshot, error) {
	conn, err := a.dialer.DialContext(ctx, "tcp", a.broker)
	if err != nil {
		return DiagnosticSnapshot{}, kafkaError("connect Kafka", err)
	}
	defer conn.Close()
	partitions, err := conn.ReadPartitions()
	if err != nil {
		return DiagnosticSnapshot{}, kafkaError("read Kafka metadata", err)
	}
	snapshot := DiagnosticSnapshot{Kind: "Kafka", Facts: map[string]any{"partition_count": len(partitions)}, Findings: []Finding{}, Capabilities: []string{"topics", "partitions", "isr"}, Unavailable: []string{}}
	if brokers, err := conn.Brokers(); err != nil {
		snapshot.Unavailable = append(snapshot.Unavailable, "brokers")
	} else {
		snapshot.Capabilities = append(snapshot.Capabilities, "brokers")
		snapshot.Facts["broker_count"] = len(brokers)
	}
	topics, underReplicated, offline := kafkaPartitionFacts(partitions)
	snapshot.Facts["topic_count"] = topics
	snapshot.Facts["under_replicated_partitions"] = underReplicated
	snapshot.Facts["offline_replicas"] = offline
	if underReplicated > 0 {
		snapshot.Findings = append(snapshot.Findings, Finding{Code: "kafka.under_replicated", Severity: "warning", Message: fmt.Sprintf("存在 %d 个 ISR 不完整分区", underReplicated)})
	}
	if offline > 0 {
		snapshot.Findings = append(snapshot.Findings, Finding{Code: "kafka.offline_replicas", Severity: "critical", Message: fmt.Sprintf("存在 %d 个离线副本", offline)})
	}
	a.inspectGroups(ctx, partitions, &snapshot)
	sort.Strings(snapshot.Capabilities)
	sort.Strings(snapshot.Unavailable)
	return snapshot, nil
}

func kafkaPartitionFacts(partitions []kafka.Partition) (topicCount, underReplicated, offline int) {
	topics := map[string]bool{}
	for _, partition := range partitions {
		topics[partition.Topic] = true
		if len(partition.Isr) < len(partition.Replicas) {
			underReplicated++
		}
		offline += len(partition.OfflineReplicas)
	}
	return len(topics), underReplicated, offline
}

// inspectGroups uses only Kafka metadata and consumer-offset APIs. Every
// optional request is independently degradable because it is commonly denied
// by Group ACLs or unsupported by older brokers.
func (a *kafkaAdapter) inspectGroups(ctx context.Context, partitions []kafka.Partition, snapshot *DiagnosticSnapshot) {
	client := &kafka.Client{Addr: kafka.TCP(a.broker), Timeout: 8 * time.Second, Transport: a.transport}
	groups, err := client.ListGroups(ctx, &kafka.ListGroupsRequest{})
	if err != nil || groups.Error != nil {
		snapshot.Unavailable = append(snapshot.Unavailable, "consumer_groups", "lag")
		return
	}
	ids := make([]string, 0, len(groups.Groups))
	for _, group := range groups.Groups {
		if group.ProtocolType == "consumer" {
			ids = append(ids, group.GroupID)
		}
	}
	snapshot.Capabilities = append(snapshot.Capabilities, "consumer_groups")
	snapshot.Facts["consumer_group_count"] = len(ids)
	if len(ids) == 0 {
		snapshot.Capabilities = append(snapshot.Capabilities, "lag")
		snapshot.Facts["consumer_lag"] = int64(0)
		return
	}
	details, err := client.DescribeGroups(ctx, &kafka.DescribeGroupsRequest{GroupIDs: ids})
	if err != nil {
		snapshot.Unavailable = append(snapshot.Unavailable, "consumer_group_details", "lag")
		return
	}
	members := 0
	for _, group := range details.Groups {
		if group.Error != nil {
			snapshot.Unavailable = append(snapshot.Unavailable, "consumer_group_details", "lag")
			return
		}
		members += len(group.Members)
	}
	snapshot.Facts["consumer_group_members"] = members
	lag, err := a.consumerLag(ctx, client, ids, partitions)
	if err != nil {
		snapshot.Unavailable = append(snapshot.Unavailable, "lag")
		return
	}
	snapshot.Capabilities = append(snapshot.Capabilities, "lag")
	snapshot.Facts["consumer_lag"] = lag
	if lag > 0 {
		snapshot.Findings = append(snapshot.Findings, Finding{Code: "kafka.consumer_lag", Severity: "warning", Message: fmt.Sprintf("消费者组累计积压为 %d 条消息", lag)})
	}
}

func (a *kafkaAdapter) consumerLag(ctx context.Context, client *kafka.Client, groupIDs []string, partitions []kafka.Partition) (int64, error) {
	leaders := make(map[string]kafka.Partition, len(partitions))
	for _, partition := range partitions {
		leaders[kafkaPartitionKey(partition.Topic, partition.ID)] = partition
	}
	var total int64
	for _, groupID := range groupIDs {
		offsets, err := client.OffsetFetch(ctx, &kafka.OffsetFetchRequest{GroupID: groupID})
		if err != nil || offsets.Error != nil {
			if err == nil {
				err = offsets.Error
			}
			return 0, err
		}
		for topic, values := range offsets.Topics {
			for _, offset := range values {
				if offset.Error != nil || offset.CommittedOffset < 0 {
					continue
				}
				partition, ok := leaders[kafkaPartitionKey(topic, offset.Partition)]
				if !ok {
					continue
				}
				leader := fmt.Sprintf("%s:%d", partition.Leader.Host, partition.Leader.Port)
				conn, err := a.dialer.DialLeader(ctx, "tcp", leader, topic, offset.Partition)
				if err != nil {
					return 0, err
				}
				last, readErr := conn.ReadLastOffset()
				closeErr := conn.Close()
				if readErr != nil {
					return 0, readErr
				}
				if closeErr != nil {
					return 0, closeErr
				}
				if last > offset.CommittedOffset {
					total += last - offset.CommittedOffset
				}
			}
		}
	}
	return total, nil
}

func kafkaPartitionKey(topic string, partition int) string {
	return topic + ":" + strconv.Itoa(partition)
}
func kafkaError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return connectorError(CategoryTimeout, operation, true, err)
	}
	return connectorError(CategoryUpstream, operation, true, err)
}

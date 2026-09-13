package kafka

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

type ConnectionInput struct {
	Brokers        []string `json:"brokers"`
	Username       string   `json:"username,omitempty"`
	Password       string   `json:"password,omitempty"`
	TLS            bool     `json:"tls,omitempty"`
	TLSServerName  string   `json:"tls_server_name,omitempty"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
}
type HealthOutput struct {
	Controller      string `json:"controller"`
	BrokerCount     int    `json:"broker_count"`
	TopicCount      int    `json:"topic_count"`
	PartitionCount  int    `json:"partition_count"`
	UnderReplicated int    `json:"under_replicated_partitions"`
	OfflineReplicas int    `json:"offline_replicas"`
	LatencyMS       int64  `json:"latency_ms"`
}
type BrokerInfo struct {
	ID   int    `json:"id"`
	Host string `json:"host"`
	Port int    `json:"port"`
	Rack string `json:"rack,omitempty"`
}
type BrokersOutput struct {
	Brokers []BrokerInfo `json:"brokers"`
}
type TopicInfo struct {
	Name              string `json:"name"`
	Partitions        int    `json:"partitions"`
	ReplicationFactor int    `json:"replication_factor"`
	UnderReplicated   int    `json:"under_replicated_partitions"`
	OfflineReplicas   int    `json:"offline_replicas"`
}
type TopicsOutput struct {
	Topics []TopicInfo `json:"topics"`
}
type ConsumerGroupInfo struct {
	ID       string `json:"id"`
	Protocol string `json:"protocol"`
	Members  int    `json:"members"`
}
type ConsumerGroupsOutput struct {
	Groups []ConsumerGroupInfo `json:"groups"`
}
type ConsumerLagInput struct {
	Limit int `json:"limit,omitempty"`
}
type ConsumerLagInfo struct {
	Group         string `json:"group"`
	Topic         string `json:"topic"`
	Partition     int    `json:"partition"`
	CurrentOffset int64  `json:"current_offset"`
	EndOffset     int64  `json:"end_offset"`
	Lag           int64  `json:"lag"`
}
type ConsumerLagOutput struct {
	Entries  []ConsumerLagInfo `json:"entries"`
	TotalLag int64             `json:"total_lag"`
}
type TopicPartitionsInput struct {
	Topic string `json:"topic"`
}
type TopicPartitionsOutput struct {
	Topic      string           `json:"topic"`
	Partitions []map[string]any `json:"partitions"`
}
type ClusterInfoOutput struct {
	Controller BrokerInfo   `json:"controller"`
	Brokers    []BrokerInfo `json:"brokers"`
}
type ToolInfo struct{ Name, Description string }

var tools = []ToolInfo{{"kafka_health", "Check Kafka cluster health and exporter-style summary."}, {"kafka_brokers", "List Kafka brokers."}, {"kafka_topics", "List bounded Kafka topic metrics."}, {"kafka_consumer_groups", "List Kafka consumer groups."}, {"kafka_consumer_lag", "Read bounded Kafka consumer lag."}, {"kafka_cluster_info", "Read Kafka controller and broker metadata."}, {"kafka_topic_partitions", "Read partitions for a specific Kafka topic."}}

func ListTools() []ToolInfo { return append([]ToolInfo(nil), tools...) }
func InputSchema(extra map[string]any) map[string]any {
	p := map[string]any{"brokers": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 1}, "username": map[string]any{"type": "string"}, "password": map[string]any{"type": "string"}, "tls": map[string]any{"type": "boolean"}, "tls_server_name": map[string]any{"type": "string"}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": 300}}
	for k, v := range extra {
		p[k] = v
	}
	return map[string]any{"type": "object", "properties": p, "additionalProperties": false}
}
func open(ctx context.Context, in ConnectionInput) (*kafka.Conn, *kafka.Dialer, error) {
	if len(in.Brokers) == 0 {
		return nil, nil, errors.New("at least one broker is required")
	}
	broker := strings.TrimSpace(in.Brokers[0])
	if broker == "" {
		return nil, nil, errors.New("broker address is required")
	}
	timeout := 10 * time.Second
	if in.TimeoutSeconds > 0 {
		timeout = time.Duration(in.TimeoutSeconds) * time.Second
	}
	d := &kafka.Dialer{Timeout: timeout, DualStack: true}
	if in.TLS {
		t := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: in.TLSServerName}
		d.TLS = t
	}
	if in.Username != "" {
		if in.Password == "" {
			return nil, nil, errors.New("password is required with username")
		}
		d.SASLMechanism = plain.Mechanism{Username: in.Username, Password: in.Password}
	}
	c, e := d.DialContext(ctx, "tcp", broker)
	if e != nil {
		return nil, nil, e
	}
	return c, d, nil
}
func Health(ctx context.Context, in ConnectionInput) (HealthOutput, error) {
	started := time.Now()
	c, _, e := open(ctx, in)
	if e != nil {
		return HealthOutput{}, e
	}
	defer c.Close()
	controller, e := c.Controller()
	if e != nil {
		return HealthOutput{}, e
	}
	ps, e := c.ReadPartitions()
	if e != nil {
		return HealthOutput{}, e
	}
	bs, e := c.Brokers()
	if e != nil {
		return HealthOutput{}, e
	}
	topics := map[string]bool{}
	ur, off := 0, 0
	for _, p := range ps {
		topics[p.Topic] = true
		if len(p.Isr) < len(p.Replicas) {
			ur++
		}
		off += len(p.OfflineReplicas)
	}
	return HealthOutput{Controller: fmt.Sprintf("%s:%d", controller.Host, controller.Port), BrokerCount: len(bs), TopicCount: len(topics), PartitionCount: len(ps), UnderReplicated: ur, OfflineReplicas: off, LatencyMS: time.Since(started).Milliseconds()}, nil
}
func Brokers(ctx context.Context, in ConnectionInput) (BrokersOutput, error) {
	c, _, e := open(ctx, in)
	if e != nil {
		return BrokersOutput{}, e
	}
	defer c.Close()
	bs, e := c.Brokers()
	if e != nil {
		return BrokersOutput{}, e
	}
	o := BrokersOutput{Brokers: []BrokerInfo{}}
	for _, b := range bs {
		o.Brokers = append(o.Brokers, BrokerInfo{ID: b.ID, Host: b.Host, Port: b.Port})
	}
	return o, nil
}
func Topics(ctx context.Context, in ConnectionInput) (TopicsOutput, error) {
	c, _, e := open(ctx, in)
	if e != nil {
		return TopicsOutput{}, e
	}
	defer c.Close()
	ps, e := c.ReadPartitions()
	if e != nil {
		return TopicsOutput{}, e
	}
	m := map[string]*TopicInfo{}
	for _, p := range ps {
		x := m[p.Topic]
		if x == nil {
			x = &TopicInfo{Name: p.Topic}
			m[p.Topic] = x
		}
		x.Partitions++
		if len(p.Replicas) > x.ReplicationFactor {
			x.ReplicationFactor = len(p.Replicas)
		}
		if len(p.Isr) < len(p.Replicas) {
			x.UnderReplicated++
		}
		x.OfflineReplicas += len(p.OfflineReplicas)
	}
	o := TopicsOutput{Topics: []TopicInfo{}}
	for _, x := range m {
		o.Topics = append(o.Topics, *x)
	}
	sort.Slice(o.Topics, func(i, j int) bool { return o.Topics[i].Name < o.Topics[j].Name })
	if len(o.Topics) > 1000 {
		o.Topics = o.Topics[:1000]
	}
	return o, nil
}
func ConsumerGroups(ctx context.Context, in ConnectionInput) (ConsumerGroupsOutput, error) {
	c, d, e := open(ctx, in)
	if e != nil {
		return ConsumerGroupsOutput{}, e
	}
	defer c.Close()
	cl := &kafka.Client{Addr: kafka.TCP(strings.TrimSpace(in.Brokers[0])), Timeout: d.Timeout, Transport: &kafka.Transport{TLS: d.TLS, SASL: d.SASLMechanism}}
	r, e := cl.ListGroups(ctx, &kafka.ListGroupsRequest{})
	if e != nil {
		return ConsumerGroupsOutput{}, e
	}
	o := ConsumerGroupsOutput{Groups: []ConsumerGroupInfo{}}
	ids := make([]string, 0, len(r.Groups))
	for _, g := range r.Groups {
		o.Groups = append(o.Groups, ConsumerGroupInfo{ID: g.GroupID, Protocol: g.ProtocolType})
		ids = append(ids, g.GroupID)
	}
	// ListGroups does not include member details. DescribeGroups is best-effort
	// because ACLs may allow discovery while denying group descriptions.
	if len(ids) > 0 {
		if details, e := cl.DescribeGroups(ctx, &kafka.DescribeGroupsRequest{GroupIDs: ids}); e == nil {
			members := make(map[string]int, len(details.Groups))
			for _, group := range details.Groups {
				if group.Error == nil {
					members[group.GroupID] = len(group.Members)
				}
			}
			for i := range o.Groups {
				o.Groups[i].Members = members[o.Groups[i].ID]
			}
		}
	}
	sort.Slice(o.Groups, func(i, j int) bool { return o.Groups[i].ID < o.Groups[j].ID })
	if len(o.Groups) > 1000 {
		o.Groups = o.Groups[:1000]
	}
	return o, nil
}
func ClusterInfo(ctx context.Context, in ConnectionInput) (ClusterInfoOutput, error) {
	c, _, e := open(ctx, in)
	if e != nil {
		return ClusterInfoOutput{}, e
	}
	defer c.Close()
	ctrl, e := c.Controller()
	if e != nil {
		return ClusterInfoOutput{}, e
	}
	bs, e := c.Brokers()
	if e != nil {
		return ClusterInfoOutput{}, e
	}
	o := ClusterInfoOutput{Controller: BrokerInfo{ID: ctrl.ID, Host: ctrl.Host, Port: ctrl.Port}}
	for _, b := range bs {
		o.Brokers = append(o.Brokers, BrokerInfo{ID: b.ID, Host: b.Host, Port: b.Port})
	}
	return o, nil
}
func TopicPartitions(ctx context.Context, in ConnectionInput, arg TopicPartitionsInput) (TopicPartitionsOutput, error) {
	if strings.TrimSpace(arg.Topic) == "" {
		return TopicPartitionsOutput{}, errors.New("topic is required")
	}
	c, _, e := open(ctx, in)
	if e != nil {
		return TopicPartitionsOutput{}, e
	}
	defer c.Close()
	ps, e := c.ReadPartitions(arg.Topic)
	if e != nil {
		return TopicPartitionsOutput{}, e
	}
	o := TopicPartitionsOutput{Topic: arg.Topic, Partitions: []map[string]any{}}
	for _, p := range ps {
		o.Partitions = append(o.Partitions, map[string]any{"partition": p.ID, "leader": fmt.Sprintf("%s:%d", p.Leader.Host, p.Leader.Port), "replicas": len(p.Replicas), "isr": len(p.Isr), "offline_replicas": len(p.OfflineReplicas)})
	}
	return o, nil
}
func ConsumerLag(ctx context.Context, in ConnectionInput, arg ConsumerLagInput) (ConsumerLagOutput, error) {
	limit := arg.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	c, d, err := open(ctx, in)
	if err != nil {
		return ConsumerLagOutput{}, err
	}
	defer c.Close()
	partitions, err := c.ReadPartitions()
	if err != nil {
		return ConsumerLagOutput{}, err
	}
	leaders := make(map[string]kafka.Partition, len(partitions))
	for _, p := range partitions {
		leaders[p.Topic+":"+fmt.Sprint(p.ID)] = p
	}
	client := &kafka.Client{Addr: kafka.TCP(strings.TrimSpace(in.Brokers[0])), Timeout: d.Timeout, Transport: &kafka.Transport{TLS: d.TLS, SASL: d.SASLMechanism}}
	groups, err := client.ListGroups(ctx, &kafka.ListGroupsRequest{})
	if err != nil {
		return ConsumerLagOutput{}, err
	}
	out := ConsumerLagOutput{Entries: []ConsumerLagInfo{}}
	for _, group := range groups.Groups {
		if len(out.Entries) >= limit {
			break
		}
		offsets, e := client.OffsetFetch(ctx, &kafka.OffsetFetchRequest{GroupID: group.GroupID})
		if e != nil || offsets.Error != nil {
			continue
		}
		for topic, values := range offsets.Topics {
			for _, offset := range values {
				if len(out.Entries) >= limit || offset.Error != nil || offset.CommittedOffset < 0 {
					continue
				}
				p, ok := leaders[topic+":"+fmt.Sprint(offset.Partition)]
				if !ok {
					continue
				}
				leader := fmt.Sprintf("%s:%d", p.Leader.Host, p.Leader.Port)
				conn, e := d.DialLeader(ctx, "tcp", leader, topic, offset.Partition)
				if e != nil {
					continue
				}
				end, e := conn.ReadLastOffset()
				_ = conn.Close()
				if e != nil {
					continue
				}
				lag := end - offset.CommittedOffset
				if lag < 0 {
					lag = 0
				}
				out.Entries = append(out.Entries, ConsumerLagInfo{Group: group.GroupID, Topic: topic, Partition: offset.Partition, CurrentOffset: offset.CommittedOffset, EndOffset: end, Lag: lag})
				out.TotalLag += lag
			}
		}
	}
	sort.Slice(out.Entries, func(i, j int) bool {
		if out.Entries[i].Group != out.Entries[j].Group {
			return out.Entries[i].Group < out.Entries[j].Group
		}
		if out.Entries[i].Topic != out.Entries[j].Topic {
			return out.Entries[i].Topic < out.Entries[j].Topic
		}
		return out.Entries[i].Partition < out.Entries[j].Partition
	})
	return out, nil
}

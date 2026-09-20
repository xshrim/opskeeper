package connector

import (
	"context"
	"fmt"
	kt "opskeeper/backend/tool/kafka"
	"time"
)

type KafkaDraftInput struct {
	Brokers        []string `json:"brokers"`
	Username       string   `json:"username,omitempty"`
	Password       string   `json:"password,omitempty"`
	TLS            bool     `json:"tls,omitempty"`
	TLSServerName  string   `json:"tls_server_name,omitempty"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
}
type KafkaDraftCheck struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latency_ms"`
}

func (s *Service) TestKafkaDraft(ctx context.Context, input KafkaDraftInput) (KafkaDraftCheck, error) {
	started := time.Now()
	out, e := kt.Health(ctx, kt.ConnectionInput{Brokers: input.Brokers, Username: input.Username, Password: input.Password, TLS: input.TLS, TLSServerName: input.TLSServerName, TimeoutSeconds: input.TimeoutSeconds})
	c := KafkaDraftCheck{Status: "failed", LatencyMS: time.Since(started).Milliseconds()}
	if e != nil {
		c.Message = fmt.Sprintf("Kafka 连接测试失败：%s", e)
		return c, nil
	}
	c.Status = "succeeded"
	c.Message = fmt.Sprintf("Kafka 连接测试通过（%d brokers，%d topics）", out.BrokerCount, out.TopicCount)
	return c, nil
}

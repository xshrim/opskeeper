package connector

import (
	"context"
	"fmt"
	rmq "opskeeper/backend/tool/rabbitmq"
	"time"
)

type RabbitMQDraftInput struct{ rmq.ConnectionInput }
type RabbitMQDraftCheck struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latency_ms"`
}

func (s *Service) TestRabbitMQDraft(ctx context.Context, in RabbitMQDraftInput) (RabbitMQDraftCheck, error) {
	start := time.Now()
	o, e := rmq.Health(ctx, in.ConnectionInput)
	r := RabbitMQDraftCheck{Status: "failed", LatencyMS: time.Since(start).Milliseconds()}
	if e != nil {
		r.Message = fmt.Sprintf("RabbitMQ 连接测试失败：%s", e)
		return r, nil
	}
	r.Status = "succeeded"
	r.Message = fmt.Sprintf("RabbitMQ 连接测试通过（%v）", o["status"])
	return r, nil
}

package connector

import (
	"context"
	"errors"
	rmq "opskeeper/backend/tool/rabbitmq"
	"strings"
)

type rabbitMQAdapter struct{ connection rmq.ConnectionInput }

func newRabbitMQAdapter(target Target, _ Limits) (Adapter, error) {
	in := rmq.ConnectionInput{URL: configString(target.Resource.Config, "url"), TimeoutSeconds: configInt(target.Resource.Config, "timeout_seconds"), TLSInsecure: configBool(target.Resource.Config, "tls_insecure")}
	s := secretFields(target.Secret)
	in.Username, in.Password = strings.TrimSpace(s["username"]), s["password"]
	if in.URL == "" {
		return nil, connectorError(CategoryConfiguration, "configure RabbitMQ", false, errors.New("url is required"))
	}
	return &rabbitMQAdapter{connection: in}, nil
}
func (a *rabbitMQAdapter) Kind() string               { return "RabbitMQ" }
func (a *rabbitMQAdapter) Capabilities() []Capability { return []Capability{CapabilityRabbitMQInspect} }
func (a *rabbitMQAdapter) Test(ctx context.Context) error {
	_, e := rmq.Health(ctx, a.connection)
	return rabbitMQError("test RabbitMQ connection", e)
}
func rabbitMQError(op string, e error) error {
	if e == nil {
		return nil
	}
	if errors.Is(e, context.DeadlineExceeded) {
		return connectorError(CategoryTimeout, op, true, e)
	}
	low := strings.ToLower(e.Error())
	if strings.Contains(low, "401") || strings.Contains(low, "403") || strings.Contains(low, "unauthorized") {
		return connectorError(CategoryAuthentication, op, false, e)
	}
	return connectorError(CategoryUpstream, op, true, e)
}

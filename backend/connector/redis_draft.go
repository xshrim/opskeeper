package connector

import (
	"context"
	"fmt"
	rt "opskeeper/backend/tool/redis"
	"time"
)

type RedisDraftInput struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Database       int    `json:"database"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}
type RedisDraftCheck struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latency_ms"`
}

func (s *Service) TestRedisDraft(ctx context.Context, in RedisDraftInput) (RedisDraftCheck, error) {
	started := time.Now()
	out := RedisDraftCheck{Status: "failed"}
	h, e := rt.Health(ctx, rt.ConnectionInput{Host: in.Host, Port: in.Port, Database: in.Database, Username: in.Username, Password: in.Password, TimeoutSeconds: in.TimeoutSeconds})
	out.LatencyMS = time.Since(started).Milliseconds()
	if e != nil {
		out.Message = fmt.Sprintf("Redis 连接测试失败：%s", e)
		return out, nil
	}
	out.Status = "succeeded"
	out.Message = fmt.Sprintf("Redis 连接测试通过（%s）", h.Version)
	return out, nil
}

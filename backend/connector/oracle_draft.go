package connector

import (
	"context"
	"fmt"
	ot "opskeeper/backend/tool/oracle"
	"time"
)

type OracleDraftInput struct{ ot.ConnectionInput }
type OracleDraftCheck struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latency_ms"`
}

func (s *Service) TestOracleDraft(ctx context.Context, in OracleDraftInput) (OracleDraftCheck, error) {
	started := time.Now()
	out := OracleDraftCheck{Status: "failed"}
	health, e := ot.Health(ctx, in.ConnectionInput)
	out.LatencyMS = time.Since(started).Milliseconds()
	if e != nil {
		out.Message = fmt.Sprintf("Oracle 连接测试失败：%s", e)
		return out, nil
	}
	out.Status = "succeeded"
	out.Message = fmt.Sprintf("Oracle 连接测试通过（%s）", health.Version)
	return out, nil
}

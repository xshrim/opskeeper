package connector

import (
	"context"
	"fmt"
	"time"

	ot "opskeeper/backend/tool/oceanbase"
)

type OceanBaseDraftInput struct{ ot.ConnectionInput }
type OceanBaseDraftCheck struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latency_ms"`
}

func (s *Service) TestOceanBaseDraft(ctx context.Context, in OceanBaseDraftInput) (OceanBaseDraftCheck, error) {
	started := time.Now()
	out := OceanBaseDraftCheck{Status: "failed"}
	health, err := ot.Health(ctx, in.ConnectionInput)
	out.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		out.Message = fmt.Sprintf("OceanBase 连接测试失败：%s", err)
		return out, nil
	}
	out.Status = "succeeded"
	out.Message = fmt.Sprintf("OceanBase %s 模式连接测试通过（%s）", health.Mode, health.Route)
	return out, nil
}

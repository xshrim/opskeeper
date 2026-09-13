package connector

import (
	"context"
	"fmt"
	"time"

	mt "opskeeper/backend/tool/minio"
)

type MinIODraftInput struct{ mt.ConnectionInput }
type MinIODraftCheck struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latency_ms"`
}

func (s *Service) TestMinIODraft(ctx context.Context, in MinIODraftInput) (MinIODraftCheck, error) {
	started := time.Now()
	out := MinIODraftCheck{Status: "failed", LatencyMS: 0}
	_, err := mt.Health(ctx, in.ConnectionInput)
	out.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		out.Message = fmt.Sprintf("MinIO 连接测试失败：%s", err)
		return out, nil
	}
	out.Status = "succeeded"
	out.Message = "MinIO 连接测试通过"
	return out, nil
}

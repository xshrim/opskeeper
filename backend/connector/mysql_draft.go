package connector

import (
	"context"
	"fmt"
	mt "opskeeper/backend/tool/mysql"
	"time"
)

type MySQLDraftInput struct{ mt.ConnectionInput }
type MySQLDraftCheck struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latency_ms"`
}

func (s *Service) TestMySQLDraft(ctx context.Context, in MySQLDraftInput) (MySQLDraftCheck, error) {
	started := time.Now()
	out := MySQLDraftCheck{Status: "failed"}
	_, err := mt.Health(ctx, in.ConnectionInput)
	out.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		out.Message = fmt.Sprintf("MySQL 连接测试失败：%s", err)
		return out, nil
	}
	out.Status = "succeeded"
	out.Message = "MySQL 连接测试通过"
	return out, nil
}

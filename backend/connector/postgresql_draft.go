package connector

import (
	"context"
	"fmt"
	"time"

	pt "opskeeper/backend/tool/postgresql"
)

type PostgreSQLDraftInput struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Database       string `json:"database"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}
type PostgreSQLDraftCheck struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latency_ms"`
}

func (s *Service) TestPostgreSQLDraft(ctx context.Context, input PostgreSQLDraftInput) (PostgreSQLDraftCheck, error) {
	started := time.Now()
	check := PostgreSQLDraftCheck{Status: "failed"}
	out, err := pt.Health(ctx, pt.ConnectionInput{Host: input.Host, Port: input.Port, Database: input.Database, Username: input.Username, Password: input.Password, TimeoutSeconds: input.TimeoutSeconds})
	check.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		check.Message = fmt.Sprintf("PostgreSQL 连接测试失败：%s", err)
		return check, nil
	}
	check.Status = "succeeded"
	check.Message = fmt.Sprintf("PostgreSQL 连接测试通过（%s）", out.ServerVersion)
	return check, nil
}

package connector

import (
	"context"
	"fmt"
	nt "opskeeper/backend/tool/nacos"
	"time"
)

type NacosDraftInput struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Scheme         string `json:"scheme"`
	ContextPath    string `json:"context_path"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	AccessToken    string `json:"access_token"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}
type NacosDraftCheck struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latency_ms"`
}

func (s *Service) TestNacosDraft(ctx context.Context, in NacosDraftInput) (NacosDraftCheck, error) {
	started := time.Now()
	out := NacosDraftCheck{Status: "failed"}
	v, e := nt.Health(ctx, nt.ConnectionInput{Host: in.Host, Port: in.Port, Scheme: in.Scheme, ContextPath: in.ContextPath, Username: in.Username, Password: in.Password, AccessToken: in.AccessToken, TimeoutSeconds: in.TimeoutSeconds})
	out.LatencyMS = time.Since(started).Milliseconds()
	if e != nil {
		out.Message = fmt.Sprintf("Nacos 连接测试失败：%s", e)
		return out, nil
	}
	out.Status = "succeeded"
	out.Message = fmt.Sprintf("Nacos 连接测试通过（%T）", v.Data)
	return out, nil
}

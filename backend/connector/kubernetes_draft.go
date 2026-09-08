package connector

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	kclient "opskeeper/backend/mcpserver/kubernetes/client"
	kt "opskeeper/backend/tool/kubernetes"
)

type KubernetesDraftInput struct {
	Kubeconfig    string `json:"kubeconfig,omitempty"`
	Server        string `json:"server,omitempty"`
	Token         string `json:"token,omitempty"`
	Context       string `json:"context,omitempty"`
	SkipTLSVerify bool   `json:"skip_tls_verify,omitempty"`
}
type KubernetesDraftCheck struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latency_ms"`
}

func (s *Service) TestKubernetesDraft(ctx context.Context, input KubernetesDraftInput) (KubernetesDraftCheck, error) {
	started := time.Now()
	check := KubernetesDraftCheck{Status: "failed"}
	connection := kclient.ConnectionInput{Server: input.Server, Token: input.Token, Context: input.Context, SkipTLSVerify: input.SkipTLSVerify}
	if input.Kubeconfig != "" {
		connection.KubeconfigBase64 = base64.StdEncoding.EncodeToString([]byte(input.Kubeconfig))
	}
	_, err := kt.ClusterInfo(ctx, connection)
	check.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		check.Message = fmt.Sprintf("Kubernetes 连接测试失败：%s", err)
		return check, nil
	}
	check.Status = "succeeded"
	check.Message = "Kubernetes 连接测试通过"
	return check, nil
}

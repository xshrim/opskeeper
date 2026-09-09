package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"opskeeper/backend/mcpserver/docker/client"
)

type DockerDraftInput struct {
	DockerHost       string `json:"host"`
	TimeoutSeconds   any    `json:"timeout"`
	DockerCA         string `json:"tls_ca"`
	DockerCert       string `json:"tls_cert"`
	DockerKey        string `json:"tls_key"`
	DockerSkipVerify bool   `json:"skip_tls_verify"`
}

type DockerDraftCheck struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latency_ms"`
}

func (s *Service) TestDockerDraft(ctx context.Context, input DockerDraftInput) (DockerDraftCheck, error) {
	timeout := int(client.TimeoutSeconds(input.TimeoutSeconds) / time.Second)
	ctx, cancel := context.WithTimeout(ctx, client.TimeoutSeconds(input.TimeoutSeconds))
	defer cancel()
	started := time.Now()
	check := DockerDraftCheck{Status: "failed"}
	err := client.PingDraft(ctx, client.ConnectionInput{
		DockerHost: input.DockerHost, DockerCA: input.DockerCA, DockerCert: input.DockerCert,
		DockerKey: input.DockerKey, DockerSkipVerify: input.DockerSkipVerify,
	})
	check.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			check.Message = fmt.Sprintf("Docker 连接测试超时（%d 秒）", timeout)
		} else {
			check.Message = err.Error()
		}
		return check, nil
	}
	check.Status = "succeeded"
	check.Message = "Docker 连接测试通过"
	return check, nil
}

type dockerAdapter struct {
	connection client.ConnectionInput
}

func newDockerAdapter(target Target) Adapter {
	connection := client.ConnectionInput{}
	setDockerConnectionFromMap(&connection, target.Resource.Config)
	var credentialValues map[string]any
	if json.Unmarshal(target.Secret, &credentialValues) == nil {
		setDockerConnectionFromMapIfEmpty(&connection, credentialValues)
	}
	return dockerAdapter{connection: connection}
}

func (dockerAdapter) Kind() string { return "Docker" }

func (dockerAdapter) Capabilities() []Capability {
	return []Capability{CapabilityDockerInfo, CapabilityDockerImages, CapabilityDockerContainers, CapabilityDockerLogs, CapabilityDockerInspect, CapabilityDockerStats}
}

func (adapter dockerAdapter) Test(ctx context.Context) error {
	return client.Ping(ctx, adapter.connection)
}

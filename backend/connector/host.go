package connector

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	host "opskeeper/backend/tool/host"
)

type hostAdapter struct{ input host.ConnectionInput }

type HostDraftInput = host.ConnectionInput

type HostDraftCheck struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latency_ms"`
}

func newHostAdapter(target Target) (Adapter, error) {
	input := host.ConnectionInput{}
	setHostInput(&input, target.Resource.Config)
	if len(target.Secret) > 0 {
		var values map[string]any
		if json.Unmarshal(target.Secret, &values) == nil {
			setHostInputIfEmpty(&input, values)
		}
	}
	return hostAdapter{input: input}, nil
}
func (hostAdapter) Kind() string { return "Host" }
func (hostAdapter) Capabilities() []Capability {
	return []Capability{CapabilityHostInfo, CapabilityHostMetrics, CapabilityHostProcesses, CapabilityHostFileLogs, CapabilityHostHealth}
}
func (adapter hostAdapter) Test(ctx context.Context) error {
	_, err := host.Info(ctx, host.InfoInput{ConnectionInput: adapter.input})
	return err
}

func (s *Service) TestHostDraft(ctx context.Context, input HostDraftInput) (HostDraftCheck, error) {
	started := time.Now()
	check := HostDraftCheck{Status: "failed"}
	_, err := host.Info(ctx, host.InfoInput{ConnectionInput: input})
	check.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		check.Message = err.Error()
		return check, nil
	}
	check.Status = "succeeded"
	check.Message = "Host 连接测试通过"
	return check, nil
}

func setHostInput(input *host.ConnectionInput, values map[string]any) {
	if input == nil {
		return
	}
	input.Host = stringValue(values, "host")
	input.Port = intValue(values, "port")
	input.Username = stringValue(values, "username")
	input.AuthMethod = strings.ToLower(stringValue(values, "auth_method"))
	input.Password = stringValue(values, "password")
	input.PrivateKey = stringValue(values, "private_key")
	input.Passphrase = stringValue(values, "passphrase")
	input.KnownHosts = stringValue(values, "known_hosts")
	input.TimeoutSeconds = intValue(values, "timeout_seconds")
}
func setHostInputIfEmpty(input *host.ConnectionInput, values map[string]any) {
	if input.Host == "" {
		input.Host = stringValue(values, "host")
	}
	if input.Port == 0 {
		input.Port = intValue(values, "port")
	}
	if input.Username == "" {
		input.Username = stringValue(values, "username")
	}
	if input.AuthMethod == "" {
		input.AuthMethod = strings.ToLower(stringValue(values, "auth_method"))
	}
	if input.Password == "" {
		input.Password = stringValue(values, "password")
	}
	if input.PrivateKey == "" {
		input.PrivateKey = stringValue(values, "private_key")
	}
	if input.Passphrase == "" {
		input.Passphrase = stringValue(values, "passphrase")
	}
	if input.KnownHosts == "" {
		input.KnownHosts = stringValue(values, "known_hosts")
	}
	if input.TimeoutSeconds == 0 {
		input.TimeoutSeconds = intValue(values, "timeout_seconds")
	}
}
func intValue(values map[string]any, key string) int {
	switch value := values[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		parsed, _ := value.Int64()
		return int(parsed)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(value))
		return parsed
	}
	return 0
}

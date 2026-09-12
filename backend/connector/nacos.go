package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	nt "opskeeper/backend/tool/nacos"
)

type nacosAdapter struct{ input nt.ConnectionInput }

func newNacosAdapter(target Target, limits Limits) (Adapter, error) {
	in := nt.ConnectionInput{Host: configString(target.Resource.Config, "host"), Port: configPort(target.Resource.Config, 8848), Scheme: configString(target.Resource.Config, "scheme"), ContextPath: configString(target.Resource.Config, "context_path"), TimeoutSeconds: intValue(target.Resource.Config, "timeout_seconds")}
	if in.Host == "" {
		return nil, connectorError(CategoryConfiguration, "configure Nacos", false, errors.New("host is required"))
	}
	if len(target.Secret) > 0 {
		var values map[string]any
		if json.Unmarshal(target.Secret, &values) == nil {
			in.Username = stringValue(values, "username")
			in.Password = stringValue(values, "password")
			in.AccessToken = stringValue(values, "access_token")
		}
	}
	if limits.Timeout > 0 && in.TimeoutSeconds == 0 {
		in.TimeoutSeconds = int(limits.Timeout.Seconds())
	}
	return &nacosAdapter{input: in}, nil
}
func (a *nacosAdapter) Kind() string               { return "Nacos" }
func (a *nacosAdapter) Capabilities() []Capability { return []Capability{CapabilityNacosHealth} }
func (a *nacosAdapter) Test(ctx context.Context) error {
	_, err := nt.Health(ctx, a.input)
	if err != nil {
		return connectorError(CategoryUpstream, "test Nacos connection", true, err)
	}
	return nil
}
func nacosError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "401") || strings.Contains(strings.ToLower(err.Error()), "403") {
		return connectorError(CategoryAuthentication, "Nacos API", false, err)
	}
	return fmt.Errorf("Nacos API: %w", err)
}

package connector

import (
	"context"
	"errors"
	"strings"

	ot "opskeeper/backend/tool/oceanbase"
)

type oceanBaseAdapter struct{ connection ot.ConnectionInput }

func newOceanBaseAdapter(target Target, _ Limits) (Adapter, error) {
	in := ot.ConnectionInput{Host: configString(target.Resource.Config, "host"), Port: configPort(target.Resource.Config, 2881), Mode: configString(target.Resource.Config, "mode"), Database: configString(target.Resource.Config, "database"), ServiceName: configString(target.Resource.Config, "service_name"), TimeoutSeconds: configInt(target.Resource.Config, "timeout_seconds")}
	secret := secretFields(target.Secret)
	in.Username, in.Password = strings.TrimSpace(secret["username"]), secret["password"]
	if in.Host == "" || in.Username == "" || in.Password == "" {
		return nil, connectorError(CategoryConfiguration, "configure OceanBase", false, errors.New("host and credential username/password are required"))
	}
	if strings.EqualFold(in.Mode, ot.ModeMySQL) && in.Database == "" {
		return nil, connectorError(CategoryConfiguration, "configure OceanBase", false, errors.New("database is required for MySQL mode"))
	}
	return &oceanBaseAdapter{connection: in}, nil
}
func (a *oceanBaseAdapter) Kind() string { return "OceanBase" }
func (a *oceanBaseAdapter) Capabilities() []Capability {
	return []Capability{CapabilityOceanBaseHealth}
}
func (a *oceanBaseAdapter) Test(ctx context.Context) error {
	_, err := ot.Health(ctx, a.connection)
	return oceanBaseError("test OceanBase connection", err)
}
func oceanBaseError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return connectorError(CategoryTimeout, operation, true, err)
	}
	low := strings.ToLower(err.Error())
	if strings.Contains(low, "access denied") || strings.Contains(low, "ora-01017") || strings.Contains(low, "authentication") {
		return connectorError(CategoryAuthentication, operation, false, err)
	}
	return connectorError(CategoryUpstream, operation, true, err)
}

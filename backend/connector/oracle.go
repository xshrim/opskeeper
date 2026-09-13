package connector

import (
	"context"
	"errors"
	"strings"

	ot "opskeeper/backend/tool/oracle"
)

type oracleAdapter struct{ connection ot.ConnectionInput }

func newOracleAdapter(target Target, _ Limits) (Adapter, error) {
	in := ot.ConnectionInput{Host: configString(target.Resource.Config, "host"), Port: configPort(target.Resource.Config, 1521), ServiceName: configString(target.Resource.Config, "service_name"), SID: configString(target.Resource.Config, "sid"), TimeoutSeconds: configInt(target.Resource.Config, "timeout_seconds"), TLS: configBool(target.Resource.Config, "tls")}
	secret := secretFields(target.Secret)
	in.Username, in.Password = strings.TrimSpace(secret["username"]), secret["password"]
	if in.Host == "" || in.Username == "" || in.Password == "" || (in.ServiceName == "" && in.SID == "") {
		return nil, connectorError(CategoryConfiguration, "configure Oracle", false, errors.New("host, service_name or sid, and credential username/password are required"))
	}
	if in.ServiceName != "" && in.SID != "" {
		return nil, connectorError(CategoryConfiguration, "configure Oracle", false, errors.New("service_name and sid are mutually exclusive"))
	}
	return &oracleAdapter{connection: in}, nil
}
func (a *oracleAdapter) Kind() string               { return "Oracle" }
func (a *oracleAdapter) Capabilities() []Capability { return []Capability{CapabilityOracleHealth} }
func (a *oracleAdapter) Test(ctx context.Context) error {
	_, e := ot.Health(ctx, a.connection)
	return oracleError("test Oracle connection", e)
}
func oracleError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return connectorError(CategoryTimeout, operation, true, err)
	}
	low := strings.ToLower(err.Error())
	if strings.Contains(low, "invalid username") || strings.Contains(low, "ora-01017") || strings.Contains(low, "authentication") {
		return connectorError(CategoryAuthentication, operation, false, err)
	}
	return connectorError(CategoryUpstream, operation, true, err)
}

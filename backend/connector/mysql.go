package connector

import (
	"context"
	"errors"
	mt "opskeeper/backend/tool/mysql"
	"strings"
)

type mysqlAdapter struct{ connection mt.ConnectionInput }

func newMySQLAdapter(target Target, _ Limits) (Adapter, error) {
	in := mt.ConnectionInput{Host: configString(target.Resource.Config, "host"), Database: configString(target.Resource.Config, "database"), Port: configPort(target.Resource.Config, 3306), TimeoutSeconds: configInt(target.Resource.Config, "timeout_seconds")}
	secret := secretFields(target.Secret)
	in.Username, in.Password = strings.TrimSpace(secret["username"]), secret["password"]
	if in.Host == "" || in.Database == "" || in.Username == "" || in.Password == "" {
		return nil, connectorError(CategoryConfiguration, "configure MySQL", false, errors.New("host, database and credential username/password are required"))
	}
	return &mysqlAdapter{connection: in}, nil
}
func (a *mysqlAdapter) Kind() string               { return "MySQL" }
func (a *mysqlAdapter) Capabilities() []Capability { return []Capability{CapabilityMySQLHealth} }
func (a *mysqlAdapter) Test(ctx context.Context) error {
	_, err := mt.Health(ctx, a.connection)
	return mysqlError("test MySQL connection", err)
}
func mysqlError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return connectorError(CategoryTimeout, operation, true, err)
	}
	low := strings.ToLower(err.Error())
	if strings.Contains(low, "access denied") || strings.Contains(low, "authentication") {
		return connectorError(CategoryAuthentication, operation, false, err)
	}
	return connectorError(CategoryUpstream, operation, true, err)
}

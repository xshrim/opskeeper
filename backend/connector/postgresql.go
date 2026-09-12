package connector

import (
	"context"
	"errors"
	"fmt"
	"strings"

	pt "opskeeper/backend/tool/postgresql"
)

type postgreSQLAdapter struct{ connection pt.ConnectionInput }

func newPostgreSQLAdapter(target Target, _ Limits) (Adapter, error) {
	input := pt.ConnectionInput{Host: configString(target.Resource.Config, "host"), Database: configString(target.Resource.Config, "database"), Port: configPort(target.Resource.Config, 5432), TimeoutSeconds: configInt(target.Resource.Config, "timeout_seconds")}
	secret := secretFields(target.Secret)
	input.Username, input.Password = strings.TrimSpace(secret["username"]), secret["password"]
	if input.Host == "" || input.Database == "" || input.Username == "" || input.Password == "" {
		return nil, connectorError(CategoryConfiguration, "configure PostgreSQL", false, errors.New("host, database and credential username/password are required"))
	}
	return &postgreSQLAdapter{connection: input}, nil
}

func (a *postgreSQLAdapter) Kind() string { return "PostgreSQL" }
func (a *postgreSQLAdapter) Capabilities() []Capability {
	return []Capability{CapabilityPostgreSQLHealth}
}
func (a *postgreSQLAdapter) Test(ctx context.Context) error {
	_, err := pt.Health(ctx, a.connection)
	return postgreSQLError("test PostgreSQL connection", err)
}

func configInt(config map[string]any, key string) int {
	switch value := config[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case string:
		var valueInt int
		_, _ = fmt.Sscan(value, &valueInt)
		return valueInt
	default:
		return 0
	}
}

func configPort(config map[string]any, fallback int) int {
	value := configInt(config, "port")
	if value > 0 && value <= 65535 {
		return value
	}
	return fallback
}

func postgreSQLError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return connectorError(CategoryTimeout, operation, true, err)
	}
	return connectorError(CategoryUpstream, operation, true, err)
}

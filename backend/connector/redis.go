package connector

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

type redisAdapter struct{ client *redis.Client }

func newRedisAdapter(target Target, _ Limits) (Adapter, error) {
	host := configString(target.Resource.Config, "host")
	if host == "" {
		return nil, connectorError(CategoryConfiguration, "configure Redis", false, errors.New("host is required"))
	}
	secret := secretFields(target.Secret)
	database := 0
	if value, ok := target.Resource.Config["database"].(float64); ok && value >= 0 {
		database = int(value)
	}
	client := redis.NewClient(&redis.Options{Addr: net.JoinHostPort(host, strconv.Itoa(configPort(target.Resource.Config, 6379))), DB: database, Username: secret["username"], Password: secret["password"]})
	return &redisAdapter{client: client}, nil
}

func (a *redisAdapter) Kind() string               { return "Redis" }
func (a *redisAdapter) Capabilities() []Capability { return []Capability{CapabilityRedisInspect} }
func (a *redisAdapter) Test(ctx context.Context) error {
	defer a.client.Close()
	return redisError("ping Redis", a.client.Ping(ctx).Err())
}
func (a *redisAdapter) InspectRedis(ctx context.Context) (DiagnosticSnapshot, error) {
	defer a.client.Close()
	if err := a.client.Ping(ctx).Err(); err != nil {
		return DiagnosticSnapshot{}, redisError("ping Redis", err)
	}
	// Redis has no bounded, read-only hot-key API. Sampling keys or using
	// redis-cli --hotkeys can scan the whole keyspace and harms production
	// instances, so surface that capability explicitly as unavailable.
	snapshot := DiagnosticSnapshot{Kind: "Redis", Facts: map[string]any{}, Findings: []Finding{}, Capabilities: []string{"connection", "memory", "clients", "replication", "slowlog"}, Unavailable: []string{"hot_keys"}}
	info, err := a.client.Info(ctx, "memory", "clients", "replication").Result()
	if err != nil {
		return DiagnosticSnapshot{}, redisError("read Redis info", err)
	}
	for key, value := range parseRedisInfo(info) {
		snapshot.Facts[key] = value
	}
	if slow, err := a.client.SlowLogGet(ctx, 20).Result(); err != nil {
		snapshot.Unavailable = append(snapshot.Unavailable, "slowlog")
	} else {
		snapshot.Facts["slowlog_entries"] = len(slow)
		if len(slow) > 0 {
			snapshot.Findings = append(snapshot.Findings, Finding{Code: "redis.slowlog", Severity: "warning", Message: "存在近期慢命令记录"})
		}
	}
	if rejected, ok := snapshot.Facts["rejected_connections"].(int64); ok && rejected > 0 {
		snapshot.Findings = append(snapshot.Findings, Finding{Code: "redis.rejected_connections", Severity: "warning", Message: "Redis 曾拒绝客户端连接"})
	}
	return snapshot, nil
}

func parseRedisInfo(raw string) map[string]any {
	result := map[string]any{}
	for _, line := range strings.Split(raw, "\n") {
		pair := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(pair) != 2 || strings.HasPrefix(pair[0], "#") {
			continue
		}
		if value, err := strconv.ParseInt(pair[1], 10, 64); err == nil {
			result[pair[0]] = value
		} else {
			result[pair[0]] = pair[1]
		}
	}
	return result
}
func redisError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return connectorError(CategoryTimeout, operation, true, err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "noauth") || strings.Contains(strings.ToLower(err.Error()), "noperm") {
		return connectorError(CategoryAuthentication, operation, false, err)
	}
	return connectorError(CategoryUpstream, operation, true, err)
}

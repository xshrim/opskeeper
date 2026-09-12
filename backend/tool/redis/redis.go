package redis

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	client "github.com/redis/go-redis/v9"
)

type ConnectionInput struct {
	Host           string `json:"host,omitempty"`
	Port           int    `json:"port,omitempty"`
	Database       int    `json:"database,omitempty"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

type HealthOutput struct {
	Version   string `json:"version"`
	Mode      string `json:"mode"`
	Role      string `json:"role"`
	LatencyMS int64  `json:"latency_ms"`
}
type ValuesOutput struct {
	Values map[string]any `json:"values"`
}
type SlowlogEntry struct {
	ID         int64  `json:"id"`
	Timestamp  int64  `json:"timestamp"`
	DurationUS int64  `json:"duration_us"`
	Command    string `json:"command"`
}
type SlowlogOutput struct {
	Entries []SlowlogEntry `json:"entries"`
}
type DatabaseInfoOutput struct {
	Database int            `json:"database"`
	Keyspace map[string]any `json:"keyspace"`
	Keys     int64          `json:"keys"`
}
type ToolInfo struct{ Name, Description string }

var tools = []ToolInfo{
	{Name: "redis_health", Description: "Check Redis connectivity and server health."},
	{Name: "redis_memory", Description: "Read Redis memory statistics."},
	{Name: "redis_clients", Description: "Read Redis client statistics."},
	{Name: "redis_replication", Description: "Read Redis replication status."},
	{Name: "redis_slowlog", Description: "Read the latest bounded Redis slowlog entries."},
	{Name: "redis_database_info", Description: "Read Redis keyspace and selected database information."},
}

func ListTools() []ToolInfo { return append([]ToolInfo(nil), tools...) }
func InputSchema(extra map[string]any) map[string]any {
	p := map[string]any{"host": map[string]any{"type": "string"}, "port": map[string]any{"type": "integer", "minimum": 1, "maximum": 65535}, "database": map[string]any{"type": "integer", "minimum": 0}, "username": map[string]any{"type": "string"}, "password": map[string]any{"type": "string"}, "timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": 300}}
	for k, v := range extra {
		p[k] = v
	}
	return map[string]any{"type": "object", "properties": p, "additionalProperties": false}
}

func open(ctx context.Context, in ConnectionInput) (*client.Client, error) {
	host := strings.TrimSpace(in.Host)
	if host == "" {
		return nil, errors.New("host is required")
	}
	port := in.Port
	if port <= 0 {
		port = 6379
	}
	if port > 65535 {
		return nil, errors.New("port must be between 1 and 65535")
	}
	t := time.Duration(in.TimeoutSeconds) * time.Second
	o := &client.Options{Addr: net.JoinHostPort(host, strconv.Itoa(port)), DB: in.Database, Username: in.Username, Password: in.Password}
	if t > 0 {
		o.DialTimeout = t
		o.ReadTimeout = t
		o.WriteTimeout = t
	}
	c := client.NewClient(o)
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}
func withClient[T any](ctx context.Context, in ConnectionInput, fn func(context.Context, *client.Client) (T, error)) (T, error) {
	var zero T
	c, err := open(ctx, in)
	if err != nil {
		return zero, err
	}
	defer c.Close()
	return fn(ctx, c)
}

func parseInfo(raw string) map[string]any {
	out := map[string]any{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		kv := strings.SplitN(line, ":", 2)
		if len(kv) != 2 {
			continue
		}
		k, v := kv[0], kv[1]
		if n, e := strconv.ParseInt(v, 10, 64); e == nil {
			out[k] = n
		} else {
			out[k] = v
		}
	}
	return out
}
func info(ctx context.Context, c *client.Client, section string) (map[string]any, error) {
	s, e := c.Info(ctx, section).Result()
	if e != nil {
		return nil, e
	}
	return parseInfo(s), nil
}
func Health(ctx context.Context, in ConnectionInput) (HealthOutput, error) {
	started := time.Now()
	return withClient(ctx, in, func(ctx context.Context, c *client.Client) (HealthOutput, error) {
		v, e := info(ctx, c, "server")
		if e != nil {
			return HealthOutput{}, e
		}
		o := HealthOutput{LatencyMS: time.Since(started).Milliseconds()}
		if x, ok := v["redis_version"].(string); ok {
			o.Version = x
		}
		if x, ok := v["redis_mode"].(string); ok {
			o.Mode = x
		}
		if x, ok := v["role"].(string); ok {
			o.Role = x
		}
		return o, nil
	})
}
func section(ctx context.Context, in ConnectionInput, name string) (ValuesOutput, error) {
	return withClient(ctx, in, func(ctx context.Context, c *client.Client) (ValuesOutput, error) {
		v, e := info(ctx, c, name)
		return ValuesOutput{Values: v}, e
	})
}
func Memory(ctx context.Context, in ConnectionInput) (ValuesOutput, error) {
	return section(ctx, in, "memory")
}
func Clients(ctx context.Context, in ConnectionInput) (ValuesOutput, error) {
	return section(ctx, in, "clients")
}
func Replication(ctx context.Context, in ConnectionInput) (ValuesOutput, error) {
	return section(ctx, in, "replication")
}
func Slowlog(ctx context.Context, in ConnectionInput) (SlowlogOutput, error) {
	return withClient(ctx, in, func(ctx context.Context, c *client.Client) (SlowlogOutput, error) {
		entries, e := c.SlowLogGet(ctx, 20).Result()
		if e != nil {
			return SlowlogOutput{}, e
		}
		out := SlowlogOutput{Entries: []SlowlogEntry{}}
		for _, x := range entries {
			cmd := ""
			if len(x.Args) > 0 {
				cmd = x.Args[0]
				if len(x.Args) > 1 {
					cmd += " (" + strconv.Itoa(len(x.Args)-1) + " args)"
				}
			}
			out.Entries = append(out.Entries, SlowlogEntry{ID: x.ID, Timestamp: x.Time.Unix(), DurationUS: int64(x.Duration), Command: cmd})
		}
		return out, nil
	})
}
func DatabaseInfo(ctx context.Context, in ConnectionInput) (DatabaseInfoOutput, error) {
	return withClient(ctx, in, func(ctx context.Context, c *client.Client) (DatabaseInfoOutput, error) {
		v, e := info(ctx, c, "keyspace")
		if e != nil {
			return DatabaseInfoOutput{}, e
		}
		keys := int64(0)
		if n, e := c.DBSize(ctx).Result(); e == nil {
			keys = n
		}
		return DatabaseInfoOutput{Database: in.Database, Keyspace: v, Keys: keys}, nil
	})
}

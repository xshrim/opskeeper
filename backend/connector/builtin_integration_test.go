//go:build integration

package connector

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5"
	"opskeeper/backend/resource"
)

func TestBuiltinPostgreSQLAndRedisSnapshots(t *testing.T) {
	ctx := context.Background()
	postgresURL := os.Getenv("OPSK_DATABASE_URL")
	redisURL := os.Getenv("OPSK_REDIS_URL")
	if postgresURL == "" || redisURL == "" {
		t.Skip("OPSK_DATABASE_URL and OPSK_REDIS_URL are required")
	}
	pgConfig, err := pgx.ParseConfig(postgresURL)
	if err != nil {
		t.Fatalf("parse PostgreSQL URL: %v", err)
	}
	pgSecret, _ := json.Marshal(map[string]string{"username": pgConfig.User, "password": pgConfig.Password})
	postgres, err := newPostgreSQLAdapter(Target{Resource: resource.Resource{Kind: "PostgreSQL", Config: map[string]any{"host": pgConfig.Host, "port": float64(pgConfig.Port), "database": pgConfig.Database}}, Secret: pgSecret}, DefaultLimits())
	if err != nil {
		t.Fatalf("new PostgreSQL adapter: %v", err)
	}
	pgSnapshot, err := postgres.(PostgreSQLInspector).InspectPostgreSQL(ctx)
	if err != nil || pgSnapshot.Kind != "PostgreSQL" || pgSnapshot.Facts["server_version"] == "" {
		t.Fatalf("PostgreSQL snapshot = %#v, %v", pgSnapshot, err)
	}

	redisConfig, err := url.Parse(redisURL)
	if err != nil {
		t.Fatalf("parse Redis URL: %v", err)
	}
	port, _ := strconv.Atoi(redisConfig.Port())
	redisSecret, _ := json.Marshal(map[string]string{"username": redisConfig.User.Username(), "password": password(redisConfig)})
	redis, err := newRedisAdapter(Target{Resource: resource.Resource{Kind: "Redis", Config: map[string]any{"host": redisConfig.Hostname(), "port": float64(port)}}, Secret: redisSecret}, DefaultLimits())
	if err != nil {
		t.Fatalf("new Redis adapter: %v", err)
	}
	redisSnapshot, err := redis.(RedisInspector).InspectRedis(ctx)
	if err != nil || redisSnapshot.Kind != "Redis" {
		t.Fatalf("Redis snapshot = %#v, %v", redisSnapshot, err)
	}
}

func password(value *url.URL) string { result, _ := value.User.Password(); return result }

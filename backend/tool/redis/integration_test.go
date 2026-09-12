//go:build integration

package redis

import (
	"context"
	"net/url"
	"os"
	"strconv"
	"testing"
)

func TestRealRedis(t *testing.T) {
	raw := os.Getenv("OPSK_REDIS_URL")
	if raw == "" {
		raw = "redis://127.0.0.1:6383/0"
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	in := ConnectionInput{Host: u.Hostname(), Port: 6379, Database: 0}
	if p := u.Port(); p != "" {
		in.Port, _ = strconv.Atoi(p)
	}
	if u.User != nil {
		in.Username = u.User.Username()
		in.Password, _ = u.User.Password()
	}
	ctx := context.Background()
	if _, err := Health(ctx, in); err != nil {
		t.Fatal(err)
	}
	if _, err := Memory(ctx, in); err != nil {
		t.Fatal(err)
	}
	if _, err := Clients(ctx, in); err != nil {
		t.Fatal(err)
	}
	if _, err := Replication(ctx, in); err != nil {
		t.Fatal(err)
	}
	if _, err := Slowlog(ctx, in); err != nil {
		t.Fatal(err)
	}
	if _, err := DatabaseInfo(ctx, in); err != nil {
		t.Fatal(err)
	}
}

//go:build integration

package postgresql

import (
	"context"
	"net/url"
	"os"
	"strconv"
	"testing"
)

func TestRealPostgreSQLFixedToolCatalog(t *testing.T) {
	raw := os.Getenv("OPSK_DATABASE_URL")
	if raw == "" {
		t.Skip("OPSK_DATABASE_URL is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse PostgreSQL URL: %v", err)
	}
	port, _ := strconv.Atoi(parsed.Port())
	password, _ := parsed.User.Password()
	input := ConnectionInput{Host: parsed.Hostname(), Port: port, Database: parsed.Path[1:], Username: parsed.User.Username(), Password: password, TimeoutSeconds: 10}
	if out, err := Health(context.Background(), input); err != nil || out.ServerVersion == "" {
		t.Fatalf("Health() = %#v, %v", out, err)
	}
	if _, err := Sessions(context.Background(), input); err != nil {
		t.Fatalf("Sessions(): %v", err)
	}
	if _, err := LongRunningQueries(context.Background(), input); err != nil {
		t.Fatalf("LongRunningQueries(): %v", err)
	}
	if _, err := Locks(context.Background(), input); err != nil {
		t.Fatalf("Locks(): %v", err)
	}
	if _, err := Replication(context.Background(), input); err != nil {
		t.Fatalf("Replication(): %v", err)
	}
	if out, err := Capacity(context.Background(), input); err != nil || out.DatabaseSizeBytes < 0 {
		t.Fatalf("Capacity() = %#v, %v", out, err)
	}
	tables, err := Tables(context.Background(), input)
	if err != nil || len(tables.Tables) == 0 {
		t.Fatalf("Tables() = %#v, %v", tables, err)
	}
	if _, err := TableColumns(context.Background(), input, TableColumnsInput{Schema: tables.Tables[0].Schema, Table: tables.Tables[0].Name}); err != nil {
		t.Fatalf("TableColumns(): %v", err)
	}
	if out, err := Performance(context.Background(), input); err != nil || len(out.Values) == 0 {
		t.Fatalf("Performance() = %#v, %v", out, err)
	}
	if out, err := Vacuum(context.Background(), input); err != nil || len(out.Values) == 0 {
		t.Fatalf("Vacuum() = %#v, %v", out, err)
	}
	if _, err := Extensions(context.Background(), input); err != nil {
		t.Fatalf("Extensions(): %v", err)
	}
	if out, err := DatabaseInfo(context.Background(), input); err != nil || out.Values["database"] == "" {
		t.Fatalf("DatabaseInfo() = %#v, %v", out, err)
	}
}

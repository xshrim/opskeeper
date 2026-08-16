//go:build integration

package operation

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestOperationTablesExist verifies the applied migration in the configured
// integration database. State-machine edge cases are unit-tested without
// sharing mutable rows with developer data.
func TestOperationTablesExist(t *testing.T) {
	url := os.Getenv("OPSK_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("OPSK_TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var count int
	err = pool.QueryRow(context.Background(), `SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name = ANY(ARRAY['mcp_server_snapshots','operation_policies','operation_requests','operation_approvals','operation_executions'])`).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Fatalf("T14 table count = %d, want 5", count)
	}
}

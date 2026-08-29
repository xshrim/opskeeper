//go:build integration

package aiengine

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresWorkflowRunStorePersistsTransitionsAndSnapshots(t *testing.T) {
	databaseURL := os.Getenv("OPSK_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("OPSK_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var scopeID, userID, workflowID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM scopes WHERE status='active' AND deleted_at IS NULL ORDER BY created_at LIMIT 1`).Scan(&scopeID); err != nil {
		t.Skipf("no active scope: %v", err)
	}
	_ = pool.QueryRow(ctx, `SELECT id::text FROM users WHERE status='active' AND deleted_at IS NULL ORDER BY created_at LIMIT 1`).Scan(&userID)
	config := `{"version":1,"enabled":true,"nodes":[{"id":"retrieve","type":"retrieval","name":"retrieve"}],"edges":[]}`
	if err := pool.QueryRow(ctx, `INSERT INTO resources(scope_id,kind,schema_version,name,config,status) VALUES($1::uuid,'Workflow',1,$2,$3::jsonb,'active') RETURNING id::text`, scopeID, "integration-workflow-"+time.Now().Format("150405.000000"), config).Scan(&workflowID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM resources WHERE id=$1::uuid`, workflowID) })
	store := NewPostgresWorkflowRunStore(pool)
	run, err := store.CreateWorkflowRun(ctx, WorkflowRunInput{WorkflowID: workflowID, WorkflowVersion: 1, ExecutionID: "integration-workflow-exec-" + time.Now().Format("150405.000000"), ScopeID: scopeID, CreatedBy: userID, Input: map[string]any{"query": "latency"}})
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != WorkflowRunPending || run.Input["query"] != "latency" {
		t.Fatalf("created run = %#v", run)
	}
	run, err = store.UpdateWorkflowRun(ctx, run.ID, WorkflowRunPatch{Status: WorkflowRunRunning, Attempt: 1, CurrentNodeID: "retrieve", State: map[string]any{"cursor": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != WorkflowRunRunning || run.CurrentNodeID != "retrieve" || run.State["cursor"] != float64(1) {
		t.Fatalf("running run = %#v", run)
	}
	run, err = store.UpdateWorkflowRun(ctx, run.ID, WorkflowRunPatch{Status: WorkflowRunSucceeded, Attempt: 1})
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != WorkflowRunSucceeded || run.CompletedAt == nil {
		t.Fatalf("completed run = %#v", run)
	}
	if _, err := store.UpdateWorkflowRun(ctx, run.ID, WorkflowRunPatch{Status: WorkflowRunRunning, Attempt: 2}); err == nil {
		t.Fatal("terminal run was allowed to resume")
	}
}

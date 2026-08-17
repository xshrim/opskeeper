package observability

import (
	"context"
	"testing"
	"time"
)

func TestSetupWithoutEndpointIsNoop(t *testing.T) {
	shutdown, err := Setup(context.Background(), "test", "test", "", Build{Version: "test", Commit: "abc"})
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
	RecordTask(context.Background(), "test", "success", time.Millisecond)
	RecordConnector(context.Background(), "test", "failure", time.Millisecond)
	RecordLLM(context.Background(), "success", 10)
	RecordError(context.Background(), "test", "expected")
}

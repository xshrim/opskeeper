package audit

import (
	"context"
	"strings"
	"testing"
)

type captureStore struct{ event Event }

func (s *captureStore) Record(_ context.Context, event Event) error {
	s.event = event
	return nil
}

func TestRecordRedactsSensitiveDetails(t *testing.T) {
	store := &captureStore{}
	service := NewService(store)
	longValue := strings.Repeat("x", 3000)
	err := service.Record(context.Background(), Event{Action: "test", Details: map[string]any{
		"token":         "visible",
		"credential_id": "safe-id",
		"nested":        map[string]any{"db_password": "visible", "message": longValue},
	}})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if store.event.Details["token"] != "[REDACTED]" || store.event.Details["credential_id"] != "safe-id" {
		t.Fatalf("sanitized details = %#v", store.event.Details)
	}
	nested := store.event.Details["nested"].(map[string]any)
	if nested["db_password"] != "[REDACTED]" || len(nested["message"].(string)) >= len(longValue) {
		t.Fatalf("sanitized nested details = %#v", nested)
	}
}

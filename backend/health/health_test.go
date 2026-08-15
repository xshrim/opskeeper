package health

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReadinessReportsFailedDependency(t *testing.T) {
	service := NewService("test-api", time.Second, []Check{
		{Name: "postgres", Run: func(context.Context) error { return nil }},
		{Name: "redis", Run: func(context.Context) error { return errors.New("unavailable") }},
	})

	report := service.Readiness(context.Background(), "test")
	if report.Status != "not_ready" {
		t.Fatalf("Readiness() status = %q, want not_ready", report.Status)
	}
	if report.Service != "test-api" {
		t.Fatalf("Readiness() service = %q, want test-api", report.Service)
	}
	if report.Checks["redis"].Status != "down" {
		t.Fatalf("Readiness() redis = %#v, want down", report.Checks["redis"])
	}
}

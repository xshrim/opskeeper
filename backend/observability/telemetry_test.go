package observability

import (
	"bytes"
	"context"
	"errors"
	"log"
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

func TestTelemetryExportErrorsUseSignalFields(t *testing.T) {
	tests := []struct {
		input, signal, message string
	}{
		{"traces export: Post \"http://collector/v1/traces\": refused", "traces", "Post \"http://collector/v1/traces\": refused"},
		{"failed to upload metrics: Post \"http://collector/v1/metrics\": refused", "metrics", "Post \"http://collector/v1/metrics\": refused"},
	}
	for _, test := range tests {
		signal, message := telemetryErrorFields(errors.New(test.input))
		if signal != test.signal || message != test.message {
			t.Errorf("telemetryErrorFields() = (%q, %q), want (%q, %q)", signal, message, test.signal, test.message)
		}
	}
}

func TestExportStatusLogsOnlyTransitions(t *testing.T) {
	var output bytes.Buffer
	status := &exportStatus{
		seen:       make(map[string]bool),
		successful: make(map[string]bool),
		logger:     log.New(&output, "", 0),
	}
	tracesFailure := errors.New("traces export: Post \"http://collector/v1/traces\": refused")
	metricsFailure := errors.New("failed to upload metrics: Post \"http://collector/v1/metrics\": refused")
	for _, item := range []struct {
		signal string
		err    error
	}{
		{"traces", nil},
		{"traces", nil},
		{"traces", tracesFailure},
		{"traces", tracesFailure},
		{"metrics", metricsFailure},
		{"metrics", metricsFailure},
		{"traces", nil},
		{"traces", nil},
	} {
		status.report(item.signal, item.err)
	}
	want := "traces export: success\ntraces export: Post \"http://collector/v1/traces\": refused\nmetrics export: Post \"http://collector/v1/metrics\": refused\ntraces export: success\n"
	if got := output.String(); got != want {
		t.Fatalf("logs = %q, want %q", got, want)
	}
}

func TestSetupWithInvalidEndpointDoesNotFailStartup(t *testing.T) {
	shutdown, err := Setup(context.Background(), "test", "test", "not a URL", Build{})
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	if shutdown == nil {
		t.Fatal("Setup() returned nil shutdown function")
	}
}

package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewTextLogger(t *testing.T) {
	var output bytes.Buffer
	logger, err := New(&output, FormatText)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logger.With("service", "opskeeper-api").Info("started")

	line := output.String()
	if strings.HasPrefix(line, "{") || !strings.Contains(line, "msg=started") || !strings.Contains(line, "service=opskeeper-api") {
		t.Fatalf("text log output = %q", line)
	}
}

func TestNewJSONLogger(t *testing.T) {
	var output bytes.Buffer
	logger, err := New(&output, FormatJSON)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logger.With("service", "opskeeper-api").Info("started")

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal JSON log: %v", err)
	}
	if record["msg"] != "started" || record["service"] != "opskeeper-api" {
		t.Fatalf("JSON log record = %#v", record)
	}
}

func TestNewRejectsUnsupportedFormat(t *testing.T) {
	if _, err := New(&bytes.Buffer{}, "pretty"); err == nil {
		t.Fatal("New() error = nil, want unsupported format error")
	}
}

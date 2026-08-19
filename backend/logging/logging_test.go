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

func TestNewRawLoggerUsesStableHeaderAndSanitizesControlCharacters(t *testing.T) {
	var output bytes.Buffer
	logger, err := New(&output, FormatRaw)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logger.With("service", "opskeeper-api", "kind", "service-start").Info("started\nnext")

	line := strings.TrimSuffix(output.String(), "\n")
	fields := strings.SplitN(line, " ", 9)
	if len(fields) != 9 {
		t.Fatalf("raw log fields = %d, output = %q", len(fields), line)
	}
	if fields[0] == "" || fields[1] != "INFO" || fields[2] != "opskeeper-api" || fields[3] != "service-start" || fields[4] != "-" || fields[5] != "-" {
		t.Fatalf("raw log header = %q", fields[:6])
	}
	if strings.ContainsAny(line, "\r\n\t") || fields[8] != "started next" {
		t.Fatalf("raw log message = %q", fields[8])
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

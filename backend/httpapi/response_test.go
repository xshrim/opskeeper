package httpapi

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
	"opskeeper/backend/logging"
)

func TestWriteInternalErrorCorrelatesResponseAndLogs(t *testing.T) {
	var output bytes.Buffer
	logger := logging.NewRaw(&output).With("service", "test-api")
	handler := middleware.RequestID(requestLogger(logger, "/", false)(requestLoggerContext(logger)(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeInternalError(writer, request, "resource lookup", errors.New("database password=secret postgres://db-user:db-secret@db.internal/opskeeper connection refused"))
	}))))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/resources", nil))

	body := response.Body.String()
	if response.Code != http.StatusInternalServerError || !strings.Contains(body, `"code":"internal_error"`) || !strings.Contains(body, `"request_id":`) {
		t.Fatalf("error response = %d %s", response.Code, body)
	}
	if strings.Contains(body, "secret") || strings.Contains(body, "db-secret") || !strings.Contains(body, "connection refused") {
		t.Fatalf("error response detail = %s", body)
	}
	logLine := output.String()
	if !strings.Contains(logLine, "request failed") || !strings.Contains(logLine, "http-request") || !strings.Contains(logLine, "error_code=internal_error") || strings.Contains(logLine, "secret") || strings.Contains(logLine, "db-secret") {
		t.Fatalf("correlated logs = %q", logLine)
	}
}

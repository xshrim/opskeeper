package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

type requestLoggerContextKey struct{}

var sensitiveErrorPattern = regexp.MustCompile(`(?i)(?:postgres(?:ql)?|mysql|redis|rediss|amqp|kafka)://[^\s,;\)\]}]+|https?://[^/@\s]+:[^/@\s]+@[^\s,;\)\]}]+|(authorization|api[- ]?key|token|secret|password|bearer|dsn|connection string)(\s*(?:[:=]|\s)\s*)[^\s,;\)\]}]+`)

func withRequestLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, requestLoggerContextKey{}, logger)
}

func requestLoggerFromContext(ctx context.Context) *slog.Logger {
	logger, _ := ctx.Value(requestLoggerContextKey{}).(*slog.Logger)
	return logger
}

func writeJSON(writer http.ResponseWriter, status int, body any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(body)
}

func writeError(writer http.ResponseWriter, request *http.Request, status int, code, message string) {
	writeLoggedError(writer, request, status, code, message)
}

func writeLoggedError(writer http.ResponseWriter, request *http.Request, status int, code, message string) {
	requestID := middleware.GetReqID(request.Context())
	message = publicErrorMessage(message)
	if recorder, ok := writer.(*statusRecorder); ok {
		recorder.errorCode = code
		recorder.errorReason = message
	}
	if logger := requestLoggerFromContext(request.Context()); logger != nil {
		level := slog.LevelWarn
		if status >= http.StatusInternalServerError {
			level = slog.LevelError
		}
		attrs := []any{"kind", "error", "reqid", requestID, "status", status, "error_type", code}
		attrs = append(attrs, "error_summary", message)
		logger.Log(request.Context(), level, "request failed", attrs...)
	}
	writeJSON(writer, status, errorBody{Error: errorDetail{
		Code:      code,
		Message:   message,
		RequestID: requestID,
	}})
}

// writeInternalError keeps the response useful while applying the same
// redaction and request correlation rules as every other API error.
func writeInternalError(writer http.ResponseWriter, request *http.Request, operation string, err error) {
	reason := publicErrorMessage(errorSummary(err))
	if reason == "" {
		reason = "unknown cause"
	}
	message := fmt.Sprintf("%s failed: %s", operation, reason)
	writeLoggedError(writer, request, http.StatusInternalServerError, "internal_error", message)
}

func errorSummary(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func publicErrorMessage(message string) string {
	message = strings.Join(strings.Fields(strings.TrimSpace(message)), " ")
	message = sensitiveErrorPattern.ReplaceAllString(message, "<redacted>")
	if len([]rune(message)) > 600 {
		message = string([]rune(message)[:600]) + "..."
	}
	return message
}

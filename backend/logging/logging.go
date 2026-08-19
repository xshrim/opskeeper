package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
)

const (
	FormatRaw  = "raw"
	FormatText = "text"
	FormatJSON = "json"
)

type handler struct {
	output io.Writer
	format string
	attrs  []slog.Attr
	groups []string
	mu     *sync.Mutex
}

type entry struct {
	Time     string `json:"time"`
	Level    string `json:"level"`
	Service  string `json:"service"`
	Kind     string `json:"kind"`
	TraceID  string `json:"traceid"`
	SpanID   string `json:"spanid"`
	Func     string `json:"func"`
	FileLine string `json:"file:line"`
	Message  string `json:"msg"`
}

func New(output io.Writer, format string) (*slog.Logger, error) {
	switch format {
	case FormatRaw, FormatText, FormatJSON:
		return slog.New(&handler{output: output, format: format, mu: &sync.Mutex{}}), nil
	default:
		return nil, fmt.Errorf("unsupported log format %q", format)
	}
}

func NewRaw(output io.Writer) *slog.Logger {
	logger, err := New(output, FormatRaw)
	if err != nil {
		panic(err)
	}
	return logger
}

func NewText(output io.Writer) *slog.Logger {
	logger, err := New(output, FormatText)
	if err != nil {
		panic(err)
	}
	return logger
}

func (h *handler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *handler) Handle(ctx context.Context, record slog.Record) error {
	values := make([]slog.Attr, 0, len(h.attrs)+record.NumAttrs())
	values = append(values, h.attrs...)
	record.Attrs(func(attr slog.Attr) bool {
		values = append(values, attr)
		return true
	})

	fields := make(map[string]string, len(values))
	extras := make([]string, 0, len(values))
	for _, attr := range values {
		h.collect(attr, "", fields, &extras)
	}

	traceID, spanID := traceFields(ctx)
	function, fileLine := source(record.PC)
	kind := fields["kind"]
	if kind == "" {
		if record.Level >= slog.LevelError {
			kind = "error"
		} else {
			kind = "event"
		}
	}
	service := fields["service"]
	if service == "" {
		service = "-"
	}
	item := entry{
		Time:     record.Time.Format("2006-01-02T15:04:05.000"),
		Level:    record.Level.String(),
		Service:  service,
		Kind:     kind,
		TraceID:  traceID,
		SpanID:   spanID,
		Func:     function,
		FileLine: fileLine,
		Message:  formatMessage(kind, record.Message, fields, extras),
	}

	line, err := h.formatEntry(item)
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err = io.WriteString(h.output, line+"\n")
	return err
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &clone
}

func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.groups = append(append([]string{}, h.groups...), name)
	return &clone
}

func (h *handler) collect(attr slog.Attr, prefix string, fields map[string]string, extras *[]string) {
	if attr.Equal(slog.Attr{}) {
		return
	}
	if attr.Value.Kind() == slog.KindGroup {
		group := joinKey(prefix, attr.Key)
		for _, child := range attr.Value.Group() {
			h.collect(child, group, fields, extras)
		}
		return
	}
	key := joinKey(prefix, attr.Key)
	if len(h.groups) != 0 {
		key = joinKey(strings.Join(h.groups, "."), key)
	}
	value := sanitize(valueString(attr.Value))
	if key == "error" {
		// Error values may contain credentials, SQL, or untrusted upstream text.
		// Callers should provide a low-cardinality error_type instead.
		return
	}
	if key == "service" || key == "kind" || key == "reqid" || key == "clientip" || key == "method" || key == "path" || key == "status" || key == "duration" {
		fields[key] = value
		return
	}
	*extras = append(*extras, key+"="+value)
}

func (h *handler) formatEntry(item entry) (string, error) {
	switch h.format {
	case FormatRaw:
		return strings.Join([]string{
			sanitizeHeader(item.Time),
			sanitizeHeader(item.Level),
			sanitizeHeader(item.Service),
			sanitizeHeader(item.Kind),
			sanitizeHeader(item.TraceID),
			sanitizeHeader(item.SpanID),
			sanitizeHeader(item.Func),
			sanitizeHeader(item.FileLine),
			item.Message,
		}, " "), nil
	case FormatText:
		return strings.Join([]string{
			"time=" + quoteText(item.Time),
			"level=" + quoteText(item.Level),
			"service=" + quoteText(item.Service),
			"kind=" + quoteText(item.Kind),
			"traceid=" + quoteText(item.TraceID),
			"spanid=" + quoteText(item.SpanID),
			"func=" + quoteText(item.Func),
			"file:line=" + quoteText(item.FileLine),
			"msg=" + quoteText(item.Message),
		}, " "), nil
	case FormatJSON:
		return marshalJSON(item)
	default:
		return "", fmt.Errorf("unsupported log format %q", h.format)
	}
}

func marshalJSON(item entry) (string, error) {
	encoded, err := json.Marshal(item)
	return string(encoded), err
}

func formatMessage(kind, message string, fields map[string]string, extras []string) string {
	if kind == "http-request" {
		return strings.Join([]string{
			valueOrDash(fields, "reqid"),
			valueOrDash(fields, "clientip"),
			valueOrDash(fields, "method"),
			valueOrDash(fields, "path"),
			valueOrDash(fields, "status"),
			valueOrDash(fields, "duration"),
		}, " ")
	}
	parts := make([]string, 0, 1+len(extras))
	if message = sanitize(message); message != "" {
		parts = append(parts, message)
	}
	parts = append(parts, extras...)
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " ")
}

func valueOrDash(fields map[string]string, key string) string {
	if value := fields[key]; value != "" {
		return value
	}
	return "-"
}

func traceFields(ctx context.Context) (string, string) {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return "-", "-"
	}
	return spanContext.TraceID().String(), spanContext.SpanID().String()
}

func source(pc uintptr) (string, string) {
	if pc == 0 {
		return "-", "-"
	}
	frame, _ := runtime.CallersFrames([]uintptr{pc}).Next()
	name := frame.Function
	if marker := strings.Index(name, "opskeeper/backend/"); marker >= 0 {
		name = name[marker+len("opskeeper/backend/"):]
	}
	name = strings.ReplaceAll(name, "/", ".")
	if name == "" {
		name = "-"
	}
	file := filepath.Base(frame.File)
	if file == "." || file == "" || frame.Line == 0 {
		return name, "-"
	}
	return name, file + ":" + strconv.Itoa(frame.Line)
}

func valueString(value slog.Value) string {
	value = value.Resolve()
	switch value.Kind() {
	case slog.KindString:
		return value.String()
	case slog.KindInt64:
		return strconv.FormatInt(value.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(value.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(value.Float64(), 'f', -1, 64)
	case slog.KindBool:
		return strconv.FormatBool(value.Bool())
	case slog.KindDuration:
		return value.Duration().String()
	case slog.KindTime:
		return value.Time().Format(time.RFC3339Nano)
	case slog.KindAny:
		return fmt.Sprint(value.Any())
	default:
		return value.String()
	}
}

func quoteText(value string) string {
	value = sanitize(value)
	if value != "" && !strings.ContainsAny(value, " \\\"") {
		return value
	}
	return strconv.Quote(value)
}

func sanitize(value string) string {
	return strings.Map(func(character rune) rune {
		if character < ' ' || character == 0x7f {
			return ' '
		}
		return character
	}, value)
}

func sanitizeHeader(value string) string {
	return strings.Join(strings.Fields(sanitize(value)), "_")
}

func joinKey(prefix, key string) string {
	if prefix == "" {
		return key
	}
	if key == "" {
		return prefix
	}
	return prefix + "." + key
}

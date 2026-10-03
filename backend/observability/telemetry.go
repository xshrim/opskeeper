package observability

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Build struct {
	Version, Commit string
}

// Setup configures the standard OTLP HTTP exporters. An empty endpoint keeps
// local development dependency-free while leaving instrumentation active as a
// no-op through the global OpenTelemetry API.
func Setup(ctx context.Context, serviceName, environment, endpoint string, build Build) (func(context.Context) error, error) {
	if strings.TrimSpace(endpoint) == "" {
		return noopShutdown(), nil
	}
	status := &exportStatus{seen: make(map[string]bool), successful: make(map[string]bool), logger: log.Default()}
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		signal, message := telemetryErrorFields(err)
		status.report(signal, errors.New(message))
	}))
	res, err := resource.New(ctx, resource.WithAttributes(
		attribute.String("service.name", serviceName),
		attribute.String("service.version", build.Version),
		attribute.String("service.instance.commit", build.Commit),
		attribute.String("deployment.environment.name", environment),
	))
	if err != nil {
		return noopShutdown(), nil
	}
	traceExporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint))
	if err != nil {
		return noopShutdown(), nil
	}
	metricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL(endpoint))
	if err != nil {
		_ = traceExporter.Shutdown(ctx)
		return noopShutdown(), nil
	}
	traces := sdktrace.NewTracerProvider(sdktrace.WithBatcher(&traceStatusExporter{SpanExporter: traceExporter, status: status}), sdktrace.WithResource(res))
	metrics := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(&metricStatusExporter{Exporter: metricExporter, status: status}, sdkmetric.WithInterval(30*time.Second))),
	)
	otel.SetTracerProvider(traces)
	otel.SetMeterProvider(metrics)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return func(shutdownCtx context.Context) error {
		// Telemetry is best-effort. A collector being stopped or unreachable must
		// never turn a successful command or graceful process shutdown into an
		// application warning or failure.
		_ = errors.Join(metrics.Shutdown(shutdownCtx), traces.Shutdown(shutdownCtx))
		return nil
	}, nil
}

type exportStatus struct {
	mu         sync.Mutex
	seen       map[string]bool
	successful map[string]bool
	logger     *log.Logger
}

func (s *exportStatus) report(signal string, err error) {
	success := err == nil
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen[signal] && s.successful[signal] == success {
		return
	}
	s.seen[signal] = true
	s.successful[signal] = success
	if err == nil {
		s.logger.Printf("%s export: success", signal)
		return
	}
	_, message := telemetryErrorFields(err)
	s.logger.Printf("%s export: %s", signal, message)
}

type traceStatusExporter struct {
	sdktrace.SpanExporter
	status *exportStatus
}

func (e *traceStatusExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	err := e.SpanExporter.ExportSpans(ctx, spans)
	if err == nil {
		e.status.report("traces", nil)
	}
	return err
}

type metricStatusExporter struct {
	sdkmetric.Exporter
	status *exportStatus
}

func (e *metricStatusExporter) Export(ctx context.Context, metrics *metricdata.ResourceMetrics) error {
	err := e.Exporter.Export(ctx, metrics)
	if err == nil {
		e.status.report("metrics", nil)
	}
	return err
}

func telemetryErrorFields(err error) (string, string) {
	message := err.Error()
	for _, item := range []struct{ prefix, signal string }{
		{"traces export: ", "traces"},
		{"failed to upload metrics: ", "metrics"},
	} {
		if strings.HasPrefix(message, item.prefix) {
			return item.signal, strings.TrimPrefix(message, item.prefix)
		}
	}
	return "unknown", message
}

// Telemetry is optional. A malformed or unavailable OTLP endpoint must never
// prevent the application process from starting or serving requests.
func noopShutdown() func(context.Context) error {
	return func(context.Context) error { return nil }
}

var instruments struct {
	once               sync.Once
	tasks              metric.Int64Counter
	taskDuration       metric.Float64Histogram
	connectors         metric.Int64Counter
	connectorDuration  metric.Float64Histogram
	llmExecutions      metric.Int64Counter
	llmTokens          metric.Int64Counter
	errors             metric.Int64Counter
	notificationQueued metric.Int64Gauge
	notificationLeased metric.Int64Gauge
	notificationDead   metric.Int64Gauge
	notificationOldest metric.Float64Gauge
}

func initializeInstruments() {
	instruments.once.Do(func() {
		meter := otel.Meter("opskeeper/backend")
		instruments.tasks, _ = meter.Int64Counter("opskeeper.tasks", metric.WithDescription("Completed background tasks"))
		instruments.taskDuration, _ = meter.Float64Histogram("opskeeper.task.duration", metric.WithUnit("s"))
		instruments.connectors, _ = meter.Int64Counter("opskeeper.connector.calls")
		instruments.connectorDuration, _ = meter.Float64Histogram("opskeeper.connector.duration", metric.WithUnit("s"))
		instruments.llmExecutions, _ = meter.Int64Counter("opskeeper.llm.executions")
		instruments.llmTokens, _ = meter.Int64Counter("opskeeper.llm.tokens")
		instruments.errors, _ = meter.Int64Counter("opskeeper.errors")
		instruments.notificationQueued, _ = meter.Int64Gauge("opskeeper.notification.queue.queued")
		instruments.notificationLeased, _ = meter.Int64Gauge("opskeeper.notification.queue.delivering")
		instruments.notificationDead, _ = meter.Int64Gauge("opskeeper.notification.queue.dead_letter")
		instruments.notificationOldest, _ = meter.Float64Gauge("opskeeper.notification.queue.oldest_age", metric.WithUnit("s"))
	})
}

func RecordTask(ctx context.Context, kind, result string, duration time.Duration) {
	initializeInstruments()
	attrs := metric.WithAttributes(attribute.String("task.kind", kind), attribute.String("result", result))
	instruments.tasks.Add(ctx, 1, attrs)
	instruments.taskDuration.Record(ctx, duration.Seconds(), attrs)
}

func RecordConnector(ctx context.Context, capability, result string, duration time.Duration) {
	initializeInstruments()
	attrs := metric.WithAttributes(attribute.String("connector.capability", capability), attribute.String("result", result))
	instruments.connectors.Add(ctx, 1, attrs)
	instruments.connectorDuration.Record(ctx, duration.Seconds(), attrs)
}

func RecordLLM(ctx context.Context, result string, totalTokens int64) {
	initializeInstruments()
	instruments.llmExecutions.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
	if totalTokens > 0 {
		instruments.llmTokens.Add(ctx, totalTokens, metric.WithAttributes(attribute.String("token.kind", "total")))
	}
}

func RecordError(ctx context.Context, component, category string) {
	initializeInstruments()
	instruments.errors.Add(ctx, 1, metric.WithAttributes(attribute.String("component", component), attribute.String("category", category)))
}

func RecordNotificationQueue(ctx context.Context, queued, delivering, deadLetter int64, oldestAge time.Duration) {
	initializeInstruments()
	instruments.notificationQueued.Record(ctx, queued)
	instruments.notificationLeased.Record(ctx, delivering)
	instruments.notificationDead.Record(ctx, deadLetter)
	instruments.notificationOldest.Record(ctx, oldestAge.Seconds())
}

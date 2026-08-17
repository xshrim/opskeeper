package observability

import (
	"context"
	"errors"
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
		return func(context.Context) error { return nil }, nil
	}
	res, err := resource.New(ctx, resource.WithAttributes(
		attribute.String("service.name", serviceName),
		attribute.String("service.version", build.Version),
		attribute.String("service.instance.commit", build.Commit),
		attribute.String("deployment.environment.name", environment),
	))
	if err != nil {
		return nil, err
	}
	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		_ = traceExporter.Shutdown(ctx)
		return nil, err
	}
	traces := sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExporter), sdktrace.WithResource(res))
	metrics := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(30*time.Second))),
	)
	otel.SetTracerProvider(traces)
	otel.SetMeterProvider(metrics)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return func(shutdownCtx context.Context) error {
		return errors.Join(metrics.Shutdown(shutdownCtx), traces.Shutdown(shutdownCtx))
	}, nil
}

var instruments struct {
	once              sync.Once
	tasks             metric.Int64Counter
	taskDuration      metric.Float64Histogram
	connectors        metric.Int64Counter
	connectorDuration metric.Float64Histogram
	llmExecutions     metric.Int64Counter
	llmTokens         metric.Int64Counter
	errors            metric.Int64Counter
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

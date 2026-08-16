package connector

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type prometheusAdapter struct {
	target   Target
	executor *httpExecutor
}

func newPrometheusAdapter(target Target, client *http.Client, limits Limits) (Adapter, error) {
	executor, err := newHTTPExecutor(target, client, limits.MaxResponseBytes)
	if err != nil {
		return nil, err
	}
	return &prometheusAdapter{target: target, executor: executor}, nil
}

func (a *prometheusAdapter) Kind() string { return "Prometheus" }

func (a *prometheusAdapter) Capabilities() []Capability {
	return []Capability{CapabilityQueryMetrics, CapabilityGetAlerts}
}

func (a *prometheusAdapter) Test(ctx context.Context) error {
	body, err := a.executor.get(ctx, "test Prometheus connection", "/api/v1/status/buildinfo", nil)
	if err != nil {
		return err
	}
	_, _, err = validateEnvelope("test Prometheus connection", body)
	return err
}

func (a *prometheusAdapter) QueryMetrics(ctx context.Context, query MetricsQuery) (Evidence, error) {
	values := url.Values{
		"query": []string{query.Query},
		"start": []string{formatUnix(query.Start)},
		"end":   []string{formatUnix(query.End)},
		"step":  []string{strconv.FormatFloat(query.Step.Seconds(), 'f', -1, 64)},
	}
	body, err := a.executor.get(ctx, "query Prometheus metrics", "/api/v1/query_range", values)
	if err != nil {
		return Evidence{}, err
	}
	resultType, count, err := validateEnvelope("query Prometheus metrics", body)
	if err != nil {
		return Evidence{}, err
	}
	return Evidence{
		CollectedAt: time.Now(), Window: &Window{Start: query.Start, End: query.End}, Data: body,
		Summary: map[string]any{"result_type": resultType, "series_count": count},
	}, nil
}

func (a *prometheusAdapter) GetAlerts(ctx context.Context, _ AlertsQuery) (Evidence, error) {
	body, err := a.executor.get(ctx, "get Prometheus alerts", "/api/v1/alerts", nil)
	if err != nil {
		return Evidence{}, err
	}
	_, count, err := validateEnvelope("get Prometheus alerts", body)
	if err != nil {
		return Evidence{}, err
	}
	return Evidence{CollectedAt: time.Now(), Data: body, Summary: map[string]any{"alert_count": count}}, nil
}

func formatUnix(value time.Time) string {
	return strconv.FormatFloat(float64(value.UnixNano())/float64(time.Second), 'f', 3, 64)
}

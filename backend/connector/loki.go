package connector

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type lokiAdapter struct {
	target   Target
	executor *httpExecutor
}

func newLokiAdapter(target Target, client *http.Client, limits Limits) (Adapter, error) {
	executor, err := newHTTPExecutor(target, client, limits.MaxResponseBytes)
	if err != nil {
		return nil, err
	}
	if tenant := configString(target.Resource.Config, "tenant_id"); tenant != "" {
		executor.secret["tenant_id"] = tenant
	}
	return &lokiAdapter{target: target, executor: executor}, nil
}

func (a *lokiAdapter) Kind() string { return "Loki" }

func (a *lokiAdapter) Capabilities() []Capability { return []Capability{CapabilityQueryLogs} }

func (a *lokiAdapter) Test(ctx context.Context) error {
	_, err := a.executor.get(ctx, "test Loki connection", "/loki/api/v1/status/buildinfo", nil)
	return err
}

func (a *lokiAdapter) QueryLogs(ctx context.Context, query LogsQuery) (Evidence, error) {
	values := url.Values{
		"query":     []string{query.Query},
		"start":     []string{strconv.FormatInt(query.Start.UnixNano(), 10)},
		"end":       []string{strconv.FormatInt(query.End.UnixNano(), 10)},
		"limit":     []string{strconv.Itoa(query.Limit)},
		"direction": []string{"backward"},
	}
	body, err := a.executor.get(ctx, "query Loki logs", "/loki/api/v1/query_range", values)
	if err != nil {
		return Evidence{}, err
	}
	resultType, count, err := validateEnvelope("query Loki logs", body)
	if err != nil {
		return Evidence{}, err
	}
	return Evidence{
		CollectedAt: time.Now(), Window: &Window{Start: query.Start, End: query.End}, Data: body,
		Summary: map[string]any{"result_type": resultType, "stream_count": count, "entry_limit": query.Limit},
	}, nil
}

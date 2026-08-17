package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"opskeeper/backend/observability"
	"opskeeper/backend/resource"
)

type ResourceReader interface {
	Get(context.Context, string) (resource.Resource, error)
}

type CredentialReader interface {
	RevealLinked(context.Context, string) ([]byte, error)
}

type Service struct {
	registry    *Registry
	resources   ResourceReader
	credentials CredentialReader
	checks      CheckStore
	limits      Limits
	slots       chan struct{}
	now         func() time.Time
}

func NewService(registry *Registry, resources ResourceReader, credentials CredentialReader, checks CheckStore, limits Limits) *Service {
	if limits.Timeout <= 0 || limits.MaxConcurrent <= 0 || limits.MaxResponseBytes <= 0 {
		limits = DefaultLimits()
	}
	return &Service{
		registry: registry, resources: resources, credentials: credentials, checks: checks,
		limits: limits, slots: make(chan struct{}, limits.MaxConcurrent), now: time.Now,
	}
}

func (s *Service) Test(ctx context.Context, actorID, resourceID string) (Check, error) {
	metricStarted := time.Now()
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return Check{}, invalid("resource_id is required")
	}
	if s.resources == nil || s.checks == nil {
		return Check{}, connectorError(CategoryInternal, "test connector", false, errors.New("connector service dependencies are unavailable"))
	}
	item, err := s.resources.Get(ctx, resourceID)
	if err != nil {
		return Check{}, err
	}
	started := s.now()
	check := Check{ResourceID: item.ID, Status: "failed", CheckedAt: started}
	if strings.TrimSpace(actorID) != "" {
		check.CheckedBy = &actorID
	}

	adapter, err := s.prepare(ctx, item)
	if err == nil {
		check.Capabilities = adapter.Capabilities()
		err = s.execute(ctx, func(runCtx context.Context) error { return adapter.Test(runCtx) })
	}
	check.LatencyMS = max(s.now().Sub(started).Milliseconds(), 0)
	if err == nil {
		check.Status = "succeeded"
		check.Message = "连接测试通过"
	} else {
		check.ErrorCategory, _ = classify(err)
		check.Message = publicMessage(err)
	}
	metricResult := "success"
	if err != nil {
		metricResult = string(check.ErrorCategory)
		observability.RecordError(ctx, "connector", metricResult)
	}
	observability.RecordConnector(ctx, "test", metricResult, time.Since(metricStarted))
	saved, saveErr := s.checks.Save(ctx, check)
	if saveErr != nil {
		return Check{}, saveErr
	}
	return saved, nil
}

func (s *Service) Latest(ctx context.Context, resourceID string) (Check, error) {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return Check{}, invalid("resource_id is required")
	}
	if s.resources == nil || s.checks == nil {
		return Check{}, connectorError(CategoryInternal, "read latest connection check", false, errors.New("connector service dependencies are unavailable"))
	}
	if _, err := s.resources.Get(ctx, resourceID); err != nil {
		return Check{}, err
	}
	return s.checks.Latest(ctx, resourceID)
}

func (s *Service) QueryMetrics(ctx context.Context, resourceID string, query MetricsQuery) (Evidence, error) {
	if err := validateWindow(query.Start, query.End, s.limits.MaxQueryRange); err != nil {
		return Evidence{}, err
	}
	if strings.TrimSpace(query.Query) == "" {
		return Evidence{}, invalid("metrics query is required")
	}
	if query.Step < s.limits.MinMetricsStep {
		return Evidence{}, invalid(fmt.Sprintf("metrics step must be at least %s", s.limits.MinMetricsStep))
	}
	item, adapter, err := s.adapter(ctx, resourceID)
	if err != nil {
		return Evidence{}, err
	}
	querier, ok := adapter.(MetricsQuerier)
	if !ok {
		return Evidence{}, connectorError(CategoryUnsupported, "query metrics", false, ErrUnsupported)
	}
	return s.collect(ctx, item.ID, CapabilityQueryMetrics, func(runCtx context.Context) (Evidence, error) {
		return querier.QueryMetrics(runCtx, query)
	})
}

func (s *Service) QueryLogs(ctx context.Context, resourceID string, query LogsQuery) (Evidence, error) {
	if err := validateWindow(query.Start, query.End, s.limits.MaxQueryRange); err != nil {
		return Evidence{}, err
	}
	if strings.TrimSpace(query.Query) == "" {
		return Evidence{}, invalid("logs query is required")
	}
	if query.Limit <= 0 || query.Limit > s.limits.MaxLogEntries {
		return Evidence{}, invalid(fmt.Sprintf("log limit must be between 1 and %d", s.limits.MaxLogEntries))
	}
	item, adapter, err := s.adapter(ctx, resourceID)
	if err != nil {
		return Evidence{}, err
	}
	querier, ok := adapter.(LogsQuerier)
	if !ok {
		return Evidence{}, connectorError(CategoryUnsupported, "query logs", false, ErrUnsupported)
	}
	return s.collect(ctx, item.ID, CapabilityQueryLogs, func(runCtx context.Context) (Evidence, error) {
		return querier.QueryLogs(runCtx, query)
	})
}

func (s *Service) QueryTraces(ctx context.Context, resourceID string, query TracesQuery) (Evidence, error) {
	if err := validateWindow(query.Start, query.End, s.limits.MaxQueryRange); err != nil {
		return Evidence{}, err
	}
	if strings.TrimSpace(query.Service) == "" {
		return Evidence{}, invalid("trace service is required")
	}
	item, adapter, err := s.adapter(ctx, resourceID)
	if err != nil {
		return Evidence{}, err
	}
	querier, ok := adapter.(TracesQuerier)
	if !ok {
		return Evidence{}, connectorError(CategoryUnsupported, "query traces", false, ErrUnsupported)
	}
	return s.collect(ctx, item.ID, CapabilityQueryTraces, func(runCtx context.Context) (Evidence, error) {
		return querier.QueryTraces(runCtx, query)
	})
}

func (s *Service) GetAlerts(ctx context.Context, resourceID string, query AlertsQuery) (Evidence, error) {
	item, adapter, err := s.adapter(ctx, resourceID)
	if err != nil {
		return Evidence{}, err
	}
	querier, ok := adapter.(AlertsQuerier)
	if !ok {
		return Evidence{}, connectorError(CategoryUnsupported, "get alerts", false, ErrUnsupported)
	}
	return s.collect(ctx, item.ID, CapabilityGetAlerts, func(runCtx context.Context) (Evidence, error) {
		return querier.GetAlerts(runCtx, query)
	})
}

func (s *Service) ReadKubernetes(ctx context.Context, resourceID string, query KubernetesQuery) (Evidence, error) {
	if query.Limit <= 0 {
		query.Limit = s.limits.MaxKubernetesItems
	}
	if query.Limit > s.limits.MaxKubernetesItems {
		return Evidence{}, invalid(fmt.Sprintf("Kubernetes item limit must not exceed %d", s.limits.MaxKubernetesItems))
	}
	item, adapter, err := s.adapter(ctx, resourceID)
	if err != nil {
		return Evidence{}, err
	}
	reader, ok := adapter.(KubernetesReader)
	if !ok {
		return Evidence{}, connectorError(CategoryUnsupported, "read Kubernetes", false, ErrUnsupported)
	}
	return s.collect(ctx, item.ID, CapabilityKubernetesRead, func(runCtx context.Context) (Evidence, error) {
		return reader.ReadKubernetes(runCtx, query)
	})
}

func (s *Service) InspectPostgreSQL(ctx context.Context, resourceID string) (Evidence, error) {
	return s.inspect(ctx, resourceID, CapabilityPostgreSQLInspect, func(adapter Adapter, runCtx context.Context) (DiagnosticSnapshot, error) {
		inspector, ok := adapter.(PostgreSQLInspector)
		if !ok {
			return DiagnosticSnapshot{}, connectorError(CategoryUnsupported, "inspect PostgreSQL", false, ErrUnsupported)
		}
		return inspector.InspectPostgreSQL(runCtx)
	})
}

func (s *Service) InspectRedis(ctx context.Context, resourceID string) (Evidence, error) {
	return s.inspect(ctx, resourceID, CapabilityRedisInspect, func(adapter Adapter, runCtx context.Context) (DiagnosticSnapshot, error) {
		inspector, ok := adapter.(RedisInspector)
		if !ok {
			return DiagnosticSnapshot{}, connectorError(CategoryUnsupported, "inspect Redis", false, ErrUnsupported)
		}
		return inspector.InspectRedis(runCtx)
	})
}

func (s *Service) InspectKafka(ctx context.Context, resourceID string) (Evidence, error) {
	return s.inspect(ctx, resourceID, CapabilityKafkaInspect, func(adapter Adapter, runCtx context.Context) (DiagnosticSnapshot, error) {
		inspector, ok := adapter.(KafkaInspector)
		if !ok {
			return DiagnosticSnapshot{}, connectorError(CategoryUnsupported, "inspect Kafka", false, ErrUnsupported)
		}
		return inspector.InspectKafka(runCtx)
	})
}

func (s *Service) inspect(ctx context.Context, resourceID string, capability Capability, run func(Adapter, context.Context) (DiagnosticSnapshot, error)) (Evidence, error) {
	item, adapter, err := s.adapter(ctx, resourceID)
	if err != nil {
		return Evidence{}, err
	}
	return s.collect(ctx, item.ID, capability, func(runCtx context.Context) (Evidence, error) {
		snapshot, err := run(adapter, runCtx)
		if err != nil {
			return Evidence{}, err
		}
		snapshot.Findings = EvaluateDiagnosticSnapshot(snapshot)
		data, err := json.Marshal(snapshot)
		if err != nil {
			return Evidence{}, connectorError(CategoryInternal, "encode diagnostic snapshot", false, err)
		}
		return Evidence{CollectedAt: s.now(), Summary: map[string]any{"kind": snapshot.Kind, "finding_count": len(snapshot.Findings), "unavailable": snapshot.Unavailable}, Data: data}, nil
	})
}

func (s *Service) adapter(ctx context.Context, resourceID string) (resource.Resource, Adapter, error) {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return resource.Resource{}, nil, invalid("resource_id is required")
	}
	if s.resources == nil {
		return resource.Resource{}, nil, connectorError(CategoryInternal, "read connector resource", false, errors.New("resource service is unavailable"))
	}
	item, err := s.resources.Get(ctx, resourceID)
	if err != nil {
		return resource.Resource{}, nil, err
	}
	adapter, err := s.prepare(ctx, item)
	return item, adapter, err
}

func (s *Service) prepare(ctx context.Context, item resource.Resource) (Adapter, error) {
	if item.Status != resource.StatusActive {
		return nil, connectorError(CategoryConfiguration, "prepare connector", false, errors.New("resource is not active"))
	}
	target := Target{Resource: item}
	if item.CredentialID != nil && strings.TrimSpace(*item.CredentialID) != "" {
		if s.credentials == nil {
			return nil, connectorError(CategoryConfiguration, "read connector credential", false, errors.New("credential service is unavailable"))
		}
		secret, err := s.credentials.RevealLinked(ctx, *item.CredentialID)
		if err != nil {
			return nil, connectorError(CategoryConfiguration, "read connector credential", false, err)
		}
		target.Secret = secret
	}
	if s.registry == nil {
		return nil, connectorError(CategoryInternal, "resolve connector", false, errors.New("connector registry is unavailable"))
	}
	return s.registry.Resolve(target)
}

func (s *Service) collect(ctx context.Context, resourceID string, capability Capability, run func(context.Context) (Evidence, error)) (result Evidence, err error) {
	started := time.Now()
	defer func() {
		metricResult := "success"
		if err != nil {
			category, _ := classify(err)
			metricResult = string(category)
			observability.RecordError(ctx, "connector", metricResult)
		}
		observability.RecordConnector(ctx, string(capability), metricResult, time.Since(started))
	}()
	result, err = executeValue(s, ctx, run)
	if err != nil {
		return Evidence{}, err
	}
	if int64(len(result.Data)) > s.limits.MaxResponseBytes {
		return Evidence{}, connectorError(CategoryResponseTooLarge, "collect connector evidence", false, ErrResponseTooLarge)
	}
	result.SourceResourceID = resourceID
	result.Capability = capability
	if result.CollectedAt.IsZero() {
		result.CollectedAt = s.now()
	}
	if result.Summary == nil {
		result.Summary = map[string]any{}
	}
	return result, nil
}

func (s *Service) execute(ctx context.Context, run func(context.Context) error) error {
	_, err := executeValue(s, ctx, func(runCtx context.Context) (struct{}, error) {
		return struct{}{}, run(runCtx)
	})
	return err
}

func executeValue[T any](s *Service, ctx context.Context, run func(context.Context) (T, error)) (T, error) {
	var zero T
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		return zero, connectorError(CategoryRateLimited, "acquire connector slot", true, ErrRateLimited)
	}
	runCtx, cancel := context.WithTimeout(ctx, s.limits.Timeout)
	defer cancel()
	var lastErr error
	for attempt := 0; attempt <= s.limits.Retries; attempt++ {
		result, err := run(runCtx)
		if err == nil {
			return result, nil
		}
		lastErr = err
		_, temporary := classify(err)
		if !temporary || attempt == s.limits.Retries || runCtx.Err() != nil {
			break
		}
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return zero, connectorError(CategoryTimeout, "execute connector", true, context.DeadlineExceeded)
	}
	return zero, lastErr
}

func validateWindow(start, end time.Time, maximum time.Duration) error {
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return invalid("query start and end must form a positive window")
	}
	if end.Sub(start) > maximum {
		return invalid(fmt.Sprintf("query window must not exceed %s", maximum))
	}
	return nil
}

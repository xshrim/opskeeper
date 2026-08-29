package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"opskeeper/backend/aiengine"
)

// AIEngineProvider exposes existing connector capabilities through the
// generic T02 context/tool contract. The connector service remains the policy
// boundary for credentials, resource lookup, limits and read-only adapters.
func (s *Service) AIEngineProvider() aiengine.ContextProvider {
	return connectorContextProvider{service: s}
}

type connectorContextProvider struct{ service *Service }

func (connectorContextProvider) Kinds() []string {
	return []string{"Kubernetes", "Prometheus", "Loki", "PostgreSQL", "Redis", "Kafka"}
}

func (p connectorContextProvider) Resolve(ctx context.Context, resource aiengine.ContextResource) ([]aiengine.Tool, []aiengine.ContextFact, error) {
	if p.service == nil || p.service.resources == nil {
		return nil, nil, fmt.Errorf("connector service is unavailable")
	}
	item, err := p.service.resources.Get(ctx, resource.ID)
	if err != nil {
		return nil, nil, err
	}
	if item.Kind != resource.Kind {
		return nil, nil, fmt.Errorf("resource kind changed while resolving context")
	}
	tools := make([]aiengine.Tool, 0, 4)
	add := func(name, description string, schema json.RawMessage, fn func(context.Context, map[string]any) (aiengine.ToolResult, error)) {
		tools = append(tools, aiengine.ToolFunc{Def: aiengine.ToolDefinition{Name: name, Description: description, InputSchema: schema, Source: "connector", ResourceID: resource.ID, ReadOnly: true}, Fn: fn})
	}
	switch resource.Kind {
	case "Prometheus":
		add("connector.query_metrics", "Query bounded Prometheus metrics.", metricsSchema, func(runCtx context.Context, args map[string]any) (aiengine.ToolResult, error) {
			query, err := parseMetricsQuery(args)
			if err != nil {
				return aiengine.ToolResult{}, err
			}
			return evidenceResult(p.service.QueryMetrics(runCtx, resource.ID, query))
		})
		add("connector.get_alerts", "Read active Prometheus alerts.", alertsSchema, func(runCtx context.Context, args map[string]any) (aiengine.ToolResult, error) {
			return evidenceResult(p.service.GetAlerts(runCtx, resource.ID, AlertsQuery{ActiveOnly: boolArg(args, "active_only", true)}))
		})
	case "Loki":
		add("connector.query_logs", "Query bounded logs.", logsSchema, func(runCtx context.Context, args map[string]any) (aiengine.ToolResult, error) {
			query, err := parseLogsQuery(args)
			if err != nil {
				return aiengine.ToolResult{}, err
			}
			return evidenceResult(p.service.QueryLogs(runCtx, resource.ID, query))
		})
	case "Kubernetes":
		add("connector.read_kubernetes", "Read an allowlisted Kubernetes resource.", kubernetesSchema, func(runCtx context.Context, args map[string]any) (aiengine.ToolResult, error) {
			query := KubernetesQuery{Resource: stringArg(args, "resource"), Namespace: stringArg(args, "namespace"), Name: stringArg(args, "name"), LabelSelector: stringArg(args, "label_selector"), Limit: int64Arg(args, "limit")}
			return evidenceResult(p.service.ReadKubernetes(runCtx, resource.ID, query))
		})
	case "PostgreSQL":
		add("connector.inspect_postgresql", "Collect a read-only PostgreSQL diagnostic snapshot.", emptySchema, func(runCtx context.Context, _ map[string]any) (aiengine.ToolResult, error) {
			return evidenceResult(p.service.InspectPostgreSQL(runCtx, resource.ID))
		})
	case "Redis":
		add("connector.inspect_redis", "Collect a read-only Redis diagnostic snapshot.", emptySchema, func(runCtx context.Context, _ map[string]any) (aiengine.ToolResult, error) {
			return evidenceResult(p.service.InspectRedis(runCtx, resource.ID))
		})
	case "Kafka":
		add("connector.inspect_kafka", "Collect a read-only Kafka diagnostic snapshot.", emptySchema, func(runCtx context.Context, _ map[string]any) (aiengine.ToolResult, error) {
			return evidenceResult(p.service.InspectKafka(runCtx, resource.ID))
		})
	}
	facts := make([]aiengine.ContextFact, 0, 1)
	if resource.Kind == "PostgreSQL" || resource.Kind == "Redis" || resource.Kind == "Kafka" {
		var result aiengine.ToolResult
		var collectErr error
		switch resource.Kind {
		case "PostgreSQL":
			result, collectErr = evidenceResult(p.service.InspectPostgreSQL(ctx, resource.ID))
		case "Redis":
			result, collectErr = evidenceResult(p.service.InspectRedis(ctx, resource.ID))
		case "Kafka":
			result, collectErr = evidenceResult(p.service.InspectKafka(ctx, resource.ID))
		}
		if collectErr != nil {
			return nil, nil, collectErr
		}
		facts = append(facts, aiengine.ContextFact{ResourceID: resource.ID, Kind: resource.Kind, Data: result.Output, Untrusted: true})
	}
	return tools, facts, nil
}

var emptySchema = json.RawMessage(`{"type":"object","additionalProperties":false}`)
var metricsSchema = json.RawMessage(`{"type":"object","required":["query","start","end","step_seconds"],"properties":{"query":{"type":"string"},"start":{"type":"string"},"end":{"type":"string"},"step_seconds":{"type":"integer"}},"additionalProperties":false}`)
var logsSchema = json.RawMessage(`{"type":"object","required":["query","start","end","limit"],"properties":{"query":{"type":"string"},"start":{"type":"string"},"end":{"type":"string"},"limit":{"type":"integer"}},"additionalProperties":false}`)
var alertsSchema = json.RawMessage(`{"type":"object","properties":{"active_only":{"type":"boolean"}},"additionalProperties":false}`)
var kubernetesSchema = json.RawMessage(`{"type":"object","required":["resource"],"properties":{"resource":{"type":"string"},"namespace":{"type":"string"},"name":{"type":"string"},"label_selector":{"type":"string"},"limit":{"type":"integer"}},"additionalProperties":false}`)

func evidenceResult(evidence Evidence, err error) (aiengine.ToolResult, error) {
	if err != nil {
		return aiengine.ToolResult{}, err
	}
	return aiengine.ToolResult{Output: evidence, Untrusted: true}, nil
}

func parseMetricsQuery(args map[string]any) (MetricsQuery, error) {
	start, err := timeArg(args, "start")
	if err != nil {
		return MetricsQuery{}, err
	}
	end, err := timeArg(args, "end")
	if err != nil {
		return MetricsQuery{}, err
	}
	step := time.Duration(int64Arg(args, "step_seconds")) * time.Second
	return MetricsQuery{Query: stringArg(args, "query"), Start: start, End: end, Step: step}, nil
}
func parseLogsQuery(args map[string]any) (LogsQuery, error) {
	start, err := timeArg(args, "start")
	if err != nil {
		return LogsQuery{}, err
	}
	end, err := timeArg(args, "end")
	if err != nil {
		return LogsQuery{}, err
	}
	return LogsQuery{Query: stringArg(args, "query"), Start: start, End: end, Limit: int(int64Arg(args, "limit"))}, nil
}
func stringArg(args map[string]any, name string) string {
	value, _ := args[name].(string)
	return strings.TrimSpace(value)
}
func boolArg(args map[string]any, name string, fallback bool) bool {
	value, ok := args[name].(bool)
	if !ok {
		return fallback
	}
	return value
}
func int64Arg(args map[string]any, name string) int64 {
	switch value := args[name].(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case float64:
		return int64(value)
	default:
		return 0
	}
}
func timeArg(args map[string]any, name string) (time.Time, error) {
	value := stringArg(args, name)
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be RFC3339: %w", name, err)
	}
	return parsed, nil
}

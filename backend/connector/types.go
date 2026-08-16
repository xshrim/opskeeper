package connector

import (
	"context"
	"encoding/json"
	"time"

	"opskeeper/backend/resource"
)

type Capability string

const (
	CapabilityKubernetesRead Capability = "kubernetes_read"
	CapabilityQueryMetrics   Capability = "query_metrics"
	CapabilityQueryLogs      Capability = "query_logs"
	CapabilityQueryTraces    Capability = "query_traces"
	CapabilityGetAlerts      Capability = "get_alerts"
)

type Target struct {
	Resource resource.Resource
	Secret   []byte
}

type Window struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type Evidence struct {
	SourceResourceID string          `json:"source_resource_id"`
	Capability       Capability      `json:"capability"`
	CollectedAt      time.Time       `json:"collected_at"`
	Window           *Window         `json:"window,omitempty"`
	Summary          map[string]any  `json:"summary"`
	Data             json.RawMessage `json:"data"`
	Partial          bool            `json:"partial"`
}

type Check struct {
	ID            string       `json:"id"`
	ResourceID    string       `json:"resource_id"`
	Status        string       `json:"status"`
	ErrorCategory Category     `json:"error_category,omitempty"`
	Message       string       `json:"message"`
	LatencyMS     int64        `json:"latency_ms"`
	Capabilities  []Capability `json:"capabilities"`
	CheckedBy     *string      `json:"checked_by,omitempty"`
	CheckedAt     time.Time    `json:"checked_at"`
}

type MetricsQuery struct {
	Query string
	Start time.Time
	End   time.Time
	Step  time.Duration
}

type LogsQuery struct {
	Query string
	Start time.Time
	End   time.Time
	Limit int
}

type TracesQuery struct {
	Service   string
	Operation string
	Start     time.Time
	End       time.Time
	Limit     int
}

type AlertsQuery struct {
	ActiveOnly bool
}

type KubernetesQuery struct {
	Resource      string
	Namespace     string
	Name          string
	LabelSelector string
	Limit         int64
}

type Adapter interface {
	Kind() string
	Capabilities() []Capability
	Test(context.Context) error
}

type MetricsQuerier interface {
	QueryMetrics(context.Context, MetricsQuery) (Evidence, error)
}

type LogsQuerier interface {
	QueryLogs(context.Context, LogsQuery) (Evidence, error)
}

type TracesQuerier interface {
	QueryTraces(context.Context, TracesQuery) (Evidence, error)
}

type AlertsQuerier interface {
	GetAlerts(context.Context, AlertsQuery) (Evidence, error)
}

type KubernetesReader interface {
	ReadKubernetes(context.Context, KubernetesQuery) (Evidence, error)
}

type Limits struct {
	Timeout            time.Duration
	Retries            int
	MaxConcurrent      int
	MaxResponseBytes   int64
	MaxQueryRange      time.Duration
	MinMetricsStep     time.Duration
	MaxLogEntries      int
	MaxKubernetesItems int64
}

func DefaultLimits() Limits {
	return Limits{
		Timeout:            10 * time.Second,
		Retries:            1,
		MaxConcurrent:      8,
		MaxResponseBytes:   4 << 20,
		MaxQueryRange:      24 * time.Hour,
		MinMetricsStep:     15 * time.Second,
		MaxLogEntries:      1000,
		MaxKubernetesItems: 500,
	}
}

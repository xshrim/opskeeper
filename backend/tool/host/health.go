package host

import (
	"context"
	"time"
)

type HealthOutput struct {
	SchemaVersion int           `json:"schema_version"`
	CollectedAt   time.Time     `json:"collected_at"`
	Target        Target        `json:"target"`
	Status        string        `json:"status"`
	Info          InfoOutput    `json:"info"`
	Metrics       MetricsOutput `json:"metrics"`
	Partial       bool          `json:"partial"`
	Unavailable   []string      `json:"unavailable,omitempty"`
}

func Health(ctx context.Context, input ConnectionInput) (HealthOutput, error) {
	info, err := Info(ctx, InfoInput{ConnectionInput: input})
	if err != nil {
		return HealthOutput{}, err
	}
	metrics, err := Metrics(ctx, MetricsInput{ConnectionInput: input})
	if err != nil {
		return HealthOutput{}, err
	}
	out := HealthOutput{SchemaVersion: 1, CollectedAt: time.Now().UTC(), Target: info.Target, Status: "healthy", Info: info, Metrics: metrics, Partial: info.Partial || metrics.Partial}
	out.Unavailable = append(out.Unavailable, info.Unavailable...)
	out.Unavailable = append(out.Unavailable, metrics.Unavailable...)
	if out.Partial {
		out.Status = "degraded"
	}
	return out, nil
}

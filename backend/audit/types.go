package audit

import (
	"context"
	"time"
)

type Event struct {
	ActorUserID string         `json:"actor_user_id"`
	Action      string         `json:"action"`
	TargetType  string         `json:"target_type"`
	TargetID    string         `json:"target_id"`
	ScopeID     string         `json:"scope_id"`
	Result      string         `json:"result"`
	RequestID   string         `json:"request_id"`
	ClientIP    string         `json:"client_ip"`
	Details     map[string]any `json:"details"`
	CreatedAt   time.Time      `json:"created_at"`
}

type Page struct {
	Items []Event `json:"items"`
	Total int64   `json:"total"`
}

type Logger interface {
	Record(context.Context, Event) error
}

type Queryer interface {
	List(context.Context, []string, int) (Page, error)
}

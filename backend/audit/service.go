package audit

import (
	"context"
	"errors"
	"strings"
)

type Store interface {
	Record(context.Context, Event) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Record(ctx context.Context, event Event) error {
	if event.Result == "" {
		event.Result = "success"
	}
	if event.Details == nil {
		event.Details = map[string]any{}
	}
	event.Details = sanitizeDetails(event.Details)
	return s.store.Record(ctx, event)
}

func (s *Service) List(ctx context.Context, scopeIDs []string, limit int) (Page, error) {
	queryer, ok := s.store.(Queryer)
	if !ok {
		return Page{}, errors.New("audit query is unavailable")
	}
	return queryer.List(ctx, scopeIDs, limit)
}

func sanitizeDetails(details map[string]any) map[string]any {
	result := make(map[string]any, len(details))
	for key, value := range details {
		if sensitiveAuditKey(key) {
			result[key] = "[REDACTED]"
			continue
		}
		result[key] = sanitizeValue(value, 0)
	}
	return result
}

func sanitizeValue(value any, depth int) any {
	if depth >= 8 {
		return "[TRUNCATED]"
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			if sensitiveAuditKey(key) {
				result[key] = "[REDACTED]"
			} else {
				result[key] = sanitizeValue(child, depth+1)
			}
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = sanitizeValue(child, depth+1)
		}
		return result
	case string:
		if len(typed) > 2048 {
			return typed[:2048] + "[TRUNCATED]"
		}
	}
	return value
}

func sensitiveAuditKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"), ".", "_"))
	switch normalized {
	case "password", "secret", "token", "access_token", "refresh_token", "api_key", "apikey", "authorization", "cookie", "set_cookie", "private_key", "credential_value":
		return true
	default:
		return strings.HasSuffix(normalized, "_password") || strings.HasSuffix(normalized, "_secret") || strings.HasSuffix(normalized, "_token") || strings.HasSuffix(normalized, "_api_key")
	}
}

package audit

import (
	"context"
	"errors"
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
	return s.store.Record(ctx, event)
}

func (s *Service) List(ctx context.Context, scopeIDs []string, limit int) (Page, error) {
	queryer, ok := s.store.(Queryer)
	if !ok {
		return Page{}, errors.New("audit query is unavailable")
	}
	return queryer.List(ctx, scopeIDs, limit)
}

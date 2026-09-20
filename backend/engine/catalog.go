package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/authorization"
)

var EngineTags = []string{"general", "diagnosis", "inspection", "workflow"}

// DefaultCapabilityTerms is the initial vocabulary for the built-in Engine.
// The catalog table remains extensible so teams can add domain-specific terms.
var DefaultCapabilityTerms = []string{
	"智能体循环",
	"上下文编排",
	"工具调用",
	"工具网关",
	"技能编排",
	"专家路由",
	"结构化输出",
	"检索增强",
	"工作流编排",
	"流式事件",
}

type CatalogEngine struct {
	ID          string         `json:"id"`
	ScopeID     string         `json:"scope_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Icon        string         `json:"icon"`
	Config      map[string]any `json:"config"`
	Status      string         `json:"status"`
	Tags        []string       `json:"tags,omitempty"`
}

type EngineInput struct {
	ScopeID     string
	Name        string
	Description string
	Icon        string
	Config      map[string]any
	Status      string
}

type EnginePatch struct {
	Name        *string
	Description *string
	Icon        *string
	Config      *map[string]any
	Status      *string
}

type Binding struct {
	ScopeID  string `json:"scope_id"`
	EngineID string `json:"engine_id"`
	Tag      string `json:"tag"`
}

type Catalog interface {
	List(context.Context, string) ([]CatalogEngine, error)
	Get(context.Context, string) (CatalogEngine, error)
	Create(context.Context, EngineInput) (CatalogEngine, error)
	Update(context.Context, string, EnginePatch) (CatalogEngine, error)
	ListCapabilityTerms(context.Context) ([]string, error)
	AddCapabilityTerm(context.Context, string, string) (string, error)
	ListBindings(context.Context, string) ([]Binding, error)
	SetBinding(context.Context, string, string, string, string) (Binding, error)
	RemoveBinding(context.Context, string, string, string) error
}

type CatalogStore struct{ pool *pgxpool.Pool }

func NewCatalogStore(pool *pgxpool.Pool) *CatalogStore { return &CatalogStore{pool: pool} }

func (s *CatalogStore) List(ctx context.Context, scopeID string) ([]CatalogEngine, error) {
	const query = `
WITH RECURSIVE chain(id, depth) AS (
 SELECT id, 0 FROM scopes WHERE id = $1::uuid AND deleted_at IS NULL
 UNION ALL
 SELECT parent.id, chain.depth + 1 FROM scopes parent JOIN chain ON parent.id = (SELECT parent_scope_id FROM scopes WHERE scopes.id = chain.id)
 WHERE parent.deleted_at IS NULL
), visible AS (
 SELECT e.*, c.depth FROM engines e JOIN chain c ON c.id = e.scope_id
 WHERE e.deleted_at IS NULL
), effective_tags AS (
 SELECT tag, engine_id, row_number() OVER (PARTITION BY tag ORDER BY depth) AS rank
 FROM engine_scope_bindings b JOIN chain c ON c.id = b.scope_id
)
SELECT v.id::text, v.scope_id::text, v.name, v.description, v.icon, v.config, v.status,
       COALESCE((SELECT jsonb_agg(t.tag ORDER BY t.tag) FROM effective_tags t WHERE t.engine_id = v.id AND t.rank = 1), '[]'::jsonb)
FROM visible v ORDER BY v.name`
	rows, err := s.pool.Query(ctx, query, scopeID)
	if err != nil {
		return nil, fmt.Errorf("list engines: %w", err)
	}
	defer rows.Close()
	items := make([]CatalogEngine, 0)
	for rows.Next() {
		var item CatalogEngine
		var config, tags []byte
		if err := rows.Scan(&item.ID, &item.ScopeID, &item.Name, &item.Description, &item.Icon, &config, &item.Status, &tags); err != nil {
			return nil, fmt.Errorf("scan engine: %w", err)
		}
		if err := json.Unmarshal(config, &item.Config); err != nil {
			return nil, fmt.Errorf("decode engine config: %w", err)
		}
		if err := json.Unmarshal(tags, &item.Tags); err != nil {
			return nil, fmt.Errorf("decode engine tags: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *CatalogStore) Get(ctx context.Context, id string) (CatalogEngine, error) {
	var item CatalogEngine
	var config []byte
	err := s.pool.QueryRow(ctx, `SELECT id::text, scope_id::text, name, description, icon, config, status FROM engines WHERE id = $1::uuid AND deleted_at IS NULL`, id).Scan(&item.ID, &item.ScopeID, &item.Name, &item.Description, &item.Icon, &config, &item.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return CatalogEngine{}, errors.New("engine not found")
	}
	if err != nil {
		return CatalogEngine{}, fmt.Errorf("get engine: %w", err)
	}
	if err := json.Unmarshal(config, &item.Config); err != nil {
		return CatalogEngine{}, fmt.Errorf("decode engine config: %w", err)
	}
	return item, nil
}

func (s *CatalogStore) Create(ctx context.Context, input EngineInput) (CatalogEngine, error) {
	config, err := json.Marshal(input.Config)
	if err != nil {
		return CatalogEngine{}, err
	}
	var id string
	err = s.pool.QueryRow(ctx, `INSERT INTO engines (scope_id, name, description, icon, config, status) VALUES ($1::uuid,$2,$3,$4,$5,$6) RETURNING id::text`, input.ScopeID, input.Name, input.Description, input.Icon, config, input.Status).Scan(&id)
	if err != nil {
		return CatalogEngine{}, mapCatalogError(err)
	}
	return s.Get(ctx, id)
}

func (s *CatalogStore) Update(ctx context.Context, id string, patch EnginePatch) (CatalogEngine, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return CatalogEngine{}, err
	}
	if patch.Name != nil {
		current.Name = strings.TrimSpace(*patch.Name)
	}
	if patch.Description != nil {
		current.Description = strings.TrimSpace(*patch.Description)
	}
	if patch.Icon != nil {
		current.Icon = strings.TrimSpace(*patch.Icon)
	}
	if patch.Config != nil {
		current.Config = *patch.Config
	}
	if patch.Status != nil {
		current.Status = strings.TrimSpace(*patch.Status)
	}
	config, err := json.Marshal(current.Config)
	if err != nil {
		return CatalogEngine{}, err
	}
	_, err = s.pool.Exec(ctx, `UPDATE engines SET name=$2, description=$3, icon=$4, config=$5, status=$6, updated_at=now() WHERE id=$1::uuid AND deleted_at IS NULL`, id, current.Name, current.Description, current.Icon, config, current.Status)
	if err != nil {
		return CatalogEngine{}, mapCatalogError(err)
	}
	return s.Get(ctx, id)
}

func (s *CatalogStore) ListCapabilityTerms(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT term FROM engine_capability_terms ORDER BY lower(term), term`)
	if err != nil {
		return nil, fmt.Errorf("list engine capability terms: %w", err)
	}
	defer rows.Close()
	terms := make([]string, 0)
	for rows.Next() {
		var term string
		if err := rows.Scan(&term); err != nil {
			return nil, fmt.Errorf("scan engine capability term: %w", err)
		}
		terms = append(terms, term)
	}
	return terms, rows.Err()
}

func (s *CatalogStore) AddCapabilityTerm(ctx context.Context, actorID, term string) (string, error) {
	var saved string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO engine_capability_terms (term, created_by)
		VALUES ($1, NULLIF($2, '')::uuid)
		ON CONFLICT (lower(term)) DO UPDATE SET term = engine_capability_terms.term
		RETURNING term`, term, actorID).Scan(&saved)
	if err != nil {
		return "", mapCatalogError(err)
	}
	return saved, nil
}

func (s *CatalogStore) ListBindings(ctx context.Context, scopeID string) ([]Binding, error) {
	rows, err := s.pool.Query(ctx, `SELECT scope_id::text, engine_id::text, tag FROM engine_scope_bindings WHERE scope_id=$1::uuid ORDER BY tag`, scopeID)
	if err != nil {
		return nil, fmt.Errorf("list engine bindings: %w", err)
	}
	defer rows.Close()
	items := make([]Binding, 0)
	for rows.Next() {
		var item Binding
		if err := rows.Scan(&item.ScopeID, &item.EngineID, &item.Tag); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *CatalogStore) SetBinding(ctx context.Context, actorID, scopeID, tag, engineID string) (Binding, error) {
	var item Binding
	err := s.pool.QueryRow(ctx, `INSERT INTO engine_scope_bindings(scope_id, engine_id, tag, created_by) VALUES($1::uuid,$2::uuid,$3,NULLIF($4,'')::uuid) ON CONFLICT(scope_id,tag) DO UPDATE SET engine_id=EXCLUDED.engine_id, updated_at=now() RETURNING scope_id::text, engine_id::text, tag`, scopeID, engineID, tag, actorID).Scan(&item.ScopeID, &item.EngineID, &item.Tag)
	if err != nil {
		return Binding{}, mapCatalogError(err)
	}
	return item, nil
}

func (s *CatalogStore) RemoveBinding(ctx context.Context, scopeID, tag, engineID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM engine_scope_bindings WHERE scope_id=$1::uuid AND tag=$2 AND engine_id=$3::uuid`, scopeID, tag, engineID)
	if err != nil {
		return fmt.Errorf("remove engine binding: %w", err)
	}
	return nil
}

type CatalogService struct{ store Catalog }

func NewCatalogService(store Catalog) *CatalogService { return &CatalogService{store: store} }

func (s *CatalogService) List(ctx context.Context, scopeID string) ([]CatalogEngine, error) {
	if !allowsScope(ctx, scopeID) {
		return nil, authorization.ErrForbidden
	}
	return s.store.List(ctx, strings.TrimSpace(scopeID))
}
func (s *CatalogService) Get(ctx context.Context, id string) (CatalogEngine, error) {
	item, err := s.store.Get(ctx, strings.TrimSpace(id))
	if err != nil {
		return CatalogEngine{}, err
	}
	if !allowsScope(ctx, item.ScopeID) {
		return CatalogEngine{}, authorization.ErrForbidden
	}
	return item, nil
}
func (s *CatalogService) Create(ctx context.Context, input EngineInput) (CatalogEngine, error) {
	input.ScopeID, input.Name, input.Icon, input.Status = strings.TrimSpace(input.ScopeID), strings.TrimSpace(input.Name), strings.TrimSpace(input.Icon), strings.TrimSpace(input.Status)
	if input.Icon == "" {
		input.Icon = "lucide:Cpu"
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Config == nil {
		input.Config = map[string]any{}
	}
	if input.Name == "" || !allowsExactScope(ctx, input.ScopeID) {
		return CatalogEngine{}, authorization.ErrForbidden
	}
	return s.store.Create(ctx, input)
}
func (s *CatalogService) Update(ctx context.Context, id string, patch EnginePatch) (CatalogEngine, error) {
	item, err := s.store.Get(ctx, id)
	if err != nil {
		return CatalogEngine{}, err
	}
	if !allowsExactScope(ctx, item.ScopeID) {
		return CatalogEngine{}, authorization.ErrForbidden
	}
	return s.store.Update(ctx, id, patch)
}
func (s *CatalogService) ListCapabilityTerms(ctx context.Context) ([]string, error) {
	return s.store.ListCapabilityTerms(ctx)
}
func (s *CatalogService) AddCapabilityTerm(ctx context.Context, actorID, term string) (string, error) {
	term = strings.TrimSpace(term)
	if term == "" || len([]rune(term)) > 80 {
		return "", fmt.Errorf("engine capability term must be between 1 and 80 characters")
	}
	if filter, restricted := authorization.ScopeFilterFromContext(ctx); restricted && len(filter.ScopeIDs) == 0 {
		return "", authorization.ErrForbidden
	}
	return s.store.AddCapabilityTerm(ctx, strings.TrimSpace(actorID), term)
}
func (s *CatalogService) ListBindings(ctx context.Context, scopeID string) ([]Binding, error) {
	if !allowsScope(ctx, scopeID) {
		return nil, authorization.ErrForbidden
	}
	return s.store.ListBindings(ctx, scopeID)
}
func (s *CatalogService) SetBinding(ctx context.Context, actorID, scopeID, tag, engineID string) (Binding, error) {
	if !allowsExactScope(ctx, scopeID) || !validEngineTag(tag) {
		return Binding{}, authorization.ErrForbidden
	}
	item, err := s.store.Get(ctx, engineID)
	if err != nil {
		return Binding{}, err
	}
	if item.Status != "active" {
		return Binding{}, authorization.ErrForbidden
	}
	return s.store.SetBinding(ctx, actorID, scopeID, tag, engineID)
}
func (s *CatalogService) RemoveBinding(ctx context.Context, scopeID, tag, engineID string) error {
	if !allowsExactScope(ctx, scopeID) || !validEngineTag(tag) || strings.TrimSpace(engineID) == "" {
		return authorization.ErrForbidden
	}
	return s.store.RemoveBinding(ctx, strings.TrimSpace(scopeID), tag, strings.TrimSpace(engineID))
}

func validEngineTag(tag string) bool {
	for _, item := range EngineTags {
		if item == tag {
			return true
		}
	}
	return false
}
func allowsScope(ctx context.Context, scopeID string) bool {
	filter, restricted := authorization.ScopeFilterFromContext(ctx)
	return !restricted || filter.Allows(scopeID)
}
func allowsExactScope(ctx context.Context, scopeID string) bool {
	filter, restricted := authorization.ScopeFilterFromContext(ctx)
	return !restricted || filter.Allows(scopeID)
}
func mapCatalogError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23505" || pgErr.Code == "23514" || pgErr.Code == "23503") {
		return fmt.Errorf("invalid engine configuration: %w", err)
	}
	return err
}

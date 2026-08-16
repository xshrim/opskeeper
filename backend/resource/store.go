package resource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/authorization"
)

type store struct{ pool *pgxpool.Pool }

var _ Store = (*store)(nil)

func NewStore(pool *pgxpool.Pool) Store { return &store{pool: pool} }

const resourceSelect = `
SELECT resource.id::text, resource.scope_id::text, resource.kind, resource.name,
       resource.schema_version, resource.external_uid, resource.source_resource_id, resource.labels,
       resource.config, resource.status, resource.credential_id::text,
       resource.created_at, resource.updated_at
  FROM resources resource
 WHERE resource.deleted_at IS NULL`

func (s *store) Create(ctx context.Context, input CreateInput) (Resource, error) {
	labels, err := json.Marshal(input.Labels)
	if err != nil {
		return Resource{}, fmt.Errorf("encode resource labels: %w", err)
	}
	config, err := json.Marshal(input.Config)
	if err != nil {
		return Resource{}, fmt.Errorf("encode resource config: %w", err)
	}
	var id string
	err = s.pool.QueryRow(ctx, `
		INSERT INTO resources (scope_id, kind, schema_version, name, external_uid, source_resource_id, labels, config, status, credential_id)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10::uuid)
		RETURNING id::text`, input.ScopeID, input.Kind, input.SchemaVersion, input.Name, input.ExternalUID, input.SourceResourceID, labels, config, input.Status, nullableString(input.CredentialID)).Scan(&id)
	if err != nil {
		return Resource{}, mapStoreError(err)
	}
	return s.Get(ctx, id)
}

func (s *store) UpsertImported(ctx context.Context, input ImportedInput) (Resource, error) {
	labels, err := json.Marshal(input.Labels)
	if err != nil {
		return Resource{}, fmt.Errorf("encode imported resource labels: %w", err)
	}
	config, err := json.Marshal(input.Config)
	if err != nil {
		return Resource{}, fmt.Errorf("encode imported resource config: %w", err)
	}
	var id string
	err = s.pool.QueryRow(ctx, `
		INSERT INTO resources (scope_id, kind, schema_version, name, external_uid, source_resource_id, labels, config, status, credential_id)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10::uuid)
		ON CONFLICT (scope_id, kind, external_uid, source_resource_id)
		WHERE deleted_at IS NULL AND external_uid <> '' AND source_resource_id <> ''
		DO UPDATE SET schema_version = EXCLUDED.schema_version, name = EXCLUDED.name,
		              labels = EXCLUDED.labels, config = EXCLUDED.config,
		              status = EXCLUDED.status, credential_id = EXCLUDED.credential_id,
		              updated_at = now(), deleted_at = NULL
		RETURNING id::text`, input.ScopeID, input.Kind, input.SchemaVersion, input.Name,
		input.ExternalUID, input.SourceResourceID, labels, config, input.Status,
		nullableString(input.CredentialID)).Scan(&id)
	if err != nil {
		return Resource{}, mapStoreError(err)
	}
	return s.Get(ctx, id)
}

func (s *store) List(ctx context.Context, pagination Pagination, kind string, labels map[string]string) (Page[Resource], error) {
	base := " FROM resources resource WHERE resource.deleted_at IS NULL"
	args := make([]any, 0, 3)
	base, args = appendResourceVisibility(base, args, ctx, "resource")
	if kind != "" {
		args = append(args, kind)
		base += " AND resource.kind = $" + strconv.Itoa(len(args))
	}
	if len(labels) > 0 {
		encoded, err := json.Marshal(labels)
		if err != nil {
			return Page[Resource]{}, fmt.Errorf("encode label filter: %w", err)
		}
		args = append(args, encoded)
		base += " AND resource.labels @> $" + strconv.Itoa(len(args)) + "::jsonb"
	}
	var total int64
	if err := s.pool.QueryRow(ctx, "SELECT count(*)"+base, args...).Scan(&total); err != nil {
		return Page[Resource]{}, fmt.Errorf("count resources: %w", err)
	}
	queryArgs := append([]any{}, args...)
	limitPosition := len(queryArgs) + 1
	offsetPosition := len(queryArgs) + 2
	queryArgs = append(queryArgs, pagination.PageSize, pagination.Offset())
	rows, err := s.pool.Query(ctx, "SELECT resource.id::text, resource.scope_id::text, resource.kind, resource.name, resource.schema_version, resource.external_uid, resource.source_resource_id, resource.labels, resource.config, resource.status, resource.credential_id::text, resource.created_at, resource.updated_at"+base+" ORDER BY resource.created_at DESC, resource.id LIMIT $"+strconv.Itoa(limitPosition)+" OFFSET $"+strconv.Itoa(offsetPosition), queryArgs...)
	if err != nil {
		return Page[Resource]{}, fmt.Errorf("list resources: %w", err)
	}
	defer rows.Close()
	items := make([]Resource, 0)
	for rows.Next() {
		item, err := scanResource(rows)
		if err != nil {
			return Page[Resource]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page[Resource]{}, fmt.Errorf("iterate resources: %w", err)
	}
	return Page[Resource]{Items: items, Page: pagination.Page, PageSize: pagination.PageSize, Total: total}, nil
}

func (s *store) Get(ctx context.Context, id string) (Resource, error) {
	query, args := visibleQuery(resourceSelect+" AND resource.id = $1::uuid", "resource", ctx, id)
	return scanResource(s.pool.QueryRow(ctx, query, args...))
}

func (s *store) Update(ctx context.Context, id string, input UpdateInput) (Resource, error) {
	set := []string{"updated_at = now()"}
	args := []any{id}
	appendValue := func(column string, value any, cast string) {
		args = append(args, value)
		set = append(set, column+" = $"+strconv.Itoa(len(args))+cast)
	}
	if input.ScopeID != nil {
		appendValue("scope_id", *input.ScopeID, "::uuid")
	}
	if input.Name != nil {
		appendValue("name", *input.Name, "")
	}
	if input.ExternalUID != nil {
		appendValue("external_uid", strings.TrimSpace(*input.ExternalUID), "")
	}
	if input.SourceResourceID != nil {
		appendValue("source_resource_id", strings.TrimSpace(*input.SourceResourceID), "")
	}
	if input.Labels != nil {
		encoded, err := json.Marshal(*input.Labels)
		if err != nil {
			return Resource{}, fmt.Errorf("encode resource labels: %w", err)
		}
		appendValue("labels", encoded, "::jsonb")
	}
	if input.Config != nil {
		encoded, err := json.Marshal(*input.Config)
		if err != nil {
			return Resource{}, fmt.Errorf("encode resource config: %w", err)
		}
		appendValue("config", encoded, "::jsonb")
	}
	if input.Status != nil {
		appendValue("status", *input.Status, "")
	}
	if input.CredentialID != nil {
		var value any
		if *input.CredentialID != nil {
			value = **input.CredentialID
		}
		appendValue("credential_id", value, "::uuid")
	}
	query, queryArgs := exactResourceQuery("UPDATE resources resource SET "+strings.Join(set, ", ")+" WHERE resource.id = $1::uuid AND resource.deleted_at IS NULL", ctx, args...)
	command, err := s.pool.Exec(ctx, query, queryArgs...)
	if err != nil {
		return Resource{}, mapStoreError(err)
	}
	if command.RowsAffected() != 1 {
		return Resource{}, ErrNotFound
	}
	return s.Get(ctx, id)
}

func (s *store) Delete(ctx context.Context, id string) error {
	query, args := exactResourceQuery("UPDATE resources resource SET deleted_at = now(), updated_at = now() WHERE resource.id = $1::uuid AND resource.deleted_at IS NULL", ctx, id)
	command, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return mapStoreError(err)
	}
	if command.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *store) GetSchema(ctx context.Context, kind string, version int) (Schema, error) {
	query := `SELECT id::text, kind, version, schema, status, display_name, description, icon, created_at FROM resource_schemas WHERE kind = $1 AND status = 'active'`
	args := []any{kind}
	if version > 0 {
		query += " AND version = $2"
		args = append(args, version)
	} else {
		query += " ORDER BY version DESC LIMIT 1"
	}
	var item Schema
	var raw []byte
	if err := s.pool.QueryRow(ctx, query, args...).Scan(&item.ID, &item.Kind, &item.Version, &raw, &item.Status, &item.DisplayName, &item.Description, &item.Icon, &item.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Schema{}, ErrSchemaNotFound
		}
		return Schema{}, fmt.Errorf("get resource schema: %w", err)
	}
	if err := json.Unmarshal(raw, &item.Schema); err != nil {
		return Schema{}, fmt.Errorf("decode resource schema: %w", err)
	}
	return item, nil
}

func (s *store) ListSchemas(ctx context.Context) ([]Schema, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT ON (kind) id::text, kind, version, schema, status, display_name, description, icon, created_at FROM resource_schemas WHERE status = 'active' ORDER BY kind, version DESC`)
	if err != nil {
		return nil, fmt.Errorf("list resource schemas: %w", err)
	}
	defer rows.Close()
	items := make([]Schema, 0)
	for rows.Next() {
		var item Schema
		var raw []byte
		if err := rows.Scan(&item.ID, &item.Kind, &item.Version, &raw, &item.Status, &item.DisplayName, &item.Description, &item.Icon, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan resource schema: %w", err)
		}
		if err := json.Unmarshal(raw, &item.Schema); err != nil {
			return nil, fmt.Errorf("decode resource schema: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) CreateRelation(ctx context.Context, input CreateRelationInput, actorID string) (Relation, error) {
	attributes, err := json.Marshal(input.Attributes)
	if err != nil {
		return Relation{}, fmt.Errorf("encode relation attributes: %w", err)
	}
	var item Relation
	var createdBy *string
	if actorID != "" {
		createdBy = &actorID
	}
	err = s.pool.QueryRow(ctx, `
        INSERT INTO resource_relations (source_resource_id, target_resource_id, relation_type, attributes, discovery_source, confidence, confirmed, created_by)
        VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8::uuid)
        RETURNING id::text, source_resource_id::text, target_resource_id::text, relation_type, attributes, discovery_source, confidence, confirmed, created_by::text, created_at`, input.SourceResourceID, input.TargetResourceID, input.RelationType, attributes, input.DiscoverySource, input.Confidence, input.Confirmed, createdBy).Scan(&item.ID, &item.SourceResourceID, &item.TargetResourceID, &item.RelationType, &attributes, &item.DiscoverySource, &item.Confidence, &item.Confirmed, &item.CreatedBy, &item.CreatedAt)
	if err != nil {
		return Relation{}, mapStoreError(err)
	}
	if err := json.Unmarshal(attributes, &item.Attributes); err != nil {
		return Relation{}, fmt.Errorf("decode relation attributes: %w", err)
	}
	return item, nil
}

func (s *store) ListRelations(ctx context.Context, resourceID string) ([]Relation, error) {
	query, args := visibleRelationListQuery(`
SELECT relation.id::text, relation.source_resource_id::text, relation.target_resource_id::text,
       relation.relation_type, relation.attributes, relation.discovery_source, relation.confidence,
       relation.confirmed, relation.created_by::text, relation.created_at
  FROM resource_relations relation
  JOIN resources source_resource ON source_resource.id = relation.source_resource_id
  JOIN resources target_resource ON target_resource.id = relation.target_resource_id
 WHERE source_resource.deleted_at IS NULL AND target_resource.deleted_at IS NULL AND (relation.source_resource_id = $1::uuid OR relation.target_resource_id = $1::uuid)`, []any{resourceID}, ctx)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list resource relations: %w", err)
	}
	defer rows.Close()
	items := make([]Relation, 0)
	for rows.Next() {
		item, err := scanRelation(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) DeleteRelation(ctx context.Context, resourceID, relationID string) error {
	query, args := visibleRelationQuery(`DELETE FROM resource_relations relation USING resources source_resource WHERE relation.source_resource_id = source_resource.id AND source_resource.deleted_at IS NULL AND relation.source_resource_id = $1::uuid AND relation.id = $2::uuid`, "source_resource", ctx, resourceID, relationID)
	command, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return mapStoreError(err)
	}
	if command.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *store) Topology(ctx context.Context, resourceID string, depth, maxNodes int) ([]TopologyNode, error) {
	query := `
WITH RECURSIVE walk(id, depth) AS (
    SELECT $1::uuid, 0
    UNION
    SELECT CASE WHEN relation.source_resource_id = walk.id THEN relation.target_resource_id ELSE relation.source_resource_id END, walk.depth + 1
      FROM walk
      JOIN resource_relations relation ON relation.source_resource_id = walk.id OR relation.target_resource_id = walk.id
     WHERE walk.depth < $2
), selected AS (
    SELECT id, min(depth) AS depth FROM walk GROUP BY id
)
SELECT resource.id::text, resource.scope_id::text, resource.kind, resource.name, resource.schema_version, resource.external_uid,
       resource.source_resource_id, resource.labels, resource.config, resource.status,
       resource.credential_id::text, resource.created_at, resource.updated_at, selected.depth
  FROM selected JOIN resources resource ON resource.id = selected.id
 WHERE resource.deleted_at IS NULL`
	args := []any{resourceID, depth}
	query, args = appendResourceVisibility(query, args, ctx, "resource")
	query += " ORDER BY selected.depth, resource.id LIMIT $" + strconv.Itoa(len(args)+1)
	args = append(args, maxNodes)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query resource topology: %w", err)
	}
	defer rows.Close()
	items := make([]TopologyNode, 0)
	for rows.Next() {
		item, depthValue, err := scanTopologyNode(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, TopologyNode{Resource: item, Depth: depthValue})
	}
	return items, rows.Err()
}

func (s *store) SetDefault(ctx context.Context, scopeID, key, resourceID string) (Default, error) {
	var item Default
	err := s.pool.QueryRow(ctx, `
        INSERT INTO scope_defaults (scope_id, default_key, resource_id)
        VALUES ($1::uuid, $2, $3::uuid)
        ON CONFLICT (scope_id, default_key) DO UPDATE SET resource_id = EXCLUDED.resource_id, updated_at = now()
        RETURNING scope_id::text, default_key, resource_id::text, created_at, updated_at`, scopeID, key, resourceID).
		Scan(&item.ScopeID, &item.DefaultKey, &item.ResourceID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Default{}, mapStoreError(err)
	}
	return item, nil
}

func (s *store) ResolveDefault(ctx context.Context, scopeID, key string) (Resource, error) {
	query := `
WITH RECURSIVE chain(id, depth) AS (
    SELECT $1::uuid, 0
    UNION ALL
    SELECT scope.parent_scope_id, chain.depth + 1
      FROM scopes scope JOIN chain ON chain.id = scope.id
     WHERE scope.parent_scope_id IS NOT NULL
)
SELECT resource.id::text, resource.scope_id::text, resource.kind, resource.name, resource.schema_version,
       resource.external_uid, resource.source_resource_id, resource.labels, resource.config,
       resource.status, resource.credential_id::text, resource.created_at, resource.updated_at
  FROM chain
  JOIN scope_defaults defaults ON defaults.scope_id = chain.id AND defaults.default_key = $2
  JOIN resources resource ON resource.id = defaults.resource_id
 WHERE resource.deleted_at IS NULL
 ORDER BY chain.depth
 LIMIT 1`
	return scanResource(s.pool.QueryRow(ctx, query, scopeID, key))
}

func scanTopologyNode(row rowScanner) (Resource, int, error) {
	var item Resource
	var labelsRaw, configRaw []byte
	var depth int
	if err := row.Scan(&item.ID, &item.ScopeID, &item.Kind, &item.Name, &item.SchemaVersion, &item.ExternalUID, &item.SourceResourceID, &labelsRaw, &configRaw, &item.Status, &item.CredentialID, &item.CreatedAt, &item.UpdatedAt, &depth); err != nil {
		return Resource{}, 0, mapStoreError(err)
	}
	if err := json.Unmarshal(labelsRaw, &item.Labels); err != nil {
		return Resource{}, 0, fmt.Errorf("decode topology labels: %w", err)
	}
	if err := json.Unmarshal(configRaw, &item.Config); err != nil {
		return Resource{}, 0, fmt.Errorf("decode topology config: %w", err)
	}
	return item, depth, nil
}

type rowScanner interface{ Scan(...any) error }

func scanResource(row rowScanner) (Resource, error) {
	var item Resource
	var labelsRaw, configRaw []byte
	if err := row.Scan(&item.ID, &item.ScopeID, &item.Kind, &item.Name, &item.SchemaVersion, &item.ExternalUID, &item.SourceResourceID, &labelsRaw, &configRaw, &item.Status, &item.CredentialID, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return Resource{}, mapStoreError(err)
	}
	if err := json.Unmarshal(labelsRaw, &item.Labels); err != nil {
		return Resource{}, fmt.Errorf("decode resource labels: %w", err)
	}
	if err := json.Unmarshal(configRaw, &item.Config); err != nil {
		return Resource{}, fmt.Errorf("decode resource config: %w", err)
	}
	return item, nil
}

func scanRelation(row rowScanner) (Relation, error) {
	var item Relation
	var attributes []byte
	if err := row.Scan(&item.ID, &item.SourceResourceID, &item.TargetResourceID, &item.RelationType, &attributes, &item.DiscoverySource, &item.Confidence, &item.Confirmed, &item.CreatedBy, &item.CreatedAt); err != nil {
		return Relation{}, mapStoreError(err)
	}
	if err := json.Unmarshal(attributes, &item.Attributes); err != nil {
		return Relation{}, fmt.Errorf("decode relation attributes: %w", err)
	}
	return item, nil
}

func visibleQuery(query, alias string, ctx context.Context, args ...any) (string, []any) {
	return appendResourceVisibility(query, args, ctx, alias)
}

func exactResourceQuery(query string, ctx context.Context, args ...any) (string, []any) {
	scopeIDs, resourceIDs, restricted := accessFilter(ctx)
	if !restricted {
		return query, args
	}
	scopePosition := len(args) + 1
	resourcePosition := len(args) + 2
	query += " AND (resource.scope_id = ANY($" + strconv.Itoa(scopePosition) + "::uuid[]) OR resource.id = ANY($" + strconv.Itoa(resourcePosition) + "::uuid[]))"
	return query, append(args, scopeIDs, resourceIDs)
}

func appendResourceVisibility(query string, args []any, ctx context.Context, alias string) (string, []any) {
	scopeIDs, resourceIDs, restricted := accessFilter(ctx)
	if !restricted {
		return query, args
	}
	scopePosition := len(args) + 1
	resourcePosition := len(args) + 2
	clause := fmt.Sprintf(` AND (%s.id = ANY($%d::uuid[]) OR %s.scope_id = ANY($%d::uuid[]) OR EXISTS (
        WITH RECURSIVE ancestors(id) AS (
            SELECT unnest($%d::uuid[])
            UNION
            SELECT scope.parent_scope_id FROM scopes scope JOIN ancestors ON ancestors.id = scope.id WHERE scope.parent_scope_id IS NOT NULL
        ) SELECT 1 FROM ancestors WHERE ancestors.id = %s.scope_id
    ))`, alias, resourcePosition, alias, scopePosition, scopePosition, alias)
	return query + clause, append(args, scopeIDs, resourceIDs)
}

func visibleRelationQuery(query, sourceAlias string, ctx context.Context, args ...any) (string, []any) {
	scopeIDs, resourceIDs, restricted := accessFilter(ctx)
	if !restricted {
		return query, args
	}
	scopePosition := len(args) + 1
	resourcePosition := len(args) + 2
	clause := fmt.Sprintf(" AND (%s.id = ANY($%d::uuid[]) OR %s.scope_id = ANY($%d::uuid[]) OR EXISTS (WITH RECURSIVE ancestors(id) AS (SELECT unnest($%d::uuid[]) UNION SELECT scope.parent_scope_id FROM scopes scope JOIN ancestors ON ancestors.id = scope.id WHERE scope.parent_scope_id IS NOT NULL) SELECT 1 FROM ancestors WHERE ancestors.id = %s.scope_id))", sourceAlias, resourcePosition, sourceAlias, scopePosition, scopePosition, sourceAlias)
	return query + clause, append(args, scopeIDs, resourceIDs)
}

func visibleRelationListQuery(query string, args []any, ctx context.Context) (string, []any) {
	scopeIDs, resourceIDs, restricted := accessFilter(ctx)
	if !restricted {
		return query, args
	}
	scopePosition := len(args) + 1
	resourcePosition := len(args) + 2
	clause := fmt.Sprintf(` AND ((source_resource.id = ANY($%d::uuid[]) OR source_resource.scope_id = ANY($%d::uuid[]) OR EXISTS (
        WITH RECURSIVE ancestors(id) AS (
            SELECT unnest($%d::uuid[])
            UNION
            SELECT scope.parent_scope_id FROM scopes scope JOIN ancestors ON ancestors.id = scope.id WHERE scope.parent_scope_id IS NOT NULL
        ) SELECT 1 FROM ancestors WHERE ancestors.id = source_resource.scope_id
	)) OR (target_resource.id = ANY($%d::uuid[]) OR target_resource.scope_id = ANY($%d::uuid[]) OR EXISTS (
        WITH RECURSIVE ancestors(id) AS (
            SELECT unnest($%d::uuid[])
            UNION
            SELECT scope.parent_scope_id FROM scopes scope JOIN ancestors ON ancestors.id = scope.id WHERE scope.parent_scope_id IS NOT NULL
        ) SELECT 1 FROM ancestors WHERE ancestors.id = target_resource.scope_id
	)))`, resourcePosition, scopePosition, scopePosition, resourcePosition, scopePosition, scopePosition)
	return query + clause, append(args, scopeIDs, resourceIDs)
}

func accessFilter(ctx context.Context) ([]string, []string, bool) {
	if filter, ok := authorization.ResourceFilterFromContext(ctx); ok {
		return filter.ScopeIDs, filter.ResourceIDs, true
	}
	if filter, ok := authorization.ScopeFilterFromContext(ctx); ok {
		return filter.ScopeIDs, nil, true
	}
	return nil, nil, false
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func mapStoreError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrConflict
		case "23503":
			return ErrNotFound
		case "23514":
			if strings.Contains(pgErr.Message, "cycle") {
				return ErrRelationCycle
			}
			return invalid("resource violates a scope or relation constraint")
		}
	}
	return err
}

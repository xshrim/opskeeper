package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/secret"
)

type ProviderReader interface {
	GetProvider(context.Context, string) (Provider, error)
	ListProviders(context.Context, string) ([]Provider, error)
	RevealProviderSecret(context.Context, string) ([]byte, error)
}

type ProviderCatalogWriter interface {
	CreateProvider(context.Context, ProviderInput) (Provider, error)
	UpdateProvider(context.Context, string, ProviderPatch) (Provider, error)
	DeleteProvider(context.Context, string) error
}

type ProviderConnectionTestWriter interface {
	RecordConnectionTest(context.Context, string, string, string, int64, time.Time) error
}

type Store interface {
	SetBinding(context.Context, ScopeProviderBinding, string) (ScopeProviderBinding, error)
	RemoveBinding(context.Context, string, Purpose) error
	ListBindings(context.Context, string) ([]ScopeProviderBinding, error)
	ResolveBinding(context.Context, string, Purpose) (ScopeProviderBinding, error)
}

type store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) Store { return &store{pool: pool} }

// ProviderStore is the independent provider catalog. It intentionally lives
// beside the model client implementation so callers use the dedicated catalog.
type ProviderStore struct {
	pool      *pgxpool.Pool
	decryptor secret.Decryptor
	encryptor secret.Encryptor
}

func NewProviderStore(pool *pgxpool.Pool, decryptors ...secret.Decryptor) *ProviderStore {
	var decryptor secret.Decryptor
	if len(decryptors) > 0 {
		decryptor = decryptors[0]
	}
	encryptor, _ := decryptor.(secret.Encryptor)
	return &ProviderStore{pool: pool, decryptor: decryptor, encryptor: encryptor}
}

func (s *ProviderStore) GetProvider(ctx context.Context, id string) (Provider, error) {
	var item Provider
	var encoded []byte
	var testStatus, testMessage string
	var testLatency int64
	var testAt *time.Time
	err := s.pool.QueryRow(ctx, `SELECT id::text, scope_id::text, name, status, config, last_connection_test_status, last_connection_test_message, last_connection_test_latency_ms, last_connection_test_at FROM providers WHERE id = $1::uuid AND deleted_at IS NULL`, id).Scan(&item.ID, &item.ScopeID, &item.Name, &item.Status, &encoded, &testStatus, &testMessage, &testLatency, &testAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Provider{}, ErrNotFound
	}
	if err != nil {
		return Provider{}, fmt.Errorf("get provider: %w", err)
	}
	if err := json.Unmarshal(encoded, &item.Config); err != nil {
		return Provider{}, fmt.Errorf("decode provider config: %w", err)
	}
	normalizeProviderConfig(&item.Config)
	if testAt != nil {
		item.LastConnectionTest = &ProviderConnectionTest{Status: testStatus, Message: testMessage, LatencyMS: testLatency, CheckedAt: *testAt}
	}
	return item, nil
}

func (s *ProviderStore) ListProviders(ctx context.Context, scopeID string) ([]Provider, error) {
	rows, err := s.pool.Query(ctx, `
WITH RECURSIVE chain(id, depth) AS (
 SELECT id, 0 FROM scopes WHERE id = $1::uuid AND deleted_at IS NULL
 UNION ALL
 SELECT parent.id, chain.depth + 1 FROM scopes parent JOIN chain ON parent.id = (SELECT parent_scope_id FROM scopes WHERE scopes.id = chain.id)
 WHERE parent.deleted_at IS NULL
), visible AS (
 SELECT p.*, c.depth FROM providers p JOIN chain c ON c.id = p.scope_id WHERE p.deleted_at IS NULL
), effective_tags AS (
 SELECT tag, provider_id, row_number() OVER (PARTITION BY tag ORDER BY depth) AS rank
 FROM provider_scope_bindings b JOIN chain c ON c.id = b.scope_id
)
SELECT v.id::text, v.scope_id::text, v.name, v.status, v.config,
       v.last_connection_test_status, v.last_connection_test_message,
       v.last_connection_test_latency_ms, v.last_connection_test_at,
       COALESCE((SELECT jsonb_agg(t.tag ORDER BY t.tag) FROM effective_tags t WHERE t.provider_id = v.id AND t.rank = 1), '[]'::jsonb)
FROM visible v ORDER BY v.name`, scopeID)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()
	items := make([]Provider, 0)
	for rows.Next() {
		var item Provider
		var encoded, tags []byte
		var testStatus, testMessage string
		var testLatency int64
		var testAt *time.Time
		if err := rows.Scan(&item.ID, &item.ScopeID, &item.Name, &item.Status, &encoded, &testStatus, &testMessage, &testLatency, &testAt, &tags); err != nil {
			return nil, fmt.Errorf("scan provider: %w", err)
		}
		if err := json.Unmarshal(encoded, &item.Config); err != nil {
			return nil, fmt.Errorf("decode provider config: %w", err)
		}
		if err := json.Unmarshal(tags, &item.Tags); err != nil {
			return nil, fmt.Errorf("decode provider tags: %w", err)
		}
		normalizeProviderConfig(&item.Config)
		if testAt != nil {
			item.LastConnectionTest = &ProviderConnectionTest{Status: testStatus, Message: testMessage, LatencyMS: testLatency, CheckedAt: *testAt}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *ProviderStore) RecordConnectionTest(ctx context.Context, providerID, status, message string, latencyMS int64, checkedAt time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE providers SET last_connection_test_status=$2, last_connection_test_message=$3, last_connection_test_latency_ms=$4, last_connection_test_at=$5, updated_at=updated_at WHERE id=$1::uuid AND deleted_at IS NULL`, providerID, status, message, latencyMS, checkedAt)
	if err != nil {
		return fmt.Errorf("record provider connection test: %w", err)
	}
	return nil
}

func (s *ProviderStore) CreateProvider(ctx context.Context, input ProviderInput) (Provider, error) {
	encoded, err := json.Marshal(input.Config)
	if err != nil {
		return Provider{}, err
	}
	var ciphertext []byte
	var keyVersion string
	if input.APIKey != nil && strings.TrimSpace(*input.APIKey) != "" {
		if s.encryptor == nil {
			return Provider{}, fmt.Errorf("provider credential encryptor is unavailable")
		}
		payload, _ := json.Marshal(map[string]string{"token": strings.TrimSpace(*input.APIKey)})
		ciphertext, keyVersion, err = s.encryptor.Encrypt(payload)
		if err != nil {
			return Provider{}, fmt.Errorf("encrypt provider credential: %w", err)
		}
	}
	var id string
	// Migration 0060 created providers.id without a database default. Generate
	// the UUID in the insert so creates work against both existing and fresh
	// databases without requiring a rewrite of an already-applied migration.
	err = s.pool.QueryRow(ctx, `INSERT INTO providers(id,scope_id,name,config,status,credential_ciphertext,credential_key_version) VALUES(gen_random_uuid(),$1::uuid,$2,$3,$4,$5,$6) RETURNING id::text`, input.ScopeID, input.Name, encoded, input.Status, ciphertext, keyVersion).Scan(&id)
	if err != nil {
		return Provider{}, mapStoreError(err)
	}
	return s.GetProvider(ctx, id)
}

func (s *ProviderStore) UpdateProvider(ctx context.Context, id string, patch ProviderPatch) (Provider, error) {
	current, err := s.GetProvider(ctx, id)
	if err != nil {
		return Provider{}, err
	}
	if patch.Name != nil {
		current.Name = strings.TrimSpace(*patch.Name)
	}
	if patch.Status != nil {
		current.Status = strings.TrimSpace(*patch.Status)
	}
	if patch.Config != nil {
		current.Config = *patch.Config
	}
	encoded, err := json.Marshal(current.Config)
	if err != nil {
		return Provider{}, err
	}
	if patch.APIKey != nil && strings.TrimSpace(*patch.APIKey) != "" {
		if s.encryptor == nil {
			return Provider{}, fmt.Errorf("provider credential encryptor is unavailable")
		}
		payload, _ := json.Marshal(map[string]string{"token": strings.TrimSpace(*patch.APIKey)})
		ciphertext, keyVersion, encryptErr := s.encryptor.Encrypt(payload)
		if encryptErr != nil {
			return Provider{}, encryptErr
		}
		_, err = s.pool.Exec(ctx, `UPDATE providers SET name=$2,status=$3,config=$4,credential_ciphertext=$5,credential_key_version=$6,updated_at=now() WHERE id=$1::uuid AND deleted_at IS NULL`, id, current.Name, current.Status, encoded, ciphertext, keyVersion)
	} else {
		_, err = s.pool.Exec(ctx, `UPDATE providers SET name=$2,status=$3,config=$4,updated_at=now() WHERE id=$1::uuid AND deleted_at IS NULL`, id, current.Name, current.Status, encoded)
	}
	if err != nil {
		return Provider{}, mapStoreError(err)
	}
	return s.GetProvider(ctx, id)
}

func (s *ProviderStore) DeleteProvider(ctx context.Context, id string) error {
	result, err := s.pool.Exec(ctx, `UPDATE providers SET deleted_at=now(), updated_at=now() WHERE id=$1::uuid AND deleted_at IS NULL`, id)
	if err != nil {
		return mapStoreError(err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func normalizeProviderConfig(config *ProviderConfig) {
	if config.Icon == "" {
		config.Icon = "lucide:Bot"
	}
	for i := range config.Models {
		config.Models[i].Tags = normalizeModelTags(config.Models[i].Tags)
		config.Models[i].Capabilities = normalizeModelTags(config.Models[i].Capabilities)
		if len(config.Models[i].Tags) == 0 {
			config.Models[i].Tags = append([]string(nil), config.Models[i].Capabilities...)
		}
		if len(config.Models[i].Capabilities) == 0 {
			config.Models[i].Capabilities = append([]string(nil), config.Models[i].Tags...)
		}
	}
}

func normalizeModelTags(tags []string) []string {
	filtered := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag != "image" {
			filtered = append(filtered, tag)
		}
	}
	return filtered
}

func (s *ProviderStore) RevealProviderSecret(ctx context.Context, id string) ([]byte, error) {
	var ciphertext []byte
	var version string
	err := s.pool.QueryRow(ctx, `SELECT credential_ciphertext, credential_key_version FROM providers WHERE id = $1::uuid AND deleted_at IS NULL AND credential_ciphertext IS NOT NULL`, id).Scan(&ciphertext, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read provider secret: %w", err)
	}
	if s.decryptor == nil {
		return nil, fmt.Errorf("provider secret decryptor is unavailable")
	}
	return s.decryptor.Decrypt(ciphertext, version)
}

func (s *store) SetBinding(ctx context.Context, input ScopeProviderBinding, actorID string) (ScopeProviderBinding, error) {
	var item ScopeProviderBinding
	err := s.pool.QueryRow(ctx, `
		INSERT INTO provider_scope_bindings (scope_id, provider_id, tag, created_by)
		VALUES ($1::uuid, $2::uuid, $3, NULLIF($4, '')::uuid)
		ON CONFLICT (scope_id, tag) DO UPDATE SET
			provider_id = EXCLUDED.provider_id,
			updated_at = now()
		RETURNING scope_id::text, provider_id::text, tag, created_at, updated_at`,
		input.ScopeID, input.ProviderID, input.Tag, actorID).Scan(
		&item.ScopeID, &item.ProviderID, &item.Tag, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return ScopeProviderBinding{}, mapStoreError(err)
	}
	return item, nil
}

func (s *store) RemoveBinding(ctx context.Context, scopeID string, purpose Purpose) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM provider_scope_bindings WHERE scope_id = $1::uuid AND tag = $2`, scopeID, purpose)
	if err != nil {
		return fmt.Errorf("remove provider binding: %w", err)
	}
	return nil
}

func (s *store) ListBindings(ctx context.Context, scopeID string) ([]ScopeProviderBinding, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT scope_id::text, provider_id::text, tag, created_at, updated_at
		FROM provider_scope_bindings WHERE scope_id = $1::uuid ORDER BY tag`, scopeID)
	if err != nil {
		return nil, fmt.Errorf("list provider bindings: %w", err)
	}
	defer rows.Close()
	items := make([]ScopeProviderBinding, 0)
	for rows.Next() {
		var item ScopeProviderBinding
		if err := rows.Scan(&item.ScopeID, &item.ProviderID, &item.Tag, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan provider binding: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) ResolveBinding(ctx context.Context, scopeID string, purpose Purpose) (ScopeProviderBinding, error) {
	var item ScopeProviderBinding
	err := s.pool.QueryRow(ctx, `
		WITH RECURSIVE chain(id, parent_scope_id, depth) AS (
			SELECT id, parent_scope_id, 0 FROM scopes
			 WHERE id = $1::uuid AND deleted_at IS NULL AND status = 'active'
			UNION ALL
			SELECT parent.id, parent.parent_scope_id, chain.depth + 1
			  FROM scopes parent JOIN chain ON parent.id = chain.parent_scope_id
			 WHERE parent.deleted_at IS NULL AND parent.status = 'active'
		)
		SELECT bindings.scope_id::text, bindings.provider_id::text,
		       bindings.tag, bindings.created_at, bindings.updated_at
		  FROM chain JOIN provider_scope_bindings bindings ON bindings.scope_id = chain.id
		 WHERE bindings.tag IN ($2, 'general')
		 ORDER BY chain.depth ASC,
		          CASE WHEN bindings.tag = $2 THEN 0 ELSE 1 END
		 LIMIT 1`, scopeID, purpose).Scan(
		&item.ScopeID, &item.ProviderID, &item.Tag, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ScopeProviderBinding{}, ErrNotFound
	}
	if err != nil {
		return ScopeProviderBinding{}, fmt.Errorf("resolve provider binding: %w", err)
	}
	return item, nil
}

func mapStoreError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503", "23514", "22P02":
			return invalid("provider binding references an invalid scope or provider")
		case "23505":
			return ErrConflict
		}
	}
	return fmt.Errorf("store Provider configuration: %w", err)
}

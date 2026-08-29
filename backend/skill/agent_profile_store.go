package skill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AgentProfileVersion is an immutable, publishable snapshot of an
// AgentProfile contract. The resource remains the stable identity while this
// record pins the exact prompt and policy used by an execution.
type AgentProfileVersion struct {
	ID                string         `json:"id"`
	ProfileResourceID string         `json:"agent_profile_id"`
	Version           int            `json:"version"`
	Config            map[string]any `json:"config"`
	Status            string         `json:"status"`
	CreatedBy         *string        `json:"created_by,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	PublishedAt       *time.Time     `json:"published_at,omitempty"`
}

type AgentProfileVersionStore interface {
	CreateAgentProfileVersion(context.Context, string, map[string]any, string) (AgentProfileVersion, error)
	ListAgentProfileVersions(context.Context, string) ([]AgentProfileVersion, error)
	GetPublishedAgentProfileVersion(context.Context, string) (AgentProfileVersion, error)
	PublishAgentProfileVersion(context.Context, string, string) (AgentProfileVersion, error)
	DisableAgentProfileVersion(context.Context, string, string) (AgentProfileVersion, error)
}

type agentProfileVersionStore struct{ pool *pgxpool.Pool }

func NewAgentProfileVersionStore(pool *pgxpool.Pool) AgentProfileVersionStore {
	return &agentProfileVersionStore{pool: pool}
}

func (s *agentProfileVersionStore) CreateAgentProfileVersion(ctx context.Context, profileID string, config map[string]any, actorID string) (AgentProfileVersion, error) {
	encoded, err := json.Marshal(config)
	if err != nil {
		return AgentProfileVersion{}, invalid("AgentProfile config is invalid")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AgentProfileVersion{}, fmt.Errorf("begin AgentProfile version: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, profileID); err != nil {
		return AgentProfileVersion{}, fmt.Errorf("lock AgentProfile versions: %w", err)
	}
	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO agent_profile_versions (agent_profile_resource_id, version, config, status, created_by)
		SELECT $1::uuid, COALESCE(max(version), 0) + 1, $2::jsonb, 'draft', NULLIF($3, '')::uuid
		FROM agent_profile_versions WHERE agent_profile_resource_id = $1::uuid
		RETURNING id::text`, profileID, encoded, actorID).Scan(&id); err != nil {
		return AgentProfileVersion{}, mapStoreError(err)
	}
	item, err := getAgentProfileVersion(ctx, tx, id)
	if err != nil {
		return AgentProfileVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AgentProfileVersion{}, fmt.Errorf("commit AgentProfile version: %w", err)
	}
	return item, nil
}

func (s *agentProfileVersionStore) ListAgentProfileVersions(ctx context.Context, profileID string) ([]AgentProfileVersion, error) {
	rows, err := s.pool.Query(ctx, agentProfileVersionSelect+` WHERE agent_profile_resource_id = $1::uuid ORDER BY version DESC`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list AgentProfile versions: %w", err)
	}
	defer rows.Close()
	items := make([]AgentProfileVersion, 0)
	for rows.Next() {
		item, scanErr := scanAgentProfileVersion(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *agentProfileVersionStore) GetPublishedAgentProfileVersion(ctx context.Context, profileID string) (AgentProfileVersion, error) {
	row := s.pool.QueryRow(ctx, agentProfileVersionSelect+` WHERE agent_profile_resource_id = $1::uuid AND status = 'published' ORDER BY version DESC LIMIT 1`, profileID)
	return scanAgentProfileVersion(row)
}

func (s *agentProfileVersionStore) PublishAgentProfileVersion(ctx context.Context, profileID, versionID string) (AgentProfileVersion, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AgentProfileVersion{}, fmt.Errorf("begin AgentProfile publish: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, profileID); err != nil {
		return AgentProfileVersion{}, err
	}
	if tag, err := tx.Exec(ctx, `UPDATE agent_profile_versions SET status = 'published', published_at = now() WHERE id = $1::uuid AND agent_profile_resource_id = $2::uuid AND status IN ('draft', 'disabled')`, versionID, profileID); err != nil {
		return AgentProfileVersion{}, mapStoreError(err)
	} else if tag.RowsAffected() == 0 {
		return AgentProfileVersion{}, ErrNotFound
	}
	item, err := getAgentProfileVersion(ctx, tx, versionID)
	if err != nil {
		return AgentProfileVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AgentProfileVersion{}, fmt.Errorf("commit AgentProfile publish: %w", err)
	}
	return item, nil
}

func (s *agentProfileVersionStore) DisableAgentProfileVersion(ctx context.Context, profileID, versionID string) (AgentProfileVersion, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE agent_profile_versions SET status = 'disabled' WHERE id = $1::uuid AND agent_profile_resource_id = $2::uuid AND status <> 'disabled'`, versionID, profileID)
	if err != nil {
		return AgentProfileVersion{}, mapStoreError(err)
	}
	if tag.RowsAffected() == 0 {
		return AgentProfileVersion{}, ErrNotFound
	}
	return getAgentProfileVersion(ctx, s.pool, versionID)
}

const agentProfileVersionSelect = `SELECT id::text, agent_profile_resource_id::text, version, config, status, created_by::text, created_at, published_at FROM agent_profile_versions`

func scanAgentProfileVersion(row rowScanner) (AgentProfileVersion, error) {
	var item AgentProfileVersion
	var encoded []byte
	if err := row.Scan(&item.ID, &item.ProfileResourceID, &item.Version, &encoded, &item.Status, &item.CreatedBy, &item.CreatedAt, &item.PublishedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AgentProfileVersion{}, ErrNotFound
		}
		return AgentProfileVersion{}, err
	}
	if err := json.Unmarshal(encoded, &item.Config); err != nil {
		return AgentProfileVersion{}, fmt.Errorf("decode AgentProfile version: %w", err)
	}
	return item, nil
}

func getAgentProfileVersion(ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (AgentProfileVersion, error) {
	return scanAgentProfileVersion(query.QueryRow(ctx, agentProfileVersionSelect+` WHERE id = $1::uuid`, id))
}

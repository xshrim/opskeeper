package persona

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PersonaVersion is an immutable, publishable snapshot of an
// Persona contract. The persona remains the stable identity while this
// record pins the exact prompt and policy used by an execution.
type PersonaVersion struct {
	ID          string         `json:"id"`
	PersonaID   string         `json:"persona_id"`
	Version     int            `json:"version"`
	Config      map[string]any `json:"config"`
	Status      string         `json:"status"`
	CreatedBy   *string        `json:"created_by,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	PublishedAt *time.Time     `json:"published_at,omitempty"`
}

type PersonaVersionStore interface {
	CreatePersonaVersion(context.Context, string, map[string]any, string) (PersonaVersion, error)
	ListPersonaVersions(context.Context, string) ([]PersonaVersion, error)
	GetPublishedPersonaVersion(context.Context, string) (PersonaVersion, error)
	PublishPersonaVersion(context.Context, string, string) (PersonaVersion, error)
	DisablePersonaVersion(context.Context, string, string) (PersonaVersion, error)
}

type personaVersionStore struct{ pool *pgxpool.Pool }

type personaCatalog struct{ pool *pgxpool.Pool }

func NewPersonaCatalog(pool *pgxpool.Pool) PersonaCatalog { return &personaCatalog{pool: pool} }

func (s *personaCatalog) GetPersona(ctx context.Context, id string) (Persona, error) {
	var item Persona
	var encoded []byte
	err := s.pool.QueryRow(ctx, `SELECT id::text, scope_id::text, name, description, config, status FROM personas WHERE id = $1::uuid AND deleted_at IS NULL`, id).
		Scan(&item.ID, &item.ScopeID, &item.Name, &item.Description, &encoded, &item.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Persona{}, ErrNotFound
	}
	if err != nil {
		return Persona{}, fmt.Errorf("get persona: %w", err)
	}
	if err := json.Unmarshal(encoded, &item.Config); err != nil {
		return Persona{}, fmt.Errorf("decode persona config: %w", err)
	}
	return item, nil
}

func (s *personaCatalog) ListPersonas(ctx context.Context, scopeID string) ([]Persona, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text, scope_id::text, name, description, config, status FROM personas WHERE scope_id = $1::uuid AND deleted_at IS NULL ORDER BY name`, scopeID)
	if err != nil {
		return nil, fmt.Errorf("list personas: %w", err)
	}
	defer rows.Close()
	items := make([]Persona, 0)
	for rows.Next() {
		var item Persona
		var encoded []byte
		if err := rows.Scan(&item.ID, &item.ScopeID, &item.Name, &item.Description, &encoded, &item.Status); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(encoded, &item.Config); err != nil {
			return nil, fmt.Errorf("decode persona config: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func NewPersonaVersionStore(pool *pgxpool.Pool) PersonaVersionStore {
	return &personaVersionStore{pool: pool}
}

func NewVersionStore(pool *pgxpool.Pool) PersonaVersionStore {
	return NewPersonaVersionStore(pool)
}

func (s *personaVersionStore) CreatePersonaVersion(ctx context.Context, personaID string, config map[string]any, actorID string) (PersonaVersion, error) {
	encoded, err := json.Marshal(config)
	if err != nil {
		return PersonaVersion{}, invalid("Persona config is invalid")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PersonaVersion{}, fmt.Errorf("begin Persona version: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, personaID); err != nil {
		return PersonaVersion{}, fmt.Errorf("lock Persona versions: %w", err)
	}
	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO persona_versions (persona_id, version, config, status, created_by)
		SELECT $1::uuid, COALESCE(max(version), 0) + 1, $2::jsonb, 'draft', NULLIF($3, '')::uuid
		FROM persona_versions WHERE persona_id = $1::uuid
		RETURNING id::text`, personaID, encoded, actorID).Scan(&id); err != nil {
		return PersonaVersion{}, mapStoreError(err)
	}
	item, err := getPersonaVersion(ctx, tx, id)
	if err != nil {
		return PersonaVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PersonaVersion{}, fmt.Errorf("commit Persona version: %w", err)
	}
	return item, nil
}

func (s *personaVersionStore) ListPersonaVersions(ctx context.Context, personaID string) ([]PersonaVersion, error) {
	rows, err := s.pool.Query(ctx, personaVersionSelect+` WHERE persona_id = $1::uuid ORDER BY version DESC`, personaID)
	if err != nil {
		return nil, fmt.Errorf("list Persona versions: %w", err)
	}
	defer rows.Close()
	items := make([]PersonaVersion, 0)
	for rows.Next() {
		item, scanErr := scanPersonaVersion(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *personaVersionStore) GetPublishedPersonaVersion(ctx context.Context, personaID string) (PersonaVersion, error) {
	row := s.pool.QueryRow(ctx, personaVersionSelect+` WHERE persona_id = $1::uuid AND status = 'published' ORDER BY version DESC LIMIT 1`, personaID)
	return scanPersonaVersion(row)
}

func (s *personaVersionStore) PublishPersonaVersion(ctx context.Context, personaID, versionID string) (PersonaVersion, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PersonaVersion{}, fmt.Errorf("begin Persona publish: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, personaID); err != nil {
		return PersonaVersion{}, err
	}
	if tag, err := tx.Exec(ctx, `UPDATE persona_versions SET status = 'published', published_at = now() WHERE id = $1::uuid AND persona_id = $2::uuid AND status IN ('draft', 'disabled')`, versionID, personaID); err != nil {
		return PersonaVersion{}, mapStoreError(err)
	} else if tag.RowsAffected() == 0 {
		return PersonaVersion{}, ErrNotFound
	}
	item, err := getPersonaVersion(ctx, tx, versionID)
	if err != nil {
		return PersonaVersion{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PersonaVersion{}, fmt.Errorf("commit Persona publish: %w", err)
	}
	return item, nil
}

func (s *personaVersionStore) DisablePersonaVersion(ctx context.Context, personaID, versionID string) (PersonaVersion, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE persona_versions SET status = 'disabled' WHERE id = $1::uuid AND persona_id = $2::uuid AND status <> 'disabled'`, versionID, personaID)
	if err != nil {
		return PersonaVersion{}, mapStoreError(err)
	}
	if tag.RowsAffected() == 0 {
		return PersonaVersion{}, ErrNotFound
	}
	return getPersonaVersion(ctx, s.pool, versionID)
}

const personaVersionSelect = `SELECT id::text, persona_id::text, version, config, status, created_by::text, created_at, published_at FROM persona_versions`

type rowScanner interface{ Scan(...any) error }

func scanPersonaVersion(row rowScanner) (PersonaVersion, error) {
	var item PersonaVersion
	var encoded []byte
	if err := row.Scan(&item.ID, &item.PersonaID, &item.Version, &encoded, &item.Status, &item.CreatedBy, &item.CreatedAt, &item.PublishedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PersonaVersion{}, ErrNotFound
		}
		return PersonaVersion{}, err
	}
	if err := json.Unmarshal(encoded, &item.Config); err != nil {
		return PersonaVersion{}, fmt.Errorf("decode Persona version: %w", err)
	}
	return item, nil
}

func getPersonaVersion(ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (PersonaVersion, error) {
	return scanPersonaVersion(query.QueryRow(ctx, personaVersionSelect+` WHERE id = $1::uuid`, id))
}

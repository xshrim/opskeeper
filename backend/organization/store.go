package organization

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/authorization"
)

const platformSelect = `
	SELECT p.id::text, p.scope_id::text, s.scope_type, s.parent_scope_id::text,
	       s.status, p.name, p.code, p.created_at, p.updated_at
	  FROM platforms p
	  JOIN scopes s ON s.id = p.scope_id
	 WHERE p.deleted_at IS NULL AND s.deleted_at IS NULL`

const teamSelect = `
	SELECT t.id::text, t.platform_id::text, t.scope_id::text, s.scope_type,
	       s.parent_scope_id::text, s.status, t.name, t.code, t.labels,
	       t.created_at, t.updated_at
	  FROM teams t
	  JOIN scopes s ON s.id = t.scope_id
	 WHERE t.deleted_at IS NULL AND s.deleted_at IS NULL`

const projectSelect = `
	SELECT p.id::text, p.platform_id::text, p.team_id::text, p.scope_id::text,
	       s.scope_type, s.parent_scope_id::text, s.status, p.name, p.code,
	       p.labels, p.source, p.created_at, p.updated_at
	  FROM projects p
	  JOIN scopes s ON s.id = p.scope_id
	 WHERE p.deleted_at IS NULL AND s.deleted_at IS NULL`

// Store defines the persistence capabilities required by organization use cases.
type Store interface {
	GetPlatform(context.Context) (Platform, error)
	CreateTeam(context.Context, CreateTeamInput) (Team, error)
	ListTeams(context.Context, Pagination) (Page[Team], error)
	GetTeam(context.Context, string) (Team, error)
	UpdateTeam(context.Context, string, UpdateTeamInput) (Team, error)
	CreateProject(context.Context, CreateProjectInput) (Project, error)
	ListProjects(context.Context, string, Pagination) (Page[Project], error)
	GetProject(context.Context, string) (Project, error)
	UpdateProject(context.Context, string, UpdateProjectInput) (Project, error)
}

type store struct {
	pool *pgxpool.Pool
}

var _ Store = (*store)(nil)

func NewStore(pool *pgxpool.Pool) Store {
	return &store{pool: pool}
}

func (s *store) GetPlatform(ctx context.Context) (Platform, error) {
	query, args := scopedQuery(platformSelect+" AND p.code = 'default'", "p", ctx)
	platform, err := scanPlatform(s.pool.QueryRow(ctx, query, args...))
	if err != nil {
		return Platform{}, mapStoreError(err)
	}
	return platform, nil
}

func (s *store) CreateTeam(ctx context.Context, input CreateTeamInput) (Team, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Team{}, fmt.Errorf("begin create team: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query, args := scopedQuery(platformSelect+" AND p.code = 'default'", "p", ctx)
	platform, err := scanPlatform(tx.QueryRow(ctx, query+" FOR UPDATE", args...))
	if err != nil {
		return Team{}, mapStoreError(err)
	}
	if platform.Scope.Status != StatusActive {
		return Team{}, ErrParentInactive
	}

	var scopeID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO scopes (tenant_id, scope_type, parent_scope_id, status)
		SELECT tenant_id, 'team', id, 'active'
		  FROM scopes
		 WHERE id = $1::uuid AND status = 'active' AND deleted_at IS NULL
		RETURNING id::text`, platform.Scope.ID).Scan(&scopeID); err != nil {
		return Team{}, mapStoreError(err)
	}

	labels, err := json.Marshal(input.Labels)
	if err != nil {
		return Team{}, fmt.Errorf("encode team labels: %w", err)
	}
	var teamID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO teams (scope_id, platform_id, name, code, labels)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5)
		RETURNING id::text`, scopeID, platform.ID, input.Name, input.Code, labels).Scan(&teamID); err != nil {
		return Team{}, mapStoreError(err)
	}

	team, err := scanTeam(tx.QueryRow(ctx, teamSelect+" AND t.id = $1::uuid", teamID))
	if err != nil {
		return Team{}, mapStoreError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Team{}, fmt.Errorf("commit create team: %w", err)
	}
	return team, nil
}

func (s *store) ListTeams(ctx context.Context, pagination Pagination) (Page[Team], error) {
	var total int64
	countQuery, countArgs := scopedQuery("SELECT count(*) FROM teams t WHERE t.deleted_at IS NULL", "t", ctx)
	if err := s.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return Page[Team]{}, fmt.Errorf("count teams: %w", err)
	}

	listQuery, listArgs := scopedQuery(teamSelect, "t", ctx)
	listQuery += " ORDER BY t.created_at DESC, t.id LIMIT $" + strconv.Itoa(len(listArgs)+1) + " OFFSET $" + strconv.Itoa(len(listArgs)+2)
	listArgs = append(listArgs, pagination.PageSize, pagination.Offset())
	rows, err := s.pool.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return Page[Team]{}, fmt.Errorf("list teams: %w", err)
	}
	defer rows.Close()

	items := make([]Team, 0)
	for rows.Next() {
		team, err := scanTeam(rows)
		if err != nil {
			return Page[Team]{}, fmt.Errorf("scan team: %w", err)
		}
		items = append(items, team)
	}
	if err := rows.Err(); err != nil {
		return Page[Team]{}, fmt.Errorf("iterate teams: %w", err)
	}
	return Page[Team]{Items: items, Page: pagination.Page, PageSize: pagination.PageSize, Total: total}, nil
}

func (s *store) GetTeam(ctx context.Context, teamID string) (Team, error) {
	query, args := scopedQuery(teamSelect+" AND t.id = $1::uuid", "t", ctx, teamID)
	team, err := scanTeam(s.pool.QueryRow(ctx, query, args...))
	if err != nil {
		return Team{}, mapStoreError(err)
	}
	return team, nil
}

func (s *store) UpdateTeam(ctx context.Context, teamID string, input UpdateTeamInput) (Team, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Team{}, fmt.Errorf("begin update team: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query, args := scopedQuery(teamSelect+" AND t.id = $1::uuid", "t", ctx, teamID)
	current, err := scanTeam(tx.QueryRow(ctx, query+" FOR UPDATE", args...))
	if err != nil {
		return Team{}, mapStoreError(err)
	}
	if input.Name != nil {
		current.Name = *input.Name
	}
	if input.Labels != nil {
		current.Labels = *input.Labels
	}
	if input.Status != nil {
		current.Status = *input.Status
		current.Scope.Status = *input.Status
	}

	labels, err := json.Marshal(current.Labels)
	if err != nil {
		return Team{}, fmt.Errorf("encode team labels: %w", err)
	}
	if input.Status != nil {
		if _, err := tx.Exec(ctx, `
			UPDATE scopes SET status = $2, updated_at = now()
			 WHERE id = $1::uuid AND deleted_at IS NULL`, current.Scope.ID, current.Scope.Status); err != nil {
			return Team{}, mapStoreError(err)
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE teams
		   SET name = $2, labels = $3, updated_at = now()
		 WHERE id = $1::uuid AND deleted_at IS NULL`, teamID, current.Name, labels); err != nil {
		return Team{}, mapStoreError(err)
	}

	updated, err := scanTeam(tx.QueryRow(ctx, teamSelect+" AND t.id = $1::uuid", teamID))
	if err != nil {
		return Team{}, mapStoreError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Team{}, fmt.Errorf("commit update team: %w", err)
	}
	return updated, nil
}

func (s *store) CreateProject(ctx context.Context, input CreateProjectInput) (Project, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Project{}, fmt.Errorf("begin create project: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query, args := scopedQuery(teamSelect+" AND t.id = $1::uuid", "t", ctx, input.TeamID)
	team, err := scanTeam(tx.QueryRow(ctx, query+" FOR UPDATE", args...))
	if err != nil {
		return Project{}, mapStoreError(err)
	}
	if team.Scope.Status != StatusActive {
		return Project{}, ErrParentInactive
	}

	var scopeID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO scopes (tenant_id, scope_type, parent_scope_id, status)
		SELECT tenant_id, 'project', id, 'active'
		  FROM scopes
		 WHERE id = $1::uuid AND status = 'active' AND deleted_at IS NULL
		RETURNING id::text`, team.Scope.ID).Scan(&scopeID); err != nil {
		return Project{}, mapStoreError(err)
	}

	labels, err := json.Marshal(input.Labels)
	if err != nil {
		return Project{}, fmt.Errorf("encode project labels: %w", err)
	}
	var projectID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO projects (scope_id, platform_id, team_id, name, code, labels, source)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7)
		RETURNING id::text`, scopeID, team.PlatformID, team.ID, input.Name, input.Code, labels, input.Source).Scan(&projectID); err != nil {
		return Project{}, mapStoreError(err)
	}

	project, err := scanProject(tx.QueryRow(ctx, projectSelect+" AND p.id = $1::uuid", projectID))
	if err != nil {
		return Project{}, mapStoreError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Project{}, fmt.Errorf("commit create project: %w", err)
	}
	return project, nil
}

func (s *store) ListProjects(ctx context.Context, teamID string, pagination Pagination) (Page[Project], error) {
	if _, err := s.GetTeam(ctx, teamID); err != nil {
		return Page[Project]{}, err
	}

	var total int64
	countQuery, countArgs := scopedQuery("SELECT count(*) FROM projects p WHERE p.team_id = $1::uuid AND p.deleted_at IS NULL", "p", ctx, teamID)
	if err := s.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return Page[Project]{}, fmt.Errorf("count projects: %w", err)
	}

	listQuery, listArgs := scopedQuery(projectSelect+" AND p.team_id = $1::uuid", "p", ctx, teamID)
	listQuery += " ORDER BY p.created_at DESC, p.id LIMIT $" + strconv.Itoa(len(listArgs)+1) + " OFFSET $" + strconv.Itoa(len(listArgs)+2)
	listArgs = append(listArgs, pagination.PageSize, pagination.Offset())
	rows, err := s.pool.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return Page[Project]{}, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	items := make([]Project, 0)
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return Page[Project]{}, fmt.Errorf("scan project: %w", err)
		}
		items = append(items, project)
	}
	if err := rows.Err(); err != nil {
		return Page[Project]{}, fmt.Errorf("iterate projects: %w", err)
	}
	return Page[Project]{Items: items, Page: pagination.Page, PageSize: pagination.PageSize, Total: total}, nil
}

func (s *store) GetProject(ctx context.Context, projectID string) (Project, error) {
	query, args := scopedQuery(projectSelect+" AND p.id = $1::uuid", "p", ctx, projectID)
	project, err := scanProject(s.pool.QueryRow(ctx, query, args...))
	if err != nil {
		return Project{}, mapStoreError(err)
	}
	return project, nil
}

func (s *store) UpdateProject(ctx context.Context, projectID string, input UpdateProjectInput) (Project, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Project{}, fmt.Errorf("begin update project: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query, args := scopedQuery(projectSelect+" AND p.id = $1::uuid", "p", ctx, projectID)
	current, err := scanProject(tx.QueryRow(ctx, query+" FOR UPDATE", args...))
	if err != nil {
		return Project{}, mapStoreError(err)
	}
	if input.Name != nil {
		current.Name = *input.Name
	}
	if input.Labels != nil {
		current.Labels = *input.Labels
	}
	if input.Status != nil {
		current.Status = *input.Status
		current.Scope.Status = *input.Status
	}

	labels, err := json.Marshal(current.Labels)
	if err != nil {
		return Project{}, fmt.Errorf("encode project labels: %w", err)
	}
	if input.Status != nil {
		if _, err := tx.Exec(ctx, `
			UPDATE scopes SET status = $2, updated_at = now()
			 WHERE id = $1::uuid AND deleted_at IS NULL`, current.Scope.ID, current.Scope.Status); err != nil {
			return Project{}, mapStoreError(err)
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE projects
		   SET name = $2, labels = $3, updated_at = now()
		 WHERE id = $1::uuid AND deleted_at IS NULL`, projectID, current.Name, labels); err != nil {
		return Project{}, mapStoreError(err)
	}

	updated, err := scanProject(tx.QueryRow(ctx, projectSelect+" AND p.id = $1::uuid", projectID))
	if err != nil {
		return Project{}, mapStoreError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Project{}, fmt.Errorf("commit update project: %w", err)
	}
	return updated, nil
}

// scopedQuery applies the server-computed authorization filter before any
// pagination, count, lock, or object lookup is executed.
func scopedQuery(query, alias string, ctx context.Context, args ...any) (string, []any) {
	filter, restricted := authorization.ScopeFilterFromContext(ctx)
	if !restricted {
		return query, args
	}
	if len(filter.ScopeIDs) == 0 {
		return query + " AND FALSE", args
	}
	position := strconv.Itoa(len(args) + 1)
	return query + " AND " + alias + ".scope_id = ANY($" + position + ")", append(args, filter.ScopeIDs)
}

type scanner interface {
	Scan(...any) error
}

func scanPlatform(row scanner) (Platform, error) {
	var platform Platform
	var parentID pgtype.Text
	if err := row.Scan(
		&platform.ID,
		&platform.Scope.ID,
		&platform.Scope.Type,
		&parentID,
		&platform.Scope.Status,
		&platform.Name,
		&platform.Code,
		&platform.CreatedAt,
		&platform.UpdatedAt,
	); err != nil {
		return Platform{}, err
	}
	platform.Scope.ParentID = nullableText(parentID)
	platform.Status = platform.Scope.Status
	return platform, nil
}

func scanTeam(row scanner) (Team, error) {
	var team Team
	var parentID pgtype.Text
	var labels []byte
	if err := row.Scan(
		&team.ID,
		&team.PlatformID,
		&team.Scope.ID,
		&team.Scope.Type,
		&parentID,
		&team.Scope.Status,
		&team.Name,
		&team.Code,
		&labels,
		&team.CreatedAt,
		&team.UpdatedAt,
	); err != nil {
		return Team{}, err
	}
	if err := json.Unmarshal(labels, &team.Labels); err != nil {
		return Team{}, fmt.Errorf("decode team labels: %w", err)
	}
	team.Scope.ParentID = nullableText(parentID)
	team.Status = team.Scope.Status
	return team, nil
}

func scanProject(row scanner) (Project, error) {
	var project Project
	var parentID pgtype.Text
	var labels []byte
	if err := row.Scan(
		&project.ID,
		&project.PlatformID,
		&project.TeamID,
		&project.Scope.ID,
		&project.Scope.Type,
		&parentID,
		&project.Scope.Status,
		&project.Name,
		&project.Code,
		&labels,
		&project.Source,
		&project.CreatedAt,
		&project.UpdatedAt,
	); err != nil {
		return Project{}, err
	}
	if err := json.Unmarshal(labels, &project.Labels); err != nil {
		return Project{}, fmt.Errorf("decode project labels: %w", err)
	}
	project.Scope.ParentID = nullableText(parentID)
	project.Status = project.Scope.Status
	return project, nil
}

func nullableText(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func mapStoreError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) {
		switch databaseError.Code {
		case "23505":
			return ErrConflict
		case "23503":
			return ErrNotFound
		case "23514":
			return ErrConflict
		}
	}
	return err
}

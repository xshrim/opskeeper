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
	       s.status, p.name, p.code, p.icon, p.created_at, p.updated_at
	  FROM platforms p
	  JOIN scopes s ON s.id = p.scope_id
	 WHERE p.deleted_at IS NULL AND s.deleted_at IS NULL`

const teamSelect = `
	SELECT t.id::text, t.platform_id::text, t.scope_id::text, s.scope_type,
	       s.parent_scope_id::text, s.status, t.name, t.description, t.icon, t.labels,
	       t.owner_ids,
	       t.created_at, t.updated_at
	  FROM teams t
	  JOIN scopes s ON s.id = t.scope_id
	 WHERE t.deleted_at IS NULL AND s.deleted_at IS NULL`

const projectSelect = `
	SELECT p.id::text, p.platform_id::text, p.team_id::text, p.scope_id::text,
	       s.scope_type, s.parent_scope_id::text, s.status, p.name, p.code,
	       p.description, p.icon, p.labels, p.importance_level, p.technical_owner_ids, p.operations_owner_ids, p.created_at, p.updated_at,
       (SELECT count(*)::int
          FROM applications a
         WHERE a.project_id = p.id AND a.deleted_at IS NULL) AS application_count,
       (SELECT count(*)::int
          FROM applications a
         WHERE a.project_id = p.id
           AND a.deleted_at IS NULL
           AND a.status IN ('active', 'healthy', 'normal', 'up')
           AND EXISTS (SELECT 1 FROM application_instances healthy_instance WHERE healthy_instance.application_id = a.id AND healthy_instance.status IN ('active', 'healthy', 'normal', 'up'))
           AND NOT EXISTS (SELECT 1 FROM application_instances unhealthy_instance WHERE unhealthy_instance.application_id = a.id AND unhealthy_instance.status NOT IN ('active', 'healthy', 'normal', 'up'))) AS healthy_application_count,
       (SELECT count(DISTINCT r.id)::int
          FROM resources r
         WHERE r.deleted_at IS NULL
           AND (r.scope_id = p.scope_id
             OR EXISTS (
                  SELECT 1
                    FROM application_instances i
                    JOIN applications a ON a.id = i.application_id
                   WHERE a.project_id = p.id
                     AND a.deleted_at IS NULL
                     AND i.target_resource_id = r.id)
             OR EXISTS (
                  SELECT 1
                    FROM application_dependencies d
                    JOIN applications a ON a.id = d.application_id
                   WHERE a.project_id = p.id
                     AND a.deleted_at IS NULL
                     AND d.target_resource_id = r.id))) AS resource_count,
       (SELECT count(*)::int
          FROM inspection_findings f
          JOIN inspection_policies policy ON policy.id = f.policy_id
          JOIN resources r ON r.id = f.target_resource_id
         WHERE policy.scope_id = p.scope_id
           AND r.scope_id = p.scope_id
           AND f.status = 'open'
           AND r.deleted_at IS NULL) AS alert_count,
       (SELECT count(*)::int
          FROM application_dependencies d
          JOIN applications a ON a.id = d.application_id
         WHERE a.project_id = p.id AND a.deleted_at IS NULL) AS dependency_count
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
	DeleteTeam(context.Context, string, DeleteInput) (DeleteSummary, error)
	CreateProject(context.Context, CreateProjectInput) (Project, error)
	ListProjects(context.Context, string, Pagination) (Page[Project], error)
	GetProject(context.Context, string) (Project, error)
	UpdateProject(context.Context, string, UpdateProjectInput) (Project, error)
	DeleteProject(context.Context, string, DeleteInput) (DeleteSummary, error)
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
	if err := validateActiveUsers(ctx, tx, input.OwnerIDs); err != nil {
		return Team{}, err
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
		INSERT INTO teams (scope_id, platform_id, name, description, icon, labels, owner_ids)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7::uuid[])
		RETURNING id::text`, scopeID, platform.ID, input.Name, input.Description, input.Icon, labels, input.OwnerIDs).Scan(&teamID); err != nil {
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
	if input.Description != nil {
		current.Description = *input.Description
	}
	if input.Labels != nil {
		current.Labels = *input.Labels
	}
	if input.Icon != nil {
		current.Icon = *input.Icon
	}
	if input.Status != nil {
		current.Status = *input.Status
		current.Scope.Status = *input.Status
	}
	if input.OwnerIDs != nil {
		current.OwnerIDs = *input.OwnerIDs
	}
	if err := validateActiveUsers(ctx, tx, current.OwnerIDs); err != nil {
		return Team{}, err
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
		   SET name = $2, description = $3, icon = $4, labels = $5, owner_ids = $6::uuid[], updated_at = now()
		 WHERE id = $1::uuid AND deleted_at IS NULL`, teamID, current.Name, current.Description, current.Icon, labels, current.OwnerIDs); err != nil {
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
	if err := validateActiveUsers(ctx, tx, input.TechnicalOwnerIDs, input.OperationsOwnerIDs); err != nil {
		return Project{}, err
	}
	for _, app := range input.Applications {
		if err := validateActiveUsers(ctx, tx, app.TechnicalOwnerIDs, app.OperationsOwnerIDs); err != nil {
			return Project{}, err
		}
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
		INSERT INTO projects (scope_id, platform_id, team_id, name, code, description, icon, labels, importance_level, technical_owner_ids, operations_owner_ids)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9, $10::uuid[], $11::uuid[])
		RETURNING id::text`, scopeID, team.PlatformID, team.ID, input.Name, input.Code, input.Description, input.Icon, labels, input.ImportanceLevel, input.TechnicalOwnerIDs, input.OperationsOwnerIDs).Scan(&projectID); err != nil {
		return Project{}, mapStoreError(err)
	}
	for _, app := range input.Applications {
		appLabels, err := json.Marshal(app.Labels)
		if err != nil {
			return Project{}, fmt.Errorf("encode application labels: %w", err)
		}
		var appID string
		if err := tx.QueryRow(ctx, `INSERT INTO applications(project_id,name,code,description,icon,runtime_kind,external_uid,labels,critical,technical_owner_ids,operations_owner_ids)
			VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10::uuid[],$11::uuid[]) RETURNING id::text`, projectID, app.Name, app.Code, app.Description, app.Icon, app.RuntimeKind, app.ExternalUID, appLabels, app.Critical, app.TechnicalOwnerIDs, app.OperationsOwnerIDs).Scan(&appID); err != nil {
			return Project{}, mapStoreError(err)
		}
		for _, instance := range app.Instances {
			selector, err := json.Marshal(instance.Selector)
			if err != nil {
				return Project{}, fmt.Errorf("encode instance selector: %w", err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO application_instances(application_id,name,runtime_kind,target_resource_id,selector,status)
				VALUES($1::uuid,$2,$3,$4::uuid,$5::jsonb,'active')`, appID, instance.Name, app.RuntimeKind, instance.TargetResourceID, selector); err != nil {
				return Project{}, mapStoreError(err)
			}
		}
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
	if input.Description != nil {
		current.Description = *input.Description
	}
	if input.Labels != nil {
		current.Labels = *input.Labels
	}
	if input.Icon != nil {
		current.Icon = *input.Icon
	}
	if input.Status != nil {
		current.Status = *input.Status
		current.Scope.Status = *input.Status
	}
	if input.ImportanceLevel != nil {
		current.ImportanceLevel = *input.ImportanceLevel
	}
	if input.TechnicalOwnerIDs != nil {
		current.TechnicalOwnerIDs = *input.TechnicalOwnerIDs
	}
	if input.OperationsOwnerIDs != nil {
		current.OperationsOwnerIDs = *input.OperationsOwnerIDs
	}
	if err := validateActiveUsers(ctx, tx, current.TechnicalOwnerIDs, current.OperationsOwnerIDs); err != nil {
		return Project{}, err
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
		   SET name = $2, description = $3, icon = $4, labels = $5, importance_level = $6, technical_owner_ids = $7::uuid[], operations_owner_ids = $8::uuid[], updated_at = now()
		 WHERE id = $1::uuid AND deleted_at IS NULL`, projectID, current.Name, current.Description, current.Icon, labels, current.ImportanceLevel, current.TechnicalOwnerIDs, current.OperationsOwnerIDs); err != nil {
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

func (s *store) DeleteProject(ctx context.Context, projectID string, input DeleteInput) (DeleteSummary, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DeleteSummary{}, fmt.Errorf("begin delete project: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	query, args := scopedQuery(projectSelect+" AND p.id = $1::uuid", "p", ctx, projectID)
	project, err := scanProject(tx.QueryRow(ctx, query+" FOR UPDATE", args...))
	if err != nil {
		return DeleteSummary{}, mapStoreError(err)
	}
	if input.ConfirmationName != project.Name {
		return DeleteSummary{}, invalid("project name confirmation does not match")
	}
	summary, err := deleteSummary(ctx, tx, []string{project.Scope.ID}, 0, 1)
	if err != nil {
		return DeleteSummary{}, err
	}
	if !input.Cascade && (summary.Applications > 0 || summary.RoleBindings > 0 || summary.Resources > 0) {
		return summary, &DeleteBlockedError{Summary: summary}
	}
	if err := archiveScopes(ctx, tx, []string{project.Scope.ID}, project.Scope.ParentID); err != nil {
		return DeleteSummary{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE projects SET deleted_at=now(), updated_at=now() WHERE id=$1::uuid AND deleted_at IS NULL`, project.ID); err != nil {
		return DeleteSummary{}, fmt.Errorf("archive project: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return DeleteSummary{}, fmt.Errorf("commit delete project: %w", err)
	}
	return summary, nil
}

func (s *store) DeleteTeam(ctx context.Context, teamID string, input DeleteInput) (DeleteSummary, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DeleteSummary{}, fmt.Errorf("begin delete team: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	query, args := scopedQuery(teamSelect+" AND t.id = $1::uuid", "t", ctx, teamID)
	team, err := scanTeam(tx.QueryRow(ctx, query+" FOR UPDATE", args...))
	if err != nil {
		return DeleteSummary{}, mapStoreError(err)
	}
	if input.ConfirmationName != team.Name {
		return DeleteSummary{}, invalid("team name confirmation does not match")
	}
	rows, err := tx.Query(ctx, `SELECT scope_id::text FROM projects WHERE team_id=$1::uuid AND deleted_at IS NULL`, team.ID)
	if err != nil {
		return DeleteSummary{}, fmt.Errorf("list team project scopes: %w", err)
	}
	scopeIDs := []string{team.Scope.ID}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return DeleteSummary{}, err
		}
		scopeIDs = append(scopeIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return DeleteSummary{}, err
	}
	summary, err := deleteSummary(ctx, tx, scopeIDs, 1, len(scopeIDs)-1)
	if err != nil {
		return DeleteSummary{}, err
	}
	if !input.Cascade && (summary.Projects > 0 || summary.Applications > 0 || summary.RoleBindings > 0 || summary.Resources > 0) {
		return summary, &DeleteBlockedError{Summary: summary}
	}
	if err := archiveScopes(ctx, tx, scopeIDs, team.Scope.ParentID); err != nil {
		return DeleteSummary{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE projects SET deleted_at=now(),updated_at=now() WHERE team_id=$1::uuid AND deleted_at IS NULL`, team.ID); err != nil {
		return DeleteSummary{}, fmt.Errorf("archive team projects: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE teams SET deleted_at=now(),updated_at=now() WHERE id=$1::uuid AND deleted_at IS NULL`, team.ID); err != nil {
		return DeleteSummary{}, fmt.Errorf("archive team: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return DeleteSummary{}, fmt.Errorf("commit delete team: %w", err)
	}
	return summary, nil
}

func deleteSummary(ctx context.Context, tx pgx.Tx, scopeIDs []string, teamCount, projectCount int) (DeleteSummary, error) {
	result := DeleteSummary{Teams: teamCount, Projects: projectCount}
	queries := []struct {
		target *int
		query  string
	}{
		{&result.Applications, `SELECT count(*)::int FROM applications a JOIN projects p ON p.id=a.project_id WHERE a.deleted_at IS NULL AND p.scope_id=ANY($1::uuid[])`},
		{&result.Instances, `SELECT count(*)::int FROM application_instances i JOIN applications a ON a.id=i.application_id JOIN projects p ON p.id=a.project_id WHERE a.deleted_at IS NULL AND p.scope_id=ANY($1::uuid[])`},
		{&result.RoleBindings, `SELECT count(*)::int FROM role_bindings WHERE scope_id=ANY($1::uuid[])`},
		{&result.Resources, `SELECT count(*)::int FROM resources WHERE deleted_at IS NULL AND scope_id=ANY($1::uuid[])`},
	}
	for _, item := range queries {
		if err := tx.QueryRow(ctx, item.query, scopeIDs).Scan(item.target); err != nil {
			return DeleteSummary{}, fmt.Errorf("build deletion summary: %w", err)
		}
	}
	return result, nil
}

// archiveScopes removes scope-owned configuration from active use, clears
// authorization, and retains resources by moving them to the parent scope.
// Application bindings are removed before application rows are archived.
func archiveScopes(ctx context.Context, tx pgx.Tx, scopeIDs []string, parentScopeID *string) error {
	if parentScopeID == nil || *parentScopeID == "" {
		return invalid("organization has no parent scope for resource preservation")
	}
	if _, err := tx.Exec(ctx, `UPDATE resources SET scope_id=$2::uuid,updated_at=now() WHERE deleted_at IS NULL AND scope_id=ANY($1::uuid[])`, scopeIDs, *parentScopeID); err != nil {
		return fmt.Errorf("preserve scoped resources: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM role_bindings WHERE scope_id=ANY($1::uuid[])`, scopeIDs); err != nil {
		return fmt.Errorf("delete scope role bindings: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM scope_defaults WHERE scope_id=ANY($1::uuid[])`, scopeIDs); err != nil {
		return fmt.Errorf("delete resource scope defaults: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM provider_scope_bindings WHERE scope_id=ANY($1::uuid[])`, scopeIDs); err != nil {
		return fmt.Errorf("delete provider scope bindings: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM engine_scope_bindings WHERE scope_id=ANY($1::uuid[])`, scopeIDs); err != nil {
		return fmt.Errorf("delete engine scope bindings: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE providers SET status='disabled',deleted_at=now(),updated_at=now() WHERE scope_id=ANY($1::uuid[]) AND deleted_at IS NULL`, scopeIDs); err != nil {
		return fmt.Errorf("archive providers: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE engines SET status='disabled',deleted_at=now(),updated_at=now() WHERE scope_id=ANY($1::uuid[]) AND deleted_at IS NULL`, scopeIDs); err != nil {
		return fmt.Errorf("archive engines: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE personas SET status='disabled',deleted_at=now(),updated_at=now() WHERE scope_id=ANY($1::uuid[]) AND deleted_at IS NULL`, scopeIDs); err != nil {
		return fmt.Errorf("archive personas: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE skills SET status='disabled',deleted_at=now(),updated_at=now() WHERE scope_id=ANY($1::uuid[]) AND deleted_at IS NULL`, scopeIDs); err != nil {
		return fmt.Errorf("archive skills: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE inspection_policies SET status='disabled',deleted_at=now(),updated_at=now() WHERE scope_id=ANY($1::uuid[]) AND deleted_at IS NULL`, scopeIDs); err != nil {
		return fmt.Errorf("archive inspection policies: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE notification_channels SET status='disabled',deleted_at=now(),updated_at=now() WHERE scope_id=ANY($1::uuid[]) AND deleted_at IS NULL`, scopeIDs); err != nil {
		return fmt.Errorf("archive notification channels: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE notification_rules SET status='disabled',deleted_at=now(),updated_at=now() WHERE scope_id=ANY($1::uuid[]) AND deleted_at IS NULL`, scopeIDs); err != nil {
		return fmt.Errorf("archive notification rules: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE notification_templates SET deleted_at=now(),updated_at=now() WHERE scope_id=ANY($1::uuid[]) AND deleted_at IS NULL`, scopeIDs); err != nil {
		return fmt.Errorf("archive notification templates: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM application_dependencies WHERE application_id IN (SELECT a.id FROM applications a JOIN projects p ON p.id=a.project_id WHERE p.scope_id=ANY($1::uuid[]))`, scopeIDs); err != nil {
		return fmt.Errorf("delete application dependencies: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM application_instances WHERE application_id IN (SELECT a.id FROM applications a JOIN projects p ON p.id=a.project_id WHERE p.scope_id=ANY($1::uuid[]))`, scopeIDs); err != nil {
		return fmt.Errorf("delete application instances: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE applications SET deleted_at=now(),status='disabled',updated_at=now() WHERE id IN (SELECT a.id FROM applications a JOIN projects p ON p.id=a.project_id WHERE p.scope_id=ANY($1::uuid[])) AND deleted_at IS NULL`, scopeIDs); err != nil {
		return fmt.Errorf("archive applications: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE scopes SET status='disabled',deleted_at=now(),updated_at=now() WHERE id=ANY($1::uuid[]) AND deleted_at IS NULL`, scopeIDs); err != nil {
		return fmt.Errorf("archive scopes: %w", err)
	}
	return nil
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

func validateActiveUsers(ctx context.Context, tx pgx.Tx, ownerLists ...[]string) error {
	ids := make([]string, 0)
	seen := make(map[string]struct{})
	for _, owners := range ownerLists {
		for _, id := range owners {
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE id = ANY($1::uuid[]) AND status = 'active' AND deleted_at IS NULL`, ids).Scan(&count); err != nil {
		return fmt.Errorf("validate responsible users: %w", err)
	}
	if count != len(ids) {
		return invalid("responsible users must be active users")
	}
	return nil
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
		&platform.Icon,
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
		&team.Description,
		&team.Icon,
		&labels,
		&team.OwnerIDs,
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
		&project.Description,
		&project.Icon,
		&labels,
		&project.ImportanceLevel,
		&project.TechnicalOwnerIDs,
		&project.OperationsOwnerIDs,
		&project.CreatedAt,
		&project.UpdatedAt,
		&project.Summary.Applications,
		&project.Summary.ApplicationsHealthy,
		&project.Summary.Resources,
		&project.Summary.Alerts,
		&project.Summary.Dependencies,
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

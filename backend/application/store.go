package application

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

const appSelect = `SELECT a.id::text, a.project_id::text, p.scope_id::text,
       a.name, a.code, a.description, a.icon,
       CASE
         WHEN a.status = 'disabled' THEN 'disabled'
         WHEN NOT EXISTS (SELECT 1 FROM application_instances missing_instance WHERE missing_instance.application_id = a.id) THEN 'unknown'
         WHEN EXISTS (SELECT 1 FROM application_instances unhealthy_instance WHERE unhealthy_instance.application_id = a.id AND unhealthy_instance.status NOT IN ('active', 'healthy', 'normal', 'up')) THEN 'warning'
         ELSE 'active'
	       END, a.runtime_kind,
	       a.external_uid, a.labels, a.critical, a.technical_owner_ids, a.operations_owner_ids, a.created_at, a.updated_at
  FROM applications a
  JOIN projects p ON p.id = a.project_id
  JOIN scopes s ON s.id = p.scope_id
 WHERE a.deleted_at IS NULL AND p.deleted_at IS NULL AND s.deleted_at IS NULL`

type rowScanner interface{ Scan(...any) error }

func scanApp(row rowScanner) (Application, error) {
	var item Application
	var labels []byte
	if err := row.Scan(&item.ID, &item.ProjectID, &item.ProjectScopeID, &item.Name, &item.Code,
		&item.Description, &item.Icon, &item.Status, &item.RuntimeKind, &item.ExternalUID,
		&labels, &item.Critical, &item.TechnicalOwnerIDs, &item.OperationsOwnerIDs, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Application{}, ErrNotFound
		}
		return Application{}, fmt.Errorf("scan application: %w", err)
	}
	if err := json.Unmarshal(labels, &item.Labels); err != nil {
		return Application{}, fmt.Errorf("decode application labels: %w", err)
	}
	if item.Labels == nil {
		item.Labels = map[string]string{}
	}
	return item, nil
}

func visibleApplications(query string, args []any, ctx context.Context) (string, []any) {
	filter, restricted := authorization.ScopeFilterFromContext(ctx)
	if !restricted {
		return query, args
	}
	if len(filter.ScopeIDs) == 0 {
		return query + " AND FALSE", args
	}
	position := strconv.Itoa(len(args) + 1)
	return query + " AND p.scope_id = ANY($" + position + ")", append(args, filter.ScopeIDs)
}

func (s *store) projectExists(ctx context.Context, projectID string) error {
	query, args := visibleApplications(`SELECT 1 FROM projects p JOIN scopes s ON s.id = p.scope_id WHERE p.id = $1::uuid AND p.deleted_at IS NULL AND s.deleted_at IS NULL`, []any{projectID}, ctx)
	var exists int
	if err := s.pool.QueryRow(ctx, query, args...).Scan(&exists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return mapStoreError(err)
	}
	return nil
}

func (s *store) Create(ctx context.Context, input CreateInput) (Application, error) {
	applications, err := s.CreateMany(ctx, []CreateInput{input})
	if err != nil {
		return Application{}, err
	}
	return applications[0], nil
}

func (s *store) CreateMany(ctx context.Context, inputs []CreateInput) ([]Application, error) {
	if len(inputs) == 0 {
		return nil, ErrInvalid
	}
	projectID := inputs[0].ProjectID
	for _, input := range inputs {
		if input.ProjectID != projectID {
			return nil, ErrInvalid
		}
	}
	if err := s.projectExists(ctx, projectID); err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create applications: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	ids := make([]string, 0, len(inputs))
	for _, input := range inputs {
		if err := validateActiveUsers(ctx, tx, input.TechnicalOwnerIDs, input.OperationsOwnerIDs); err != nil {
			return nil, err
		}
		labels, err := json.Marshal(normalizeLabels(input.Labels))
		if err != nil {
			return nil, fmt.Errorf("encode application labels: %w", err)
		}
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO applications(project_id,name,code,description,icon,runtime_kind,external_uid,labels,critical,technical_owner_ids,operations_owner_ids)
VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10::uuid[],$11::uuid[]) RETURNING id::text`, input.ProjectID, input.Name, input.Code,
			input.Description, input.Icon, input.RuntimeKind, input.ExternalUID, labels, input.Critical, input.TechnicalOwnerIDs, input.OperationsOwnerIDs).Scan(&id); err != nil {
			return nil, mapStoreError(err)
		}
		if err := insertInstances(ctx, tx, id, input.Instances); err != nil {
			return nil, err
		}
		if err := insertDependencies(ctx, tx, id, input.Dependencies); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	for index, input := range inputs {
		if err := insertRelations(ctx, tx, ids[index], input.Relations); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create applications: %w", err)
	}
	applications := make([]Application, 0, len(ids))
	for _, id := range ids {
		item, err := s.Get(ctx, projectID, id)
		if err != nil {
			return nil, err
		}
		applications = append(applications, item)
	}
	return applications, nil
}

func (s *store) Get(ctx context.Context, projectID, applicationID string) (Application, error) {
	args := []any{applicationID}
	query := appSelect + ` AND a.id=$1::uuid`
	if strings.TrimSpace(projectID) != "" {
		args = append(args, projectID)
		query += ` AND a.project_id=$2::uuid`
	}
	query, args = visibleApplications(query, args, ctx)
	item, err := scanApp(s.pool.QueryRow(ctx, query, args...))
	if err != nil {
		return Application{}, err
	}
	item.Instances, err = s.listInstances(ctx, item.ID)
	if err != nil {
		return Application{}, err
	}
	item.Dependencies, err = s.listDependencies(ctx, item.ID)
	if err != nil {
		return Application{}, err
	}
	item.Relations, err = s.listRelations(ctx, item.ID)
	if err != nil {
		return Application{}, err
	}
	return item, nil
}

func (s *store) List(ctx context.Context, projectID string) ([]Application, error) {
	query, args := visibleApplications(appSelect+` AND a.project_id=$1::uuid`, []any{projectID}, ctx)
	query += ` ORDER BY a.name, a.id`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	defer rows.Close()
	items := make([]Application, 0)
	for rows.Next() {
		item, err := scanApp(rows)
		if err != nil {
			return nil, err
		}
		item.Instances, err = s.listInstances(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		item.Dependencies, err = s.listDependencies(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		item.Relations, err = s.listRelations(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// DiagnosticCandidates intentionally has a scope-oriented contract. A user
// may start a diagnosis without organization:read, so its caller supplies the
// diagnosis:start authorization filter through ctx.
func (s *store) DiagnosticCandidates(ctx context.Context, scopeID string) ([]Application, error) {
	query, args := visibleApplications(appSelect+` AND resource_scope_contains($1::uuid, p.scope_id)`, []any{scopeID}, ctx)
	query += ` ORDER BY a.name, a.id`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list diagnosis application candidates: %w", err)
	}
	defer rows.Close()
	items := make([]Application, 0)
	for rows.Next() {
		item, err := scanApp(rows)
		if err != nil {
			return nil, err
		}
		item.Instances, err = s.listInstances(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		item.Dependencies, err = s.listDependencies(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) diagnosticScopeType(ctx context.Context, scopeID string) (string, error) {
	var scopeType string
	if err := s.pool.QueryRow(ctx, `SELECT scope_type FROM scopes WHERE id=$1::uuid AND deleted_at IS NULL`, scopeID).Scan(&scopeType); err != nil {
		return "", mapStoreError(err)
	}
	return scopeType, nil
}

func (s *store) DiagnosticScopeTeams(ctx context.Context, scopeID string) ([]DiagnosticScopeTeam, error) {
	scopeType, err := s.diagnosticScopeType(ctx, scopeID)
	if err != nil {
		return nil, err
	}
	query := `SELECT team.name, team_scope.status, count(DISTINCT project.id)::int
  FROM teams team
  JOIN scopes team_scope ON team_scope.id=team.scope_id AND team_scope.deleted_at IS NULL
  LEFT JOIN projects project ON project.team_id=team.id AND project.deleted_at IS NULL
 WHERE team.deleted_at IS NULL
   AND (($2='platform' AND team_scope.parent_scope_id=$1::uuid) OR ($2='team' AND team_scope.id=$1::uuid))`
	args := []any{scopeID, scopeType}
	if filter, restricted := authorization.ScopeFilterFromContext(ctx); restricted {
		if len(filter.ScopeIDs) == 0 {
			return []DiagnosticScopeTeam{}, nil
		}
		query += fmt.Sprintf(" AND team_scope.id = ANY($%d::uuid[])", len(args)+1)
		args = append(args, filter.ScopeIDs)
	}
	query += ` GROUP BY team.id, team.name, team_scope.status ORDER BY team.name, team.id`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list diagnostic scope teams: %w", err)
	}
	defer rows.Close()
	items := make([]DiagnosticScopeTeam, 0)
	for rows.Next() {
		var item DiagnosticScopeTeam
		if err := rows.Scan(&item.Name, &item.Status, &item.ProjectCount); err != nil {
			return nil, fmt.Errorf("scan diagnostic scope team: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) DiagnosticScopeProjects(ctx context.Context, scopeID, teamName string) ([]DiagnosticScopeProject, error) {
	query := `SELECT team.name, project.name, project.code, project_scope.status, count(application.id)::int
  FROM projects project
  JOIN teams team ON team.id=project.team_id AND team.deleted_at IS NULL
  JOIN scopes project_scope ON project_scope.id=project.scope_id AND project_scope.deleted_at IS NULL
  LEFT JOIN applications application ON application.project_id=project.id AND application.deleted_at IS NULL
 WHERE project.deleted_at IS NULL
   AND resource_scope_contains($1::uuid, project.scope_id)
   AND ($2='' OR team.name=$2)`
	args := []any{scopeID, teamName}
	if filter, restricted := authorization.ScopeFilterFromContext(ctx); restricted {
		if len(filter.ScopeIDs) == 0 {
			return []DiagnosticScopeProject{}, nil
		}
		query += fmt.Sprintf(" AND project.scope_id = ANY($%d::uuid[])", len(args)+1)
		args = append(args, filter.ScopeIDs)
	}
	query += ` GROUP BY team.id,team.name,project.id,project.name,project.code,project_scope.status ORDER BY team.name,project.name,project.code`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list diagnostic scope projects: %w", err)
	}
	defer rows.Close()
	items := make([]DiagnosticScopeProject, 0)
	for rows.Next() {
		var item DiagnosticScopeProject
		if err := rows.Scan(&item.TeamName, &item.Name, &item.Code, &item.Status, &item.ApplicationCount); err != nil {
			return nil, fmt.Errorf("scan diagnostic scope project: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) DiagnosticScopeApplications(ctx context.Context, scopeID, teamName, projectCode string, offset, limit int) (DiagnosticScopeApplicationPage, error) {
	if offset < 0 {
		offset = 0
	}
	// shortcut: cap each model-facing page at 100 rows; add cursor-based
	// pagination only if interactive scope searches need larger batches.
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	query := `SELECT team.name, project.name, project.code, application.name, application.code, application.runtime_kind, application.status,
       application.critical, count(instance.id)::int, count(*) OVER()::int
  FROM applications application
  JOIN projects project ON project.id=application.project_id AND project.deleted_at IS NULL
  JOIN teams team ON team.id=project.team_id AND team.deleted_at IS NULL
 WHERE application.deleted_at IS NULL
   AND resource_scope_contains($1::uuid, project.scope_id)
   AND ($2='' OR team.name=$2)
   AND ($3='' OR project.code=$3)
 GROUP BY team.id,team.name,project.id,project.name,project.code,application.id,application.name,application.code,application.runtime_kind,application.status,application.critical
 ORDER BY application.name,application.code
 LIMIT $4 OFFSET $5`
	args := []any{scopeID, teamName, projectCode, limit, offset}
	if filter, restricted := authorization.ScopeFilterFromContext(ctx); restricted {
		if len(filter.ScopeIDs) == 0 {
			return DiagnosticScopeApplicationPage{Items: []DiagnosticScopeApplication{}, Offset: offset, Limit: limit}, nil
		}
		query = strings.Replace(query, "   AND ($3='' OR project.code=$3)", "   AND ($3='' OR project.code=$3)\n   AND project.scope_id = ANY($6::uuid[])", 1)
		args = append(args, filter.ScopeIDs)
	}
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return DiagnosticScopeApplicationPage{}, fmt.Errorf("list diagnostic scope applications: %w", err)
	}
	defer rows.Close()
	page := DiagnosticScopeApplicationPage{Items: make([]DiagnosticScopeApplication, 0, limit), Offset: offset, Limit: limit}
	for rows.Next() {
		var item DiagnosticScopeApplication
		var total int
		var instanceCount int
		if err := rows.Scan(&item.TeamName, &item.ProjectName, &item.ProjectCode, &item.Name, &item.Code, &item.RuntimeKind, &item.Status, &item.Critical, &instanceCount, &total); err != nil {
			return DiagnosticScopeApplicationPage{}, fmt.Errorf("scan diagnostic scope application: %w", err)
		}
		item.InstanceCount = instanceCount
		page.Items = append(page.Items, item)
		page.Total = total
	}
	return page, rows.Err()
}

func (s *store) DiagnosticApplicationContextByLocator(ctx context.Context, scopeID, name, code string) (DiagnosticContext, error) {
	name = strings.TrimSpace(name)
	code = strings.TrimSpace(code)
	if name == "" && code == "" {
		return DiagnosticContext{}, ErrInvalid
	}
	query := `SELECT application.id::text, project.scope_id::text
  FROM applications application
  JOIN projects project ON project.id=application.project_id AND project.deleted_at IS NULL
 WHERE application.deleted_at IS NULL
   AND resource_scope_contains($1::uuid, project.scope_id)
   AND (($2<>'' AND application.name=$2) OR ($3<>'' AND application.code=$3))`
	args := []any{scopeID, name, code}
	if filter, restricted := authorization.ScopeFilterFromContext(ctx); restricted {
		if len(filter.ScopeIDs) == 0 {
			return DiagnosticContext{}, ErrNotFound
		}
		query += fmt.Sprintf(" AND project.scope_id = ANY($%d::uuid[])", len(args)+1)
		args = append(args, filter.ScopeIDs)
	}
	query += ` ORDER BY application.name, application.code LIMIT 2`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return DiagnosticContext{}, fmt.Errorf("resolve diagnostic application: %w", err)
	}
	defer rows.Close()
	var applicationID, applicationScopeID string
	count := 0
	for rows.Next() {
		count++
		if count == 1 {
			if err := rows.Scan(&applicationID, &applicationScopeID); err != nil {
				return DiagnosticContext{}, err
			}
		} else {
			var ignoredID, ignoredScope string
			if err := rows.Scan(&ignoredID, &ignoredScope); err != nil {
				return DiagnosticContext{}, err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return DiagnosticContext{}, err
	}
	if count == 0 {
		return DiagnosticContext{}, ErrNotFound
	}
	if count > 1 {
		return DiagnosticContext{}, fmt.Errorf("application locator is ambiguous")
	}
	return s.DiagnosticContext(ctx, applicationScopeID, applicationID)
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *store) Update(ctx context.Context, projectID, applicationID string, input UpdateInput) (Application, error) {
	current, err := s.Get(ctx, projectID, applicationID)
	if err != nil {
		return Application{}, err
	}
	if input.Name != nil {
		current.Name = *input.Name
	}
	if input.Code != nil {
		current.Code = *input.Code
	}
	if input.Description != nil {
		current.Description = *input.Description
	}
	if input.Icon != nil {
		current.Icon = *input.Icon
	}
	if input.Status != nil {
		current.Status = *input.Status
	}
	if input.RuntimeKind != nil {
		current.RuntimeKind = *input.RuntimeKind
	}
	if input.ExternalUID != nil {
		current.ExternalUID = *input.ExternalUID
	}
	if input.Labels != nil {
		current.Labels = normalizeLabels(*input.Labels)
	}
	if input.Critical != nil {
		current.Critical = *input.Critical
	}
	if input.TechnicalOwnerIDs != nil {
		current.TechnicalOwnerIDs = *input.TechnicalOwnerIDs
	}
	if input.OperationsOwnerIDs != nil {
		current.OperationsOwnerIDs = *input.OperationsOwnerIDs
	}
	if err := validateActiveUsers(ctx, s.pool, current.TechnicalOwnerIDs, current.OperationsOwnerIDs); err != nil {
		return Application{}, err
	}
	labels, err := json.Marshal(normalizeLabels(current.Labels))
	if err != nil {
		return Application{}, fmt.Errorf("encode application labels: %w", err)
	}
	args := []any{applicationID, current.Name, current.Code, current.Description, current.Icon, current.Status, current.RuntimeKind, current.ExternalUID, labels, current.Critical, current.TechnicalOwnerIDs, current.OperationsOwnerIDs}
	query := `UPDATE applications a SET name=$2,code=$3,description=$4,icon=$5,status=$6,runtime_kind=$7,external_uid=$8,labels=$9,critical=$10,technical_owner_ids=$11::uuid[],operations_owner_ids=$12::uuid[],updated_at=now()
WHERE a.id=$1::uuid AND a.deleted_at IS NULL`
	if strings.TrimSpace(projectID) != "" {
		args = append(args, projectID)
		query += ` AND a.project_id=$13::uuid`
	}
	query, args = visibleApplications(query+` RETURNING a.id`, args, ctx)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Application{}, fmt.Errorf("begin update application: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var updatedID string
	if err := tx.QueryRow(ctx, query, args...).Scan(&updatedID); err != nil {
		return Application{}, mapStoreError(err)
	}
	if input.Instances != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM application_instances WHERE application_id=$1::uuid`, updatedID); err != nil {
			return Application{}, fmt.Errorf("replace application instances: %w", err)
		}
		if err := insertInstances(ctx, tx, updatedID, *input.Instances); err != nil {
			return Application{}, err
		}
	}
	if input.Dependencies != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM application_dependencies WHERE application_id=$1::uuid`, updatedID); err != nil {
			return Application{}, fmt.Errorf("replace application dependencies: %w", err)
		}
		if err := insertDependencies(ctx, tx, updatedID, *input.Dependencies); err != nil {
			return Application{}, err
		}
	}
	if input.Relations != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM application_relations WHERE application_id=$1::uuid`, updatedID); err != nil {
			return Application{}, fmt.Errorf("replace application relations: %w", err)
		}
		if err := insertRelations(ctx, tx, updatedID, *input.Relations); err != nil {
			return Application{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Application{}, fmt.Errorf("commit update application: %w", err)
	}
	return s.Get(ctx, projectID, updatedID)
}

type activeUserQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func validateActiveUsers(ctx context.Context, db activeUserQuerier, ownerLists ...[]string) error {
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
	if err := db.QueryRow(ctx, `SELECT count(*) FROM users WHERE id = ANY($1::uuid[]) AND status = 'active' AND deleted_at IS NULL`, ids).Scan(&count); err != nil {
		return fmt.Errorf("validate responsible users: %w", err)
	}
	if count != len(ids) {
		return ErrInvalid
	}
	return nil
}

func (s *store) Delete(ctx context.Context, projectID, applicationID string) error {
	// Delete the application graph in one transaction. Instances and dependency
	// rows are application-owned bindings; target resources are never deleted.
	current, err := s.Get(ctx, projectID, applicationID)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete application: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM application_dependencies WHERE application_id=$1::uuid`, current.ID); err != nil {
		return fmt.Errorf("delete application dependencies: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM application_relations WHERE application_id=$1::uuid OR target_application_id=$1::uuid`, current.ID); err != nil {
		return fmt.Errorf("delete application relations: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM application_instances WHERE application_id=$1::uuid`, current.ID); err != nil {
		return fmt.Errorf("delete application instances: %w", err)
	}
	command, err := tx.Exec(ctx, `UPDATE applications SET deleted_at=now(),status='disabled',updated_at=now() WHERE id=$1::uuid AND deleted_at IS NULL`, current.ID)
	if err != nil {
		return mapStoreError(err)
	}
	if command.RowsAffected() != 1 {
		return ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete application: %w", err)
	}
	return nil
}

func (s *store) CreateInstance(ctx context.Context, projectID, applicationID string, input CreateInstanceInput) (Instance, error) {
	if _, err := s.Get(ctx, projectID, applicationID); err != nil {
		return Instance{}, err
	}
	selector, err := json.Marshal(normalizeObject(input.Selector))
	if err != nil {
		return Instance{}, fmt.Errorf("encode application selector: %w", err)
	}
	logs, err := json.Marshal(normalizeObject(input.LogBinding))
	if err != nil {
		return Instance{}, fmt.Errorf("encode application log binding: %w", err)
	}
	var id string
	if err := s.pool.QueryRow(ctx, `INSERT INTO application_instances(application_id,name,runtime_kind,target_resource_id,selector,log_binding,status)
VALUES($1::uuid,$2,$3,$4::uuid,$5::jsonb,$6::jsonb,COALESCE(NULLIF($7,''),'active')) RETURNING id::text`, applicationID,
		input.Name, input.RuntimeKind, input.TargetResourceID, selector, logs, input.Status).Scan(&id); err != nil {
		return Instance{}, mapStoreError(err)
	}
	return s.getInstance(ctx, projectID, applicationID, id)
}

func (s *store) getInstance(ctx context.Context, projectID, applicationID, instanceID string) (Instance, error) {
	args := []any{instanceID, applicationID}
	query := `SELECT i.id::text,i.application_id::text,i.name,i.runtime_kind,i.target_resource_id::text,
       r.name,r.kind,i.selector,i.log_binding,i.status
  FROM application_instances i
  JOIN applications a ON a.id=i.application_id
  JOIN projects p ON p.id=a.project_id
  JOIN resources r ON r.id=i.target_resource_id
 WHERE i.id=$1::uuid AND i.application_id=$2::uuid AND a.deleted_at IS NULL AND p.deleted_at IS NULL AND r.deleted_at IS NULL`
	if strings.TrimSpace(projectID) != "" {
		args = append(args, projectID)
		query += ` AND a.project_id=$3::uuid`
	}
	query, args = visibleApplications(query, args, ctx)
	var item Instance
	var selector, logs []byte
	if err := s.pool.QueryRow(ctx, query, args...).Scan(&item.ID, &item.ApplicationID, &item.Name, &item.RuntimeKind,
		&item.TargetResourceID, &item.TargetResourceName, &item.TargetResourceKind, &selector, &logs, &item.Status); err != nil {
		return Instance{}, mapStoreError(err)
	}
	if err := json.Unmarshal(selector, &item.Selector); err != nil {
		return Instance{}, fmt.Errorf("decode application selector: %w", err)
	}
	if err := json.Unmarshal(logs, &item.LogBinding); err != nil {
		return Instance{}, fmt.Errorf("decode application log binding: %w", err)
	}
	return item, nil
}

func (s *store) listInstances(ctx context.Context, applicationID string) ([]Instance, error) {
	query := `SELECT i.id::text,i.application_id::text,i.name,i.runtime_kind,i.target_resource_id::text,
       r.name,r.kind,i.selector,i.log_binding,i.status
  FROM application_instances i JOIN resources r ON r.id=i.target_resource_id
 WHERE i.application_id=$1::uuid AND r.deleted_at IS NULL ORDER BY i.name,i.id`
	rows, err := s.pool.Query(ctx, query, applicationID)
	if err != nil {
		return nil, fmt.Errorf("list application instances: %w", err)
	}
	defer rows.Close()
	items := make([]Instance, 0)
	for rows.Next() {
		var item Instance
		var selector, logs []byte
		if err := rows.Scan(&item.ID, &item.ApplicationID, &item.Name, &item.RuntimeKind, &item.TargetResourceID,
			&item.TargetResourceName, &item.TargetResourceKind, &selector, &logs, &item.Status); err != nil {
			return nil, fmt.Errorf("scan application instance: %w", err)
		}
		if err := json.Unmarshal(selector, &item.Selector); err != nil {
			return nil, fmt.Errorf("decode application selector: %w", err)
		}
		if err := json.Unmarshal(logs, &item.LogBinding); err != nil {
			return nil, fmt.Errorf("decode application log binding: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) DeleteInstance(ctx context.Context, projectID, applicationID, instanceID string) error {
	if _, err := s.Get(ctx, projectID, applicationID); err != nil {
		return err
	}
	var deletedID string
	if err := s.pool.QueryRow(ctx, `DELETE FROM application_instances WHERE id=$1::uuid AND application_id=$2::uuid RETURNING id::text`, instanceID, applicationID).Scan(&deletedID); err != nil {
		return mapStoreError(err)
	}
	return nil
}

func (s *store) listDependencies(ctx context.Context, applicationID string) ([]Dependency, error) {
	rows, err := s.pool.Query(ctx, `SELECT d.id::text,d.application_id::text,d.target_resource_id::text,r.name,r.kind,
       d.dependency_kind,d.binding,d.required,d.status
  FROM application_dependencies d JOIN resources r ON r.id=d.target_resource_id
 WHERE d.application_id=$1::uuid AND r.deleted_at IS NULL ORDER BY d.dependency_kind,r.name,d.id`, applicationID)
	if err != nil {
		return nil, fmt.Errorf("list application dependencies: %w", err)
	}
	defer rows.Close()
	items := make([]Dependency, 0)
	for rows.Next() {
		var item Dependency
		var binding []byte
		if err := rows.Scan(&item.ID, &item.ApplicationID, &item.TargetResourceID, &item.TargetResourceName,
			&item.TargetResourceKind, &item.DependencyKind, &binding, &item.Required, &item.Status); err != nil {
			return nil, fmt.Errorf("scan application dependency: %w", err)
		}
		if err := json.Unmarshal(binding, &item.Binding); err != nil {
			return nil, fmt.Errorf("decode application binding: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) ContextResourceIDs(ctx context.Context, scopeID, applicationID string) ([]string, error) {
	query, args := visibleApplications(`SELECT DISTINCT r.id::text
  FROM applications a JOIN projects p ON p.id=a.project_id
  JOIN (
    SELECT application_id, target_resource_id FROM application_instances WHERE status <> 'disabled'
    UNION ALL
    SELECT application_id, target_resource_id FROM application_dependencies WHERE status <> 'disabled'
  ) relation ON relation.application_id=a.id
  JOIN resources r ON r.id=relation.target_resource_id
 WHERE a.id=$1::uuid AND p.scope_id=$2::uuid AND a.deleted_at IS NULL AND r.deleted_at IS NULL
   AND resource_scope_contains(r.scope_id,p.scope_id)`, []any{applicationID, scopeID}, ctx)
	query += ` ORDER BY r.id::text`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list application context resources: %w", err)
	}
	defer rows.Close()
	items := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan application context resource: %w", err)
		}
		items = append(items, id)
	}
	return items, rows.Err()
}

func (s *store) DiagnosticContext(ctx context.Context, scopeID, applicationID string) (DiagnosticContext, error) {
	item, err := s.Get(ctx, "", applicationID)
	if err != nil {
		return DiagnosticContext{}, err
	}
	if item.ProjectScopeID != scopeID {
		return DiagnosticContext{}, ErrNotFound
	}
	var project DiagnosticProject
	var labels []byte
	if err := s.pool.QueryRow(ctx, `SELECT id::text,name,code,description,labels
  FROM projects
 WHERE id=$1::uuid AND scope_id=$2::uuid AND deleted_at IS NULL`, item.ProjectID, scopeID).Scan(
		&project.ID, &project.Name, &project.Code, &project.Description, &labels); err != nil {
		return DiagnosticContext{}, mapStoreError(err)
	}
	if err := json.Unmarshal(labels, &project.Labels); err != nil {
		return DiagnosticContext{}, fmt.Errorf("decode diagnostic project labels: %w", err)
	}
	resources := make([]DiagnosticResource, 0, len(item.Instances)+len(item.Dependencies))
	allowedResources, err := s.diagnosticResources(ctx, scopeID, item)
	if err != nil {
		return DiagnosticContext{}, err
	}
	for _, instance := range item.Instances {
		resource, ok := allowedResources[instance.TargetResourceID]
		if !ok {
			continue
		}
		resources = append(resources, DiagnosticResource{
			ID: instance.TargetResourceID, Type: resource.DisplayName, Name: resource.Name, Kind: resource.Kind,
			Relation: "runtime", RuntimeKind: instance.RuntimeKind, Status: instance.Status,
			Binding: sanitizeInstanceSelector(instance.RuntimeKind, instance.Selector),
		})
	}
	for _, dependency := range item.Dependencies {
		resource, ok := allowedResources[dependency.TargetResourceID]
		if !ok {
			continue
		}
		resources = append(resources, DiagnosticResource{
			ID: dependency.TargetResourceID, Type: resource.DisplayName, Name: resource.Name, Kind: resource.Kind,
			Relation: "association", Status: dependency.Status, Binding: sanitizeBinding(dependency.DependencyKind, dependency.Binding),
		})
	}
	dependencies, err := s.listDiagnosticApplicationReferences(ctx, applicationID, false)
	if err != nil {
		return DiagnosticContext{}, err
	}
	dependents, err := s.listDiagnosticApplicationReferences(ctx, applicationID, true)
	if err != nil {
		return DiagnosticContext{}, err
	}
	return DiagnosticContext{
		Application: DiagnosticApplication{
			ID: item.ID, ProjectID: item.ProjectID, Name: item.Name, Code: item.Code, Icon: item.Icon,
			Description: item.Description, RuntimeKind: item.RuntimeKind, Status: item.Status,
			Critical: item.Critical, Labels: cloneLabels(item.Labels),
			Instances: diagnosticInstances(item.Instances, allowedResources), ResourceDependencies: diagnosticDependencies(item.Dependencies, allowedResources),
		},
		Project: project, Resources: resources, Dependencies: dependencies, Dependents: dependents,
	}, nil
}

func diagnosticInstances(items []Instance, allowed map[string]diagnosticResourceMetadata) []DiagnosticInstance {
	result := make([]DiagnosticInstance, 0, len(items))
	for _, item := range items {
		if _, ok := allowed[item.TargetResourceID]; !ok {
			continue
		}
		result = append(result, DiagnosticInstance{
			ID: item.ID, Type: allowed[item.TargetResourceID].DisplayName, Name: item.Name, RuntimeKind: item.RuntimeKind, TargetResourceID: item.TargetResourceID,
			TargetResourceName: item.TargetResourceName, TargetResourceKind: item.TargetResourceKind,
			Selector: sanitizeInstanceSelector(item.RuntimeKind, item.Selector), Status: item.Status,
		})
	}
	return result
}

func diagnosticDependencies(items []Dependency, allowed map[string]diagnosticResourceMetadata) []DiagnosticDependency {
	result := make([]DiagnosticDependency, 0, len(items))
	for _, item := range items {
		if _, ok := allowed[item.TargetResourceID]; !ok {
			continue
		}
		result = append(result, DiagnosticDependency{
			ID: item.ID, Type: allowed[item.TargetResourceID].DisplayName, TargetResourceID: item.TargetResourceID, TargetResourceName: item.TargetResourceName,
			TargetResourceKind: item.TargetResourceKind, DependencyKind: item.DependencyKind,
			Binding: sanitizeBinding(item.DependencyKind, item.Binding), Required: item.Required, Status: item.Status,
		})
	}
	return result
}

type diagnosticResourceMetadata struct {
	Name        string
	Kind        string
	DisplayName string
}

func (s *store) diagnosticResources(ctx context.Context, scopeID string, item Application) (map[string]diagnosticResourceMetadata, error) {
	ids := make([]string, 0, len(item.Instances)+len(item.Dependencies))
	seen := make(map[string]struct{})
	for _, instance := range item.Instances {
		if _, ok := seen[instance.TargetResourceID]; !ok {
			ids = append(ids, instance.TargetResourceID)
			seen[instance.TargetResourceID] = struct{}{}
		}
	}
	for _, dependency := range item.Dependencies {
		if _, ok := seen[dependency.TargetResourceID]; !ok {
			ids = append(ids, dependency.TargetResourceID)
			seen[dependency.TargetResourceID] = struct{}{}
		}
	}
	result := make(map[string]diagnosticResourceMetadata, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	query, args := appendResourceScopeVisibility(
		`SELECT r.id::text,r.name,r.kind,COALESCE(NULLIF(rs.display_name,''),r.kind) FROM resources r LEFT JOIN resource_schemas rs ON rs.kind=r.kind AND rs.version=r.schema_version WHERE r.id=ANY($1::uuid[]) AND resource_scope_contains(r.scope_id,$2::uuid) AND r.deleted_at IS NULL`,
		[]any{ids, scopeID}, ctx, "resources",
	)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list diagnostic application resources: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var metadata diagnosticResourceMetadata
		if err := rows.Scan(&id, &metadata.Name, &metadata.Kind, &metadata.DisplayName); err != nil {
			return nil, fmt.Errorf("scan diagnostic application resource: %w", err)
		}
		result[id] = metadata
	}
	return result, rows.Err()
}

func (s *store) listDiagnosticApplicationReferences(ctx context.Context, applicationID string, incoming bool) ([]DiagnosticApplicationReference, error) {
	var query string
	args := []any{applicationID}
	if incoming {
		query = `SELECT source.id::text,source.project_id::text,source_project.name,source_project.code,
       source.name,source.code,source.description,source.runtime_kind,source.status,'depends_on',relation.status
  FROM application_relations relation
  JOIN applications source ON source.id=relation.application_id
  JOIN projects source_project ON source_project.id=source.project_id
  JOIN applications target ON target.id=relation.target_application_id
 WHERE relation.target_application_id=$1::uuid AND source.deleted_at IS NULL
   AND target.deleted_at IS NULL AND source_project.deleted_at IS NULL
   AND target.id=$1::uuid`
		query, args = appendApplicationScopeFilter(query, args, "source_project", ctx)
	} else {
		query = `SELECT target.id::text,target.project_id::text,target_project.name,target_project.code,
       target.name,target.code,target.description,target.runtime_kind,target.status,'depends_on',relation.status
  FROM application_relations relation
  JOIN applications source ON source.id=relation.application_id
  JOIN applications target ON target.id=relation.target_application_id
  JOIN projects target_project ON target_project.id=target.project_id
 WHERE relation.application_id=$1::uuid AND source.deleted_at IS NULL
   AND target.deleted_at IS NULL AND target_project.deleted_at IS NULL
   AND source.id=$1::uuid`
		query, args = appendApplicationScopeFilter(query, args, "target_project", ctx)
	}
	query += ` ORDER BY 5,1`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list diagnostic application references: %w", err)
	}
	defer rows.Close()
	items := make([]DiagnosticApplicationReference, 0)
	for rows.Next() {
		var item DiagnosticApplicationReference
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.ProjectName, &item.ProjectCode, &item.Name, &item.Code, &item.Description, &item.RuntimeKind, &item.Status, &item.RelationKind, &item.LinkStatus); err != nil {
			return nil, fmt.Errorf("scan diagnostic application reference: %w", err)
		}
		item.Type = "应用"
		items = append(items, item)
	}
	return items, rows.Err()
}

func appendApplicationScopeFilter(query string, args []any, alias string, ctx context.Context) (string, []any) {
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

func appendResourceScopeVisibility(query string, args []any, ctx context.Context, alias string) (string, []any) {
	filter, restricted := authorization.ResourceUseFilterFromContext(ctx)
	if !restricted {
		filter, restricted = authorization.ResourceFilterFromContext(ctx)
	}
	if !restricted {
		if scopeFilter, ok := authorization.ScopeFilterFromContext(ctx); ok {
			filter.ScopeIDs = scopeFilter.ScopeIDs
			restricted = true
		}
	}
	if !restricted {
		return query, args
	}
	if len(filter.ScopeIDs) == 0 && len(filter.ResourceIDs) == 0 {
		return query + " AND FALSE", args
	}
	scopePosition := len(args) + 1
	resourcePosition := len(args) + 2
	clause := fmt.Sprintf(` AND (%s.id = ANY($%d::uuid[]) OR %s.scope_id = ANY($%d::uuid[]) OR EXISTS (
        WITH RECURSIVE ancestors(id) AS (
            SELECT unnest($%d::uuid[])
            UNION
            SELECT scope.parent_scope_id FROM scopes scope JOIN ancestors ON ancestors.id=scope.id WHERE scope.parent_scope_id IS NOT NULL
        ) SELECT 1 FROM ancestors WHERE ancestors.id=%s.scope_id
    ))`, alias, resourcePosition, alias, scopePosition, scopePosition, alias)
	return query + clause, append(args, filter.ScopeIDs, filter.ResourceIDs)
}

func cloneLabels(value map[string]string) map[string]string {
	if value == nil {
		return nil
	}
	result := make(map[string]string, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func sanitizeBinding(kind string, value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	result := make(map[string]any, len(value))
	for key, item := range value {
		key = strings.TrimSpace(key)
		if !allowedDependencyBindingField(kind, key) {
			continue
		}
		if kind == "repository" && key == "path" {
			if _, exists := result["directory"]; exists {
				continue
			}
			key = "directory"
		}
		switch typed := item.(type) {
		case string:
			result[key] = safeBindingText(typed)
		case bool, float64, int, int64:
			result[key] = typed
		}
	}
	return result
}

// Runtime selectors are configuration facts that help a diagnosis focus on
// the application's concrete workload. Keep only the fields understood by
// the application binding validator; selectors must never expose arbitrary
// user-provided JSON to the model.
func sanitizeInstanceSelector(kind string, value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	allowed := map[string]struct{}{}
	switch kind {
	case RuntimeHost:
		allowed["process_command_line"] = struct{}{}
	case RuntimeDocker:
		allowed["container_name"] = struct{}{}
	case RuntimeKubernetes:
		for _, key := range []string{"workload_name", "namespace", "workload_kind"} {
			allowed[key] = struct{}{}
		}
	default:
		return nil
	}
	result := make(map[string]any, len(value))
	for key, item := range value {
		key = strings.TrimSpace(key)
		if _, ok := allowed[key]; !ok {
			continue
		}
		switch typed := item.(type) {
		case string:
			result[key] = safeBindingText(typed)
		case bool, float64, int, int64:
			result[key] = typed
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func safeBindingText(value string) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > 512 {
		return string([]rune(value)[:512])
	}
	return value
}

func (s *store) Workspace(ctx context.Context, projectID string) (Workspace, error) {
	if err := s.projectExists(ctx, projectID); err != nil {
		return Workspace{}, err
	}
	apps, err := s.List(ctx, projectID)
	if err != nil {
		return Workspace{}, err
	}
	resources, err := s.workspaceResources(ctx, projectID)
	if err != nil {
		return Workspace{}, err
	}
	alerts, err := s.workspaceAlerts(ctx, projectID)
	if err != nil {
		return Workspace{}, err
	}
	summary := ProjectSummary{ProjectID: projectID, Applications: len(apps), Resources: len(resources), Alerts: len(alerts)}
	for _, item := range apps {
		summary.Instances += len(item.Instances)
		summary.Dependencies += len(item.Dependencies)
	}
	return Workspace{Summary: summary, Resources: resources, Alerts: alerts, Applications: apps}, nil
}

func (s *store) Topology(ctx context.Context, projectID string) (Topology, error) {
	if err := s.projectExists(ctx, projectID); err != nil {
		return Topology{}, err
	}
	var project TopologyNode
	var projectScopeType string
	if err := s.pool.QueryRow(ctx, `SELECT project.id::text,project.name,project.scope_id::text,scope.scope_type
  FROM projects project JOIN scopes scope ON scope.id=project.scope_id
 WHERE project.id=$1::uuid AND project.deleted_at IS NULL`, projectID).Scan(&project.ID, &project.Name, &project.ScopeID, &projectScopeType); err != nil {
		return Topology{}, mapStoreError(err)
	}
	project.Kind = "project"
	project.Metadata = map[string]any{"scope_type": projectScopeType}
	nodes := []TopologyNode{project}
	edges := make([]TopologyEdge, 0)
	apps, err := s.List(ctx, projectID)
	if err != nil {
		return Topology{}, err
	}
	appIDs := make([]string, 0, len(apps))
	appSet := make(map[string]struct{}, len(apps))
	for _, app := range apps {
		appIDs = append(appIDs, app.ID)
		appSet[app.ID] = struct{}{}
		nodes = append(nodes, TopologyNode{ID: app.ID, Kind: "application", Name: app.Name, ProjectID: projectID, ProjectName: project.Name, Metadata: map[string]any{"code": app.Code, "icon": app.Icon, "runtime_kind": app.RuntimeKind, "description": app.Description, "status": app.Status, "critical": app.Critical, "labels": app.Labels, "instance_count": len(app.Instances)}})
		edges = append(edges, TopologyEdge{ID: "contains:" + app.ID, SourceID: projectID, TargetID: app.ID, Relation: "contains"})
	}
	resourceQuery, resourceArgs := appendResourceScopeVisibility(`SELECT DISTINCT r.id::text,r.scope_id::text,r.name,r.kind,r.subtype,r.labels,
  EXISTS (SELECT 1 FROM project_topology_resources ptr WHERE ptr.project_id=$2::uuid AND ptr.resource_id=r.id),resource_scope.scope_type
  FROM resources r
	JOIN scopes resource_scope ON resource_scope.id=r.scope_id
	JOIN projects current_project ON current_project.id=$2::uuid
	-- Project resources are always topology nodes; visible platform/team resources need an explicit topology membership or application binding.
	WHERE r.deleted_at IS NULL AND (r.scope_id=current_project.scope_id OR EXISTS (SELECT 1 FROM project_topology_resources topology_resource WHERE topology_resource.project_id=$2::uuid AND topology_resource.resource_id=r.id) OR EXISTS (SELECT 1 FROM application_instances i WHERE i.application_id=ANY($1::uuid[]) AND i.target_resource_id=r.id)
		OR EXISTS (SELECT 1 FROM application_dependencies d WHERE d.application_id=ANY($1::uuid[]) AND d.target_resource_id=r.id))`, []any{appIDs, projectID}, ctx, "r")
	resourceRows, err := s.pool.Query(ctx, resourceQuery, resourceArgs...)
	if err != nil {
		return Topology{}, fmt.Errorf("list application topology resources: %w", err)
	}
	resourceSet := make(map[string]struct{})
	for resourceRows.Next() {
		var node TopologyNode
		var subtype string
		var rawLabels []byte
		var topologyMember bool
		var scopeType string
		if err := resourceRows.Scan(&node.ID, &node.ScopeID, &node.Name, &node.ResourceKind, &subtype, &rawLabels, &topologyMember, &scopeType); err != nil {
			resourceRows.Close()
			return Topology{}, fmt.Errorf("scan application topology resource: %w", err)
		}
		node.Kind = "resource"
		labels := map[string]string{}
		if err := json.Unmarshal(rawLabels, &labels); err != nil {
			resourceRows.Close()
			return Topology{}, fmt.Errorf("decode application topology resource labels: %w", err)
		}
		node.Metadata = map[string]any{"subtype": subtype, "labels": labels, "external": node.ScopeID != project.ScopeID, "topology_member": topologyMember, "scope_type": scopeType}
		nodes = append(nodes, node)
		resourceSet[node.ID] = struct{}{}
	}
	if err := resourceRows.Err(); err != nil {
		resourceRows.Close()
		return Topology{}, err
	}
	resourceRows.Close()
	instanceRows, err := s.pool.Query(ctx, `SELECT i.id::text,i.application_id::text,i.target_resource_id::text FROM application_instances i WHERE i.application_id=ANY($1::uuid[])`, appIDs)
	if err != nil {
		return Topology{}, fmt.Errorf("list application topology instances: %w", err)
	}
	for instanceRows.Next() {
		var id, source, target string
		if err := instanceRows.Scan(&id, &source, &target); err != nil {
			instanceRows.Close()
			return Topology{}, err
		}
		if _, ok := resourceSet[target]; ok {
			edges = append(edges, TopologyEdge{ID: "instance:" + id, SourceID: source, TargetID: target, Relation: "runs_on"})
		}
	}
	instanceRows.Close()
	dependencyRows, err := s.pool.Query(ctx, `SELECT d.id::text,d.application_id::text,d.target_resource_id::text,d.dependency_kind,d.binding FROM application_dependencies d WHERE d.application_id=ANY($1::uuid[])`, appIDs)
	if err != nil {
		return Topology{}, fmt.Errorf("list application topology bindings: %w", err)
	}
	for dependencyRows.Next() {
		var id, source, target, kind string
		var binding []byte
		if err := dependencyRows.Scan(&id, &source, &target, &kind, &binding); err != nil {
			dependencyRows.Close()
			return Topology{}, err
		}
		if _, ok := resourceSet[target]; ok {
			var metadata map[string]any
			_ = json.Unmarshal(binding, &metadata)
			edges = append(edges, TopologyEdge{ID: "binding:" + id, SourceID: source, TargetID: target, Relation: "uses", Metadata: map[string]any{"kind": kind, "binding": sanitizeBinding(kind, metadata)}})
		}
	}
	dependencyRows.Close()
	resourceIDs := make([]string, 0, len(resourceSet))
	for resourceID := range resourceSet {
		resourceIDs = append(resourceIDs, resourceID)
	}
	if len(resourceIDs) > 0 {
		relationRows, err := s.pool.Query(ctx, `SELECT id::text,source_resource_id::text,target_resource_id::text,relation_type,attributes
  FROM resource_relations
 WHERE source_resource_id=ANY($1::uuid[]) AND target_resource_id=ANY($1::uuid[])`, resourceIDs)
		if err != nil {
			return Topology{}, fmt.Errorf("list project resource topology relations: %w", err)
		}
		for relationRows.Next() {
			var id, source, target, relation string
			var rawAttributes []byte
			if err := relationRows.Scan(&id, &source, &target, &relation, &rawAttributes); err != nil {
				relationRows.Close()
				return Topology{}, fmt.Errorf("scan project resource topology relation: %w", err)
			}
			attributes := map[string]any{}
			if err := json.Unmarshal(rawAttributes, &attributes); err != nil {
				relationRows.Close()
				return Topology{}, fmt.Errorf("decode project resource topology relation: %w", err)
			}
			edges = append(edges, TopologyEdge{ID: "resource_relation:" + id, SourceID: source, TargetID: target, Relation: relation, Metadata: attributes})
		}
		if err := relationRows.Err(); err != nil {
			relationRows.Close()
			return Topology{}, err
		}
		relationRows.Close()
	}
	relationQuery, relationArgs := appendApplicationScopeFilter(`SELECT relation.id::text,
	       source.id::text,source.name,source.code,source.runtime_kind,source.status,source.project_id::text,source_project.name,source_project.code,
	       target.id::text,target.name,target.code,target.runtime_kind,target.status,target.project_id::text,target_project.name,target_project.code,relation.status
  FROM application_relations relation
  JOIN applications source ON source.id=relation.application_id
  JOIN projects source_project ON source_project.id=source.project_id
  JOIN applications target ON target.id=relation.target_application_id
  JOIN projects target_project ON target_project.id=target.project_id
	WHERE (relation.application_id=ANY($1::uuid[]) OR relation.target_application_id=ANY($1::uuid[]))
	  AND source.deleted_at IS NULL AND source_project.deleted_at IS NULL
	  AND target.deleted_at IS NULL AND target_project.deleted_at IS NULL`, []any{appIDs}, "source_project", ctx)
	relationQuery, relationArgs = appendApplicationScopeFilter(relationQuery, relationArgs, "target_project", ctx)
	relationRows, err := s.pool.Query(ctx, relationQuery, relationArgs...)
	if err != nil {
		return Topology{}, fmt.Errorf("list application topology relations: %w", err)
	}
	projectSet := map[string]struct{}{projectID: {}}
	ensureProject := func(id, name, code string) {
		if _, ok := projectSet[id]; ok {
			return
		}
		nodes = append(nodes, TopologyNode{ID: id, Kind: "project", Name: name, Metadata: map[string]any{"code": code, "external": true}})
		projectSet[id] = struct{}{}
	}
	ensureApplication := func(id, name, code, runtimeKind, status, owningProjectID, owningProjectName, owningProjectCode string) {
		ensureProject(owningProjectID, owningProjectName, owningProjectCode)
		if _, ok := appSet[id]; ok {
			return
		}
		nodes = append(nodes, TopologyNode{ID: id, Kind: "application", Name: name, ProjectID: owningProjectID, ProjectName: owningProjectName, Metadata: map[string]any{"code": code, "runtime_kind": runtimeKind, "status": status, "external": owningProjectID != projectID}})
		appSet[id] = struct{}{}
		edges = append(edges, TopologyEdge{ID: "contains:" + id, SourceID: owningProjectID, TargetID: id, Relation: "contains"})
	}
	for relationRows.Next() {
		var id, sourceID, sourceName, sourceCode, sourceRuntime, sourceStatus, sourceProjectID, sourceProjectName, sourceProjectCode string
		var targetID, targetName, targetCode, targetRuntime, targetStatus, targetProjectID, targetProjectName, targetProjectCode, status string
		if err := relationRows.Scan(&id, &sourceID, &sourceName, &sourceCode, &sourceRuntime, &sourceStatus, &sourceProjectID, &sourceProjectName, &sourceProjectCode,
			&targetID, &targetName, &targetCode, &targetRuntime, &targetStatus, &targetProjectID, &targetProjectName, &targetProjectCode, &status); err != nil {
			relationRows.Close()
			return Topology{}, err
		}
		ensureApplication(sourceID, sourceName, sourceCode, sourceRuntime, sourceStatus, sourceProjectID, sourceProjectName, sourceProjectCode)
		ensureApplication(targetID, targetName, targetCode, targetRuntime, targetStatus, targetProjectID, targetProjectName, targetProjectCode)
		edges = append(edges, TopologyEdge{ID: "relation:" + id, SourceID: sourceID, TargetID: targetID, Relation: "depends_on", Metadata: map[string]any{"status": status}})
	}
	if err := relationRows.Err(); err != nil {
		return Topology{}, err
	}
	annotationQuery, annotationArgs := appendApplicationScopeFilter(`SELECT annotation.id::text,annotation.name,annotation.metadata
  FROM project_topology_annotations annotation
  JOIN projects annotation_project ON annotation_project.id=annotation.project_id
 WHERE annotation.project_id=$1::uuid AND annotation_project.deleted_at IS NULL`, []any{projectID}, "annotation_project", ctx)
	annotationRows, err := s.pool.Query(ctx, annotationQuery, annotationArgs...)
	if err != nil {
		return Topology{}, fmt.Errorf("list project topology annotations: %w", err)
	}
	for annotationRows.Next() {
		var node TopologyNode
		var metadata []byte
		if err := annotationRows.Scan(&node.ID, &node.Name, &metadata); err != nil {
			annotationRows.Close()
			return Topology{}, fmt.Errorf("scan project topology annotation: %w", err)
		}
		if err := json.Unmarshal(metadata, &node.Metadata); err != nil {
			annotationRows.Close()
			return Topology{}, fmt.Errorf("decode project topology annotation: %w", err)
		}
		node.Kind = "annotation"
		node.ProjectID = projectID
		nodes = append(nodes, node)
	}
	if err := annotationRows.Err(); err != nil {
		annotationRows.Close()
		return Topology{}, err
	}
	annotationRows.Close()
	nodeSet := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		nodeSet[node.ID] = struct{}{}
	}
	manualEdgeRows, err := s.pool.Query(ctx, `SELECT id::text,source_node_id::text,target_node_id::text,relation,metadata
  FROM project_topology_edges WHERE project_id=$1::uuid ORDER BY created_at,id`, projectID)
	if err != nil {
		return Topology{}, fmt.Errorf("list project topology edges: %w", err)
	}
	for manualEdgeRows.Next() {
		var edge TopologyEdge
		var metadata []byte
		if err := manualEdgeRows.Scan(&edge.ID, &edge.SourceID, &edge.TargetID, &edge.Relation, &metadata); err != nil {
			manualEdgeRows.Close()
			return Topology{}, fmt.Errorf("scan project topology edge: %w", err)
		}
		if _, ok := nodeSet[edge.SourceID]; !ok {
			continue
		}
		if _, ok := nodeSet[edge.TargetID]; !ok {
			continue
		}
		if err := json.Unmarshal(metadata, &edge.Metadata); err != nil {
			manualEdgeRows.Close()
			return Topology{}, fmt.Errorf("decode project topology edge: %w", err)
		}
		edge.ID = "topology:" + edge.ID
		edges = append(edges, edge)
	}
	if err := manualEdgeRows.Err(); err != nil {
		manualEdgeRows.Close()
		return Topology{}, err
	}
	manualEdgeRows.Close()
	positionRows, err := s.pool.Query(ctx, `SELECT node_id::text,position_x,position_y FROM project_topology_positions WHERE project_id=$1::uuid`, projectID)
	if err != nil {
		return Topology{}, fmt.Errorf("list project topology positions: %w", err)
	}
	positions := make(map[string]TopologyPositionInput)
	for positionRows.Next() {
		var id string
		var position TopologyPositionInput
		if err := positionRows.Scan(&id, &position.X, &position.Y); err != nil {
			positionRows.Close()
			return Topology{}, fmt.Errorf("scan project topology position: %w", err)
		}
		positions[id] = position
	}
	if err := positionRows.Err(); err != nil {
		positionRows.Close()
		return Topology{}, err
	}
	positionRows.Close()
	for index := range nodes {
		if position, ok := positions[nodes[index].ID]; ok {
			nodes[index].Position = &TopologyPosition{X: position.X, Y: position.Y}
		}
	}
	return Topology{Nodes: nodes, Edges: edges}, nil
}

func (s *store) SetTopologyPosition(ctx context.Context, projectID, nodeID string, input TopologyPositionInput) error {
	if err := s.projectExists(ctx, projectID); err != nil {
		return err
	}
	topology, err := s.Topology(ctx, projectID)
	if err != nil {
		return err
	}
	found := false
	for _, node := range topology.Nodes {
		if node.ID == nodeID {
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO project_topology_positions(project_id,node_id,position_x,position_y)
VALUES($1::uuid,$2::uuid,$3,$4) ON CONFLICT(project_id,node_id) DO UPDATE SET position_x=EXCLUDED.position_x,position_y=EXCLUDED.position_y,updated_at=now()`, projectID, nodeID, input.X, input.Y)
	if err != nil {
		return fmt.Errorf("save project topology position: %w", err)
	}
	return nil
}

func (s *store) CreateTopologyAnnotation(ctx context.Context, projectID string, input TopologyAnnotationInput) (TopologyNode, error) {
	if err := s.projectExists(ctx, projectID); err != nil {
		return TopologyNode{}, err
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		return TopologyNode{}, fmt.Errorf("encode topology annotation metadata: %w", err)
	}
	var node TopologyNode
	if err := s.pool.QueryRow(ctx, `INSERT INTO project_topology_annotations(project_id,name,metadata)
VALUES($1::uuid,$2,$3::jsonb) RETURNING id::text,name,metadata`, projectID, strings.TrimSpace(input.Name), metadata).Scan(&node.ID, &node.Name, &node.Metadata); err != nil {
		return TopologyNode{}, fmt.Errorf("create project topology annotation: %w", err)
	}
	node.Kind = "annotation"
	node.ProjectID = projectID
	return node, nil
}

func (s *store) AddTopologyResource(ctx context.Context, projectID string, input TopologyResourceInput) error {
	if err := s.projectExists(ctx, projectID); err != nil {
		return err
	}
	query, args := appendResourceScopeVisibility(`SELECT r.scope_id::text,p.scope_id::text FROM resources r JOIN scopes resource_scope ON resource_scope.id=r.scope_id JOIN projects p ON p.id=$2::uuid WHERE r.id=$1::uuid AND r.deleted_at IS NULL AND resource_scope.deleted_at IS NULL AND p.deleted_at IS NULL`, []any{input.ResourceID, projectID}, ctx, "r")
	var resourceScopeID, projectScopeID string
	if err := s.pool.QueryRow(ctx, query, args...).Scan(&resourceScopeID, &projectScopeID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return mapStoreError(err)
	}
	if resourceScopeID == projectScopeID {
		return nil
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO project_topology_resources(project_id,resource_id) VALUES($1::uuid,$2::uuid) ON CONFLICT DO NOTHING`, projectID, input.ResourceID)
	if err != nil {
		return fmt.Errorf("add resource to project topology: %w", err)
	}
	return nil
}

func (s *store) RemoveTopologyResource(ctx context.Context, projectID, resourceID string) error {
	if err := s.projectExists(ctx, projectID); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM project_topology_resources WHERE project_id=$1::uuid AND resource_id=$2::uuid`, projectID, resourceID)
	if err != nil {
		return fmt.Errorf("remove resource from project topology: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *store) DeleteTopologyAnnotation(ctx context.Context, projectID, annotationID string) error {
	if err := s.projectExists(ctx, projectID); err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM project_topology_edges WHERE project_id=$1::uuid AND (source_node_id=$2::uuid OR target_node_id=$2::uuid)`, projectID, annotationID); err != nil {
		return fmt.Errorf("delete topology annotation edges: %w", err)
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM project_topology_annotations WHERE project_id=$1::uuid AND id=$2::uuid`, projectID, annotationID)
	if err != nil {
		return fmt.Errorf("delete project topology annotation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *store) CreateTopologyEdge(ctx context.Context, projectID string, input TopologyEdgeInput) (TopologyEdge, error) {
	if err := s.projectExists(ctx, projectID); err != nil {
		return TopologyEdge{}, err
	}
	topology, err := s.Topology(ctx, projectID)
	if err != nil {
		return TopologyEdge{}, err
	}
	validNodes := make(map[string]struct{}, len(topology.Nodes))
	for _, node := range topology.Nodes {
		validNodes[node.ID] = struct{}{}
	}
	if _, ok := validNodes[input.SourceID]; !ok {
		return TopologyEdge{}, ErrNotFound
	}
	if _, ok := validNodes[input.TargetID]; !ok {
		return TopologyEdge{}, ErrNotFound
	}
	if input.Metadata == nil {
		input.Metadata = map[string]any{}
	}
	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		return TopologyEdge{}, fmt.Errorf("encode project topology edge metadata: %w", err)
	}
	var edge TopologyEdge
	if err := s.pool.QueryRow(ctx, `INSERT INTO project_topology_edges(project_id,source_node_id,target_node_id,relation,metadata)
VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5::jsonb) RETURNING id::text,source_node_id::text,target_node_id::text,relation,metadata`, projectID, input.SourceID, input.TargetID, strings.TrimSpace(input.Relation), metadata).Scan(&edge.ID, &edge.SourceID, &edge.TargetID, &edge.Relation, &edge.Metadata); err != nil {
		return TopologyEdge{}, fmt.Errorf("create project topology edge: %w", err)
	}
	edge.ID = "topology:" + edge.ID
	return edge, nil
}

func (s *store) DeleteTopologyEdge(ctx context.Context, projectID, edgeID string) error {
	if err := s.projectExists(ctx, projectID); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM project_topology_edges WHERE project_id=$1::uuid AND id=$2::uuid`, projectID, edgeID)
	if err != nil {
		return fmt.Errorf("delete project topology edge: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *store) workspaceResources(ctx context.Context, projectID string) ([]RelatedResource, error) {
	query, args := appendResourceScopeVisibility(`WITH project_scope AS (
  SELECT project.scope_id FROM projects project WHERE project.id=$1::uuid
), project_resources AS (
  SELECT resource.id
    FROM resources resource JOIN project_scope scope ON resource_scope_contains(resource.scope_id,scope.scope_id)
   WHERE resource.deleted_at IS NULL
  UNION
  SELECT topology_resource.resource_id
    FROM project_topology_resources topology_resource
   WHERE topology_resource.project_id=$1::uuid
  UNION
  SELECT instance.target_resource_id
    FROM application_instances instance
    JOIN applications application ON application.id=instance.application_id
   WHERE application.project_id=$1::uuid AND application.deleted_at IS NULL
  UNION
  SELECT dependency.target_resource_id
    FROM application_dependencies dependency
    JOIN applications application ON application.id=dependency.application_id
   WHERE application.project_id=$1::uuid AND application.deleted_at IS NULL
), application_resource_references AS (
  SELECT app_refs.target_resource_id,COUNT(DISTINCT app_refs.application_id)::integer AS reference_count
    FROM (
      SELECT instance.target_resource_id,instance.application_id
        FROM application_instances instance
        JOIN applications application ON application.id=instance.application_id
       WHERE application.project_id=$1::uuid AND application.deleted_at IS NULL
      UNION
      SELECT dependency.target_resource_id,dependency.application_id
        FROM application_dependencies dependency
        JOIN applications application ON application.id=dependency.application_id
       WHERE application.project_id=$1::uuid AND application.deleted_at IS NULL
    ) app_refs
   GROUP BY app_refs.target_resource_id
)
SELECT r.id::text,r.name,r.kind,resource_scope.scope_type,COALESCE(reference.reference_count,0)
  FROM resources r
  JOIN scopes resource_scope ON resource_scope.id=r.scope_id
  JOIN project_resources bound_resource ON bound_resource.id=r.id
  LEFT JOIN application_resource_references reference ON reference.target_resource_id=r.id
 WHERE r.deleted_at IS NULL`, []any{projectID}, ctx, "r")
	rows, err := s.pool.Query(ctx, query+` ORDER BY resource_scope.scope_type,r.name,r.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("list project workspace resources: %w", err)
	}
	defer rows.Close()
	items := make([]RelatedResource, 0)
	for rows.Next() {
		var item RelatedResource
		if err := rows.Scan(&item.ID, &item.Name, &item.Kind, &item.ScopeType, &item.ApplicationReferenceCount); err != nil {
			return nil, fmt.Errorf("scan project workspace resource: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) workspaceAlerts(ctx context.Context, projectID string) ([]Alert, error) {
	query, args := appendResourceScopeVisibility(`SELECT f.id::text,f.target_resource_id::text,f.severity,f.message,f.status
  FROM inspection_findings f
  JOIN inspection_policies policy ON policy.id=f.policy_id
  JOIN resources r ON r.id=f.target_resource_id
	JOIN projects project ON project.id=$1::uuid
 WHERE policy.scope_id=project.scope_id
   AND f.scope_id=policy.scope_id AND f.status='open' AND r.deleted_at IS NULL`, []any{projectID}, ctx, "r")
	rows, err := s.pool.Query(ctx, query+` ORDER BY f.severity DESC,f.last_observed_at DESC,f.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("list project workspace alerts: %w", err)
	}
	defer rows.Close()
	items := make([]Alert, 0)
	for rows.Next() {
		var item Alert
		if err := rows.Scan(&item.ID, &item.TargetResourceID, &item.Severity, &item.Title, &item.Status); err != nil {
			return nil, fmt.Errorf("scan project workspace alert: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func insertInstances(ctx context.Context, tx pgx.Tx, applicationID string, instances []CreateInstanceInput) error {
	for _, input := range instances {
		selector, err := json.Marshal(normalizeObject(input.Selector))
		if err != nil {
			return fmt.Errorf("encode application selector: %w", err)
		}
		logs, err := json.Marshal(normalizeObject(input.LogBinding))
		if err != nil {
			return fmt.Errorf("encode application log binding: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO application_instances(application_id,name,runtime_kind,target_resource_id,selector,log_binding,status)
VALUES($1::uuid,$2,$3,$4::uuid,$5::jsonb,$6::jsonb,COALESCE(NULLIF($7,''),'active'))`, applicationID,
			input.Name, input.RuntimeKind, input.TargetResourceID, selector, logs, input.Status); err != nil {
			return mapStoreError(err)
		}
	}
	return nil
}

func insertDependencies(ctx context.Context, tx pgx.Tx, applicationID string, dependencies []CreateDependencyInput) error {
	for _, input := range dependencies {
		binding, err := json.Marshal(normalizeObject(input.Binding))
		if err != nil {
			return fmt.Errorf("encode application dependency binding: %w", err)
		}
		status := input.Status
		if status == "" {
			status = "unknown"
		}
		required := true
		if input.Required != nil {
			required = *input.Required
		}
		if _, err := tx.Exec(ctx, `INSERT INTO application_dependencies(application_id,target_resource_id,dependency_kind,binding,required,status)
VALUES($1::uuid,$2::uuid,$3,$4::jsonb,$5,$6)`, applicationID, input.TargetResourceID, input.DependencyKind, binding, required, status); err != nil {
			return mapStoreError(err)
		}
	}
	return nil
}

func insertRelations(ctx context.Context, tx pgx.Tx, applicationID string, relations []CreateRelationInput) error {
	for _, input := range relations {
		var valid bool
		query := `SELECT EXISTS (
  SELECT 1 FROM applications source
		JOIN applications target ON target.id=$2::uuid
		JOIN projects target_project ON target_project.id=target.project_id
 WHERE source.id=$1::uuid AND source.deleted_at IS NULL
   AND target.deleted_at IS NULL AND target_project.deleted_at IS NULL`
		args := []any{applicationID, input.TargetApplicationID}
		query, args = appendApplicationScopeFilter(query, args, "target_project", ctx)
		query += ")"
		if err := tx.QueryRow(ctx, query, args...).Scan(&valid); err != nil {
			return fmt.Errorf("validate application relation target: %w", err)
		}
		if !valid {
			return ErrInvalid
		}
		status := input.Status
		if status == "" {
			status = "active"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO application_relations(application_id,target_application_id,status)
VALUES($1::uuid,$2::uuid,$3)`, applicationID, input.TargetApplicationID, status); err != nil {
			return mapStoreError(err)
		}
	}
	return nil
}

func (s *store) listRelations(ctx context.Context, applicationID string) ([]Relation, error) {
	query := `SELECT relation.id::text,relation.application_id::text,relation.target_application_id::text,
       target.name,target.code,'depends_on',relation.status
  FROM application_relations relation
  JOIN applications target ON target.id=relation.target_application_id
	JOIN projects target_project ON target_project.id=target.project_id
 WHERE relation.application_id=$1::uuid AND target.deleted_at IS NULL AND target_project.deleted_at IS NULL`
	query, args := appendApplicationScopeFilter(query, []any{applicationID}, "target_project", ctx)
	query += ` ORDER BY target.name,relation.id`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list application relations: %w", err)
	}
	defer rows.Close()
	items := make([]Relation, 0)
	for rows.Next() {
		var item Relation
		if err := rows.Scan(&item.ID, &item.ApplicationID, &item.TargetApplicationID, &item.TargetApplicationName, &item.TargetApplicationCode, &item.RelationKind, &item.Status); err != nil {
			return nil, fmt.Errorf("scan application relation: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func normalizeObject(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func normalizeLabels(value map[string]string) map[string]string {
	if value == nil {
		return map[string]string{}
	}
	return value
}

func mapStoreError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503":
			return ErrNotFound
		case "23505":
			return fmt.Errorf("%w: application identity already exists", ErrInvalid)
		case "23514", "22P02":
			return fmt.Errorf("%w: application relation is invalid", ErrInvalid)
		}
	}
	return fmt.Errorf("application store: %w", err)
}

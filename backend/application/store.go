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
       a.name, a.code, a.description, a.icon, a.status, a.source,
       a.external_uid, a.labels, a.created_at, a.updated_at
  FROM applications a
  JOIN projects p ON p.id = a.project_id
  JOIN scopes s ON s.id = p.scope_id
 WHERE a.deleted_at IS NULL AND p.deleted_at IS NULL AND s.deleted_at IS NULL`

type rowScanner interface{ Scan(...any) error }

func scanApp(row rowScanner) (Application, error) {
	var item Application
	var labels []byte
	if err := row.Scan(&item.ID, &item.ProjectID, &item.ProjectScopeID, &item.Name, &item.Code,
		&item.Description, &item.Icon, &item.Status, &item.Source, &item.ExternalUID,
		&labels, &item.CreatedAt, &item.UpdatedAt); err != nil {
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
	if err := s.projectExists(ctx, input.ProjectID); err != nil {
		return Application{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Application{}, fmt.Errorf("begin create application: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id string
	labels, err := json.Marshal(normalizeLabels(input.Labels))
	if err != nil {
		return Application{}, fmt.Errorf("encode application labels: %w", err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO applications(project_id,name,code,description,icon,source,external_uid,labels)
VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8) RETURNING id::text`, input.ProjectID, input.Name, input.Code,
		input.Description, input.Icon, input.Source, input.ExternalUID, labels).Scan(&id); err != nil {
		return Application{}, mapStoreError(err)
	}
	if err := insertRelations(ctx, tx, id, input.Instances, input.Dependencies); err != nil {
		return Application{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Application{}, fmt.Errorf("commit create application: %w", err)
	}
	return s.Get(ctx, input.ProjectID, id)
}

func (s *store) Import(ctx context.Context, input ImportInput) (Application, error) {
	if err := s.projectExists(ctx, input.ProjectID); err != nil {
		return Application{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Application{}, fmt.Errorf("begin import application: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	labels, err := json.Marshal(normalizeLabels(input.Labels))
	if err != nil {
		return Application{}, fmt.Errorf("encode imported application labels: %w", err)
	}
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO applications(project_id,name,code,description,icon,source,external_uid,labels)
VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (project_id, external_uid) WHERE deleted_at IS NULL AND external_uid <> ''
DO UPDATE SET name=EXCLUDED.name, code=EXCLUDED.code, description=EXCLUDED.description,
              icon=EXCLUDED.icon, source=EXCLUDED.source, labels=EXCLUDED.labels,
              status='active', updated_at=now(), deleted_at=NULL
RETURNING id::text`, input.ProjectID, input.Name, input.Code, input.Description, input.Icon,
		input.Source, input.ExternalUID, labels).Scan(&id)
	if err != nil {
		return Application{}, mapStoreError(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM application_instances WHERE application_id=$1::uuid`, id); err != nil {
		return Application{}, mapStoreError(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM application_dependencies WHERE application_id=$1::uuid`, id); err != nil {
		return Application{}, mapStoreError(err)
	}
	if err := insertRelations(ctx, tx, id, input.Instances, input.Dependencies); err != nil {
		return Application{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Application{}, fmt.Errorf("commit import application: %w", err)
	}
	return s.Get(ctx, input.ProjectID, id)
}

func (s *store) Get(ctx context.Context, projectID, applicationID string) (Application, error) {
	args := []any{applicationID}
	query := appSelect + ` AND a.id=$1::uuid`
	if strings.TrimSpace(projectID) != "" {
		args = append(args, projectID)
		query += ` AND a.project_id=$2::uuid`
	}
	query, args = visibleApplications(query, args, ctx)
	return scanApp(s.pool.QueryRow(ctx, query, args...))
}

func (s *store) List(ctx context.Context, projectID string) ([]Application, error) {
	query, args := visibleApplications(appSelect+` AND a.project_id=$1::uuid ORDER BY a.name, a.id`, []any{projectID}, ctx)
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
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) Update(ctx context.Context, projectID, applicationID string, input UpdateInput) (Application, error) {
	current, err := s.Get(ctx, projectID, applicationID)
	if err != nil {
		return Application{}, err
	}
	if input.Name != nil {
		current.Name = *input.Name
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
	if input.Labels != nil {
		current.Labels = normalizeLabels(*input.Labels)
	}
	labels, err := json.Marshal(normalizeLabels(current.Labels))
	if err != nil {
		return Application{}, fmt.Errorf("encode application labels: %w", err)
	}
	args := []any{applicationID, current.Name, current.Description, current.Icon, current.Status, labels}
	query := `UPDATE applications a SET name=$2,description=$3,icon=$4,status=$5,labels=$6,updated_at=now()
WHERE a.id=$1::uuid AND a.deleted_at IS NULL`
	if strings.TrimSpace(projectID) != "" {
		args = append(args, projectID)
		query += ` AND a.project_id=$7::uuid`
	}
	query, args = visibleApplications(query+` RETURNING a.id`, args, ctx)
	var updatedID string
	if err := s.pool.QueryRow(ctx, query, args...).Scan(&updatedID); err != nil {
		return Application{}, mapStoreError(err)
	}
	return s.Get(ctx, projectID, updatedID)
}

func (s *store) Delete(ctx context.Context, projectID, applicationID string) error {
	args := []any{applicationID}
	query := `UPDATE applications a SET deleted_at=now(),status='disabled',updated_at=now()
WHERE a.id=$1::uuid AND a.deleted_at IS NULL`
	if strings.TrimSpace(projectID) != "" {
		args = append(args, projectID)
		query += ` AND a.project_id=$2::uuid`
	}
	query, args = visibleApplications(query+` RETURNING a.id`, args, ctx)
	var deletedID string
	if err := s.pool.QueryRow(ctx, query, args...).Scan(&deletedID); err != nil {
		return mapStoreError(err)
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
VALUES($1::uuid,$2,$3,$4::uuid,$5::jsonb,$6::jsonb,COALESCE(NULLIF($7,''),'unknown')) RETURNING id::text`, applicationID,
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

func (s *store) CreateDependency(ctx context.Context, projectID, applicationID string, input CreateDependencyInput) (Dependency, error) {
	if _, err := s.Get(ctx, projectID, applicationID); err != nil {
		return Dependency{}, err
	}
	binding, err := json.Marshal(normalizeObject(input.Binding))
	if err != nil {
		return Dependency{}, fmt.Errorf("encode application binding: %w", err)
	}
	var id string
	if err := s.pool.QueryRow(ctx, `INSERT INTO application_dependencies(application_id,target_resource_id,dependency_kind,binding,required,status)
VALUES($1::uuid,$2::uuid,$3,$4::jsonb,$5,COALESCE(NULLIF($6,''),'unknown')) RETURNING id::text`, applicationID,
		input.TargetResourceID, input.DependencyKind, binding, input.Required, input.Status).Scan(&id); err != nil {
		return Dependency{}, mapStoreError(err)
	}
	return s.getDependency(ctx, projectID, applicationID, id)
}

func (s *store) getDependency(ctx context.Context, projectID, applicationID, dependencyID string) (Dependency, error) {
	args := []any{dependencyID, applicationID}
	query := `SELECT d.id::text,d.application_id::text,d.target_resource_id::text,r.name,r.kind,
       d.dependency_kind,d.binding,d.required,d.status
  FROM application_dependencies d
  JOIN applications a ON a.id=d.application_id
  JOIN projects p ON p.id=a.project_id
  JOIN resources r ON r.id=d.target_resource_id
 WHERE d.id=$1::uuid AND d.application_id=$2::uuid AND a.deleted_at IS NULL AND p.deleted_at IS NULL AND r.deleted_at IS NULL`
	if strings.TrimSpace(projectID) != "" {
		args = append(args, projectID)
		query += ` AND a.project_id=$3::uuid`
	}
	query, args = visibleApplications(query, args, ctx)
	var item Dependency
	var binding []byte
	if err := s.pool.QueryRow(ctx, query, args...).Scan(&item.ID, &item.ApplicationID, &item.TargetResourceID,
		&item.TargetResourceName, &item.TargetResourceKind, &item.DependencyKind, &binding, &item.Required, &item.Status); err != nil {
		return Dependency{}, mapStoreError(err)
	}
	if err := json.Unmarshal(binding, &item.Binding); err != nil {
		return Dependency{}, fmt.Errorf("decode application binding: %w", err)
	}
	return item, nil
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

func (s *store) DeleteDependency(ctx context.Context, projectID, applicationID, dependencyID string) error {
	if _, err := s.Get(ctx, projectID, applicationID); err != nil {
		return err
	}
	var deletedID string
	if err := s.pool.QueryRow(ctx, `DELETE FROM application_dependencies WHERE id=$1::uuid AND application_id=$2::uuid RETURNING id::text`, dependencyID, applicationID).Scan(&deletedID); err != nil {
		return mapStoreError(err)
	}
	return nil
}

func (s *store) ContextResourceIDs(ctx context.Context, scopeID, applicationID string) ([]string, error) {
	query, args := visibleApplications(`SELECT DISTINCT r.id::text
  FROM applications a JOIN projects p ON p.id=a.project_id
  JOIN (
    SELECT application_id, target_resource_id FROM application_instances
    UNION ALL
    SELECT application_id, target_resource_id FROM application_dependencies
  ) relation ON relation.application_id=a.id
  JOIN resources r ON r.id=relation.target_resource_id
 WHERE a.id=$1::uuid AND p.scope_id=$2::uuid AND a.deleted_at IS NULL AND r.deleted_at IS NULL AND r.status <> 'disabled'
 ORDER BY r.id`, []any{applicationID, scopeID}, ctx)
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

func (s *store) workspaceResources(ctx context.Context, projectID string) ([]RelatedResource, error) {
	rows, err := s.pool.Query(ctx, `WITH project_resources AS (
  SELECT r.id,r.name,r.kind,r.status,'project'::text AS role
    FROM resources r JOIN projects p ON p.scope_id=r.scope_id
   WHERE p.id=$1::uuid AND r.deleted_at IS NULL
  UNION
  SELECT r.id,r.name,r.kind,r.status,'instance'::text
    FROM resources r JOIN application_instances i ON i.target_resource_id=r.id
    JOIN applications a ON a.id=i.application_id
   WHERE a.project_id=$1::uuid AND a.deleted_at IS NULL AND r.deleted_at IS NULL
  UNION
  SELECT r.id,r.name,r.kind,r.status,'dependency'::text
    FROM resources r JOIN application_dependencies d ON d.target_resource_id=r.id
    JOIN applications a ON a.id=d.application_id
   WHERE a.project_id=$1::uuid AND a.deleted_at IS NULL AND r.deleted_at IS NULL
)
SELECT id::text,name,kind,status,role FROM project_resources ORDER BY name,id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project workspace resources: %w", err)
	}
	defer rows.Close()
	items := make([]RelatedResource, 0)
	for rows.Next() {
		var item RelatedResource
		if err := rows.Scan(&item.ID, &item.Name, &item.Kind, &item.Status, &item.Role); err != nil {
			return nil, fmt.Errorf("scan project workspace resource: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *store) workspaceAlerts(ctx context.Context, projectID string) ([]Alert, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT f.id::text,f.severity,f.message,f.status
  FROM inspection_findings f
  JOIN inspection_policies policy ON policy.id=f.policy_id
  JOIN resources r ON r.id=f.target_resource_id
  JOIN projects p ON p.scope_id=r.scope_id
 WHERE policy.scope_id=p.scope_id AND p.id=$1::uuid AND f.status='open' AND r.deleted_at IS NULL
 ORDER BY f.severity DESC,f.last_observed_at DESC,f.id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project workspace alerts: %w", err)
	}
	defer rows.Close()
	items := make([]Alert, 0)
	for rows.Next() {
		var item Alert
		if err := rows.Scan(&item.ID, &item.Severity, &item.Title, &item.Status); err != nil {
			return nil, fmt.Errorf("scan project workspace alert: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func insertRelations(ctx context.Context, tx pgx.Tx, applicationID string, instances []CreateInstanceInput, dependencies []CreateDependencyInput) error {
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
VALUES($1::uuid,$2,$3,$4::uuid,$5::jsonb,$6::jsonb,COALESCE(NULLIF($7,''),'unknown'))`, applicationID,
			input.Name, input.RuntimeKind, input.TargetResourceID, selector, logs, input.Status); err != nil {
			return mapStoreError(err)
		}
	}
	for _, input := range dependencies {
		binding, err := json.Marshal(normalizeObject(input.Binding))
		if err != nil {
			return fmt.Errorf("encode application binding: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO application_dependencies(application_id,target_resource_id,dependency_kind,binding,required,status)
VALUES($1::uuid,$2::uuid,$3,$4::jsonb,$5,COALESCE(NULLIF($6,''),'unknown'))`, applicationID,
			input.TargetResourceID, input.DependencyKind, binding, input.Required, input.Status); err != nil {
			return mapStoreError(err)
		}
	}
	return nil
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

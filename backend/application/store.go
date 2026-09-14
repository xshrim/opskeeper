package application

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"opskeeper/backend/authorization"
	"strings"
)

type store struct{ pool *pgxpool.Pool }

func projectVisible(ctx context.Context, scopeExpr string) (string, []any) {
	filter, restricted := authorization.ScopeFilterFromContext(ctx)
	if !restricted {
		return "", nil
	}
	if len(filter.ScopeIDs) == 0 {
		return " AND FALSE", nil
	}
	return " AND " + scopeExpr + " = ANY($1)", []any{filter.ScopeIDs}
}

func NewStore(pool *pgxpool.Pool) Store { return &store{pool: pool} }

const appSelect = `SELECT id::text, project_id::text, name, code, description, icon, status, source, external_uid, labels, created_at, updated_at FROM applications WHERE deleted_at IS NULL`

func scanApp(row pgx.Row) (Application, error) {
	var a Application
	var labels []byte
	err := row.Scan(&a.ID, &a.ProjectID, &a.Name, &a.Code, &a.Description, &a.Icon, &a.Status, &a.Source, &a.ExternalUID, &labels, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return a, ErrNotFound
		}
		return a, err
	}
	_ = json.Unmarshal(labels, &a.Labels)
	if a.Labels == nil {
		a.Labels = map[string]string{}
	}
	return a, nil
}
func (s *store) Create(ctx context.Context, in CreateInput) (Application, error) {
	labels, _ := json.Marshal(in.Labels)
	var id string
	err := s.pool.QueryRow(ctx, `INSERT INTO applications(project_id,name,code,description,icon,source,external_uid,labels) VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8) RETURNING id::text`, in.ProjectID, in.Name, in.Code, in.Description, in.Icon, in.Source, in.ExternalUID, labels).Scan(&id)
	if err != nil {
		return Application{}, err
	}
	return s.Get(ctx, id)
}
func (s *store) Get(ctx context.Context, id string) (Application, error) {
	a, err := scanApp(s.pool.QueryRow(ctx, appSelect+` AND id=$1::uuid`, id))
	if err != nil {
		return a, err
	}
	a.Instances, _ = s.listInstances(ctx, id)
	a.Dependencies, _ = s.listDependencies(ctx, id)
	return a, nil
}
func (s *store) List(ctx context.Context, pid string) ([]Application, error) {
	visibility, visibilityArgs := projectVisible(ctx, "p.scope_id")
	query := appSelect + ` AND project_id=$1::uuid AND EXISTS (SELECT 1 FROM projects p WHERE p.id=applications.project_id` + visibility + `) ORDER BY name`
	args := []any{pid}
	if len(visibilityArgs) > 0 {
		query = strings.Replace(query, "$1", "$2", 1)
		args = append(args, visibilityArgs[0])
	}
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Application{}
	for rows.Next() {
		a, scanErr := scanApp(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		a.Instances, _ = s.listInstances(ctx, a.ID)
		a.Dependencies, _ = s.listDependencies(ctx, a.ID)
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *store) Update(ctx context.Context, id string, in UpdateInput) (Application, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return Application{}, err
	}
	if in.Name != nil {
		current.Name = *in.Name
	}
	if in.Description != nil {
		current.Description = *in.Description
	}
	if in.Icon != nil {
		current.Icon = *in.Icon
	}
	if in.Status != nil {
		current.Status = *in.Status
	}
	if in.Labels != nil {
		current.Labels = *in.Labels
	}
	labels, _ := json.Marshal(current.Labels)
	_, err = s.pool.Exec(ctx, `UPDATE applications SET name=$2,description=$3,icon=$4,status=$5,labels=$6,updated_at=now() WHERE id=$1::uuid AND deleted_at IS NULL`, id, current.Name, current.Description, current.Icon, current.Status, labels)
	if err != nil {
		return Application{}, err
	}
	return s.Get(ctx, id)
}
func (s *store) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE applications SET deleted_at=now(),status='disabled' WHERE id=$1::uuid AND deleted_at IS NULL`, id)
	return err
}
func (s *store) CreateInstance(ctx context.Context, in CreateInstanceInput) (Instance, error) {
	selector, _ := json.Marshal(in.Selector)
	logs, _ := json.Marshal(in.LogBinding)
	var id string
	err := s.pool.QueryRow(ctx, `INSERT INTO application_instances(application_id,name,runtime_kind,target_resource_id,selector,log_binding,status) VALUES($1::uuid,$2,$3,$4::uuid,$5,$6,COALESCE(NULLIF($7,''),'unknown')) RETURNING id::text`, in.ApplicationID, in.Name, in.RuntimeKind, in.TargetResourceID, selector, logs, in.Status).Scan(&id)
	if err != nil {
		return Instance{}, err
	}
	return s.getInstance(ctx, id)
}
func (s *store) getInstance(ctx context.Context, id string) (Instance, error) {
	var i Instance
	var selector, logs []byte
	err := s.pool.QueryRow(ctx, `SELECT i.id::text,i.application_id::text,i.name,i.runtime_kind,i.target_resource_id::text,r.name,r.kind,i.selector,i.log_binding,i.status FROM application_instances i JOIN resources r ON r.id=i.target_resource_id WHERE i.id=$1::uuid`, id).Scan(&i.ID, &i.ApplicationID, &i.Name, &i.RuntimeKind, &i.TargetResourceID, &i.TargetResourceName, &i.TargetResourceKind, &selector, &logs, &i.Status)
	if err != nil {
		return i, err
	}
	_ = json.Unmarshal(selector, &i.Selector)
	_ = json.Unmarshal(logs, &i.LogBinding)
	return i, nil
}
func (s *store) listInstances(ctx context.Context, aid string) ([]Instance, error) {
	rows, err := s.pool.Query(ctx, `SELECT i.id::text,i.application_id::text,i.name,i.runtime_kind,i.target_resource_id::text,r.name,r.kind,i.selector,i.log_binding,i.status FROM application_instances i JOIN resources r ON r.id=i.target_resource_id WHERE i.application_id=$1::uuid ORDER BY i.name`, aid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Instance{}
	for rows.Next() {
		var i Instance
		var selector, logs []byte
		if err := rows.Scan(&i.ID, &i.ApplicationID, &i.Name, &i.RuntimeKind, &i.TargetResourceID, &i.TargetResourceName, &i.TargetResourceKind, &selector, &logs, &i.Status); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(selector, &i.Selector)
		_ = json.Unmarshal(logs, &i.LogBinding)
		out = append(out, i)
	}
	return out, rows.Err()
}
func (s *store) DeleteInstance(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM application_instances WHERE id=$1::uuid`, id)
	return err
}
func (s *store) CreateDependency(ctx context.Context, in CreateDependencyInput) (Dependency, error) {
	binding, _ := json.Marshal(in.Binding)
	var id string
	err := s.pool.QueryRow(ctx, `INSERT INTO application_dependencies(application_id,target_resource_id,dependency_kind,binding,required,status) VALUES($1::uuid,$2::uuid,$3,$4,$5,COALESCE(NULLIF($6,''),'unknown')) RETURNING id::text`, in.ApplicationID, in.TargetResourceID, in.DependencyKind, binding, in.Required, in.Status).Scan(&id)
	if err != nil {
		return Dependency{}, err
	}
	return s.getDependency(ctx, id)
}
func (s *store) getDependency(ctx context.Context, id string) (Dependency, error) {
	var d Dependency
	var binding []byte
	err := s.pool.QueryRow(ctx, `SELECT d.id::text,d.application_id::text,d.target_resource_id::text,r.name,r.kind,d.dependency_kind,d.binding,d.required,d.status FROM application_dependencies d JOIN resources r ON r.id=d.target_resource_id WHERE d.id=$1::uuid`, id).Scan(&d.ID, &d.ApplicationID, &d.TargetResourceID, &d.TargetResourceName, &d.TargetResourceKind, &d.DependencyKind, &binding, &d.Required, &d.Status)
	if err != nil {
		return d, err
	}
	_ = json.Unmarshal(binding, &d.Binding)
	return d, nil
}
func (s *store) listDependencies(ctx context.Context, aid string) ([]Dependency, error) {
	rows, err := s.pool.Query(ctx, `SELECT d.id::text,d.application_id::text,d.target_resource_id::text,r.name,r.kind,d.dependency_kind,d.binding,d.required,d.status FROM application_dependencies d JOIN resources r ON r.id=d.target_resource_id WHERE d.application_id=$1::uuid ORDER BY d.dependency_kind,r.name`, aid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Dependency{}
	for rows.Next() {
		var d Dependency
		var binding []byte
		if err := rows.Scan(&d.ID, &d.ApplicationID, &d.TargetResourceID, &d.TargetResourceName, &d.TargetResourceKind, &d.DependencyKind, &binding, &d.Required, &d.Status); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(binding, &d.Binding)
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *store) DeleteDependency(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM application_dependencies WHERE id=$1::uuid`, id)
	return err
}
func (s *store) Workspace(ctx context.Context, pid string) (Workspace, error) {
	apps, err := s.List(ctx, pid)
	if err != nil {
		return Workspace{}, err
	}
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT r.id::text,r.name,r.kind,r.status,'项目资源' FROM resources r JOIN projects p ON p.scope_id=r.scope_id WHERE p.id=$1::uuid AND r.deleted_at IS NULL UNION SELECT DISTINCT r.id::text,r.name,r.kind,r.status,'关联资源' FROM resources r JOIN application_instances i ON i.target_resource_id=r.id JOIN applications a ON a.id=i.application_id WHERE a.project_id=$1::uuid AND a.deleted_at IS NULL UNION SELECT DISTINCT r.id::text,r.name,r.kind,r.status,'依赖' FROM resources r JOIN application_dependencies d ON d.target_resource_id=r.id JOIN applications a ON a.id=d.application_id WHERE a.project_id=$1::uuid AND a.deleted_at IS NULL ORDER BY 2`, pid)
	if err != nil {
		return Workspace{}, err
	}
	defer rows.Close()
	resources := []RelatedResource{}
	for rows.Next() {
		var r RelatedResource
		if err := rows.Scan(&r.ID, &r.Name, &r.Kind, &r.Status, &r.Role); err != nil {
			return Workspace{}, err
		}
		resources = append(resources, r)
	}
	summary := ProjectSummary{ProjectID: pid, Applications: len(apps), Resources: len(resources)}
	for _, a := range apps {
		summary.Instances += len(a.Instances)
		summary.Dependencies += len(a.Dependencies)
	}
	return Workspace{Summary: summary, Resources: resources, Alerts: []Alert{}, Applications: apps}, rows.Err()
}

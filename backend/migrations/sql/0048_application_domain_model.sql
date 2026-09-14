CREATE TABLE applications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    code text NOT NULL CHECK (code ~ '^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$'),
    description text NOT NULL DEFAULT '',
    icon text NOT NULL DEFAULT 'AppWindow',
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled','unknown')),
    source text NOT NULL DEFAULT 'manual',
    external_uid text NOT NULL DEFAULT '',
    labels jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (project_id, code)
);
CREATE INDEX applications_project_idx ON applications(project_id) WHERE deleted_at IS NULL;

CREATE TABLE application_instances (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id uuid NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    runtime_kind text NOT NULL CHECK (runtime_kind IN ('virtual_machine','containerized','cloud_native')),
    target_resource_id uuid NOT NULL REFERENCES resources(id),
    selector jsonb NOT NULL DEFAULT '{}'::jsonb,
    log_binding jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL DEFAULT 'unknown' CHECK (status IN ('active','disabled','unknown')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (application_id, name)
);
CREATE INDEX application_instances_application_idx ON application_instances(application_id);
CREATE INDEX application_instances_target_idx ON application_instances(target_resource_id);

CREATE TABLE application_dependencies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id uuid NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    target_resource_id uuid NOT NULL REFERENCES resources(id),
    dependency_kind text NOT NULL CHECK (dependency_kind IN ('messaging','cache','database','registry','configuration','repository','storage','observability','other')),
    binding jsonb NOT NULL DEFAULT '{}'::jsonb,
    required boolean NOT NULL DEFAULT true,
    status text NOT NULL DEFAULT 'unknown' CHECK (status IN ('active','disabled','unknown')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (application_id, target_resource_id, dependency_kind)
);
CREATE INDEX application_dependencies_application_idx ON application_dependencies(application_id);
CREATE INDEX application_dependencies_target_idx ON application_dependencies(target_resource_id);

CREATE OR REPLACE FUNCTION validate_application_target()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE project_scope uuid; target_scope uuid;
BEGIN
  SELECT p.scope_id INTO project_scope FROM applications a JOIN projects p ON p.id=a.project_id WHERE a.id=NEW.application_id AND a.deleted_at IS NULL;
  SELECT scope_id INTO target_scope FROM resources WHERE id=NEW.target_resource_id AND deleted_at IS NULL;
  IF project_scope IS NULL OR target_scope IS NULL OR NOT resource_scope_contains(target_scope, project_scope) THEN
    RAISE EXCEPTION 'application target resource is outside project scope' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END; $$;
CREATE TRIGGER application_instances_validate_target BEFORE INSERT OR UPDATE OF application_id,target_resource_id ON application_instances FOR EACH ROW EXECUTE FUNCTION validate_application_target();
CREATE TRIGGER application_dependencies_validate_target BEFORE INSERT OR UPDATE OF application_id,target_resource_id ON application_dependencies FOR EACH ROW EXECUTE FUNCTION validate_application_target();

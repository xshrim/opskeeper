CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE scopes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id text NOT NULL DEFAULT 'default',
    scope_type text NOT NULL CHECK (scope_type IN ('platform', 'team', 'project')),
    parent_scope_id uuid REFERENCES scopes(id),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CHECK (
        (scope_type = 'platform' AND parent_scope_id IS NULL)
        OR (scope_type IN ('team', 'project') AND parent_scope_id IS NOT NULL)
    )
);

CREATE INDEX scopes_parent_scope_id_idx ON scopes(parent_scope_id) WHERE deleted_at IS NULL;
CREATE INDEX scopes_tenant_type_idx ON scopes(tenant_id, scope_type) WHERE deleted_at IS NULL;

CREATE OR REPLACE FUNCTION validate_scope_parent()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    parent_type text;
    parent_tenant text;
BEGIN
    IF NEW.scope_type = 'platform' THEN
        IF NEW.parent_scope_id IS NOT NULL THEN
            RAISE EXCEPTION 'platform scope cannot have a parent' USING ERRCODE = '23514';
        END IF;
        RETURN NEW;
    END IF;

    SELECT scope_type, tenant_id
      INTO parent_type, parent_tenant
      FROM scopes
     WHERE id = NEW.parent_scope_id
       AND deleted_at IS NULL;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'scope parent does not exist' USING ERRCODE = '23503';
    END IF;
    IF parent_tenant <> NEW.tenant_id THEN
        RAISE EXCEPTION 'scope parent belongs to a different tenant' USING ERRCODE = '23514';
    END IF;
    IF NEW.scope_type = 'team' AND parent_type <> 'platform' THEN
        RAISE EXCEPTION 'team scope parent must be platform' USING ERRCODE = '23514';
    END IF;
    IF NEW.scope_type = 'project' AND parent_type <> 'team' THEN
        RAISE EXCEPTION 'project scope parent must be team' USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER scopes_validate_parent
BEFORE INSERT OR UPDATE OF tenant_id, scope_type, parent_scope_id ON scopes
FOR EACH ROW EXECUTE FUNCTION validate_scope_parent();

CREATE TABLE platforms (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL UNIQUE REFERENCES scopes(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    code text NOT NULL UNIQUE CHECK (code ~ '^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$'),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE TABLE teams (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL UNIQUE REFERENCES scopes(id),
    platform_id uuid NOT NULL REFERENCES platforms(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    code text NOT NULL CHECK (code ~ '^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$'),
    labels jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(labels) = 'object'),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (platform_id, code)
);

CREATE INDEX teams_platform_id_idx ON teams(platform_id) WHERE deleted_at IS NULL;

CREATE TABLE projects (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL UNIQUE REFERENCES scopes(id),
    platform_id uuid NOT NULL REFERENCES platforms(id),
    team_id uuid NOT NULL REFERENCES teams(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    code text NOT NULL CHECK (code ~ '^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$'),
    labels jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(labels) = 'object'),
    source text NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'kubernetes')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (team_id, code)
);

CREATE INDEX projects_platform_id_idx ON projects(platform_id) WHERE deleted_at IS NULL;
CREATE INDEX projects_team_id_idx ON projects(team_id) WHERE deleted_at IS NULL;

CREATE OR REPLACE FUNCTION validate_organization_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    actual_scope_type text;
    actual_parent uuid;
    expected_parent uuid;
BEGIN
    SELECT scope_type, parent_scope_id
      INTO actual_scope_type, actual_parent
      FROM scopes
     WHERE id = NEW.scope_id
       AND deleted_at IS NULL;

    IF TG_TABLE_NAME = 'platforms' THEN
        IF actual_scope_type <> 'platform' OR actual_parent IS NOT NULL THEN
            RAISE EXCEPTION 'platform must reference a platform root scope' USING ERRCODE = '23514';
        END IF;
    ELSIF TG_TABLE_NAME = 'teams' THEN
        SELECT scope_id INTO expected_parent FROM platforms WHERE id = NEW.platform_id AND deleted_at IS NULL;
        IF actual_scope_type <> 'team' OR actual_parent IS DISTINCT FROM expected_parent THEN
            RAISE EXCEPTION 'team scope does not match platform' USING ERRCODE = '23514';
        END IF;
    ELSIF TG_TABLE_NAME = 'projects' THEN
        SELECT scope_id INTO expected_parent FROM teams WHERE id = NEW.team_id AND platform_id = NEW.platform_id AND deleted_at IS NULL;
        IF actual_scope_type <> 'project' OR actual_parent IS DISTINCT FROM expected_parent THEN
            RAISE EXCEPTION 'project scope does not match team and platform' USING ERRCODE = '23514';
        END IF;
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER platforms_validate_scope
BEFORE INSERT OR UPDATE OF scope_id ON platforms
FOR EACH ROW EXECUTE FUNCTION validate_organization_scope();

CREATE TRIGGER teams_validate_scope
BEFORE INSERT OR UPDATE OF scope_id, platform_id ON teams
FOR EACH ROW EXECUTE FUNCTION validate_organization_scope();

CREATE TRIGGER projects_validate_scope
BEFORE INSERT OR UPDATE OF scope_id, platform_id, team_id ON projects
FOR EACH ROW EXECUTE FUNCTION validate_organization_scope();

DO $$
DECLARE
    root_scope_id uuid;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM platforms WHERE code = 'default' AND deleted_at IS NULL) THEN
        INSERT INTO scopes (scope_type, status)
        VALUES ('platform', 'active')
        RETURNING id INTO root_scope_id;

        INSERT INTO platforms (scope_id, name, code, status)
        VALUES (root_scope_id, 'OpsKeeper', 'default', 'active');
    END IF;
END;
$$;

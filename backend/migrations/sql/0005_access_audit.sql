CREATE TABLE groups (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    description text NOT NULL DEFAULT '' CHECK (length(description) <= 500),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX groups_scope_name_unique ON groups(scope_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX groups_scope_idx ON groups(scope_id) WHERE deleted_at IS NULL;

CREATE TABLE group_members (
    group_id uuid NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (group_id, user_id)
);

CREATE INDEX group_members_user_idx ON group_members(user_id, group_id);

ALTER TABLE role_bindings DROP CONSTRAINT role_bindings_subject_id_fkey;
ALTER TABLE role_bindings DROP CONSTRAINT role_bindings_subject_type_check;
ALTER TABLE role_bindings ADD CONSTRAINT role_bindings_subject_type_check CHECK (subject_type IN ('user', 'group'));

CREATE OR REPLACE FUNCTION validate_role_binding()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    role_scope_type text;
    binding_scope_type text;
    subject_exists boolean;
BEGIN
    SELECT scope_type INTO role_scope_type FROM roles WHERE id = NEW.role_id;
    SELECT scope_type INTO binding_scope_type
      FROM scopes WHERE id = NEW.scope_id AND deleted_at IS NULL;
    IF NEW.subject_type = 'user' THEN
        SELECT EXISTS (SELECT 1 FROM users WHERE id = NEW.subject_id AND deleted_at IS NULL)
          INTO subject_exists;
    ELSE
        SELECT EXISTS (SELECT 1 FROM groups WHERE id = NEW.subject_id AND deleted_at IS NULL AND status = 'active')
          INTO subject_exists;
    END IF;

    IF role_scope_type IS NULL OR binding_scope_type IS NULL OR role_scope_type <> binding_scope_type THEN
        RAISE EXCEPTION 'role binding scope type does not match role' USING ERRCODE = '23514';
    END IF;
    IF NOT subject_exists THEN
        RAISE EXCEPTION 'role binding subject does not exist or is inactive' USING ERRCODE = '23503';
    END IF;
    RETURN NEW;
END;
$$;

INSERT INTO role_permissions (role_id, permission)
SELECT roles.id, 'member:grant'
  FROM roles
 WHERE roles.name IN ('PlatformAdmin', 'TeamAdmin', 'ProjectAdmin')
ON CONFLICT DO NOTHING;

CREATE TABLE audit_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    action text NOT NULL CHECK (length(btrim(action)) BETWEEN 1 AND 120),
    target_type text NOT NULL DEFAULT '',
    target_id text NOT NULL DEFAULT '',
    scope_id uuid REFERENCES scopes(id) ON DELETE SET NULL,
    result text NOT NULL CHECK (result IN ('success', 'failure')),
    request_id text NOT NULL DEFAULT '' CHECK (length(request_id) <= 128),
    client_ip text NOT NULL DEFAULT '' CHECK (length(client_ip) <= 255),
    details jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details) = 'object'),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_events_created_at_idx ON audit_events(created_at DESC);
CREATE INDEX audit_events_actor_idx ON audit_events(actor_user_id, created_at DESC);
CREATE INDEX audit_events_scope_idx ON audit_events(scope_id, created_at DESC);

CREATE TABLE authorization_revision (
    id boolean PRIMARY KEY DEFAULT true CHECK (id),
    revision bigint NOT NULL DEFAULT 1
);

INSERT INTO authorization_revision (id, revision) VALUES (true, 1);

CREATE OR REPLACE FUNCTION bump_authorization_revision()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE authorization_revision SET revision = revision + 1 WHERE id = true;
    RETURN COALESCE(NEW, OLD);
END;
$$;

CREATE TRIGGER users_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON users
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER scopes_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON scopes
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER groups_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON groups
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER group_members_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON group_members
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER roles_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON roles
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER role_permissions_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON role_permissions
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER role_bindings_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON role_bindings
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

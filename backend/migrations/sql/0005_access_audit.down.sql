DROP TRIGGER IF EXISTS role_bindings_authorization_revision ON role_bindings;
DROP TRIGGER IF EXISTS role_permissions_authorization_revision ON role_permissions;
DROP TRIGGER IF EXISTS roles_authorization_revision ON roles;
DROP TRIGGER IF EXISTS group_members_authorization_revision ON group_members;
DROP TRIGGER IF EXISTS groups_authorization_revision ON groups;
DROP TRIGGER IF EXISTS scopes_authorization_revision ON scopes;
DROP TRIGGER IF EXISTS users_authorization_revision ON users;
DROP FUNCTION IF EXISTS bump_authorization_revision();
DROP TABLE authorization_revision;

DROP TABLE audit_events;
DROP TABLE group_members;
DROP TABLE groups;

DELETE FROM role_bindings WHERE subject_type = 'group';
ALTER TABLE role_bindings DROP CONSTRAINT role_bindings_subject_type_check;
ALTER TABLE role_bindings ADD CONSTRAINT role_bindings_subject_type_check CHECK (subject_type = 'user');
ALTER TABLE role_bindings ADD CONSTRAINT role_bindings_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES users(id);

CREATE OR REPLACE FUNCTION validate_role_binding()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    role_scope_type text;
    binding_scope_type text;
    user_exists boolean;
BEGIN
    SELECT scope_type INTO role_scope_type FROM roles WHERE id = NEW.role_id;
    SELECT scope_type INTO binding_scope_type
      FROM scopes WHERE id = NEW.scope_id AND deleted_at IS NULL;
    SELECT EXISTS (SELECT 1 FROM users WHERE id = NEW.subject_id AND deleted_at IS NULL)
      INTO user_exists;
    IF role_scope_type IS NULL OR binding_scope_type IS NULL OR role_scope_type <> binding_scope_type THEN
        RAISE EXCEPTION 'role binding scope type does not match role' USING ERRCODE = '23514';
    END IF;
    IF NOT user_exists THEN
        RAISE EXCEPTION 'role binding subject user does not exist' USING ERRCODE = '23503';
    END IF;
    RETURN NEW;
END;
$$;

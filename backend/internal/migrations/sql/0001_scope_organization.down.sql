DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS platforms;

DROP FUNCTION IF EXISTS validate_organization_scope();

DROP TRIGGER IF EXISTS scopes_validate_parent ON scopes;
DROP FUNCTION IF EXISTS validate_scope_parent();
DROP TABLE IF EXISTS scopes;

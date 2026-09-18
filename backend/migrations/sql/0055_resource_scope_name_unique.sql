DROP INDEX IF EXISTS resources_scope_kind_name_unique;

CREATE UNIQUE INDEX resources_scope_name_unique
    ON resources(scope_id, lower(name)) WHERE deleted_at IS NULL;

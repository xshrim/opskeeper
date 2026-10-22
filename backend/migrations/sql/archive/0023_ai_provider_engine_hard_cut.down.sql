DROP TRIGGER IF EXISTS scope_ai_provider_bindings_updated_at ON scope_ai_provider_bindings;
DROP TRIGGER IF EXISTS scope_ai_provider_bindings_authorization_revision ON scope_ai_provider_bindings;
DROP TRIGGER IF EXISTS scope_ai_provider_bindings_validate ON scope_ai_provider_bindings;
DROP FUNCTION IF EXISTS scope_ai_provider_bindings_touch();
DROP FUNCTION IF EXISTS validate_scope_ai_provider_binding();
DROP TABLE IF EXISTS scope_ai_provider_bindings;
ALTER TABLE diagnosis_sessions
    DROP COLUMN IF EXISTS model_name,
    DROP COLUMN IF EXISTS provider_resource_id;

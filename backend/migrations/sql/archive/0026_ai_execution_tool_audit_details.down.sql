DROP INDEX IF EXISTS ai_execution_tool_calls_provider_idx;
ALTER TABLE ai_execution_tool_calls
    DROP COLUMN IF EXISTS provider_resource_id,
    DROP COLUMN IF EXISTS model_name,
    DROP COLUMN IF EXISTS error_code,
    DROP COLUMN IF EXISTS started_at,
    DROP COLUMN IF EXISTS completed_at,
    DROP COLUMN IF EXISTS duration_ms;

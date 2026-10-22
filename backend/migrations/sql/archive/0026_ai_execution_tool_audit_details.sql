ALTER TABLE ai_execution_tool_calls
    ADD COLUMN provider_resource_id text,
    ADD COLUMN model_name text,
    ADD COLUMN error_code text,
    ADD COLUMN started_at timestamptz,
    ADD COLUMN completed_at timestamptz,
    ADD COLUMN duration_ms bigint NOT NULL DEFAULT 0 CHECK (duration_ms >= 0);

CREATE INDEX ai_execution_tool_calls_provider_idx
    ON ai_execution_tool_calls (provider_resource_id);

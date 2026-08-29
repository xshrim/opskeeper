ALTER TABLE ai_endpoint_scope_defaults
    ADD COLUMN model_name text NOT NULL DEFAULT '__endpoint__'
    CHECK (length(btrim(model_name)) BETWEEN 1 AND 200);

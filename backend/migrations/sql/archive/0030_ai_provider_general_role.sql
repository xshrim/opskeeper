-- Rename the catch-all AIProvider routing role from "default" to "general".
ALTER TABLE scope_ai_provider_bindings
    DROP CONSTRAINT IF EXISTS scope_ai_provider_bindings_tag_check;

UPDATE scope_ai_provider_bindings
   SET tag = 'general'
 WHERE tag = 'default';

ALTER TABLE scope_ai_provider_bindings
    ADD CONSTRAINT scope_ai_provider_bindings_tag_check
    CHECK (tag IN ('general', 'diagnosis', 'inspection', 'workflow'));

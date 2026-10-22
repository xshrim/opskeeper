DROP TRIGGER IF EXISTS engine_scope_bindings_authorization_revision ON engine_scope_bindings;
DROP TRIGGER IF EXISTS engines_authorization_revision ON engines;
DROP TABLE IF EXISTS engine_scope_bindings;
DROP TABLE IF EXISTS engines;

ALTER TABLE provider_scope_bindings DROP CONSTRAINT IF EXISTS provider_scope_bindings_tag_check;
UPDATE provider_scope_bindings SET tag = 'default' WHERE tag = 'general';
ALTER TABLE provider_scope_bindings
  ADD CONSTRAINT provider_scope_bindings_tag_check CHECK (tag IN ('default', 'diagnosis', 'inspection', 'workflow'));

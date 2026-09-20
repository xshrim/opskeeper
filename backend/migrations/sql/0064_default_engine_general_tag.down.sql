-- Restore the pre-migration defaults when rolling back this migration.
INSERT INTO engine_scope_bindings (scope_id, engine_id, tag)
SELECT engine.scope_id, engine.id, tags.tag
  FROM engines engine
 CROSS JOIN unnest(ARRAY['diagnosis', 'inspection', 'workflow']) AS tags(tag)
 WHERE engine.name = 'OpsKeeper Engine'
ON CONFLICT (scope_id, tag) DO NOTHING;

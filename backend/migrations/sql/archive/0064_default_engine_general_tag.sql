-- The built-in Engine should provide the general fallback by default.
-- Scenario-specific defaults remain opt-in through the Engine editor.
DELETE FROM engine_scope_bindings binding
USING engines engine
WHERE binding.engine_id = engine.id
  AND engine.name = 'OpsKeeper Engine'
  AND binding.tag <> 'general';

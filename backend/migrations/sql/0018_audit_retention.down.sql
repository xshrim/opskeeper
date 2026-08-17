DROP FUNCTION IF EXISTS prune_audit_events(timestamptz, text, text);
DROP TRIGGER IF EXISTS audit_retention_runs_append_only ON audit_retention_runs;
DROP TRIGGER IF EXISTS audit_events_append_only ON audit_events;
DROP FUNCTION IF EXISTS protect_audit_rows();
DROP TABLE IF EXISTS audit_retention_runs;

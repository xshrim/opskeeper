DROP INDEX IF EXISTS diagnosis_sessions_application_idx;
ALTER TABLE diagnosis_sessions DROP COLUMN IF EXISTS application_id;

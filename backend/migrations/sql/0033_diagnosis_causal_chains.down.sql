DROP TABLE IF EXISTS diagnosis_causal_chains;
DROP INDEX IF EXISTS diagnosis_evidence_run_idx;
ALTER TABLE diagnosis_evidence DROP COLUMN IF EXISTS run_id;
DROP TABLE IF EXISTS diagnosis_runs;

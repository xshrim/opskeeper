ALTER TABLE diagnosis_sessions
    ADD COLUMN collaboration smallint NOT NULL DEFAULT 2 CHECK (collaboration BETWEEN 1 AND 5);

ALTER TABLE diagnosis_runs
    ADD COLUMN collaboration smallint NOT NULL DEFAULT 2 CHECK (collaboration BETWEEN 1 AND 5);

ALTER TABLE diagnosis_sessions
    ADD COLUMN application_id uuid REFERENCES applications(id) ON DELETE SET NULL;

CREATE INDEX diagnosis_sessions_application_idx
    ON diagnosis_sessions(application_id)
    WHERE application_id IS NOT NULL;

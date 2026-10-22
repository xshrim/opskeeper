ALTER TABLE diagnosis_messages
    ADD COLUMN metadata jsonb NOT NULL DEFAULT '{}'::jsonb;

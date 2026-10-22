ALTER TABLE teams
    ADD COLUMN code text NOT NULL DEFAULT ('team-' || replace(gen_random_uuid()::text, '-', ''))
    CHECK (code ~ '^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$');

ALTER TABLE teams ADD CONSTRAINT teams_platform_id_code_key UNIQUE (platform_id, code);

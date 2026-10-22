ALTER TABLE teams
    ADD COLUMN owner_ids uuid[] NOT NULL DEFAULT '{}';

ALTER TABLE projects
    ADD COLUMN importance_level text NOT NULL DEFAULT 'ITP4'
        CHECK (importance_level IN ('ITP1', 'ITP2', 'ITP3', 'ITP4')),
    ADD COLUMN technical_owner_ids uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN operations_owner_ids uuid[] NOT NULL DEFAULT '{}';

ALTER TABLE applications
    ADD COLUMN critical boolean NOT NULL DEFAULT false,
    ADD COLUMN technical_owner_ids uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN operations_owner_ids uuid[] NOT NULL DEFAULT '{}';

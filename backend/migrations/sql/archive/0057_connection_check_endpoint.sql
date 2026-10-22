ALTER TABLE resource_connection_checks
    ADD COLUMN endpoint text NOT NULL DEFAULT '';

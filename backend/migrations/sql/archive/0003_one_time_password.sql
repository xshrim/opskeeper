ALTER TABLE credentials
    ADD COLUMN must_change_password boolean NOT NULL DEFAULT false;

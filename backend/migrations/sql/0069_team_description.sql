ALTER TABLE teams
    ADD COLUMN description text NOT NULL DEFAULT '' CHECK (length(description) <= 1000);

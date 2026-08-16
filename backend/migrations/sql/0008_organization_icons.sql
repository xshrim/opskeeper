ALTER TABLE teams
    ADD COLUMN icon text NOT NULL DEFAULT 'team';

ALTER TABLE projects
    ADD COLUMN icon text NOT NULL DEFAULT 'project';

ALTER TABLE platforms
    ADD COLUMN icon text NOT NULL DEFAULT 'platform';

ALTER TABLE teams
    ADD CONSTRAINT teams_icon_length CHECK (length(icon) BETWEEN 1 AND 64);

ALTER TABLE projects
    ADD CONSTRAINT projects_icon_length CHECK (length(icon) BETWEEN 1 AND 64);

ALTER TABLE platforms
    ADD CONSTRAINT platforms_icon_length CHECK (length(icon) BETWEEN 1 AND 64);

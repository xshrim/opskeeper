ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_icon_length;
ALTER TABLE teams DROP CONSTRAINT IF EXISTS teams_icon_length;
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_icon_length;
ALTER TABLE platforms DROP CONSTRAINT IF EXISTS platforms_icon_length;

ALTER TABLE teams ADD CONSTRAINT teams_icon_length CHECK (length(icon) BETWEEN 1 AND 64);
ALTER TABLE projects ADD CONSTRAINT projects_icon_length CHECK (length(icon) BETWEEN 1 AND 64);
ALTER TABLE platforms ADD CONSTRAINT platforms_icon_length CHECK (length(icon) BETWEEN 1 AND 64);

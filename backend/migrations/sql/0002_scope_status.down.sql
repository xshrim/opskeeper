ALTER TABLE platforms ADD COLUMN status text;
UPDATE platforms AS organization
   SET status = scope.status
  FROM scopes AS scope
 WHERE scope.id = organization.scope_id;
ALTER TABLE platforms ALTER COLUMN status SET DEFAULT 'active';
ALTER TABLE platforms ALTER COLUMN status SET NOT NULL;
ALTER TABLE platforms ADD CONSTRAINT platforms_status_check CHECK (status IN ('active', 'disabled'));

ALTER TABLE teams ADD COLUMN status text;
UPDATE teams AS organization
   SET status = scope.status
  FROM scopes AS scope
 WHERE scope.id = organization.scope_id;
ALTER TABLE teams ALTER COLUMN status SET DEFAULT 'active';
ALTER TABLE teams ALTER COLUMN status SET NOT NULL;
ALTER TABLE teams ADD CONSTRAINT teams_status_check CHECK (status IN ('active', 'disabled'));

ALTER TABLE projects ADD COLUMN status text;
UPDATE projects AS organization
   SET status = scope.status
  FROM scopes AS scope
 WHERE scope.id = organization.scope_id;
ALTER TABLE projects ALTER COLUMN status SET DEFAULT 'active';
ALTER TABLE projects ALTER COLUMN status SET NOT NULL;
ALTER TABLE projects ADD CONSTRAINT projects_status_check CHECK (status IN ('active', 'disabled'));

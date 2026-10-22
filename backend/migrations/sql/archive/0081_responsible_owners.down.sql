ALTER TABLE applications
    DROP COLUMN IF EXISTS operations_owner_ids,
    DROP COLUMN IF EXISTS technical_owner_ids,
    DROP COLUMN IF EXISTS critical;

ALTER TABLE projects
    DROP COLUMN IF EXISTS operations_owner_ids,
    DROP COLUMN IF EXISTS technical_owner_ids,
    DROP COLUMN IF EXISTS importance_level;

ALTER TABLE teams
    DROP COLUMN IF EXISTS owner_ids;

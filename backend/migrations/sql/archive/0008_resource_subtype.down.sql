DROP INDEX IF EXISTS resources_subtype_idx;
ALTER TABLE resources DROP COLUMN IF EXISTS subtype;

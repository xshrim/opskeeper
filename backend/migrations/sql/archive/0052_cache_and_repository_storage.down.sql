DROP TABLE IF EXISTS repository_bundles;
DROP INDEX IF EXISTS cache_entries_expiry_idx;
DROP TABLE IF EXISTS cache_entries;
UPDATE resource_schemas
SET schema = jsonb_set(schema, '{properties,storage_backend,enum}', '["local","s3"]'::jsonb, true)
WHERE kind = 'Repository' AND schema ? 'properties' AND schema->'properties' ? 'storage_backend';

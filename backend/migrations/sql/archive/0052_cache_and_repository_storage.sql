CREATE UNLOGGED TABLE cache_entries (
    cache_key text PRIMARY KEY,
    value jsonb NOT NULL,
    expires_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX cache_entries_expiry_idx ON cache_entries(expires_at);

CREATE TABLE repository_bundles (
    resource_id uuid PRIMARY KEY REFERENCES resources(id) ON DELETE CASCADE,
    bundle bytea NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

UPDATE resource_schemas
SET schema = jsonb_set(schema, '{properties,storage_backend,enum}', '["local","postgres","s3"]'::jsonb, true)
WHERE kind = 'Repository' AND schema ? 'properties' AND schema->'properties' ? 'storage_backend';

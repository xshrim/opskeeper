-- Keep resource classification queryable without removing the existing JSON config.
ALTER TABLE resources
    ADD COLUMN subtype text NOT NULL DEFAULT ''
    CHECK (length(btrim(subtype)) <= 120);

UPDATE resources
   SET subtype = btrim(config ->> 'subtype')
 WHERE btrim(COALESCE(config ->> 'subtype', '')) <> '';

CREATE INDEX resources_subtype_idx ON resources(subtype) WHERE deleted_at IS NULL;

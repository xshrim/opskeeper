ALTER TABLE personas
    ADD COLUMN tags jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD CONSTRAINT personas_tags_check CHECK (jsonb_typeof(tags) = 'array');

UPDATE personas
   SET tags = config->'tags'
 WHERE jsonb_typeof(config->'tags') = 'array';

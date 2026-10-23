ALTER TABLE personas ADD COLUMN identifier text;

WITH normalized AS (
    SELECT id,
           scope_id,
           COALESCE(
               NULLIF(trim(both '-' FROM regexp_replace(lower(COALESCE(config->>'identifier', name)), '[^a-z0-9_-]+', '-', 'g')), ''),
               'persona-' || left(replace(id::text, '-', ''), 8)
           ) AS base_identifier
      FROM personas
), ranked AS (
    SELECT id,
           base_identifier,
           row_number() OVER (PARTITION BY scope_id, lower(base_identifier) ORDER BY id) AS duplicate_number
      FROM normalized
)
UPDATE personas AS persona
   SET identifier = CASE
       WHEN ranked.duplicate_number = 1 THEN ranked.base_identifier
       ELSE ranked.base_identifier || '-' || left(replace(persona.id::text, '-', ''), 8)
   END
  FROM ranked
 WHERE persona.id = ranked.id;

ALTER TABLE personas ALTER COLUMN identifier SET NOT NULL;
ALTER TABLE personas ADD CONSTRAINT personas_identifier_check
    CHECK (length(btrim(identifier)) BETWEEN 1 AND 200);
ALTER TABLE personas ADD CONSTRAINT personas_identifier_no_whitespace
    CHECK (identifier !~ '[[:space:]]');

CREATE UNIQUE INDEX personas_scope_identifier_unique
    ON personas (scope_id, lower(identifier))
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX skills_scope_name_unique
    ON skills (scope_id, lower(name))
    WHERE deleted_at IS NULL;

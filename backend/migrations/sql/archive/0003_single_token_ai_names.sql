UPDATE skills
   SET name = regexp_replace(btrim(name), '[[:space:]]+', '-', 'g')
 WHERE name ~ '[[:space:]]';

UPDATE personas
   SET name = regexp_replace(btrim(name), '[[:space:]]+', '-', 'g')
 WHERE name ~ '[[:space:]]';

ALTER TABLE skills
    ADD CONSTRAINT skills_name_no_whitespace CHECK (name !~ '[[:space:]]');

ALTER TABLE personas
    ADD CONSTRAINT personas_name_no_whitespace CHECK (name !~ '[[:space:]]');

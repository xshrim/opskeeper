ALTER TABLE users
    ADD COLUMN username text,
    ADD COLUMN phone text;

UPDATE users
   SET username = 'user-' || left(replace(id::text, '-', ''), 16)
 WHERE username IS NULL;

ALTER TABLE users
    ALTER COLUMN email DROP NOT NULL,
    ALTER COLUMN username SET DEFAULT ('user-' || left(replace(gen_random_uuid()::text, '-', ''), 16)),
    ALTER COLUMN username SET NOT NULL,
    ADD CONSTRAINT users_username_format CHECK (username ~ '^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$');

UPDATE users SET email = NULL WHERE btrim(email) = '';

ALTER TABLE users
    ADD CONSTRAINT users_email_nonempty CHECK (email IS NULL OR length(btrim(email)) > 0),
    ADD CONSTRAINT users_phone_format CHECK (phone IS NULL OR phone ~ '^\+?[0-9]{3,32}$');

CREATE UNIQUE INDEX users_username_unique ON users (lower(username)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX users_phone_unique ON users (phone) WHERE deleted_at IS NULL AND phone IS NOT NULL AND phone <> '';

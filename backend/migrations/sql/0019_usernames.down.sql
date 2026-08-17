DROP INDEX IF EXISTS users_username_unique;
DROP INDEX IF EXISTS users_phone_unique;
UPDATE users SET email = username || '@invalid.local' WHERE email IS NULL;
ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_username_format,
    DROP CONSTRAINT IF EXISTS users_email_nonempty,
    DROP CONSTRAINT IF EXISTS users_phone_format,
    DROP COLUMN IF EXISTS username,
    DROP COLUMN IF EXISTS phone;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;

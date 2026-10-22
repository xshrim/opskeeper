ALTER TABLE users
    DROP COLUMN must_change_password,
    DROP COLUMN password_changed_at,
    DROP COLUMN password_hash;

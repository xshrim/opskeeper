ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_avatar_data_check,
    DROP COLUMN IF EXISTS avatar_updated_at,
    DROP COLUMN IF EXISTS avatar_data,
    DROP COLUMN IF EXISTS avatar_content_type,
    DROP COLUMN IF EXISTS sidebar_collapsed,
    DROP COLUMN IF EXISTS sidebar_mode,
    DROP COLUMN IF EXISTS theme;

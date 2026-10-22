ALTER TABLE users
    ADD COLUMN theme text NOT NULL DEFAULT 'auto' CHECK (theme IN ('auto', 'light', 'dark')),
    ADD COLUMN sidebar_mode text NOT NULL DEFAULT 'fixed' CHECK (sidebar_mode IN ('fixed', 'hover')),
    ADD COLUMN sidebar_collapsed boolean NOT NULL DEFAULT false,
    ADD COLUMN avatar_content_type text CHECK (avatar_content_type IN ('image/jpeg', 'image/png')),
    ADD COLUMN avatar_data bytea,
    ADD COLUMN avatar_updated_at timestamptz,
    ADD CONSTRAINT users_avatar_data_check CHECK (
        (avatar_data IS NULL AND avatar_content_type IS NULL AND avatar_updated_at IS NULL)
        OR (avatar_data IS NOT NULL AND avatar_content_type IS NOT NULL AND avatar_updated_at IS NOT NULL)
    );

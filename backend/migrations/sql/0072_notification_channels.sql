ALTER TABLE notification_channel_versions
    ADD COLUMN key_version text NOT NULL DEFAULT '';

CREATE TABLE notification_channel_test_limits (
    channel_id uuid PRIMARY KEY,
    scope_id uuid NOT NULL REFERENCES scopes(id),
    window_started_at timestamptz NOT NULL,
    attempts integer NOT NULL CHECK (attempts > 0),
    FOREIGN KEY (channel_id, scope_id) REFERENCES notification_channels(id, scope_id) ON DELETE CASCADE
);

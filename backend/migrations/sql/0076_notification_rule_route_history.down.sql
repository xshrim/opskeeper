DROP INDEX IF EXISTS notification_rule_routes_active_unique;

ALTER TABLE notification_rule_routes
    DROP COLUMN IF EXISTS retired_at,
    ADD CONSTRAINT notification_rule_routes_rule_id_channel_id_template_version_id_key
        UNIQUE (rule_id, channel_id, template_version_id);

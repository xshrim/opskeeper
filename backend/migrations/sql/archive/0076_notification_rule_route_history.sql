ALTER TABLE notification_rule_routes
    ADD COLUMN retired_at timestamptz;

DO $$
DECLARE
    route_unique_constraint text;
BEGIN
    SELECT constraint_row.conname
      INTO route_unique_constraint
      FROM pg_constraint constraint_row
     WHERE constraint_row.conrelid = 'notification_rule_routes'::regclass
       AND constraint_row.contype = 'u'
       AND pg_get_constraintdef(constraint_row.oid) = 'UNIQUE (rule_id, channel_id, template_version_id)';

    IF route_unique_constraint IS NULL THEN
        RAISE EXCEPTION 'notification rule route uniqueness constraint was not found';
    END IF;

    EXECUTE format('ALTER TABLE notification_rule_routes DROP CONSTRAINT %I', route_unique_constraint);
END;
$$;

CREATE UNIQUE INDEX notification_rule_routes_active_unique
    ON notification_rule_routes(rule_id, channel_id, template_version_id)
    WHERE retired_at IS NULL;

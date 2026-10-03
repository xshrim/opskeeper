DROP TRIGGER IF EXISTS notification_deliveries_route_immutable ON notification_deliveries;
DROP TRIGGER IF EXISTS notification_delivery_attempts_append_only ON notification_delivery_attempts;
DROP TRIGGER IF EXISTS notification_events_immutable ON notification_events;
DROP FUNCTION IF EXISTS reject_notification_route_snapshot_mutation();
DROP FUNCTION IF EXISTS reject_notification_attempt_mutation();
DROP FUNCTION IF EXISTS reject_notification_event_mutation();
DROP TABLE IF EXISTS notification_delivery_attempts;
DROP INDEX IF EXISTS notification_deliveries_event_idx;
ALTER TABLE notification_deliveries
    DROP CONSTRAINT IF EXISTS notification_deliveries_routed_event_check,
    DROP CONSTRAINT IF EXISTS notification_deliveries_snapshot_hash_check,
    DROP CONSTRAINT IF EXISTS notification_deliveries_template_version_scope_fk,
    DROP CONSTRAINT IF EXISTS notification_deliveries_channel_version_scope_fk,
    DROP CONSTRAINT IF EXISTS notification_deliveries_channel_scope_fk,
    DROP CONSTRAINT IF EXISTS notification_deliveries_route_scope_fk,
    DROP CONSTRAINT IF EXISTS notification_deliveries_policy_rule_scope_fk,
    DROP CONSTRAINT IF EXISTS notification_deliveries_event_policy_scope_fk,
    DROP CONSTRAINT IF EXISTS notification_deliveries_rule_scope_fk,
    DROP CONSTRAINT IF EXISTS notification_deliveries_event_scope_fk,
    DROP CONSTRAINT IF EXISTS notification_deliveries_id_scope_unique,
    DROP CONSTRAINT IF EXISTS notification_deliveries_scope_fk,
    DROP COLUMN IF EXISTS lease_expires_at,
    DROP COLUMN IF EXISTS lease_owner,
    DROP COLUMN IF EXISTS max_attempts,
    DROP COLUMN IF EXISTS snapshot_hash,
    DROP COLUMN IF EXISTS route_snapshot,
    DROP COLUMN IF EXISTS template_version_id,
    DROP COLUMN IF EXISTS channel_version_id,
    DROP COLUMN IF EXISTS route_id,
    DROP COLUMN IF EXISTS rule_id,
    DROP COLUMN IF EXISTS policy_id,
    DROP COLUMN IF EXISTS event_id,
    DROP COLUMN IF EXISTS scope_id;
ALTER TABLE notification_deliveries
    DROP CONSTRAINT IF EXISTS notification_deliveries_status_check;
UPDATE notification_deliveries SET status='failed' WHERE status='dead_letter';
ALTER TABLE notification_deliveries
    ADD CONSTRAINT notification_deliveries_status_check CHECK (status IN ('queued', 'delivering', 'succeeded', 'failed'));
DROP TABLE IF EXISTS notification_events;
DROP TABLE IF EXISTS notification_rule_routes;
DROP TABLE IF EXISTS inspection_policy_notification_rules;
DROP TABLE IF EXISTS notification_rules;
DROP TABLE IF EXISTS notification_template_versions;
DROP TABLE IF EXISTS notification_templates;
DROP TABLE IF EXISTS notification_channel_versions;
ALTER TABLE notification_channels
    DROP CONSTRAINT IF EXISTS notification_channels_id_scope_unique,
    DROP COLUMN IF EXISTS share_with_children,
    DROP COLUMN IF EXISTS config_version;
ALTER TABLE inspection_findings
    DROP CONSTRAINT IF EXISTS inspection_findings_id_scope_unique,
    DROP CONSTRAINT IF EXISTS inspection_findings_target_scope_fk,
    DROP CONSTRAINT IF EXISTS inspection_findings_scope_fk,
    DROP COLUMN IF EXISTS scope_id;
ALTER TABLE inspection_runs DROP CONSTRAINT IF EXISTS inspection_runs_id_scope_unique;
ALTER TABLE inspection_policies DROP CONSTRAINT IF EXISTS inspection_policies_id_scope_unique;
ALTER TABLE resources DROP CONSTRAINT IF EXISTS resources_id_scope_unique;

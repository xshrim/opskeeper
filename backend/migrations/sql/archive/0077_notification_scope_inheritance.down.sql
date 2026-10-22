DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM inspection_policy_notification_rules WHERE scope_id <> rule_scope_id)
       OR EXISTS (SELECT 1 FROM notification_deliveries WHERE scope_id <> rule_scope_id) THEN
        RAISE EXCEPTION 'cannot roll back notification Scope inheritance while child bindings or deliveries exist';
    END IF;
END;
$$;

DROP TRIGGER IF EXISTS notification_templates_prevent_share_revocation ON notification_templates;
DROP TRIGGER IF EXISTS notification_channels_prevent_share_revocation ON notification_channels;
DROP TRIGGER IF EXISTS notification_rules_prevent_share_revocation ON notification_rules;
DROP TRIGGER IF EXISTS inspection_policy_notification_rules_validate_scope ON inspection_policy_notification_rules;
DROP FUNCTION IF EXISTS reject_notification_share_revocation();
DROP FUNCTION IF EXISTS validate_policy_notification_rule_scope();

ALTER TABLE notification_deliveries
    DROP CONSTRAINT notification_deliveries_rule_scope_fk,
    DROP CONSTRAINT notification_deliveries_route_scope_fk,
    DROP CONSTRAINT notification_deliveries_channel_scope_fk,
    DROP CONSTRAINT notification_deliveries_channel_version_scope_fk,
    DROP CONSTRAINT notification_deliveries_template_version_scope_fk,
    DROP COLUMN rule_scope_id,
    ADD CONSTRAINT notification_deliveries_rule_scope_fk
        FOREIGN KEY (rule_id, scope_id) REFERENCES notification_rules(id, scope_id) ON DELETE RESTRICT,
    ADD CONSTRAINT notification_deliveries_route_scope_fk
        FOREIGN KEY (route_id, scope_id, rule_id, channel_id, channel_version_id, template_version_id)
        REFERENCES notification_rule_routes(id, scope_id, rule_id, channel_id, channel_version_id, template_version_id) ON DELETE RESTRICT,
    ADD CONSTRAINT notification_deliveries_channel_scope_fk
        FOREIGN KEY (channel_id, scope_id) REFERENCES notification_channels(id, scope_id) ON DELETE RESTRICT,
    ADD CONSTRAINT notification_deliveries_channel_version_scope_fk
        FOREIGN KEY (channel_version_id, scope_id) REFERENCES notification_channel_versions(id, scope_id) ON DELETE RESTRICT,
    ADD CONSTRAINT notification_deliveries_template_version_scope_fk
        FOREIGN KEY (template_version_id, scope_id) REFERENCES notification_template_versions(id, scope_id) ON DELETE RESTRICT;

ALTER TABLE inspection_policy_notification_rules
    DROP CONSTRAINT inspection_policy_notification_rules_rule_owner_scope_fk,
    DROP COLUMN rule_scope_id,
    ADD CONSTRAINT inspection_policy_notification_rules_rule_id_scope_id_fkey
        FOREIGN KEY (rule_id, scope_id) REFERENCES notification_rules(id, scope_id) ON DELETE CASCADE;

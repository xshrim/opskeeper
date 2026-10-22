ALTER TABLE inspection_policy_notification_rules
    ADD COLUMN rule_scope_id uuid;

UPDATE inspection_policy_notification_rules
   SET rule_scope_id = scope_id;

ALTER TABLE inspection_policy_notification_rules
    ALTER COLUMN rule_scope_id SET NOT NULL,
    DROP CONSTRAINT inspection_policy_notification_rules_rule_id_scope_id_fkey,
    ADD CONSTRAINT inspection_policy_notification_rules_rule_owner_scope_fk
        FOREIGN KEY (rule_id, rule_scope_id) REFERENCES notification_rules(id, scope_id) ON DELETE CASCADE;

ALTER TABLE notification_deliveries
    ADD COLUMN rule_scope_id uuid;

UPDATE notification_deliveries
   SET rule_scope_id = scope_id;

ALTER TABLE notification_deliveries
    ALTER COLUMN rule_scope_id SET NOT NULL,
    DROP CONSTRAINT notification_deliveries_rule_scope_fk,
    DROP CONSTRAINT notification_deliveries_route_scope_fk,
    DROP CONSTRAINT notification_deliveries_channel_scope_fk,
    DROP CONSTRAINT notification_deliveries_channel_version_scope_fk,
    DROP CONSTRAINT notification_deliveries_template_version_scope_fk,
    ADD CONSTRAINT notification_deliveries_rule_scope_fk
        FOREIGN KEY (rule_id, rule_scope_id) REFERENCES notification_rules(id, scope_id) ON DELETE RESTRICT,
    ADD CONSTRAINT notification_deliveries_route_scope_fk
        FOREIGN KEY (route_id, rule_scope_id, rule_id, channel_id, channel_version_id, template_version_id)
        REFERENCES notification_rule_routes(id, scope_id, rule_id, channel_id, channel_version_id, template_version_id) ON DELETE RESTRICT,
    ADD CONSTRAINT notification_deliveries_channel_scope_fk
        FOREIGN KEY (channel_id, rule_scope_id) REFERENCES notification_channels(id, scope_id) ON DELETE RESTRICT,
    ADD CONSTRAINT notification_deliveries_channel_version_scope_fk
        FOREIGN KEY (channel_version_id, rule_scope_id) REFERENCES notification_channel_versions(id, scope_id) ON DELETE RESTRICT,
    ADD CONSTRAINT notification_deliveries_template_version_scope_fk
        FOREIGN KEY (template_version_id, rule_scope_id) REFERENCES notification_template_versions(id, scope_id) ON DELETE RESTRICT;

CREATE FUNCTION validate_policy_notification_rule_scope() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    can_share boolean;
    route_count integer;
    all_routes_shared boolean;
BEGIN
    IF NEW.scope_id = NEW.rule_scope_id THEN
        RETURN NEW;
    END IF;

    IF NOT resource_scope_contains(NEW.rule_scope_id, NEW.scope_id) THEN
        RAISE EXCEPTION 'notification rule is outside the policy Scope chain' USING ERRCODE = '23514';
    END IF;

    SELECT rule.share_with_children
      INTO can_share
      FROM notification_rules rule
     WHERE rule.id = NEW.rule_id
       AND rule.scope_id = NEW.rule_scope_id
       AND rule.status = 'active'
       AND rule.deleted_at IS NULL;

    IF NOT COALESCE(can_share, false) THEN
        RAISE EXCEPTION 'notification rule is not shared with child Scopes' USING ERRCODE = '23514';
    END IF;

    SELECT count(*), COALESCE(bool_and(channel.share_with_children AND template_record.share_with_children), false)
      INTO route_count, all_routes_shared
      FROM notification_rule_routes route
      JOIN notification_channels channel ON channel.id = route.channel_id AND channel.scope_id = route.scope_id
      JOIN notification_template_versions template ON template.id = route.template_version_id AND template.scope_id = route.scope_id
      JOIN notification_templates template_record ON template_record.id = template.template_id AND template_record.scope_id = template.scope_id
     WHERE route.rule_id = NEW.rule_id
       AND route.scope_id = NEW.rule_scope_id
       AND route.retired_at IS NULL;

    IF route_count = 0 OR NOT all_routes_shared THEN
        RAISE EXCEPTION 'notification rule routes must share their channels and templates with child Scopes' USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER inspection_policy_notification_rules_validate_scope
    BEFORE INSERT OR UPDATE OF scope_id, rule_scope_id, rule_id ON inspection_policy_notification_rules
    FOR EACH ROW EXECUTE FUNCTION validate_policy_notification_rule_scope();

CREATE FUNCTION reject_notification_share_revocation() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    in_use boolean;
BEGIN
    IF OLD.share_with_children AND NOT NEW.share_with_children THEN
        IF TG_TABLE_NAME = 'notification_rules' THEN
            SELECT EXISTS (
                SELECT 1 FROM inspection_policy_notification_rules binding
                 WHERE binding.rule_id = OLD.id
                   AND binding.rule_scope_id = OLD.scope_id
                   AND binding.scope_id <> binding.rule_scope_id
            ) INTO in_use;
        ELSIF TG_TABLE_NAME = 'notification_channels' THEN
            SELECT EXISTS (
                SELECT 1
                  FROM notification_rule_routes route
                  JOIN inspection_policy_notification_rules binding
                    ON binding.rule_id = route.rule_id AND binding.rule_scope_id = route.scope_id
                 WHERE route.channel_id = OLD.id
                   AND route.scope_id = OLD.scope_id
                   AND route.retired_at IS NULL
                   AND binding.scope_id <> binding.rule_scope_id
            ) INTO in_use;
        ELSE
            SELECT EXISTS (
                SELECT 1
                  FROM notification_rule_routes route
                  JOIN notification_template_versions version
                    ON version.id = route.template_version_id AND version.scope_id = route.scope_id
                  JOIN inspection_policy_notification_rules binding
                    ON binding.rule_id = route.rule_id AND binding.rule_scope_id = route.scope_id
                 WHERE version.template_id = OLD.id
                   AND version.scope_id = OLD.scope_id
                   AND route.retired_at IS NULL
                   AND binding.scope_id <> binding.rule_scope_id
            ) INTO in_use;
        END IF;

        IF in_use THEN
            RAISE EXCEPTION 'notification sharing cannot be revoked while child policies use the object' USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER notification_rules_prevent_share_revocation
    BEFORE UPDATE OF share_with_children ON notification_rules
    FOR EACH ROW EXECUTE FUNCTION reject_notification_share_revocation();
CREATE TRIGGER notification_channels_prevent_share_revocation
    BEFORE UPDATE OF share_with_children ON notification_channels
    FOR EACH ROW EXECUTE FUNCTION reject_notification_share_revocation();
CREATE TRIGGER notification_templates_prevent_share_revocation
    BEFORE UPDATE OF share_with_children ON notification_templates
    FOR EACH ROW EXECUTE FUNCTION reject_notification_share_revocation();

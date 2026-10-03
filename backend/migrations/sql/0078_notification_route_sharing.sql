CREATE OR REPLACE FUNCTION validate_policy_notification_rule_scope() RETURNS trigger LANGUAGE plpgsql AS $$
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
       AND rule.deleted_at IS NULL
     FOR UPDATE;

    IF NOT COALESCE(can_share, false) THEN
        RAISE EXCEPTION 'notification rule is not shared with child Scopes' USING ERRCODE = '23514';
    END IF;

    PERFORM 1
      FROM notification_rule_routes route
      JOIN notification_channels channel ON channel.id = route.channel_id AND channel.scope_id = route.scope_id
      JOIN notification_template_versions template ON template.id = route.template_version_id AND template.scope_id = route.scope_id
      JOIN notification_templates template_record ON template_record.id = template.template_id AND template_record.scope_id = template.scope_id
     WHERE route.rule_id = NEW.rule_id
       AND route.scope_id = NEW.rule_scope_id
       AND route.retired_at IS NULL
     ORDER BY channel.id, template_record.id
     FOR SHARE OF channel, template_record;

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

CREATE FUNCTION validate_notification_route_child_sharing() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    has_child_bindings boolean;
    channel_shared boolean;
    template_shared boolean;
BEGIN
    PERFORM 1
      FROM notification_rules rule
     WHERE rule.id = NEW.rule_id
       AND rule.scope_id = NEW.scope_id
     FOR UPDATE;

    SELECT EXISTS (
        SELECT 1 FROM inspection_policy_notification_rules binding
         WHERE binding.rule_id = NEW.rule_id
           AND binding.rule_scope_id = NEW.scope_id
           AND binding.scope_id <> binding.rule_scope_id
    ) INTO has_child_bindings;

    IF NOT has_child_bindings THEN
        RETURN NEW;
    END IF;

    SELECT channel.share_with_children, template_record.share_with_children
      INTO channel_shared, template_shared
      FROM notification_channels channel
      JOIN notification_template_versions version
        ON version.id = NEW.template_version_id AND version.scope_id = NEW.scope_id
      JOIN notification_templates template_record
        ON template_record.id = version.template_id AND template_record.scope_id = version.scope_id
     WHERE channel.id = NEW.channel_id
       AND channel.scope_id = NEW.scope_id
     FOR SHARE OF channel, template_record;

    IF NOT COALESCE(channel_shared, false) OR NOT COALESCE(template_shared, false) THEN
        RAISE EXCEPTION 'notification routes used by child policies must share their channels and templates' USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER notification_rule_routes_validate_child_sharing_insert
    BEFORE INSERT ON notification_rule_routes
    FOR EACH ROW EXECUTE FUNCTION validate_notification_route_child_sharing();
CREATE TRIGGER notification_rule_routes_validate_child_sharing_update
    BEFORE UPDATE OF scope_id, rule_id, channel_id, template_version_id ON notification_rule_routes
    FOR EACH ROW EXECUTE FUNCTION validate_notification_route_child_sharing();

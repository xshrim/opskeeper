ALTER TABLE inspection_policies
    ADD CONSTRAINT inspection_policies_id_scope_unique UNIQUE (id, scope_id);

ALTER TABLE inspection_runs
    ADD CONSTRAINT inspection_runs_id_scope_unique UNIQUE (id, scope_id);

ALTER TABLE resources
    ADD CONSTRAINT resources_id_scope_unique UNIQUE (id, scope_id);

ALTER TABLE inspection_findings
    ADD COLUMN scope_id uuid;

UPDATE inspection_findings finding
   SET scope_id = policy.scope_id
  FROM inspection_policies policy
 WHERE policy.id = finding.policy_id;

ALTER TABLE inspection_findings
    ALTER COLUMN scope_id SET NOT NULL,
    ADD CONSTRAINT inspection_findings_scope_fk FOREIGN KEY (scope_id) REFERENCES scopes(id),
    ADD CONSTRAINT inspection_findings_target_scope_fk FOREIGN KEY (target_resource_id, scope_id) REFERENCES resources(id, scope_id) ON DELETE RESTRICT,
    ADD CONSTRAINT inspection_findings_id_scope_unique UNIQUE (id, scope_id);

ALTER TABLE notification_channels
    ADD COLUMN config_version integer NOT NULL DEFAULT 1 CHECK (config_version > 0),
    ADD COLUMN share_with_children boolean NOT NULL DEFAULT false,
    ADD CONSTRAINT notification_channels_id_scope_unique UNIQUE (id, scope_id);

CREATE TABLE notification_channel_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    channel_id uuid NOT NULL,
    version integer NOT NULL CHECK (version > 0),
    provider_config_ciphertext bytea NOT NULL DEFAULT ''::bytea,
    provider_config_hash text NOT NULL CHECK (provider_config_hash ~ '^[0-9a-f]{64}$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (channel_id, version),
    UNIQUE (id, scope_id),
    UNIQUE (id, scope_id, channel_id),
    FOREIGN KEY (channel_id, scope_id) REFERENCES notification_channels(id, scope_id) ON DELETE RESTRICT
);

CREATE TABLE notification_templates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    share_with_children boolean NOT NULL DEFAULT false,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (id, scope_id)
);
CREATE UNIQUE INDEX notification_templates_scope_name_unique
    ON notification_templates(scope_id, lower(name)) WHERE deleted_at IS NULL;

CREATE TABLE notification_template_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    template_id uuid NOT NULL,
    version integer NOT NULL CHECK (version > 0),
    format text NOT NULL CHECK (format IN ('text', 'markdown', 'json')),
    title_template text NOT NULL DEFAULT '',
    body_template text NOT NULL DEFAULT '',
    payload_template jsonb,
    variables jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(variables) = 'array'),
    content_hash text NOT NULL CHECK (content_hash ~ '^[0-9a-f]{64}$'),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'disabled')),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    UNIQUE (template_id, version),
    UNIQUE (id, scope_id),
    FOREIGN KEY (template_id, scope_id) REFERENCES notification_templates(id, scope_id) ON DELETE RESTRICT,
    CHECK ((format = 'json' AND payload_template IS NOT NULL) OR (format <> 'json' AND payload_template IS NULL))
);

CREATE TABLE notification_rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    share_with_children boolean NOT NULL DEFAULT false,
    event_types text[] NOT NULL CHECK (
        cardinality(event_types) > 0 AND
        event_types <@ ARRAY['finding.opened','finding.reopened','finding.severity_changed','finding.resolved','inspection.failed','inspection.degraded','notification.test']::text[]
    ),
    minimum_severity text NOT NULL DEFAULT 'info' CHECK (minimum_severity IN ('info', 'warning', 'critical')),
    filters jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(filters) = 'object'),
    cooldown_seconds integer NOT NULL DEFAULT 0 CHECK (cooldown_seconds >= 0),
    aggregation_seconds integer NOT NULL DEFAULT 0 CHECK (aggregation_seconds >= 0),
    max_batch_size integer NOT NULL DEFAULT 1 CHECK (max_batch_size BETWEEN 1 AND 1000),
    silence jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(silence) = 'object'),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (id, scope_id)
);
CREATE UNIQUE INDEX notification_rules_scope_name_unique
    ON notification_rules(scope_id, lower(name)) WHERE deleted_at IS NULL;

CREATE TABLE inspection_policy_notification_rules (
    scope_id uuid NOT NULL REFERENCES scopes(id),
    policy_id uuid NOT NULL,
    rule_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (policy_id, rule_id),
    UNIQUE (policy_id, rule_id, scope_id),
    FOREIGN KEY (policy_id, scope_id) REFERENCES inspection_policies(id, scope_id) ON DELETE CASCADE,
    FOREIGN KEY (rule_id, scope_id) REFERENCES notification_rules(id, scope_id) ON DELETE CASCADE
);

CREATE TABLE notification_rule_routes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    rule_id uuid NOT NULL,
    channel_id uuid NOT NULL,
    channel_version_id uuid NOT NULL,
    template_version_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, scope_id),
    UNIQUE (id, scope_id, rule_id, channel_id, channel_version_id, template_version_id),
    UNIQUE (rule_id, channel_id, template_version_id),
    FOREIGN KEY (rule_id, scope_id) REFERENCES notification_rules(id, scope_id) ON DELETE CASCADE,
    FOREIGN KEY (channel_id, scope_id) REFERENCES notification_channels(id, scope_id) ON DELETE RESTRICT,
    FOREIGN KEY (channel_version_id, scope_id, channel_id) REFERENCES notification_channel_versions(id, scope_id, channel_id) ON DELETE RESTRICT,
    FOREIGN KEY (template_version_id, scope_id) REFERENCES notification_template_versions(id, scope_id) ON DELETE RESTRICT
);

CREATE TABLE notification_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    event_type text NOT NULL CHECK (event_type IN ('finding.opened', 'finding.reopened', 'finding.severity_changed', 'finding.resolved', 'inspection.failed', 'inspection.degraded', 'notification.test')),
    event_key text NOT NULL UNIQUE CHECK (length(btrim(event_key)) BETWEEN 1 AND 300),
    policy_id uuid,
    run_id uuid,
    finding_id uuid,
    finding_identity text NOT NULL DEFAULT '',
    target_resource_id uuid,
    severity text CHECK (severity IS NULL OR severity IN ('info', 'warning', 'critical')),
    payload jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    content_hash text NOT NULL CHECK (content_hash ~ '^[0-9a-f]{64}$'),
    occurred_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, scope_id),
    UNIQUE (id, scope_id, policy_id),
    FOREIGN KEY (policy_id, scope_id) REFERENCES inspection_policies(id, scope_id) ON DELETE RESTRICT,
    FOREIGN KEY (run_id, scope_id) REFERENCES inspection_runs(id, scope_id) ON DELETE RESTRICT,
    FOREIGN KEY (finding_id, scope_id) REFERENCES inspection_findings(id, scope_id) ON DELETE RESTRICT,
    FOREIGN KEY (target_resource_id, scope_id) REFERENCES resources(id, scope_id) ON DELETE RESTRICT,
    CHECK ((event_type IN ('finding.opened', 'finding.reopened', 'finding.severity_changed', 'finding.resolved') AND finding_id IS NOT NULL AND run_id IS NOT NULL AND policy_id IS NOT NULL) OR event_type IN ('inspection.failed', 'inspection.degraded', 'notification.test'))
);
CREATE INDEX notification_events_scope_occurred_idx ON notification_events(scope_id, occurred_at DESC);
CREATE INDEX notification_events_run_idx ON notification_events(run_id) WHERE run_id IS NOT NULL;

ALTER TABLE notification_deliveries
    ADD COLUMN scope_id uuid,
    ADD COLUMN event_id uuid,
    ADD COLUMN policy_id uuid,
    ADD COLUMN rule_id uuid,
    ADD COLUMN route_id uuid,
    ADD COLUMN channel_version_id uuid,
    ADD COLUMN template_version_id uuid,
    ADD COLUMN route_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(route_snapshot) = 'object'),
    ADD COLUMN snapshot_hash text,
    ADD COLUMN max_attempts integer NOT NULL DEFAULT 5 CHECK (max_attempts BETWEEN 1 AND 20),
    ADD COLUMN lease_owner text NOT NULL DEFAULT '',
    ADD COLUMN lease_expires_at timestamptz;

UPDATE notification_deliveries delivery
   SET scope_id = COALESCE((SELECT run.scope_id FROM inspection_runs run WHERE run.id = delivery.run_id), channel.scope_id)
  FROM notification_channels channel
 WHERE channel.id = delivery.channel_id;

ALTER TABLE notification_deliveries
    DROP CONSTRAINT notification_deliveries_status_check,
    ALTER COLUMN scope_id SET NOT NULL,
    ADD CONSTRAINT notification_deliveries_status_check CHECK (status IN ('queued', 'delivering', 'succeeded', 'failed', 'dead_letter')),
    ADD CONSTRAINT notification_deliveries_scope_fk FOREIGN KEY (scope_id) REFERENCES scopes(id),
    ADD CONSTRAINT notification_deliveries_id_scope_unique UNIQUE (id, scope_id),
    ADD CONSTRAINT notification_deliveries_event_scope_fk FOREIGN KEY (event_id, scope_id) REFERENCES notification_events(id, scope_id) ON DELETE RESTRICT,
    ADD CONSTRAINT notification_deliveries_event_policy_scope_fk FOREIGN KEY (event_id, scope_id, policy_id) REFERENCES notification_events(id, scope_id, policy_id) ON DELETE RESTRICT,
    ADD CONSTRAINT notification_deliveries_rule_scope_fk FOREIGN KEY (rule_id, scope_id) REFERENCES notification_rules(id, scope_id) ON DELETE RESTRICT,
    ADD CONSTRAINT notification_deliveries_policy_rule_scope_fk FOREIGN KEY (policy_id, rule_id, scope_id) REFERENCES inspection_policy_notification_rules(policy_id, rule_id, scope_id) ON DELETE RESTRICT,
    ADD CONSTRAINT notification_deliveries_route_scope_fk FOREIGN KEY (route_id, scope_id, rule_id, channel_id, channel_version_id, template_version_id) REFERENCES notification_rule_routes(id, scope_id, rule_id, channel_id, channel_version_id, template_version_id) ON DELETE RESTRICT,
    ADD CONSTRAINT notification_deliveries_channel_scope_fk FOREIGN KEY (channel_id, scope_id) REFERENCES notification_channels(id, scope_id) ON DELETE RESTRICT,
    ADD CONSTRAINT notification_deliveries_channel_version_scope_fk FOREIGN KEY (channel_version_id, scope_id) REFERENCES notification_channel_versions(id, scope_id) ON DELETE RESTRICT,
    ADD CONSTRAINT notification_deliveries_template_version_scope_fk FOREIGN KEY (template_version_id, scope_id) REFERENCES notification_template_versions(id, scope_id) ON DELETE RESTRICT,
    ADD CONSTRAINT notification_deliveries_snapshot_hash_check CHECK (event_id IS NULL OR snapshot_hash ~ '^[0-9a-f]{64}$'),
    ADD CONSTRAINT notification_deliveries_routed_event_check CHECK (event_id IS NULL OR (policy_id IS NOT NULL AND rule_id IS NOT NULL AND route_id IS NOT NULL AND channel_version_id IS NOT NULL AND template_version_id IS NOT NULL));

CREATE INDEX notification_deliveries_event_idx ON notification_deliveries(event_id) WHERE event_id IS NOT NULL;

CREATE TABLE notification_delivery_attempts (
    id bigserial PRIMARY KEY,
    scope_id uuid NOT NULL REFERENCES scopes(id),
    delivery_id uuid NOT NULL,
    attempt integer NOT NULL CHECK (attempt > 0),
    status text NOT NULL CHECK (status IN ('succeeded', 'retrying', 'dead_letter', 'failed')),
    response_status integer,
    response_body text NOT NULL DEFAULT '',
    error_code text NOT NULL DEFAULT '',
    error_message text NOT NULL DEFAULT '',
    started_at timestamptz NOT NULL,
    completed_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (delivery_id, attempt),
    FOREIGN KEY (delivery_id, scope_id) REFERENCES notification_deliveries(id, scope_id) ON DELETE RESTRICT
);

CREATE FUNCTION reject_notification_event_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'notification events are immutable';
END;
$$;
CREATE TRIGGER notification_events_immutable
    BEFORE UPDATE OR DELETE ON notification_events
    FOR EACH ROW EXECUTE FUNCTION reject_notification_event_mutation();

CREATE FUNCTION reject_notification_attempt_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'notification delivery attempts are append-only';
END;
$$;
CREATE TRIGGER notification_delivery_attempts_append_only
    BEFORE UPDATE OR DELETE ON notification_delivery_attempts
    FOR EACH ROW EXECUTE FUNCTION reject_notification_attempt_mutation();

CREATE FUNCTION reject_notification_route_snapshot_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.scope_id, NEW.event_id, NEW.rule_id, NEW.channel_id, NEW.channel_version_id, NEW.template_version_id, NEW.idempotency_key, NEW.route_snapshot, NEW.snapshot_hash)
       IS DISTINCT FROM ROW(OLD.scope_id, OLD.event_id, OLD.rule_id, OLD.channel_id, OLD.channel_version_id, OLD.template_version_id, OLD.idempotency_key, OLD.route_snapshot, OLD.snapshot_hash) THEN
        RAISE EXCEPTION 'notification delivery route snapshot is immutable';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER notification_deliveries_route_immutable
    BEFORE UPDATE ON notification_deliveries
    FOR EACH ROW EXECUTE FUNCTION reject_notification_route_snapshot_mutation();

CREATE TABLE inspection_policies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 200),
    cron text NOT NULL CHECK (length(btrim(cron)) BETWEEN 1 AND 200),
    timezone text NOT NULL DEFAULT 'UTC' CHECK (length(btrim(timezone)) BETWEEN 1 AND 100),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    target_labels jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(target_labels) = 'object'),
    skill_resource_ids uuid[] NOT NULL DEFAULT '{}',
    timeout_seconds integer NOT NULL DEFAULT 120 CHECK (timeout_seconds BETWEEN 1 AND 3600),
    retries integer NOT NULL DEFAULT 1 CHECK (retries BETWEEN 0 AND 10),
    max_concurrent integer NOT NULL DEFAULT 1 CHECK (max_concurrent BETWEEN 1 AND 64),
    max_tool_calls integer NOT NULL DEFAULT 12 CHECK (max_tool_calls BETWEEN 1 AND 100),
    max_tokens bigint NOT NULL DEFAULT 20000 CHECK (max_tokens BETWEEN 1 AND 200000),
    maintenance_windows jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(maintenance_windows) = 'array'),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX inspection_policies_scope_idx ON inspection_policies(scope_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX inspection_policies_scope_name_unique ON inspection_policies(scope_id, lower(name)) WHERE deleted_at IS NULL;

CREATE TABLE inspection_policy_targets (
    policy_id uuid NOT NULL REFERENCES inspection_policies(id) ON DELETE CASCADE,
    resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    PRIMARY KEY (policy_id, resource_id)
);

CREATE TABLE inspection_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    policy_id uuid NOT NULL REFERENCES inspection_policies(id) ON DELETE RESTRICT,
    scope_id uuid NOT NULL REFERENCES scopes(id),
    window_start timestamptz NOT NULL,
    window_end timestamptz NOT NULL,
    trigger text NOT NULL CHECK (trigger IN ('schedule', 'manual', 'retry')),
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'skipped')),
    policy_snapshot jsonb NOT NULL CHECK (jsonb_typeof(policy_snapshot) = 'object'),
    target_snapshot jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(target_snapshot) = 'array'),
    score integer CHECK (score BETWEEN 0 AND 100),
    deterministic_completed boolean NOT NULL DEFAULT false,
    llm_status text NOT NULL DEFAULT 'not_requested' CHECK (llm_status IN ('not_requested', 'succeeded', 'degraded', 'failed')),
    error_code text NOT NULL DEFAULT '',
    error_message text NOT NULL DEFAULT '',
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (policy_id, window_start)
);
CREATE INDEX inspection_runs_scope_created_idx ON inspection_runs(scope_id, created_at DESC);
CREATE INDEX inspection_runs_policy_window_idx ON inspection_runs(policy_id, window_start DESC);

CREATE TABLE inspection_run_steps (
    id bigserial PRIMARY KEY,
    run_id uuid NOT NULL REFERENCES inspection_runs(id) ON DELETE CASCADE,
    sequence integer NOT NULL CHECK (sequence > 0),
    kind text NOT NULL CHECK (length(btrim(kind)) BETWEEN 1 AND 120),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'skipped')),
    detail text NOT NULL DEFAULT '',
    started_at timestamptz,
    completed_at timestamptz,
    UNIQUE (run_id, sequence)
);

CREATE TABLE inspection_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id uuid NOT NULL UNIQUE REFERENCES inspection_runs(id) ON DELETE CASCADE,
    idempotency_key text NOT NULL UNIQUE CHECK (length(btrim(idempotency_key)) BETWEEN 1 AND 200),
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'leased', 'succeeded', 'failed')),
    attempt integer NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    max_attempts integer NOT NULL DEFAULT 2 CHECK (max_attempts BETWEEN 1 AND 11),
    lease_owner text NOT NULL DEFAULT '',
    lease_expires_at timestamptz,
    heartbeat_at timestamptz,
    available_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    error_code text NOT NULL DEFAULT '',
    error_message text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX inspection_jobs_claim_idx ON inspection_jobs(status, available_at) WHERE status IN ('queued', 'leased');

CREATE TABLE inspection_findings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    policy_id uuid NOT NULL REFERENCES inspection_policies(id) ON DELETE RESTRICT,
    target_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    rule text NOT NULL CHECK (length(btrim(rule)) BETWEEN 1 AND 200),
    -- identity_key represents the enduring condition. fingerprint represents
    -- the observation in one scheduling window and is used for event dedupe.
    identity_key text NOT NULL CHECK (length(identity_key) = 64),
    fingerprint text NOT NULL CHECK (length(fingerprint) = 64),
    severity text NOT NULL CHECK (severity IN ('info', 'warning', 'critical')),
    message text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved')),
    first_observed_at timestamptz NOT NULL,
    last_observed_at timestamptz NOT NULL,
    resolved_at timestamptz,
    last_run_id uuid REFERENCES inspection_runs(id) ON DELETE SET NULL,
    UNIQUE (policy_id, identity_key)
);
CREATE INDEX inspection_findings_scope_idx ON inspection_findings(policy_id, status, last_observed_at DESC);

CREATE TABLE inspection_health_snapshots (
    id bigserial PRIMARY KEY,
    run_id uuid NOT NULL REFERENCES inspection_runs(id) ON DELETE CASCADE,
    policy_id uuid NOT NULL REFERENCES inspection_policies(id) ON DELETE RESTRICT,
    target_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    score integer NOT NULL CHECK (score BETWEEN 0 AND 100),
    reasons jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(reasons) = 'array'),
    collected_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX inspection_health_snapshots_target_idx ON inspection_health_snapshots(target_resource_id, collected_at DESC);

CREATE TABLE notification_channels (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    kind text NOT NULL CHECK (kind = 'webhook'),
    webhook_url text NOT NULL CHECK (length(btrim(webhook_url)) BETWEEN 1 AND 2000),
    credential_id uuid REFERENCES resource_credentials(id) ON DELETE SET NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    rate_limit_per_minute integer NOT NULL DEFAULT 30 CHECK (rate_limit_per_minute BETWEEN 1 AND 600),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE UNIQUE INDEX notification_channels_scope_name_unique ON notification_channels(scope_id, lower(name)) WHERE deleted_at IS NULL;

CREATE TABLE notification_deliveries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id uuid NOT NULL REFERENCES notification_channels(id) ON DELETE RESTRICT,
    finding_id uuid REFERENCES inspection_findings(id) ON DELETE SET NULL,
    run_id uuid REFERENCES inspection_runs(id) ON DELETE SET NULL,
    idempotency_key text NOT NULL UNIQUE,
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'delivering', 'succeeded', 'failed')),
    attempt integer NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    available_at timestamptz NOT NULL DEFAULT now(),
    response_status integer,
    response_body text NOT NULL DEFAULT '',
    error_message text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
CREATE INDEX notification_deliveries_claim_idx ON notification_deliveries(status, available_at) WHERE status IN ('queued', 'delivering');

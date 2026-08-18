CREATE TABLE mcp_server_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    server_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    scope_id uuid NOT NULL REFERENCES scopes(id),
    protocol_version text NOT NULL DEFAULT '',
    server_name text NOT NULL DEFAULT '',
    server_version text NOT NULL DEFAULT '',
    tools jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(tools) = 'array'),
    content_hash text NOT NULL CHECK (length(content_hash) = 64),
    status text NOT NULL CHECK (status IN ('succeeded', 'failed')),
    error_message text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX mcp_server_snapshots_server_created_idx ON mcp_server_snapshots(server_resource_id, created_at DESC);

CREATE TABLE operation_policies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 160),
    target_kinds text[] NOT NULL DEFAULT '{}',
    operation_names text[] NOT NULL DEFAULT '{}',
    minimum_risk text NOT NULL CHECK (minimum_risk IN ('read_only', 'low', 'medium', 'high')),
    approval_required boolean NOT NULL DEFAULT true,
    approver_permission text NOT NULL DEFAULT 'operation:approve',
    expires_after_seconds integer NOT NULL DEFAULT 1800 CHECK (expires_after_seconds BETWEEN 60 AND 86400),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (scope_id, name)
);

CREATE TABLE operation_requests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    target_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    requested_by uuid NOT NULL REFERENCES users(id),
    source text NOT NULL CHECK (source IN ('user', 'skill', 'mcp')),
    operation_name text NOT NULL CHECK (length(btrim(operation_name)) BETWEEN 1 AND 200),
    risk_level text NOT NULL CHECK (risk_level IN ('read_only', 'low', 'medium', 'high')),
    parameters jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(parameters) = 'object'),
    parameters_hash text NOT NULL CHECK (length(parameters_hash) = 64),
    impact_summary text NOT NULL DEFAULT '',
    rollback_summary text NOT NULL DEFAULT '',
    dry_run jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(dry_run) = 'object'),
    idempotency_key text NOT NULL CHECK (length(btrim(idempotency_key)) BETWEEN 1 AND 250),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'expired', 'executing', 'succeeded', 'failed', 'cancelled')),
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (scope_id, idempotency_key)
);
CREATE INDEX operation_requests_scope_status_idx ON operation_requests(scope_id, status, created_at DESC);

CREATE TABLE operation_approvals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    operation_request_id uuid NOT NULL REFERENCES operation_requests(id) ON DELETE CASCADE,
    approver_user_id uuid NOT NULL REFERENCES users(id),
    decision text NOT NULL CHECK (decision IN ('approved', 'rejected')),
    parameters_hash text NOT NULL CHECK (length(parameters_hash) = 64),
    comment text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (operation_request_id, approver_user_id)
);

CREATE TABLE operation_executions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    operation_request_id uuid NOT NULL UNIQUE REFERENCES operation_requests(id) ON DELETE RESTRICT,
    executor text NOT NULL CHECK (executor IN ('kubernetes_job', 'mcp')),
    idempotency_key text NOT NULL UNIQUE,
    status text NOT NULL CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    result jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(result) = 'object'),
    error_message text NOT NULL DEFAULT '',
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO role_permissions (role_id, permission)
SELECT id, 'operation:approve' FROM roles
 WHERE name IN ('PlatformAdmin', 'TeamAdmin', 'ProjectAdmin')
ON CONFLICT DO NOTHING;

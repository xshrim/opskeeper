CREATE TABLE diagnosis_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id) ON DELETE RESTRICT,
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    status text NOT NULL CHECK (status IN ('queued', 'planning', 'collecting', 'analyzing', 'succeeded', 'failed', 'cancelled')),
    title text NOT NULL DEFAULT '' CHECK (length(title) <= 200),
    error_code text NOT NULL DEFAULT '' CHECK (length(error_code) <= 120),
    error_message text NOT NULL DEFAULT '' CHECK (length(error_message) <= 1000),
    started_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX diagnosis_sessions_scope_idx ON diagnosis_sessions(scope_id, created_at DESC);
CREATE INDEX diagnosis_sessions_actor_idx ON diagnosis_sessions(actor_user_id, created_at DESC);

CREATE TABLE diagnosis_targets (
    session_id uuid NOT NULL REFERENCES diagnosis_sessions(id) ON DELETE CASCADE,
    resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, resource_id)
);

CREATE TABLE diagnosis_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES diagnosis_sessions(id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content text NOT NULL CHECK (length(content) <= 16000),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX diagnosis_messages_session_idx ON diagnosis_messages(session_id, created_at, id);

CREATE TABLE diagnosis_plans (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL UNIQUE REFERENCES diagnosis_sessions(id) ON DELETE CASCADE,
    summary text NOT NULL DEFAULT '' CHECK (length(summary) <= 2000),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE diagnosis_plan_steps (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id uuid NOT NULL REFERENCES diagnosis_plans(id) ON DELETE CASCADE,
    sequence integer NOT NULL CHECK (sequence > 0),
    phase text NOT NULL CHECK (phase IN ('plan', 'collect', 'verify', 'summarize')),
    status text NOT NULL CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'skipped')),
    title text NOT NULL CHECK (length(title) BETWEEN 1 AND 300),
    detail text NOT NULL DEFAULT '' CHECK (length(detail) <= 2000),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (plan_id, sequence)
);

CREATE TABLE diagnosis_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES diagnosis_sessions(id) ON DELETE CASCADE,
    event_type text NOT NULL CHECK (length(event_type) BETWEEN 1 AND 120),
    payload jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(payload) = 'object'),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX diagnosis_events_session_idx ON diagnosis_events(session_id, id);

CREATE TABLE diagnosis_evidence (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES diagnosis_sessions(id) ON DELETE CASCADE,
    target_resource_id uuid REFERENCES resources(id) ON DELETE RESTRICT,
    source_resource_id uuid REFERENCES resources(id) ON DELETE RESTRICT,
    capability text NOT NULL DEFAULT '' CHECK (length(capability) <= 120),
    collected_at timestamptz NOT NULL,
    window_start timestamptz,
    window_end timestamptz,
    content_hash text NOT NULL CHECK (length(content_hash) = 64),
    summary jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(summary) = 'object'),
    content jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(content) IN ('object', 'array')),
    partial boolean NOT NULL DEFAULT false,
    untrusted boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((window_start IS NULL AND window_end IS NULL) OR (window_start IS NOT NULL AND window_end IS NOT NULL AND window_start <= window_end))
);

CREATE INDEX diagnosis_evidence_session_idx ON diagnosis_evidence(session_id, created_at, id);

CREATE TABLE diagnosis_hypotheses (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES diagnosis_sessions(id) ON DELETE CASCADE,
    statement text NOT NULL CHECK (length(statement) BETWEEN 1 AND 4000),
    status text NOT NULL CHECK (status IN ('pending', 'supported', 'rejected', 'needs_verification')),
    confidence numeric(4,3) NOT NULL DEFAULT 0 CHECK (confidence >= 0 AND confidence <= 1),
    evidence_ids uuid[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX diagnosis_hypotheses_session_idx ON diagnosis_hypotheses(session_id, created_at, id);

CREATE TABLE diagnosis_reports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL UNIQUE REFERENCES diagnosis_sessions(id) ON DELETE CASCADE,
    status text NOT NULL CHECK (status IN ('succeeded', 'warning', 'failed')),
    conclusion text NOT NULL DEFAULT '' CHECK (length(conclusion) <= 8000),
    recommendations jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(recommendations) = 'array'),
    evidence_ids uuid[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

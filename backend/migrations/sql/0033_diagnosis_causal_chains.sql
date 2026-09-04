CREATE TABLE diagnosis_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES diagnosis_sessions(id) ON DELETE CASCADE,
    sequence integer NOT NULL CHECK (sequence > 0),
    question_message_id uuid REFERENCES diagnosis_messages(id) ON DELETE SET NULL,
    status text NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'succeeded', 'failed', 'cancelled')),
    started_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    UNIQUE (session_id, sequence)
);

CREATE INDEX diagnosis_runs_session_idx ON diagnosis_runs(session_id, sequence);

ALTER TABLE diagnosis_evidence
    ADD COLUMN run_id uuid REFERENCES diagnosis_runs(id) ON DELETE SET NULL;

CREATE INDEX diagnosis_evidence_run_idx ON diagnosis_evidence(run_id, created_at, id);

CREATE TABLE diagnosis_causal_chains (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES diagnosis_sessions(id) ON DELETE CASCADE,
    run_id uuid NOT NULL UNIQUE REFERENCES diagnosis_runs(id) ON DELETE CASCADE,
    version integer NOT NULL CHECK (version > 0),
    status text NOT NULL CHECK (status IN ('active', 'superseded', 'partial')),
    summary text NOT NULL DEFAULT '' CHECK (length(summary) <= 2000),
    nodes jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(nodes) = 'array'),
    links jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(links) = 'array'),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (session_id, version)
);

CREATE INDEX diagnosis_causal_chains_session_idx
    ON diagnosis_causal_chains(session_id, version DESC);

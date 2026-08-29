CREATE TABLE ai_execution_events (
    execution_id text NOT NULL,
    sequence bigint NOT NULL CHECK (sequence > 0),
    type text NOT NULL,
    status text,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (execution_id, sequence)
);

CREATE INDEX ai_execution_events_order_idx ON ai_execution_events (execution_id, sequence);

CREATE TABLE ai_execution_tool_calls (
    execution_id text NOT NULL,
    sequence integer NOT NULL CHECK (sequence > 0),
    resource_id uuid,
    tool_name text NOT NULL,
    arguments jsonb NOT NULL DEFAULT '{}'::jsonb,
    output jsonb,
    status text NOT NULL,
    error_message text,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (execution_id, sequence)
);

CREATE INDEX ai_execution_tool_calls_resource_idx ON ai_execution_tool_calls (resource_id);

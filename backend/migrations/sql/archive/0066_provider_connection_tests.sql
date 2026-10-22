ALTER TABLE providers
    ADD COLUMN last_connection_test_status text NOT NULL DEFAULT '',
    ADD COLUMN last_connection_test_message text NOT NULL DEFAULT '',
    ADD COLUMN last_connection_test_latency_ms bigint NOT NULL DEFAULT 0 CHECK (last_connection_test_latency_ms >= 0),
    ADD COLUMN last_connection_test_at timestamptz;

ALTER TABLE providers
    DROP COLUMN last_connection_test_at,
    DROP COLUMN last_connection_test_latency_ms,
    DROP COLUMN last_connection_test_message,
    DROP COLUMN last_connection_test_status;

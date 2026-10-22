CREATE TABLE notification_rule_cooldowns (
    scope_id uuid NOT NULL REFERENCES scopes(id),
    rule_id uuid NOT NULL,
    cooldown_key text NOT NULL CHECK (length(cooldown_key) = 64),
    last_queued_at timestamptz NOT NULL,
    PRIMARY KEY (rule_id, cooldown_key),
    FOREIGN KEY (rule_id, scope_id) REFERENCES notification_rules(id, scope_id) ON DELETE CASCADE
);

CREATE INDEX notification_rule_cooldowns_last_queued_idx ON notification_rule_cooldowns(last_queued_at);

ALTER TABLE diagnosis_sessions
    ADD COLUMN lease_owner text NOT NULL DEFAULT '',
    ADD COLUMN lease_expires_at timestamptz;

CREATE INDEX diagnosis_sessions_recoverable_idx
    ON diagnosis_sessions(status, lease_expires_at, updated_at)
    WHERE status IN ('queued', 'planning', 'collecting', 'analyzing');

ALTER TABLE inspection_policy_targets
    DROP CONSTRAINT inspection_policy_targets_pkey;

ALTER TABLE inspection_policy_targets
    ALTER COLUMN resource_id DROP NOT NULL;

ALTER TABLE inspection_policy_targets
    ADD COLUMN application_id uuid REFERENCES applications(id) ON DELETE RESTRICT,
    ADD CONSTRAINT inspection_policy_targets_exactly_one_target
        CHECK ((resource_id IS NOT NULL) <> (application_id IS NOT NULL));

CREATE UNIQUE INDEX inspection_policy_targets_resource_unique
    ON inspection_policy_targets(policy_id, resource_id)
    WHERE resource_id IS NOT NULL;

CREATE UNIQUE INDEX inspection_policy_targets_application_unique
    ON inspection_policy_targets(policy_id, application_id)
    WHERE application_id IS NOT NULL;

CREATE INDEX inspection_policy_targets_application_idx
    ON inspection_policy_targets(application_id)
    WHERE application_id IS NOT NULL;

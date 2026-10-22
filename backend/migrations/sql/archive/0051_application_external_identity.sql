CREATE UNIQUE INDEX applications_project_external_uid_idx
    ON applications(project_id, external_uid)
    WHERE deleted_at IS NULL AND external_uid <> '';

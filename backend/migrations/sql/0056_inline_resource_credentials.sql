ALTER TABLE resources
    ADD COLUMN credential_ciphertext bytea,
    ADD COLUMN credential_key_version text NOT NULL DEFAULT '',
    ADD COLUMN credential_purpose text NOT NULL DEFAULT '' CHECK (length(credential_purpose) <= 500);

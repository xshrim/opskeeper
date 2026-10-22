ALTER TABLE users
    ADD COLUMN password_hash text,
    ADD COLUMN password_changed_at timestamptz,
    ADD COLUMN must_change_password boolean NOT NULL DEFAULT false;

UPDATE users AS user_record
   SET password_hash = credential.password_hash,
       password_changed_at = credential.password_changed_at,
       must_change_password = credential.must_change_password
  FROM credentials AS credential
 WHERE credential.user_id = user_record.id;

ALTER TABLE users
    ALTER COLUMN password_hash SET NOT NULL,
    ALTER COLUMN password_changed_at SET NOT NULL,
    ALTER COLUMN password_changed_at SET DEFAULT now();

DROP TABLE credentials;

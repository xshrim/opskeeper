ALTER TABLE diagnosis_sessions
    ADD COLUMN scope_shared boolean NOT NULL DEFAULT false;

-- Preserve access to historical sessions that predate user ownership rules.
-- Newly created sessions are private unless their creator explicitly shares
-- them with the diagnosis scope.
UPDATE diagnosis_sessions
   SET scope_shared = true
 WHERE actor_user_id IS NULL;

CREATE INDEX diagnosis_sessions_visible_idx
    ON diagnosis_sessions(scope_id, scope_shared, actor_user_id, created_at DESC);

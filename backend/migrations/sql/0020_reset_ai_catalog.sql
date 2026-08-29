-- The AI resource contract changed incompatibly. Existing catalog entries are
-- retired so operators can recreate AIProvider and AIEndpoint resources under
-- the new schema without carrying forward legacy configuration.
UPDATE resources
   SET deleted_at = COALESCE(deleted_at, now()), updated_at = now()
 WHERE kind IN ('AIProvider', 'AIEndpoint');

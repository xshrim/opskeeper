-- This migration intentionally performs an irreversible hard cut. Restoring
-- deleted resource rows would recreate the retired architecture and could not
-- recover the original encrypted credentials or resource permissions.
DO $$
BEGIN
  RAISE EXCEPTION '0067_remove_legacy_ai_resources is irreversible';
END $$;

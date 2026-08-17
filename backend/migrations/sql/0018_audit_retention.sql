CREATE TABLE audit_retention_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cutoff timestamptz NOT NULL,
    export_reference text NOT NULL CHECK (length(btrim(export_reference)) BETWEEN 1 AND 512),
    requested_by text NOT NULL CHECK (length(btrim(requested_by)) BETWEEN 1 AND 255),
    deleted_count bigint NOT NULL CHECK (deleted_count >= 0),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE OR REPLACE FUNCTION protect_audit_rows()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' AND current_setting('opsk.audit_retention', true) = 'enabled' THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION '% is not allowed on append-only table %', TG_OP, TG_TABLE_NAME;
END;
$$;

CREATE TRIGGER audit_events_append_only
BEFORE UPDATE OR DELETE ON audit_events
FOR EACH ROW EXECUTE FUNCTION protect_audit_rows();

CREATE TRIGGER audit_retention_runs_append_only
BEFORE UPDATE OR DELETE ON audit_retention_runs
FOR EACH ROW EXECUTE FUNCTION protect_audit_rows();

CREATE OR REPLACE FUNCTION prune_audit_events(
    p_cutoff timestamptz,
    p_export_reference text,
    p_requested_by text
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
    removed bigint;
BEGIN
    IF p_cutoff IS NULL OR p_cutoff >= now() - interval '30 days' THEN
        RAISE EXCEPTION 'audit cutoff must be at least 30 days old';
    END IF;
    IF length(btrim(COALESCE(p_export_reference, ''))) NOT BETWEEN 1 AND 512 THEN
        RAISE EXCEPTION 'export reference is required';
    END IF;
    IF length(btrim(COALESCE(p_requested_by, ''))) NOT BETWEEN 1 AND 255 THEN
        RAISE EXCEPTION 'requested by is required';
    END IF;

    PERFORM pg_advisory_xact_lock(5715441255318348116);
    PERFORM set_config('opsk.audit_retention', 'enabled', true);
    DELETE FROM audit_events WHERE created_at < p_cutoff;
    GET DIAGNOSTICS removed = ROW_COUNT;
    INSERT INTO audit_retention_runs (cutoff, export_reference, requested_by, deleted_count)
    VALUES (p_cutoff, btrim(p_export_reference), btrim(p_requested_by), removed);
    RETURN removed;
END;
$$;

COMMENT ON FUNCTION prune_audit_events(timestamptz, text, text) IS
'Delete exported audit events older than the retention cutoff and append an immutable retention record.';

REVOKE ALL ON FUNCTION prune_audit_events(timestamptz, text, text) FROM PUBLIC;

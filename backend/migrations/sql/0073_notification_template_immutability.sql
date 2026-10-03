CREATE FUNCTION prevent_published_notification_template_changes() RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.status <> 'draft' AND (
        NEW.template_id IS DISTINCT FROM OLD.template_id OR
        NEW.scope_id IS DISTINCT FROM OLD.scope_id OR
        NEW.version IS DISTINCT FROM OLD.version OR
        NEW.format IS DISTINCT FROM OLD.format OR
        NEW.title_template IS DISTINCT FROM OLD.title_template OR
        NEW.body_template IS DISTINCT FROM OLD.body_template OR
        NEW.payload_template IS DISTINCT FROM OLD.payload_template OR
        NEW.variables IS DISTINCT FROM OLD.variables OR
        NEW.content_hash IS DISTINCT FROM OLD.content_hash OR
        NEW.created_by IS DISTINCT FROM OLD.created_by OR
        NEW.created_at IS DISTINCT FROM OLD.created_at OR
        NEW.published_at IS DISTINCT FROM OLD.published_at OR
        NOT (NEW.status = OLD.status OR (OLD.status = 'published' AND NEW.status = 'disabled'))
    ) THEN
        RAISE EXCEPTION 'published notification template versions are immutable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER notification_template_versions_immutable
BEFORE UPDATE ON notification_template_versions
FOR EACH ROW EXECUTE FUNCTION prevent_published_notification_template_changes();

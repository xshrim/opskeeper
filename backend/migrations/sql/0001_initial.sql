--
-- PostgreSQL database dump
--


-- Dumped from database version 16.15 (Debian 16.15-1.pgdg12+2)
-- Dumped by pg_dump version 16.15 (Debian 16.15-1.pgdg12+2)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


--
-- Name: ai_workflow_runs_touch(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION ai_workflow_runs_touch() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN NEW.updated_at = now(); RETURN NEW; END $$;


--
-- Name: bump_authorization_revision(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION bump_authorization_revision() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    UPDATE authorization_revision SET revision = revision + 1 WHERE id = true;
    RETURN COALESCE(NEW, OLD);
END;
$$;


--
-- Name: prevent_published_notification_template_changes(); Type: FUNCTION; Schema: public; Owner: -
--

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


--
-- Name: protect_audit_rows(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION protect_audit_rows() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF TG_OP = 'DELETE' AND current_setting('opsk.audit_retention', true) = 'enabled' THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION '% is not allowed on append-only table %', TG_OP, TG_TABLE_NAME;
END;
$$;


--
-- Name: prune_audit_events(timestamp with time zone, text, text); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION prune_audit_events(p_cutoff timestamp with time zone, p_export_reference text, p_requested_by text) RETURNS bigint
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


--
-- Name: FUNCTION prune_audit_events(p_cutoff timestamp with time zone, p_export_reference text, p_requested_by text); Type: COMMENT; Schema: public; Owner: -
--

COMMENT ON FUNCTION prune_audit_events(p_cutoff timestamp with time zone, p_export_reference text, p_requested_by text) IS 'Delete exported audit events older than the retention cutoff and append an immutable retention record.';


--
-- Name: reject_notification_attempt_mutation(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION reject_notification_attempt_mutation() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    RAISE EXCEPTION 'notification delivery attempts are append-only';
END;
$$;


--
-- Name: reject_notification_event_mutation(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION reject_notification_event_mutation() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    RAISE EXCEPTION 'notification events are immutable';
END;
$$;


--
-- Name: reject_notification_route_snapshot_mutation(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION reject_notification_route_snapshot_mutation() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF ROW(NEW.scope_id, NEW.event_id, NEW.rule_id, NEW.channel_id, NEW.channel_version_id, NEW.template_version_id, NEW.idempotency_key, NEW.route_snapshot, NEW.snapshot_hash)
       IS DISTINCT FROM ROW(OLD.scope_id, OLD.event_id, OLD.rule_id, OLD.channel_id, OLD.channel_version_id, OLD.template_version_id, OLD.idempotency_key, OLD.route_snapshot, OLD.snapshot_hash) THEN
        RAISE EXCEPTION 'notification delivery route snapshot is immutable';
    END IF;
    RETURN NEW;
END;
$$;


--
-- Name: reject_notification_share_revocation(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION reject_notification_share_revocation() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    in_use boolean;
BEGIN
    IF OLD.share_with_children AND NOT NEW.share_with_children THEN
        IF TG_TABLE_NAME = 'notification_rules' THEN
            SELECT EXISTS (
                SELECT 1 FROM inspection_policy_notification_rules binding
                 WHERE binding.rule_id = OLD.id
                   AND binding.rule_scope_id = OLD.scope_id
                   AND binding.scope_id <> binding.rule_scope_id
            ) INTO in_use;
        ELSIF TG_TABLE_NAME = 'notification_channels' THEN
            SELECT EXISTS (
                SELECT 1
                  FROM notification_rule_routes route
                  JOIN inspection_policy_notification_rules binding
                    ON binding.rule_id = route.rule_id AND binding.rule_scope_id = route.scope_id
                 WHERE route.channel_id = OLD.id
                   AND route.scope_id = OLD.scope_id
                   AND route.retired_at IS NULL
                   AND binding.scope_id <> binding.rule_scope_id
            ) INTO in_use;
        ELSE
            SELECT EXISTS (
                SELECT 1
                  FROM notification_rule_routes route
                  JOIN notification_template_versions version
                    ON version.id = route.template_version_id AND version.scope_id = route.scope_id
                  JOIN inspection_policy_notification_rules binding
                    ON binding.rule_id = route.rule_id AND binding.rule_scope_id = route.scope_id
                 WHERE version.template_id = OLD.id
                   AND version.scope_id = OLD.scope_id
                   AND route.retired_at IS NULL
                   AND binding.scope_id <> binding.rule_scope_id
            ) INTO in_use;
        END IF;

        IF in_use THEN
            RAISE EXCEPTION 'notification sharing cannot be revoked while child policies use the object' USING ERRCODE = '23514';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;


--
-- Name: resource_scope_contains(uuid, uuid); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION resource_scope_contains(ancestor_id uuid, descendant_id uuid) RETURNS boolean
    LANGUAGE sql STABLE
    AS $$
    WITH RECURSIVE chain(id) AS (
        SELECT descendant_id
        UNION
        SELECT scopes.parent_scope_id
          FROM scopes
          JOIN chain ON chain.id = scopes.id
         WHERE scopes.parent_scope_id IS NOT NULL
    )
    SELECT EXISTS (SELECT 1 FROM chain WHERE id = ancestor_id);
$$;


--
-- Name: scope_ai_provider_bindings_touch(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION scope_ai_provider_bindings_touch() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN NEW.updated_at = now(); RETURN NEW; END $$;


--
-- Name: validate_application_runtime(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION validate_application_runtime() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE app_runtime text; resource_kind text;
BEGIN
  SELECT runtime_kind INTO app_runtime FROM applications WHERE id = NEW.application_id AND deleted_at IS NULL;
  SELECT kind INTO resource_kind FROM resources WHERE id = NEW.target_resource_id AND deleted_at IS NULL;
  IF app_runtime IS NULL THEN
    RAISE EXCEPTION 'application runtime kind is missing' USING ERRCODE = '23514';
  END IF;
  IF NEW.runtime_kind <> app_runtime
     OR (app_runtime = 'virtual_machine' AND resource_kind <> 'Host')
     OR (app_runtime = 'containerized' AND resource_kind <> 'Docker')
     OR (app_runtime = 'cloud_native' AND resource_kind <> 'Kubernetes') THEN
    RAISE EXCEPTION 'application instance resource kind does not match application runtime kind' USING ERRCODE = '23514';
  END IF;
  IF app_runtime = 'virtual_machine' AND COALESCE(NEW.selector->>'process_command_line', '') = '' THEN
    RAISE EXCEPTION 'Host instances require selector.process_command_line' USING ERRCODE = '23514';
  END IF;
  IF app_runtime = 'containerized' AND COALESCE(NEW.selector->>'container_name', '') = '' THEN
    RAISE EXCEPTION 'Docker instances require selector.container_name' USING ERRCODE = '23514';
  END IF;
  IF app_runtime = 'cloud_native' AND (COALESCE(NEW.selector->>'namespace', '') = '' OR COALESCE(NEW.selector->>'workload_kind', '') = '' OR COALESCE(NEW.selector->>'workload_name', '') = '') THEN
    RAISE EXCEPTION 'Kubernetes instances require namespace, workload_kind and workload_name' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$;


--
-- Name: validate_application_target(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION validate_application_target() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE project_scope uuid; target_scope uuid;
BEGIN
  SELECT p.scope_id INTO project_scope FROM applications a JOIN projects p ON p.id=a.project_id WHERE a.id=NEW.application_id AND a.deleted_at IS NULL;
  SELECT scope_id INTO target_scope FROM resources WHERE id=NEW.target_resource_id AND deleted_at IS NULL;
  IF project_scope IS NULL OR target_scope IS NULL OR NOT resource_scope_contains(target_scope, project_scope) THEN
    RAISE EXCEPTION 'application target resource is outside project scope' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END; $$;


--
-- Name: validate_engine_scope_binding(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION validate_engine_scope_binding() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE engine_scope uuid; engine_status text;
BEGIN
    SELECT scope_id, status INTO engine_scope, engine_status
      FROM engines WHERE id = NEW.engine_id AND deleted_at IS NULL;
    IF engine_scope IS NULL OR engine_status <> 'active' OR NOT resource_scope_contains(engine_scope, NEW.scope_id) THEN
        RAISE EXCEPTION 'Engine is unavailable from scope' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;


--
-- Name: validate_llm_scope_default(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION validate_llm_scope_default() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE endpoint_scope uuid; endpoint_kind text; endpoint_status text;
BEGIN
  SELECT scope_id, kind, status INTO endpoint_scope, endpoint_kind, endpoint_status
    FROM resources WHERE id = NEW.ai_endpoint_resource_id AND deleted_at IS NULL;
  IF endpoint_kind IS DISTINCT FROM 'AIEndpoint' OR endpoint_status IS DISTINCT FROM 'active'
     OR NOT resource_scope_contains(endpoint_scope, NEW.scope_id) THEN
    RAISE EXCEPTION 'default AIEndpoint is unavailable from scope' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END; $$;


--
-- Name: validate_notification_route_child_sharing(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION validate_notification_route_child_sharing() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    has_child_bindings boolean;
    channel_shared boolean;
    template_shared boolean;
BEGIN
    PERFORM 1
      FROM notification_rules rule
     WHERE rule.id = NEW.rule_id
       AND rule.scope_id = NEW.scope_id
     FOR UPDATE;

    SELECT EXISTS (
        SELECT 1 FROM inspection_policy_notification_rules binding
         WHERE binding.rule_id = NEW.rule_id
           AND binding.rule_scope_id = NEW.scope_id
           AND binding.scope_id <> binding.rule_scope_id
    ) INTO has_child_bindings;

    IF NOT has_child_bindings THEN
        RETURN NEW;
    END IF;

    SELECT channel.share_with_children, template_record.share_with_children
      INTO channel_shared, template_shared
      FROM notification_channels channel
      JOIN notification_template_versions version
        ON version.id = NEW.template_version_id AND version.scope_id = NEW.scope_id
      JOIN notification_templates template_record
        ON template_record.id = version.template_id AND template_record.scope_id = version.scope_id
     WHERE channel.id = NEW.channel_id
       AND channel.scope_id = NEW.scope_id
     FOR SHARE OF channel, template_record;

    IF NOT COALESCE(channel_shared, false) OR NOT COALESCE(template_shared, false) THEN
        RAISE EXCEPTION 'notification routes used by child policies must share their channels and templates' USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;


--
-- Name: validate_organization_scope(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION validate_organization_scope() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    actual_scope_type text;
    actual_parent uuid;
    expected_parent uuid;
BEGIN
    SELECT scope_type, parent_scope_id
      INTO actual_scope_type, actual_parent
      FROM scopes
     WHERE id = NEW.scope_id
       AND deleted_at IS NULL;

    IF TG_TABLE_NAME = 'platforms' THEN
        IF actual_scope_type <> 'platform' OR actual_parent IS NOT NULL THEN
            RAISE EXCEPTION 'platform must reference a platform root scope' USING ERRCODE = '23514';
        END IF;
    ELSIF TG_TABLE_NAME = 'teams' THEN
        SELECT scope_id INTO expected_parent FROM platforms WHERE id = NEW.platform_id AND deleted_at IS NULL;
        IF actual_scope_type <> 'team' OR actual_parent IS DISTINCT FROM expected_parent THEN
            RAISE EXCEPTION 'team scope does not match platform' USING ERRCODE = '23514';
        END IF;
    ELSIF TG_TABLE_NAME = 'projects' THEN
        SELECT scope_id INTO expected_parent FROM teams WHERE id = NEW.team_id AND platform_id = NEW.platform_id AND deleted_at IS NULL;
        IF actual_scope_type <> 'project' OR actual_parent IS DISTINCT FROM expected_parent THEN
            RAISE EXCEPTION 'project scope does not match team and platform' USING ERRCODE = '23514';
        END IF;
    END IF;

    RETURN NEW;
END;
$$;


--
-- Name: validate_policy_notification_rule_scope(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION validate_policy_notification_rule_scope() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    can_share boolean;
    route_count integer;
    all_routes_shared boolean;
BEGIN
    IF NEW.scope_id = NEW.rule_scope_id THEN
        RETURN NEW;
    END IF;

    IF NOT resource_scope_contains(NEW.rule_scope_id, NEW.scope_id) THEN
        RAISE EXCEPTION 'notification rule is outside the policy Scope chain' USING ERRCODE = '23514';
    END IF;

    SELECT rule.share_with_children
      INTO can_share
      FROM notification_rules rule
     WHERE rule.id = NEW.rule_id
       AND rule.scope_id = NEW.rule_scope_id
       AND rule.status = 'active'
       AND rule.deleted_at IS NULL
     FOR UPDATE;

    IF NOT COALESCE(can_share, false) THEN
        RAISE EXCEPTION 'notification rule is not shared with child Scopes' USING ERRCODE = '23514';
    END IF;

    PERFORM 1
      FROM notification_rule_routes route
      JOIN notification_channels channel ON channel.id = route.channel_id AND channel.scope_id = route.scope_id
      JOIN notification_template_versions template ON template.id = route.template_version_id AND template.scope_id = route.scope_id
      JOIN notification_templates template_record ON template_record.id = template.template_id AND template_record.scope_id = template.scope_id
     WHERE route.rule_id = NEW.rule_id
       AND route.scope_id = NEW.rule_scope_id
       AND route.retired_at IS NULL
     ORDER BY channel.id, template_record.id
     FOR SHARE OF channel, template_record;

    SELECT count(*), COALESCE(bool_and(channel.share_with_children AND template_record.share_with_children), false)
      INTO route_count, all_routes_shared
      FROM notification_rule_routes route
      JOIN notification_channels channel ON channel.id = route.channel_id AND channel.scope_id = route.scope_id
      JOIN notification_template_versions template ON template.id = route.template_version_id AND template.scope_id = route.scope_id
      JOIN notification_templates template_record ON template_record.id = template.template_id AND template_record.scope_id = template.scope_id
     WHERE route.rule_id = NEW.rule_id
       AND route.scope_id = NEW.rule_scope_id
       AND route.retired_at IS NULL;

    IF route_count = 0 OR NOT all_routes_shared THEN
        RAISE EXCEPTION 'notification rule routes must share their channels and templates with child Scopes' USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;


--
-- Name: validate_provider_scope_binding(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION validate_provider_scope_binding() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE provider_scope uuid; provider_status text;
BEGIN
    SELECT scope_id, status INTO provider_scope, provider_status
      FROM providers WHERE id = NEW.provider_id AND deleted_at IS NULL;
    IF provider_scope IS NULL OR provider_status <> 'active' OR NOT resource_scope_contains(provider_scope, NEW.scope_id) THEN
        RAISE EXCEPTION 'Provider is unavailable from scope' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;


--
-- Name: validate_resource_relation(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION validate_resource_relation() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    source_scope uuid;
    target_scope uuid;
    cycle_found boolean;
BEGIN
    IF NEW.source_resource_id = NEW.target_resource_id THEN
        RAISE EXCEPTION 'resource relation cannot point to itself' USING ERRCODE = '23514';
    END IF;
    SELECT scope_id INTO source_scope FROM resources WHERE id = NEW.source_resource_id AND deleted_at IS NULL;
    SELECT scope_id INTO target_scope FROM resources WHERE id = NEW.target_resource_id AND deleted_at IS NULL;
    IF source_scope IS NULL OR target_scope IS NULL OR NOT resource_scope_contains(target_scope, source_scope) THEN
        RAISE EXCEPTION 'resource relation target is outside the source scope chain' USING ERRCODE = '23514';
    END IF;

    WITH RECURSIVE reachable(id) AS (
        SELECT NEW.target_resource_id
        UNION
        SELECT relation.target_resource_id
          FROM resource_relations relation
          JOIN reachable ON reachable.id = relation.source_resource_id
         WHERE relation.id <> COALESCE(NEW.id, '00000000-0000-0000-0000-000000000000')::uuid
    )
    SELECT EXISTS (SELECT 1 FROM reachable WHERE id = NEW.source_resource_id) INTO cycle_found;
    IF cycle_found THEN
        RAISE EXCEPTION 'resource relation would create a cycle' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;


--
-- Name: validate_resource_role_binding(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION validate_resource_role_binding() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NEW.subject_type = 'user' AND NOT EXISTS (
        SELECT 1 FROM users WHERE id = NEW.subject_id AND deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'resource role binding subject user does not exist' USING ERRCODE = '23503';
    END IF;
    IF NEW.subject_type = 'group' AND NOT EXISTS (
        SELECT 1 FROM groups WHERE id = NEW.subject_id AND deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'resource role binding subject group does not exist' USING ERRCODE = '23503';
    END IF;
    RETURN NEW;
END;
$$;


--
-- Name: validate_resource_scope_move(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION validate_resource_scope_move() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    relation record;
    other_scope uuid;
BEGIN
    IF NEW.scope_id = OLD.scope_id THEN
        RETURN NEW;
    END IF;
    FOR relation IN
        SELECT source_resource_id, target_resource_id
          FROM resource_relations
         WHERE source_resource_id = NEW.id OR target_resource_id = NEW.id
    LOOP
        IF relation.source_resource_id = NEW.id THEN
            SELECT scope_id INTO other_scope FROM resources WHERE id = relation.target_resource_id;
            IF NOT resource_scope_contains(other_scope, NEW.scope_id) THEN
                RAISE EXCEPTION 'resource scope move would invalidate relation' USING ERRCODE = '23514';
            END IF;
        ELSE
            SELECT scope_id INTO other_scope FROM resources WHERE id = relation.source_resource_id;
            IF NOT resource_scope_contains(NEW.scope_id, other_scope) THEN
                RAISE EXCEPTION 'resource scope move would invalidate relation' USING ERRCODE = '23514';
            END IF;
        END IF;
    END LOOP;
    RETURN NEW;
END;
$$;


--
-- Name: validate_role_binding(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION validate_role_binding() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    role_scope_type text;
    binding_scope_type text;
    subject_exists boolean;
BEGIN
    SELECT scope_type INTO role_scope_type FROM roles WHERE id = NEW.role_id;
    SELECT scope_type INTO binding_scope_type
      FROM scopes WHERE id = NEW.scope_id AND deleted_at IS NULL;
    IF NEW.subject_type = 'user' THEN
        SELECT EXISTS (SELECT 1 FROM users WHERE id = NEW.subject_id AND deleted_at IS NULL)
          INTO subject_exists;
    ELSE
        SELECT EXISTS (SELECT 1 FROM groups WHERE id = NEW.subject_id AND deleted_at IS NULL AND status = 'active')
          INTO subject_exists;
    END IF;

    IF role_scope_type IS NULL OR binding_scope_type IS NULL OR role_scope_type <> binding_scope_type THEN
        RAISE EXCEPTION 'role binding scope type does not match role' USING ERRCODE = '23514';
    END IF;
    IF NOT subject_exists THEN
        RAISE EXCEPTION 'role binding subject does not exist or is inactive' USING ERRCODE = '23503';
    END IF;
    RETURN NEW;
END;
$$;


--
-- Name: validate_scope_ai_provider_binding(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION validate_scope_ai_provider_binding() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE provider_scope uuid; provider_kind text; provider_status text;
BEGIN
    SELECT scope_id, kind, status INTO provider_scope, provider_kind, provider_status
      FROM resources WHERE id = NEW.provider_resource_id AND deleted_at IS NULL;
    IF provider_kind IS DISTINCT FROM 'AIProvider'
       OR provider_status IS DISTINCT FROM 'active'
       OR NOT resource_scope_contains(provider_scope, NEW.scope_id) THEN
        RAISE EXCEPTION 'AIProvider is unavailable from scope' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;


--
-- Name: validate_scope_default(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION validate_scope_default() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    default_scope uuid;
    resource_scope uuid;
    scope_status text;
BEGIN
    SELECT scope_id INTO resource_scope FROM resources WHERE id = NEW.resource_id AND deleted_at IS NULL;
    SELECT id, status INTO default_scope, scope_status FROM scopes WHERE id = NEW.scope_id AND deleted_at IS NULL;
    IF default_scope IS NULL OR scope_status <> 'active' OR resource_scope IS NULL OR NOT resource_scope_contains(resource_scope, NEW.scope_id) THEN
        RAISE EXCEPTION 'scope default resource is not visible from scope' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;


--
-- Name: validate_scope_parent(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION validate_scope_parent() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    parent_type text;
    parent_tenant text;
BEGIN
    IF NEW.scope_type = 'platform' THEN
        IF NEW.parent_scope_id IS NOT NULL THEN
            RAISE EXCEPTION 'platform scope cannot have a parent' USING ERRCODE = '23514';
        END IF;
        RETURN NEW;
    END IF;

    SELECT scope_type, tenant_id
      INTO parent_type, parent_tenant
      FROM scopes
     WHERE id = NEW.parent_scope_id
       AND deleted_at IS NULL;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'scope parent does not exist' USING ERRCODE = '23503';
    END IF;
    IF parent_tenant <> NEW.tenant_id THEN
        RAISE EXCEPTION 'scope parent belongs to a different tenant' USING ERRCODE = '23514';
    END IF;
    IF NEW.scope_type = 'team' AND parent_type <> 'platform' THEN
        RAISE EXCEPTION 'team scope parent must be platform' USING ERRCODE = '23514';
    END IF;
    IF NEW.scope_type = 'project' AND parent_type <> 'team' THEN
        RAISE EXCEPTION 'project scope parent must be team' USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;


--
-- Name: validate_skill_scope_default(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION validate_skill_scope_default() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
    skill_scope uuid;
    skill_status text;
    version_skill uuid;
    version_status text;
BEGIN
    SELECT scope_id, status INTO skill_scope, skill_status
      FROM skills WHERE id = NEW.skill_id AND deleted_at IS NULL;
    SELECT skill_id, status INTO version_skill, version_status
      FROM skill_versions WHERE id = NEW.skill_version_id;
    IF skill_status IS DISTINCT FROM 'active'
       OR version_skill IS DISTINCT FROM NEW.skill_id
       OR version_status IS DISTINCT FROM 'published'
       OR NOT resource_scope_contains(skill_scope, NEW.scope_id) THEN
        RAISE EXCEPTION 'default Skill version is unavailable from scope' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;


--
-- Name: validate_skill_version(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION validate_skill_version() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM skills WHERE id = NEW.skill_id AND deleted_at IS NULL) THEN
        RAISE EXCEPTION 'skill version must belong to an active Skill' USING ERRCODE = '23514';
    END IF;
    IF TG_OP = 'UPDATE' AND (
        NEW.skill_id <> OLD.skill_id OR NEW.version <> OLD.version
        OR NEW.manifest <> OLD.manifest OR NEW.input_schema <> OLD.input_schema
        OR NEW.output_schema <> OLD.output_schema OR NEW.tools <> OLD.tools
        OR NEW.risk_level <> OLD.risk_level
    ) THEN
        RAISE EXCEPTION 'skill version content is immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: ai_workflow_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE ai_workflow_runs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    workflow_resource_id uuid NOT NULL,
    workflow_version integer NOT NULL,
    execution_id text NOT NULL,
    scope_id uuid NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    current_node_id text DEFAULT ''::text NOT NULL,
    attempt integer DEFAULT 0 NOT NULL,
    input jsonb DEFAULT '{}'::jsonb NOT NULL,
    state jsonb DEFAULT '{}'::jsonb NOT NULL,
    error_code text DEFAULT ''::text NOT NULL,
    error_message text DEFAULT ''::text NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    CONSTRAINT ai_workflow_runs_attempt_check CHECK ((attempt >= 0)),
    CONSTRAINT ai_workflow_runs_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'running'::text, 'waiting_approval'::text, 'succeeded'::text, 'failed'::text, 'cancelled'::text]))),
    CONSTRAINT ai_workflow_runs_workflow_version_check CHECK ((workflow_version > 0))
);


--
-- Name: application_dependencies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE application_dependencies (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    application_id uuid NOT NULL,
    target_resource_id uuid NOT NULL,
    dependency_kind text NOT NULL,
    binding jsonb DEFAULT '{}'::jsonb NOT NULL,
    required boolean DEFAULT true NOT NULL,
    status text DEFAULT 'unknown'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT application_dependencies_dependency_kind_check CHECK ((dependency_kind = ANY (ARRAY['messaging'::text, 'cache'::text, 'database'::text, 'registry'::text, 'configuration'::text, 'repository'::text, 'storage'::text, 'observability'::text, 'other'::text]))),
    CONSTRAINT application_dependencies_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text, 'unknown'::text])))
);


--
-- Name: application_instances; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE application_instances (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    application_id uuid NOT NULL,
    name text NOT NULL,
    runtime_kind text NOT NULL,
    target_resource_id uuid NOT NULL,
    selector jsonb DEFAULT '{}'::jsonb NOT NULL,
    log_binding jsonb DEFAULT '{}'::jsonb NOT NULL,
    status text DEFAULT 'unknown'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT application_instances_name_check CHECK (((length(btrim(name)) >= 1) AND (length(btrim(name)) <= 120))),
    CONSTRAINT application_instances_runtime_kind_check CHECK ((runtime_kind = ANY (ARRAY['virtual_machine'::text, 'containerized'::text, 'cloud_native'::text]))),
    CONSTRAINT application_instances_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text, 'unknown'::text])))
);


--
-- Name: applications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE applications (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    project_id uuid NOT NULL,
    name text NOT NULL,
    code text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    icon text DEFAULT 'AppWindow'::text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    external_uid text DEFAULT ''::text NOT NULL,
    labels jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    runtime_kind text DEFAULT 'virtual_machine'::text NOT NULL,
    critical boolean DEFAULT false NOT NULL,
    technical_owner_ids uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    operations_owner_ids uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    CONSTRAINT applications_code_check CHECK ((code ~ '^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$'::text)),
    CONSTRAINT applications_icon_length CHECK (((length(icon) >= 1) AND (length(icon) <= 262144))),
    CONSTRAINT applications_name_check CHECK (((length(btrim(name)) >= 1) AND (length(btrim(name)) <= 120))),
    CONSTRAINT applications_runtime_kind_check CHECK ((runtime_kind = ANY (ARRAY['virtual_machine'::text, 'containerized'::text, 'cloud_native'::text]))),
    CONSTRAINT applications_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text, 'unknown'::text])))
);


--
-- Name: audit_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE audit_events (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    actor_user_id uuid,
    action text NOT NULL,
    target_type text DEFAULT ''::text NOT NULL,
    target_id text DEFAULT ''::text NOT NULL,
    scope_id uuid,
    result text NOT NULL,
    request_id text DEFAULT ''::text NOT NULL,
    client_ip text DEFAULT ''::text NOT NULL,
    details jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT audit_events_action_check CHECK (((length(btrim(action)) >= 1) AND (length(btrim(action)) <= 120))),
    CONSTRAINT audit_events_client_ip_check CHECK ((length(client_ip) <= 255)),
    CONSTRAINT audit_events_details_check CHECK ((jsonb_typeof(details) = 'object'::text)),
    CONSTRAINT audit_events_request_id_check CHECK ((length(request_id) <= 128)),
    CONSTRAINT audit_events_result_check CHECK ((result = ANY (ARRAY['success'::text, 'failure'::text])))
);


--
-- Name: audit_retention_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE audit_retention_runs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    cutoff timestamp with time zone NOT NULL,
    export_reference text NOT NULL,
    requested_by text NOT NULL,
    deleted_count bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT audit_retention_runs_deleted_count_check CHECK ((deleted_count >= 0)),
    CONSTRAINT audit_retention_runs_export_reference_check CHECK (((length(btrim(export_reference)) >= 1) AND (length(btrim(export_reference)) <= 512))),
    CONSTRAINT audit_retention_runs_requested_by_check CHECK (((length(btrim(requested_by)) >= 1) AND (length(btrim(requested_by)) <= 255)))
);


--
-- Name: authorization_revision; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE authorization_revision (
    id boolean DEFAULT true NOT NULL,
    revision bigint DEFAULT 1 NOT NULL,
    CONSTRAINT authorization_revision_id_check CHECK (id)
);


--
-- Name: cache_entries; Type: TABLE; Schema: public; Owner: -
--

CREATE UNLOGGED TABLE cache_entries (
    cache_key text NOT NULL,
    value jsonb NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: diagnosis_causal_chains; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE diagnosis_causal_chains (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    session_id uuid NOT NULL,
    run_id uuid NOT NULL,
    version integer NOT NULL,
    status text NOT NULL,
    summary text DEFAULT ''::text NOT NULL,
    nodes jsonb DEFAULT '[]'::jsonb NOT NULL,
    links jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT diagnosis_causal_chains_links_check CHECK ((jsonb_typeof(links) = 'array'::text)),
    CONSTRAINT diagnosis_causal_chains_nodes_check CHECK ((jsonb_typeof(nodes) = 'array'::text)),
    CONSTRAINT diagnosis_causal_chains_status_check CHECK ((status = ANY (ARRAY['active'::text, 'superseded'::text, 'partial'::text]))),
    CONSTRAINT diagnosis_causal_chains_summary_check CHECK ((length(summary) <= 2000)),
    CONSTRAINT diagnosis_causal_chains_version_check CHECK ((version > 0))
);


--
-- Name: diagnosis_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE diagnosis_events (
    id bigint NOT NULL,
    session_id uuid NOT NULL,
    event_type text NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT diagnosis_events_event_type_check CHECK (((length(event_type) >= 1) AND (length(event_type) <= 120))),
    CONSTRAINT diagnosis_events_payload_check CHECK ((jsonb_typeof(payload) = 'object'::text))
);


--
-- Name: diagnosis_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE diagnosis_events ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME diagnosis_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: diagnosis_evidence; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE diagnosis_evidence (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    session_id uuid NOT NULL,
    target_resource_id uuid,
    source_resource_id uuid,
    capability text DEFAULT ''::text NOT NULL,
    collected_at timestamp with time zone NOT NULL,
    window_start timestamp with time zone,
    window_end timestamp with time zone,
    content_hash text NOT NULL,
    summary jsonb DEFAULT '{}'::jsonb NOT NULL,
    content jsonb DEFAULT '{}'::jsonb NOT NULL,
    partial boolean DEFAULT false NOT NULL,
    untrusted boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    run_id uuid,
    CONSTRAINT diagnosis_evidence_capability_check CHECK ((length(capability) <= 120)),
    CONSTRAINT diagnosis_evidence_check CHECK ((((window_start IS NULL) AND (window_end IS NULL)) OR ((window_start IS NOT NULL) AND (window_end IS NOT NULL) AND (window_start <= window_end)))),
    CONSTRAINT diagnosis_evidence_content_check CHECK ((jsonb_typeof(content) = ANY (ARRAY['object'::text, 'array'::text]))),
    CONSTRAINT diagnosis_evidence_content_hash_check CHECK ((length(content_hash) = 64)),
    CONSTRAINT diagnosis_evidence_summary_check CHECK ((jsonb_typeof(summary) = 'object'::text))
);


--
-- Name: diagnosis_hypotheses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE diagnosis_hypotheses (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    session_id uuid NOT NULL,
    statement text NOT NULL,
    status text NOT NULL,
    confidence numeric(4,3) DEFAULT 0 NOT NULL,
    evidence_ids uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT diagnosis_hypotheses_confidence_check CHECK (((confidence >= (0)::numeric) AND (confidence <= (1)::numeric))),
    CONSTRAINT diagnosis_hypotheses_statement_check CHECK (((length(statement) >= 1) AND (length(statement) <= 4000))),
    CONSTRAINT diagnosis_hypotheses_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'supported'::text, 'rejected'::text, 'needs_verification'::text])))
);


--
-- Name: diagnosis_messages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE diagnosis_messages (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    session_id uuid NOT NULL,
    role text NOT NULL,
    content text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT diagnosis_messages_content_check CHECK ((length(content) <= 16000)),
    CONSTRAINT diagnosis_messages_role_check CHECK ((role = ANY (ARRAY['user'::text, 'assistant'::text, 'system'::text])))
);


--
-- Name: diagnosis_plan_steps; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE diagnosis_plan_steps (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    plan_id uuid NOT NULL,
    sequence integer NOT NULL,
    phase text NOT NULL,
    status text NOT NULL,
    title text NOT NULL,
    detail text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT diagnosis_plan_steps_detail_check CHECK ((length(detail) <= 2000)),
    CONSTRAINT diagnosis_plan_steps_phase_check CHECK ((phase = ANY (ARRAY['plan'::text, 'collect'::text, 'verify'::text, 'summarize'::text]))),
    CONSTRAINT diagnosis_plan_steps_sequence_check CHECK ((sequence > 0)),
    CONSTRAINT diagnosis_plan_steps_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'running'::text, 'succeeded'::text, 'failed'::text, 'skipped'::text]))),
    CONSTRAINT diagnosis_plan_steps_title_check CHECK (((length(title) >= 1) AND (length(title) <= 300)))
);


--
-- Name: diagnosis_plans; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE diagnosis_plans (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    session_id uuid NOT NULL,
    summary text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT diagnosis_plans_summary_check CHECK ((length(summary) <= 2000))
);


--
-- Name: diagnosis_reports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE diagnosis_reports (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    session_id uuid NOT NULL,
    status text NOT NULL,
    conclusion text DEFAULT ''::text NOT NULL,
    recommendations jsonb DEFAULT '[]'::jsonb NOT NULL,
    evidence_ids uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT diagnosis_reports_conclusion_check CHECK ((length(conclusion) <= 8000)),
    CONSTRAINT diagnosis_reports_recommendations_check CHECK ((jsonb_typeof(recommendations) = 'array'::text)),
    CONSTRAINT diagnosis_reports_status_check CHECK ((status = ANY (ARRAY['succeeded'::text, 'warning'::text, 'failed'::text])))
);


--
-- Name: diagnosis_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE diagnosis_runs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    session_id uuid NOT NULL,
    sequence integer NOT NULL,
    question_message_id uuid,
    status text DEFAULT 'running'::text NOT NULL,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    CONSTRAINT diagnosis_runs_sequence_check CHECK ((sequence > 0)),
    CONSTRAINT diagnosis_runs_status_check CHECK ((status = ANY (ARRAY['running'::text, 'succeeded'::text, 'failed'::text, 'cancelled'::text])))
);


--
-- Name: diagnosis_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE diagnosis_sessions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    scope_id uuid NOT NULL,
    actor_user_id uuid,
    status text NOT NULL,
    title text DEFAULT ''::text NOT NULL,
    error_code text DEFAULT ''::text NOT NULL,
    error_message text DEFAULT ''::text NOT NULL,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    provider_id uuid,
    model_name text,
    application_id uuid,
    CONSTRAINT diagnosis_sessions_error_code_check CHECK ((length(error_code) <= 120)),
    CONSTRAINT diagnosis_sessions_error_message_check CHECK ((length(error_message) <= 1000)),
    CONSTRAINT diagnosis_sessions_status_check CHECK ((status = ANY (ARRAY['queued'::text, 'planning'::text, 'collecting'::text, 'analyzing'::text, 'succeeded'::text, 'failed'::text, 'cancelled'::text]))),
    CONSTRAINT diagnosis_sessions_title_check CHECK ((length(title) <= 200))
);


--
-- Name: diagnosis_targets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE diagnosis_targets (
    session_id uuid NOT NULL,
    resource_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: engine_capability_terms; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE engine_capability_terms (
    term text NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT engine_capability_terms_term_check CHECK (((length(btrim(term)) >= 1) AND (length(btrim(term)) <= 80)))
);


--
-- Name: engine_execution_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE engine_execution_events (
    execution_id text NOT NULL,
    sequence bigint NOT NULL,
    type text NOT NULL,
    status text,
    occurred_at timestamp with time zone DEFAULT now() NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    CONSTRAINT ai_execution_events_sequence_check CHECK ((sequence > 0))
);


--
-- Name: engine_execution_tool_calls; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE engine_execution_tool_calls (
    execution_id text NOT NULL,
    sequence integer NOT NULL,
    resource_id uuid,
    tool_name text NOT NULL,
    arguments jsonb DEFAULT '{}'::jsonb NOT NULL,
    output jsonb,
    status text NOT NULL,
    error_message text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    provider_id text,
    model_name text,
    error_code text,
    started_at timestamp with time zone,
    completed_at timestamp with time zone,
    duration_ms bigint DEFAULT 0 NOT NULL,
    CONSTRAINT ai_execution_tool_calls_duration_ms_check CHECK ((duration_ms >= 0)),
    CONSTRAINT ai_execution_tool_calls_sequence_check CHECK ((sequence > 0))
);


--
-- Name: engine_scope_bindings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE engine_scope_bindings (
    scope_id uuid NOT NULL,
    engine_id uuid NOT NULL,
    tag text NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT engine_scope_bindings_tag_check CHECK ((tag = ANY (ARRAY['general'::text, 'diagnosis'::text, 'inspection'::text, 'workflow'::text])))
);


--
-- Name: engines; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE engines (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id text DEFAULT 'default'::text NOT NULL,
    scope_id uuid NOT NULL,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    icon text DEFAULT 'Cpu'::text NOT NULL,
    config jsonb DEFAULT '{}'::jsonb NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT engines_config_check CHECK ((jsonb_typeof(config) = 'object'::text)),
    CONSTRAINT engines_name_check CHECK (((length(btrim(name)) >= 1) AND (length(btrim(name)) <= 200))),
    CONSTRAINT engines_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text, 'unknown'::text])))
);


--
-- Name: group_members; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE group_members (
    group_id uuid NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    created_by uuid
);


--
-- Name: groups; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE groups (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    scope_id uuid NOT NULL,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT groups_description_check CHECK ((length(description) <= 500)),
    CONSTRAINT groups_name_check CHECK (((length(btrim(name)) >= 1) AND (length(btrim(name)) <= 120))),
    CONSTRAINT groups_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text])))
);


--
-- Name: inspection_findings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE inspection_findings (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    policy_id uuid NOT NULL,
    target_resource_id uuid NOT NULL,
    rule text NOT NULL,
    identity_key text NOT NULL,
    fingerprint text NOT NULL,
    severity text NOT NULL,
    message text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'open'::text NOT NULL,
    first_observed_at timestamp with time zone NOT NULL,
    last_observed_at timestamp with time zone NOT NULL,
    resolved_at timestamp with time zone,
    last_run_id uuid,
    scope_id uuid NOT NULL,
    CONSTRAINT inspection_findings_fingerprint_check CHECK ((length(fingerprint) = 64)),
    CONSTRAINT inspection_findings_identity_key_check CHECK ((length(identity_key) = 64)),
    CONSTRAINT inspection_findings_rule_check CHECK (((length(btrim(rule)) >= 1) AND (length(btrim(rule)) <= 200))),
    CONSTRAINT inspection_findings_severity_check CHECK ((severity = ANY (ARRAY['info'::text, 'warning'::text, 'critical'::text]))),
    CONSTRAINT inspection_findings_status_check CHECK ((status = ANY (ARRAY['open'::text, 'resolved'::text])))
);


--
-- Name: inspection_health_snapshots; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE inspection_health_snapshots (
    id bigint NOT NULL,
    run_id uuid NOT NULL,
    policy_id uuid NOT NULL,
    target_resource_id uuid NOT NULL,
    score integer NOT NULL,
    reasons jsonb DEFAULT '[]'::jsonb NOT NULL,
    collected_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT inspection_health_snapshots_reasons_check CHECK ((jsonb_typeof(reasons) = 'array'::text)),
    CONSTRAINT inspection_health_snapshots_score_check CHECK (((score >= 0) AND (score <= 100)))
);


--
-- Name: inspection_health_snapshots_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE inspection_health_snapshots_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: inspection_health_snapshots_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE inspection_health_snapshots_id_seq OWNED BY inspection_health_snapshots.id;


--
-- Name: inspection_jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE inspection_jobs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    run_id uuid NOT NULL,
    idempotency_key text NOT NULL,
    status text DEFAULT 'queued'::text NOT NULL,
    attempt integer DEFAULT 0 NOT NULL,
    max_attempts integer DEFAULT 2 NOT NULL,
    lease_owner text DEFAULT ''::text NOT NULL,
    lease_expires_at timestamp with time zone,
    heartbeat_at timestamp with time zone,
    available_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    error_code text DEFAULT ''::text NOT NULL,
    error_message text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT inspection_jobs_attempt_check CHECK ((attempt >= 0)),
    CONSTRAINT inspection_jobs_idempotency_key_check CHECK (((length(btrim(idempotency_key)) >= 1) AND (length(btrim(idempotency_key)) <= 200))),
    CONSTRAINT inspection_jobs_max_attempts_check CHECK (((max_attempts >= 1) AND (max_attempts <= 11))),
    CONSTRAINT inspection_jobs_status_check CHECK ((status = ANY (ARRAY['queued'::text, 'leased'::text, 'succeeded'::text, 'failed'::text])))
);


--
-- Name: inspection_policies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE inspection_policies (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    scope_id uuid NOT NULL,
    name text NOT NULL,
    cron text NOT NULL,
    timezone text DEFAULT 'UTC'::text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    target_labels jsonb DEFAULT '{}'::jsonb NOT NULL,
    timeout_seconds integer DEFAULT 120 NOT NULL,
    retries integer DEFAULT 1 NOT NULL,
    max_concurrent integer DEFAULT 1 NOT NULL,
    max_tool_calls integer DEFAULT 12 NOT NULL,
    max_tokens bigint DEFAULT 20000 NOT NULL,
    maintenance_windows jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    persona_id uuid,
    CONSTRAINT inspection_policies_cron_check CHECK (((length(btrim(cron)) >= 1) AND (length(btrim(cron)) <= 200))),
    CONSTRAINT inspection_policies_maintenance_windows_check CHECK ((jsonb_typeof(maintenance_windows) = 'array'::text)),
    CONSTRAINT inspection_policies_max_concurrent_check CHECK (((max_concurrent >= 1) AND (max_concurrent <= 64))),
    CONSTRAINT inspection_policies_max_tokens_check CHECK (((max_tokens >= 1) AND (max_tokens <= 200000))),
    CONSTRAINT inspection_policies_max_tool_calls_check CHECK (((max_tool_calls >= 1) AND (max_tool_calls <= 100))),
    CONSTRAINT inspection_policies_name_check CHECK (((length(btrim(name)) >= 1) AND (length(btrim(name)) <= 200))),
    CONSTRAINT inspection_policies_retries_check CHECK (((retries >= 0) AND (retries <= 10))),
    CONSTRAINT inspection_policies_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text]))),
    CONSTRAINT inspection_policies_target_labels_check CHECK ((jsonb_typeof(target_labels) = 'object'::text)),
    CONSTRAINT inspection_policies_timeout_seconds_check CHECK (((timeout_seconds >= 1) AND (timeout_seconds <= 3600))),
    CONSTRAINT inspection_policies_timezone_check CHECK (((length(btrim(timezone)) >= 1) AND (length(btrim(timezone)) <= 100)))
);


--
-- Name: inspection_policy_notification_rules; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE inspection_policy_notification_rules (
    scope_id uuid NOT NULL,
    policy_id uuid NOT NULL,
    rule_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    rule_scope_id uuid NOT NULL
);


--
-- Name: inspection_policy_targets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE inspection_policy_targets (
    policy_id uuid NOT NULL,
    resource_id uuid NOT NULL
);


--
-- Name: inspection_run_steps; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE inspection_run_steps (
    id bigint NOT NULL,
    run_id uuid NOT NULL,
    sequence integer NOT NULL,
    kind text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    detail text DEFAULT ''::text NOT NULL,
    started_at timestamp with time zone,
    completed_at timestamp with time zone,
    CONSTRAINT inspection_run_steps_kind_check CHECK (((length(btrim(kind)) >= 1) AND (length(btrim(kind)) <= 120))),
    CONSTRAINT inspection_run_steps_sequence_check CHECK ((sequence > 0)),
    CONSTRAINT inspection_run_steps_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'running'::text, 'succeeded'::text, 'failed'::text, 'skipped'::text])))
);


--
-- Name: inspection_run_steps_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE inspection_run_steps_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: inspection_run_steps_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE inspection_run_steps_id_seq OWNED BY inspection_run_steps.id;


--
-- Name: inspection_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE inspection_runs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    policy_id uuid NOT NULL,
    scope_id uuid NOT NULL,
    window_start timestamp with time zone NOT NULL,
    window_end timestamp with time zone NOT NULL,
    trigger text NOT NULL,
    status text DEFAULT 'queued'::text NOT NULL,
    policy_snapshot jsonb NOT NULL,
    target_snapshot jsonb DEFAULT '[]'::jsonb NOT NULL,
    score integer,
    deterministic_completed boolean DEFAULT false NOT NULL,
    llm_status text DEFAULT 'not_requested'::text NOT NULL,
    error_code text DEFAULT ''::text NOT NULL,
    error_message text DEFAULT ''::text NOT NULL,
    started_at timestamp with time zone,
    completed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT inspection_runs_llm_status_check CHECK ((llm_status = ANY (ARRAY['not_requested'::text, 'succeeded'::text, 'degraded'::text, 'failed'::text]))),
    CONSTRAINT inspection_runs_policy_snapshot_check CHECK ((jsonb_typeof(policy_snapshot) = 'object'::text)),
    CONSTRAINT inspection_runs_score_check CHECK (((score >= 0) AND (score <= 100))),
    CONSTRAINT inspection_runs_status_check CHECK ((status = ANY (ARRAY['queued'::text, 'running'::text, 'succeeded'::text, 'failed'::text, 'skipped'::text]))),
    CONSTRAINT inspection_runs_target_snapshot_check CHECK ((jsonb_typeof(target_snapshot) = 'array'::text)),
    CONSTRAINT inspection_runs_trigger_check CHECK ((trigger = ANY (ARRAY['schedule'::text, 'manual'::text, 'retry'::text])))
);


--
-- Name: mcp_server_snapshots; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE mcp_server_snapshots (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    server_resource_id uuid NOT NULL,
    scope_id uuid NOT NULL,
    protocol_version text DEFAULT ''::text NOT NULL,
    server_name text DEFAULT ''::text NOT NULL,
    server_version text DEFAULT ''::text NOT NULL,
    tools jsonb DEFAULT '[]'::jsonb NOT NULL,
    content_hash text NOT NULL,
    status text NOT NULL,
    error_message text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT mcp_server_snapshots_content_hash_check CHECK ((length(content_hash) = 64)),
    CONSTRAINT mcp_server_snapshots_status_check CHECK ((status = ANY (ARRAY['succeeded'::text, 'failed'::text]))),
    CONSTRAINT mcp_server_snapshots_tools_check CHECK ((jsonb_typeof(tools) = 'array'::text))
);


--
-- Name: notification_channel_test_limits; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE notification_channel_test_limits (
    channel_id uuid NOT NULL,
    scope_id uuid NOT NULL,
    window_started_at timestamp with time zone NOT NULL,
    attempts integer NOT NULL,
    CONSTRAINT notification_channel_test_limits_attempts_check CHECK ((attempts > 0))
);


--
-- Name: notification_channel_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE notification_channel_versions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    scope_id uuid NOT NULL,
    channel_id uuid NOT NULL,
    version integer NOT NULL,
    provider_config_ciphertext bytea DEFAULT '\x'::bytea NOT NULL,
    provider_config_hash text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    key_version text DEFAULT ''::text NOT NULL,
    CONSTRAINT notification_channel_versions_provider_config_hash_check CHECK ((provider_config_hash ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT notification_channel_versions_version_check CHECK ((version > 0))
);


--
-- Name: notification_channels; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE notification_channels (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    scope_id uuid NOT NULL,
    name text NOT NULL,
    kind text NOT NULL,
    webhook_url text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    rate_limit_per_minute integer DEFAULT 30 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    config_version integer DEFAULT 1 NOT NULL,
    share_with_children boolean DEFAULT false NOT NULL,
    CONSTRAINT notification_channels_config_version_check CHECK ((config_version > 0)),
    CONSTRAINT notification_channels_kind_check CHECK ((kind = 'webhook'::text)),
    CONSTRAINT notification_channels_name_check CHECK (((length(btrim(name)) >= 1) AND (length(btrim(name)) <= 120))),
    CONSTRAINT notification_channels_rate_limit_per_minute_check CHECK (((rate_limit_per_minute >= 1) AND (rate_limit_per_minute <= 600))),
    CONSTRAINT notification_channels_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text]))),
    CONSTRAINT notification_channels_webhook_url_check CHECK (((length(btrim(webhook_url)) >= 1) AND (length(btrim(webhook_url)) <= 2000)))
);


--
-- Name: notification_deliveries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE notification_deliveries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    channel_id uuid NOT NULL,
    finding_id uuid,
    run_id uuid,
    idempotency_key text NOT NULL,
    status text DEFAULT 'queued'::text NOT NULL,
    attempt integer DEFAULT 0 NOT NULL,
    available_at timestamp with time zone DEFAULT now() NOT NULL,
    response_status integer,
    response_body text DEFAULT ''::text NOT NULL,
    error_message text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    scope_id uuid NOT NULL,
    event_id uuid,
    policy_id uuid,
    rule_id uuid,
    route_id uuid,
    channel_version_id uuid,
    template_version_id uuid,
    route_snapshot jsonb DEFAULT '{}'::jsonb NOT NULL,
    snapshot_hash text,
    max_attempts integer DEFAULT 5 NOT NULL,
    lease_owner text DEFAULT ''::text NOT NULL,
    lease_expires_at timestamp with time zone,
    rule_scope_id uuid NOT NULL,
    CONSTRAINT notification_deliveries_attempt_check CHECK ((attempt >= 0)),
    CONSTRAINT notification_deliveries_max_attempts_check CHECK (((max_attempts >= 1) AND (max_attempts <= 20))),
    CONSTRAINT notification_deliveries_route_snapshot_check CHECK ((jsonb_typeof(route_snapshot) = 'object'::text)),
    CONSTRAINT notification_deliveries_routed_event_check CHECK (((event_id IS NULL) OR ((policy_id IS NOT NULL) AND (rule_id IS NOT NULL) AND (route_id IS NOT NULL) AND (channel_version_id IS NOT NULL) AND (template_version_id IS NOT NULL)))),
    CONSTRAINT notification_deliveries_snapshot_hash_check CHECK (((event_id IS NULL) OR (snapshot_hash ~ '^[0-9a-f]{64}$'::text))),
    CONSTRAINT notification_deliveries_status_check CHECK ((status = ANY (ARRAY['queued'::text, 'delivering'::text, 'succeeded'::text, 'failed'::text, 'dead_letter'::text])))
);


--
-- Name: notification_delivery_attempts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE notification_delivery_attempts (
    id bigint NOT NULL,
    scope_id uuid NOT NULL,
    delivery_id uuid NOT NULL,
    attempt integer NOT NULL,
    status text NOT NULL,
    response_status integer,
    response_body text DEFAULT ''::text NOT NULL,
    error_code text DEFAULT ''::text NOT NULL,
    error_message text DEFAULT ''::text NOT NULL,
    started_at timestamp with time zone NOT NULL,
    completed_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT notification_delivery_attempts_attempt_check CHECK ((attempt > 0)),
    CONSTRAINT notification_delivery_attempts_status_check CHECK ((status = ANY (ARRAY['succeeded'::text, 'retrying'::text, 'dead_letter'::text, 'failed'::text])))
);


--
-- Name: notification_delivery_attempts_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE notification_delivery_attempts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: notification_delivery_attempts_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE notification_delivery_attempts_id_seq OWNED BY notification_delivery_attempts.id;


--
-- Name: notification_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE notification_events (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    scope_id uuid NOT NULL,
    event_type text NOT NULL,
    event_key text NOT NULL,
    policy_id uuid,
    run_id uuid,
    finding_id uuid,
    finding_identity text DEFAULT ''::text NOT NULL,
    target_resource_id uuid,
    severity text,
    payload jsonb NOT NULL,
    content_hash text NOT NULL,
    occurred_at timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT notification_events_check CHECK ((((event_type = ANY (ARRAY['finding.opened'::text, 'finding.reopened'::text, 'finding.severity_changed'::text, 'finding.resolved'::text])) AND (finding_id IS NOT NULL) AND (run_id IS NOT NULL) AND (policy_id IS NOT NULL)) OR (event_type = ANY (ARRAY['inspection.failed'::text, 'inspection.degraded'::text, 'notification.test'::text])))),
    CONSTRAINT notification_events_content_hash_check CHECK ((content_hash ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT notification_events_event_key_check CHECK (((length(btrim(event_key)) >= 1) AND (length(btrim(event_key)) <= 300))),
    CONSTRAINT notification_events_event_type_check CHECK ((event_type = ANY (ARRAY['finding.opened'::text, 'finding.reopened'::text, 'finding.severity_changed'::text, 'finding.resolved'::text, 'inspection.failed'::text, 'inspection.degraded'::text, 'notification.test'::text]))),
    CONSTRAINT notification_events_payload_check CHECK ((jsonb_typeof(payload) = 'object'::text)),
    CONSTRAINT notification_events_severity_check CHECK (((severity IS NULL) OR (severity = ANY (ARRAY['info'::text, 'warning'::text, 'critical'::text]))))
);


--
-- Name: notification_rule_cooldowns; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE notification_rule_cooldowns (
    scope_id uuid NOT NULL,
    rule_id uuid NOT NULL,
    cooldown_key text NOT NULL,
    last_queued_at timestamp with time zone NOT NULL,
    CONSTRAINT notification_rule_cooldowns_cooldown_key_check CHECK ((length(cooldown_key) = 64))
);


--
-- Name: notification_rule_routes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE notification_rule_routes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    scope_id uuid NOT NULL,
    rule_id uuid NOT NULL,
    channel_id uuid NOT NULL,
    channel_version_id uuid NOT NULL,
    template_version_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    retired_at timestamp with time zone
);


--
-- Name: notification_rules; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE notification_rules (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    scope_id uuid NOT NULL,
    name text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    share_with_children boolean DEFAULT false NOT NULL,
    event_types text[] NOT NULL,
    minimum_severity text DEFAULT 'info'::text NOT NULL,
    filters jsonb DEFAULT '{}'::jsonb NOT NULL,
    cooldown_seconds integer DEFAULT 0 NOT NULL,
    aggregation_seconds integer DEFAULT 0 NOT NULL,
    max_batch_size integer DEFAULT 1 NOT NULL,
    silence jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT notification_rules_aggregation_seconds_check CHECK ((aggregation_seconds >= 0)),
    CONSTRAINT notification_rules_cooldown_seconds_check CHECK ((cooldown_seconds >= 0)),
    CONSTRAINT notification_rules_event_types_check CHECK (((cardinality(event_types) > 0) AND (event_types <@ ARRAY['finding.opened'::text, 'finding.reopened'::text, 'finding.severity_changed'::text, 'finding.resolved'::text, 'inspection.failed'::text, 'inspection.degraded'::text, 'notification.test'::text]))),
    CONSTRAINT notification_rules_filters_check CHECK ((jsonb_typeof(filters) = 'object'::text)),
    CONSTRAINT notification_rules_max_batch_size_check CHECK (((max_batch_size >= 1) AND (max_batch_size <= 1000))),
    CONSTRAINT notification_rules_minimum_severity_check CHECK ((minimum_severity = ANY (ARRAY['info'::text, 'warning'::text, 'critical'::text]))),
    CONSTRAINT notification_rules_name_check CHECK (((length(btrim(name)) >= 1) AND (length(btrim(name)) <= 120))),
    CONSTRAINT notification_rules_silence_check CHECK ((jsonb_typeof(silence) = 'object'::text)),
    CONSTRAINT notification_rules_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text])))
);


--
-- Name: notification_template_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE notification_template_versions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    scope_id uuid NOT NULL,
    template_id uuid NOT NULL,
    version integer NOT NULL,
    format text NOT NULL,
    title_template text DEFAULT ''::text NOT NULL,
    body_template text DEFAULT ''::text NOT NULL,
    payload_template jsonb,
    variables jsonb DEFAULT '[]'::jsonb NOT NULL,
    content_hash text NOT NULL,
    status text DEFAULT 'draft'::text NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    published_at timestamp with time zone,
    CONSTRAINT notification_template_versions_check CHECK ((((format = 'json'::text) AND (payload_template IS NOT NULL)) OR ((format <> 'json'::text) AND (payload_template IS NULL)))),
    CONSTRAINT notification_template_versions_content_hash_check CHECK ((content_hash ~ '^[0-9a-f]{64}$'::text)),
    CONSTRAINT notification_template_versions_format_check CHECK ((format = ANY (ARRAY['text'::text, 'markdown'::text, 'json'::text]))),
    CONSTRAINT notification_template_versions_status_check CHECK ((status = ANY (ARRAY['draft'::text, 'published'::text, 'disabled'::text]))),
    CONSTRAINT notification_template_versions_variables_check CHECK ((jsonb_typeof(variables) = 'array'::text)),
    CONSTRAINT notification_template_versions_version_check CHECK ((version > 0))
);


--
-- Name: notification_templates; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE notification_templates (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    scope_id uuid NOT NULL,
    name text NOT NULL,
    share_with_children boolean DEFAULT false NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT notification_templates_name_check CHECK (((length(btrim(name)) >= 1) AND (length(btrim(name)) <= 120)))
);


--
-- Name: persona_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE persona_versions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    persona_id uuid NOT NULL,
    version integer NOT NULL,
    config jsonb NOT NULL,
    status text DEFAULT 'draft'::text NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    published_at timestamp with time zone,
    CONSTRAINT agent_profile_versions_status_check CHECK ((status = ANY (ARRAY['draft'::text, 'published'::text, 'disabled'::text]))),
    CONSTRAINT agent_profile_versions_version_check CHECK ((version > 0))
);


--
-- Name: personas; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE personas (
    id uuid NOT NULL,
    tenant_id text DEFAULT 'default'::text NOT NULL,
    scope_id uuid NOT NULL,
    name text NOT NULL,
    identifier text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    config jsonb DEFAULT '{}'::jsonb NOT NULL,
    tags jsonb DEFAULT '[]'::jsonb NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT personas_config_check CHECK ((jsonb_typeof(config) = 'object'::text)),
    CONSTRAINT personas_identifier_check CHECK (((length(btrim(identifier)) >= 1) AND (length(btrim(identifier)) <= 200))),
    CONSTRAINT personas_identifier_no_whitespace CHECK ((identifier !~ '[[:space:]]'::text)),
    CONSTRAINT personas_name_check CHECK (((length(btrim(name)) >= 1) AND (length(btrim(name)) <= 200))),
    CONSTRAINT personas_name_no_whitespace CHECK ((name !~ '[[:space:]]'::text)),
    CONSTRAINT personas_tags_check CHECK ((jsonb_typeof(tags) = 'array'::text)),
    CONSTRAINT personas_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text, 'unknown'::text])))
);


--
-- Name: platforms; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE platforms (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    scope_id uuid NOT NULL,
    name text NOT NULL,
    code text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    icon text DEFAULT 'platform'::text NOT NULL,
    CONSTRAINT platforms_code_check CHECK ((code ~ '^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$'::text)),
    CONSTRAINT platforms_icon_length CHECK (((length(icon) >= 1) AND (length(icon) <= 262144))),
    CONSTRAINT platforms_name_check CHECK (((length(btrim(name)) >= 1) AND (length(btrim(name)) <= 120)))
);


--
-- Name: projects; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE projects (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    scope_id uuid NOT NULL,
    platform_id uuid NOT NULL,
    team_id uuid NOT NULL,
    name text NOT NULL,
    code text NOT NULL,
    labels jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    icon text DEFAULT 'project'::text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    importance_level text NOT NULL,
    technical_owner_ids uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    operations_owner_ids uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    CONSTRAINT projects_code_check CHECK ((code ~ '^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$'::text)),
    CONSTRAINT projects_icon_length CHECK (((length(icon) >= 1) AND (length(icon) <= 262144))),
    CONSTRAINT projects_importance_level_check CHECK ((importance_level = ANY (ARRAY['ITP1'::text, 'ITP2'::text, 'ITP3'::text, 'ITP4'::text]))),
    CONSTRAINT projects_labels_check CHECK ((jsonb_typeof(labels) = 'object'::text)),
    CONSTRAINT projects_name_check CHECK (((length(btrim(name)) >= 1) AND (length(btrim(name)) <= 120)))
);


--
-- Name: provider_scope_bindings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE provider_scope_bindings (
    scope_id uuid NOT NULL,
    provider_id uuid NOT NULL,
    tag text NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT provider_scope_bindings_tag_check CHECK ((tag = ANY (ARRAY['general'::text, 'diagnosis'::text, 'inspection'::text, 'workflow'::text])))
);


--
-- Name: providers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE providers (
    id uuid NOT NULL,
    tenant_id text DEFAULT 'default'::text NOT NULL,
    scope_id uuid NOT NULL,
    name text NOT NULL,
    config jsonb DEFAULT '{}'::jsonb NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    credential_ciphertext bytea,
    credential_key_version text DEFAULT ''::text NOT NULL,
    credential_purpose text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    last_connection_test_status text DEFAULT ''::text NOT NULL,
    last_connection_test_message text DEFAULT ''::text NOT NULL,
    last_connection_test_latency_ms bigint DEFAULT 0 NOT NULL,
    last_connection_test_at timestamp with time zone,
    CONSTRAINT providers_config_check CHECK ((jsonb_typeof(config) = 'object'::text)),
    CONSTRAINT providers_last_connection_test_latency_ms_check CHECK ((last_connection_test_latency_ms >= 0)),
    CONSTRAINT providers_name_check CHECK (((length(btrim(name)) >= 1) AND (length(btrim(name)) <= 200))),
    CONSTRAINT providers_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text, 'unknown'::text])))
);


--
-- Name: repository_bundles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE repository_bundles (
    resource_id uuid NOT NULL,
    bundle bytea NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: resource_connection_checks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE resource_connection_checks (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    resource_id uuid NOT NULL,
    status text NOT NULL,
    error_category text DEFAULT ''::text NOT NULL,
    message text DEFAULT ''::text NOT NULL,
    latency_ms bigint DEFAULT 0 NOT NULL,
    capabilities text[] DEFAULT '{}'::text[] NOT NULL,
    checked_by uuid,
    checked_at timestamp with time zone DEFAULT now() NOT NULL,
    endpoint text DEFAULT ''::text NOT NULL,
    CONSTRAINT resource_connection_checks_error_category_check CHECK ((error_category = ANY (ARRAY[''::text, 'configuration'::text, 'authentication'::text, 'timeout'::text, 'rate_limited'::text, 'response_too_large'::text, 'upstream'::text, 'unsupported'::text, 'internal'::text]))),
    CONSTRAINT resource_connection_checks_latency_ms_check CHECK ((latency_ms >= 0)),
    CONSTRAINT resource_connection_checks_message_check CHECK ((length(message) <= 500)),
    CONSTRAINT resource_connection_checks_status_check CHECK ((status = ANY (ARRAY['succeeded'::text, 'failed'::text])))
);


--
-- Name: resource_relations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE resource_relations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    source_resource_id uuid NOT NULL,
    target_resource_id uuid NOT NULL,
    relation_type text NOT NULL,
    attributes jsonb DEFAULT '{}'::jsonb NOT NULL,
    discovery_source text DEFAULT 'manual'::text NOT NULL,
    confidence numeric(5,4) DEFAULT 1 NOT NULL,
    confirmed boolean DEFAULT true NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT resource_relations_attributes_check CHECK ((jsonb_typeof(attributes) = 'object'::text)),
    CONSTRAINT resource_relations_confidence_check CHECK (((confidence >= (0)::numeric) AND (confidence <= (1)::numeric))),
    CONSTRAINT resource_relations_discovery_source_check CHECK ((discovery_source = ANY (ARRAY['manual'::text, 'discovery'::text, 'import'::text]))),
    CONSTRAINT resource_relations_relation_type_check CHECK ((relation_type = ANY (ARRAY['contains'::text, 'deployed_on'::text, 'depends_on'::text, 'observed_by'::text, 'exposes'::text, 'served_by_mcp'::text])))
);


--
-- Name: resource_role_bindings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE resource_role_bindings (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    subject_type text NOT NULL,
    subject_id uuid NOT NULL,
    role_id uuid NOT NULL,
    resource_id uuid NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT resource_role_bindings_subject_type_check CHECK ((subject_type = ANY (ARRAY['user'::text, 'group'::text])))
);


--
-- Name: resource_role_permissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE resource_role_permissions (
    role_id uuid NOT NULL,
    permission text NOT NULL
);


--
-- Name: resource_roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE resource_roles (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    builtin boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: resource_schemas; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE resource_schemas (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    kind text NOT NULL,
    version integer NOT NULL,
    schema jsonb NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    display_name text DEFAULT ''::text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    icon text DEFAULT 'resource'::text NOT NULL,
    CONSTRAINT resource_schemas_kind_check CHECK (((length(btrim(kind)) >= 1) AND (length(btrim(kind)) <= 120))),
    CONSTRAINT resource_schemas_schema_check CHECK ((jsonb_typeof(schema) = 'object'::text)),
    CONSTRAINT resource_schemas_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text]))),
    CONSTRAINT resource_schemas_version_check CHECK ((version > 0))
);


--
-- Name: resource_sync_states; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE resource_sync_states (
    resource_id uuid NOT NULL,
    state text DEFAULT 'never'::text NOT NULL,
    last_started_at timestamp with time zone,
    last_completed_at timestamp with time zone,
    last_error text DEFAULT ''::text NOT NULL,
    external_version text DEFAULT ''::text NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT resource_sync_states_last_error_check CHECK ((length(last_error) <= 2000)),
    CONSTRAINT resource_sync_states_state_check CHECK ((state = ANY (ARRAY['never'::text, 'running'::text, 'succeeded'::text, 'failed'::text])))
);


--
-- Name: resources; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE resources (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id text DEFAULT 'default'::text NOT NULL,
    scope_id uuid NOT NULL,
    kind text NOT NULL,
    schema_version integer DEFAULT 1 NOT NULL,
    name text NOT NULL,
    external_uid text DEFAULT ''::text NOT NULL,
    source_resource_id text DEFAULT ''::text NOT NULL,
    labels jsonb DEFAULT '{}'::jsonb NOT NULL,
    config jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    subtype text DEFAULT ''::text NOT NULL,
    agent_ref uuid,
    credential_ciphertext bytea,
    credential_key_version text DEFAULT ''::text NOT NULL,
    credential_purpose text DEFAULT ''::text NOT NULL,
    CONSTRAINT resources_config_check CHECK ((jsonb_typeof(config) = 'object'::text)),
    CONSTRAINT resources_credential_purpose_check CHECK ((length(credential_purpose) <= 500)),
    CONSTRAINT resources_external_uid_check CHECK ((length(external_uid) <= 512)),
    CONSTRAINT resources_kind_check CHECK (((length(btrim(kind)) >= 1) AND (length(btrim(kind)) <= 120))),
    CONSTRAINT resources_labels_check CHECK ((jsonb_typeof(labels) = 'object'::text)),
    CONSTRAINT resources_name_check CHECK (((length(btrim(name)) >= 1) AND (length(btrim(name)) <= 200))),
    CONSTRAINT resources_schema_version_check CHECK ((schema_version > 0)),
    CONSTRAINT resources_source_resource_id_check CHECK ((length(source_resource_id) <= 512)),
    CONSTRAINT resources_subtype_check CHECK ((length(btrim(subtype)) <= 120))
);


--
-- Name: role_bindings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE role_bindings (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    subject_type text NOT NULL,
    subject_id uuid NOT NULL,
    role_id uuid NOT NULL,
    scope_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT role_bindings_subject_type_check CHECK ((subject_type = ANY (ARRAY['user'::text, 'group'::text])))
);


--
-- Name: role_permissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE role_permissions (
    role_id uuid NOT NULL,
    permission text NOT NULL
);


--
-- Name: roles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE roles (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    scope_type text NOT NULL,
    builtin boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT roles_scope_type_check CHECK ((scope_type = ANY (ARRAY['platform'::text, 'team'::text, 'project'::text])))
);


--
-- Name: scope_ai_provider_bindings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE scope_ai_provider_bindings (
    scope_id uuid NOT NULL,
    provider_resource_id uuid NOT NULL,
    tag text NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT scope_ai_provider_bindings_tag_check CHECK ((tag = ANY (ARRAY['general'::text, 'diagnosis'::text, 'inspection'::text, 'workflow'::text])))
);


--
-- Name: scope_defaults; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE scope_defaults (
    scope_id uuid NOT NULL,
    default_key text NOT NULL,
    resource_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT scope_defaults_default_key_check CHECK (((length(btrim(default_key)) >= 1) AND (length(btrim(default_key)) <= 120)))
);


--
-- Name: scopes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE scopes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id text DEFAULT 'default'::text NOT NULL,
    scope_type text NOT NULL,
    parent_scope_id uuid,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT scopes_check CHECK ((((scope_type = 'platform'::text) AND (parent_scope_id IS NULL)) OR ((scope_type = ANY (ARRAY['team'::text, 'project'::text])) AND (parent_scope_id IS NOT NULL)))),
    CONSTRAINT scopes_scope_type_check CHECK ((scope_type = ANY (ARRAY['platform'::text, 'team'::text, 'project'::text]))),
    CONSTRAINT scopes_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text])))
);


--
-- Name: sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE sessions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    access_token_hash bytea NOT NULL,
    refresh_token_hash bytea NOT NULL,
    access_expires_at timestamp with time zone NOT NULL,
    refresh_expires_at timestamp with time zone NOT NULL,
    user_agent text DEFAULT ''::text NOT NULL,
    client_ip text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL,
    revoked_at timestamp with time zone,
    CONSTRAINT sessions_client_ip_check CHECK ((length(client_ip) <= 255)),
    CONSTRAINT sessions_user_agent_check CHECK ((length(user_agent) <= 512))
);


--
-- Name: skill_scope_defaults; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE skill_scope_defaults (
    scope_id uuid NOT NULL,
    skill_id uuid NOT NULL,
    skill_version_id uuid NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: skill_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE skill_versions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    skill_id uuid NOT NULL,
    version integer NOT NULL,
    manifest jsonb NOT NULL,
    input_schema jsonb NOT NULL,
    output_schema jsonb NOT NULL,
    tools jsonb DEFAULT '[]'::jsonb NOT NULL,
    risk_level text NOT NULL,
    status text DEFAULT 'draft'::text NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    published_at timestamp with time zone,
    CONSTRAINT skill_versions_check CHECK ((((status = 'published'::text) AND (published_at IS NOT NULL)) OR (status <> 'published'::text))),
    CONSTRAINT skill_versions_input_schema_check CHECK ((jsonb_typeof(input_schema) = 'object'::text)),
    CONSTRAINT skill_versions_manifest_check CHECK ((jsonb_typeof(manifest) = 'object'::text)),
    CONSTRAINT skill_versions_manifest_name_no_whitespace CHECK (((manifest ? 'name'::text) AND ((manifest ->> 'name'::text) !~ '[[:space:]]'::text))),
    CONSTRAINT skill_versions_output_schema_check CHECK ((jsonb_typeof(output_schema) = 'object'::text)),
    CONSTRAINT skill_versions_risk_level_check CHECK ((risk_level = ANY (ARRAY['read_only'::text, 'controlled'::text, 'high'::text]))),
    CONSTRAINT skill_versions_status_check CHECK ((status = ANY (ARRAY['draft'::text, 'published'::text, 'disabled'::text]))),
    CONSTRAINT skill_versions_tools_check CHECK ((jsonb_typeof(tools) = 'array'::text)),
    CONSTRAINT skill_versions_version_check CHECK ((version > 0))
);


--
-- Name: skills; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE skills (
    id uuid NOT NULL,
    tenant_id text DEFAULT 'default'::text NOT NULL,
    scope_id uuid NOT NULL,
    name text NOT NULL,
    identifier text NOT NULL,
    category text DEFAULT 'diagnosis'::text NOT NULL,
    tags jsonb DEFAULT '[]'::jsonb NOT NULL,
    maintainer text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT skills_category_check CHECK ((category = ANY (ARRAY['diagnosis'::text, 'monitoring'::text, 'optimization'::text, 'maintenance'::text]))),
    CONSTRAINT skills_identifier_check CHECK (((length(btrim(identifier)) >= 1) AND (length(btrim(identifier)) <= 200))),
    CONSTRAINT skills_name_check CHECK (((length(btrim(name)) >= 1) AND (length(btrim(name)) <= 200))),
    CONSTRAINT skills_name_no_whitespace CHECK ((name !~ '[[:space:]]'::text)),
    CONSTRAINT skills_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text, 'unknown'::text]))),
    CONSTRAINT skills_tags_check CHECK ((jsonb_typeof(tags) = 'array'::text))
);


--
-- Name: teams; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE teams (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    scope_id uuid NOT NULL,
    platform_id uuid NOT NULL,
    name text NOT NULL,
    labels jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    icon text DEFAULT 'team'::text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    owner_ids uuid[] DEFAULT '{}'::uuid[] NOT NULL,
    CONSTRAINT teams_description_check CHECK ((length(description) <= 1000)),
    CONSTRAINT teams_icon_length CHECK (((length(icon) >= 1) AND (length(icon) <= 262144))),
    CONSTRAINT teams_labels_check CHECK ((jsonb_typeof(labels) = 'object'::text)),
    CONSTRAINT teams_name_check CHECK (((length(btrim(name)) >= 1) AND (length(btrim(name)) <= 120)))
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE users (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    email text,
    display_name text NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    username text DEFAULT ('user-'::text || "left"(replace((gen_random_uuid())::text, '-'::text, ''::text), 16)) NOT NULL,
    phone text,
    theme text DEFAULT 'auto'::text NOT NULL,
    sidebar_mode text DEFAULT 'fixed'::text NOT NULL,
    sidebar_collapsed boolean DEFAULT false NOT NULL,
    avatar_content_type text,
    avatar_data bytea,
    avatar_updated_at timestamp with time zone,
    password_hash text NOT NULL,
    password_changed_at timestamp with time zone DEFAULT now() NOT NULL,
    must_change_password boolean DEFAULT false NOT NULL,
    CONSTRAINT users_avatar_content_type_check CHECK ((avatar_content_type = ANY (ARRAY['image/jpeg'::text, 'image/png'::text]))),
    CONSTRAINT users_avatar_data_check CHECK ((((avatar_data IS NULL) AND (avatar_content_type IS NULL) AND (avatar_updated_at IS NULL)) OR ((avatar_data IS NOT NULL) AND (avatar_content_type IS NOT NULL) AND (avatar_updated_at IS NOT NULL)))),
    CONSTRAINT users_display_name_check CHECK (((length(btrim(display_name)) >= 1) AND (length(btrim(display_name)) <= 120))),
    CONSTRAINT users_email_nonempty CHECK (((email IS NULL) OR (length(btrim(email)) > 0))),
    CONSTRAINT users_phone_format CHECK (((phone IS NULL) OR (phone ~ '^\+?[0-9]{3,32}$'::text))),
    CONSTRAINT users_sidebar_mode_check CHECK ((sidebar_mode = ANY (ARRAY['fixed'::text, 'hover'::text]))),
    CONSTRAINT users_status_check CHECK ((status = ANY (ARRAY['active'::text, 'disabled'::text, 'locked'::text]))),
    CONSTRAINT users_theme_check CHECK ((theme = ANY (ARRAY['auto'::text, 'light'::text, 'dark'::text]))),
    CONSTRAINT users_username_format CHECK ((username ~ '^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$'::text))
);


--
-- Name: inspection_health_snapshots id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_health_snapshots ALTER COLUMN id SET DEFAULT nextval('inspection_health_snapshots_id_seq'::regclass);


--
-- Name: inspection_run_steps id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_run_steps ALTER COLUMN id SET DEFAULT nextval('inspection_run_steps_id_seq'::regclass);


--
-- Name: notification_delivery_attempts id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_delivery_attempts ALTER COLUMN id SET DEFAULT nextval('notification_delivery_attempts_id_seq'::regclass);


--
-- Data for Name: ai_workflow_runs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: application_dependencies; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: application_instances; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: applications; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: audit_events; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: audit_retention_runs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: authorization_revision; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO authorization_revision VALUES (true, 234);


--
-- Data for Name: cache_entries; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: diagnosis_causal_chains; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: diagnosis_events; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: diagnosis_evidence; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: diagnosis_hypotheses; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: diagnosis_messages; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: diagnosis_plan_steps; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: diagnosis_plans; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: diagnosis_reports; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: diagnosis_runs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: diagnosis_sessions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: diagnosis_targets; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: engine_capability_terms; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO engine_capability_terms VALUES ('智能体循环', NULL, '2026-10-22 01:22:40.44285+00');
INSERT INTO engine_capability_terms VALUES ('上下文编排', NULL, '2026-10-22 01:22:40.44285+00');
INSERT INTO engine_capability_terms VALUES ('工具调用', NULL, '2026-10-22 01:22:40.44285+00');
INSERT INTO engine_capability_terms VALUES ('工具网关', NULL, '2026-10-22 01:22:40.44285+00');
INSERT INTO engine_capability_terms VALUES ('技能编排', NULL, '2026-10-22 01:22:40.44285+00');
INSERT INTO engine_capability_terms VALUES ('专家路由', NULL, '2026-10-22 01:22:40.44285+00');
INSERT INTO engine_capability_terms VALUES ('结构化输出', NULL, '2026-10-22 01:22:40.44285+00');
INSERT INTO engine_capability_terms VALUES ('检索增强', NULL, '2026-10-22 01:22:40.44285+00');
INSERT INTO engine_capability_terms VALUES ('工作流编排', NULL, '2026-10-22 01:22:40.44285+00');
INSERT INTO engine_capability_terms VALUES ('流式事件', NULL, '2026-10-22 01:22:40.44285+00');


--
-- Data for Name: engine_execution_events; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: engine_execution_tool_calls; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: engine_scope_bindings; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO engine_scope_bindings VALUES ('2817fc61-c924-4760-84a7-06a0d3f8995f', 'a6689eb4-6fad-415f-87b4-d436d54d4cdc', 'general', NULL, '2026-10-22 01:22:40.306365+00', '2026-10-22 01:22:40.306365+00');


--
-- Data for Name: engines; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO engines VALUES ('a6689eb4-6fad-415f-87b4-d436d54d4cdc', 'default', '2817fc61-c924-4760-84a7-06a0d3f8995f', 'OpsKeeper Engine', '统一承载诊断、巡检、技能和工作流的智能执行引擎。', 'lucide:Cpu', '{"capabilities": ["智能体循环", "上下文编排", "工具调用", "工具网关", "技能编排", "专家路由", "结构化输出", "检索增强", "工作流编排", "流式事件"]}', 'active', '2026-10-22 01:22:40.306365+00', '2026-10-22 01:22:40.306365+00', NULL);


--
-- Data for Name: group_members; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: groups; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: inspection_findings; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: inspection_health_snapshots; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: inspection_jobs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: inspection_policies; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: inspection_policy_notification_rules; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: inspection_policy_targets; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: inspection_run_steps; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: inspection_runs; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: mcp_server_snapshots; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: notification_channel_test_limits; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: notification_channel_versions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: notification_channels; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: notification_deliveries; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: notification_delivery_attempts; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: notification_events; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: notification_rule_cooldowns; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: notification_rule_routes; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: notification_rules; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: notification_template_versions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: notification_templates; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: persona_versions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: personas; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: platforms; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO platforms VALUES ('0e2355e0-eda3-4810-a9f3-4d78f9ecccd5', '2817fc61-c924-4760-84a7-06a0d3f8995f', 'OpsKeeper', 'default', '2026-10-22 01:22:31.617466+00', '2026-10-22 01:22:31.617466+00', NULL, 'lucide:Building2');


--
-- Data for Name: projects; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: provider_scope_bindings; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: providers; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: repository_bundles; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: resource_connection_checks; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: resource_relations; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: resource_role_bindings; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: resource_role_permissions; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO resource_role_permissions VALUES ('9cc6ce00-5f1e-4cca-be06-bceb4ece9806', 'inspection:execute');
INSERT INTO resource_role_permissions VALUES ('9cc6ce00-5f1e-4cca-be06-bceb4ece9806', 'inspection:manage');
INSERT INTO resource_role_permissions VALUES ('9cc6ce00-5f1e-4cca-be06-bceb4ece9806', 'diagnosis:read');
INSERT INTO resource_role_permissions VALUES ('9cc6ce00-5f1e-4cca-be06-bceb4ece9806', 'diagnosis:start');
INSERT INTO resource_role_permissions VALUES ('9cc6ce00-5f1e-4cca-be06-bceb4ece9806', 'discovery:run');
INSERT INTO resource_role_permissions VALUES ('9cc6ce00-5f1e-4cca-be06-bceb4ece9806', 'relation:manage');
INSERT INTO resource_role_permissions VALUES ('9cc6ce00-5f1e-4cca-be06-bceb4ece9806', 'resource:use');
INSERT INTO resource_role_permissions VALUES ('9cc6ce00-5f1e-4cca-be06-bceb4ece9806', 'resource:delete');
INSERT INTO resource_role_permissions VALUES ('9cc6ce00-5f1e-4cca-be06-bceb4ece9806', 'resource:update');
INSERT INTO resource_role_permissions VALUES ('9cc6ce00-5f1e-4cca-be06-bceb4ece9806', 'resource:read');
INSERT INTO resource_role_permissions VALUES ('3b78297e-d41a-4911-84ef-e68a66f73440', 'inspection:execute');
INSERT INTO resource_role_permissions VALUES ('3b78297e-d41a-4911-84ef-e68a66f73440', 'diagnosis:read');
INSERT INTO resource_role_permissions VALUES ('3b78297e-d41a-4911-84ef-e68a66f73440', 'diagnosis:start');
INSERT INTO resource_role_permissions VALUES ('3b78297e-d41a-4911-84ef-e68a66f73440', 'discovery:run');
INSERT INTO resource_role_permissions VALUES ('3b78297e-d41a-4911-84ef-e68a66f73440', 'resource:use');
INSERT INTO resource_role_permissions VALUES ('3b78297e-d41a-4911-84ef-e68a66f73440', 'resource:read');
INSERT INTO resource_role_permissions VALUES ('6007eec6-9c72-4670-9222-59093c573681', 'diagnosis:read');
INSERT INTO resource_role_permissions VALUES ('6007eec6-9c72-4670-9222-59093c573681', 'resource:read');
INSERT INTO resource_role_permissions VALUES ('9cc6ce00-5f1e-4cca-be06-bceb4ece9806', 'notification:read');
INSERT INTO resource_role_permissions VALUES ('9cc6ce00-5f1e-4cca-be06-bceb4ece9806', 'notification:manage');
INSERT INTO resource_role_permissions VALUES ('9cc6ce00-5f1e-4cca-be06-bceb4ece9806', 'notification:test');
INSERT INTO resource_role_permissions VALUES ('9cc6ce00-5f1e-4cca-be06-bceb4ece9806', 'notification:retry');
INSERT INTO resource_role_permissions VALUES ('6007eec6-9c72-4670-9222-59093c573681', 'notification:read');


--
-- Data for Name: resource_roles; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO resource_roles VALUES ('9cc6ce00-5f1e-4cca-be06-bceb4ece9806', 'ResourceAdmin', true, '2026-10-22 01:22:31.617466+00');
INSERT INTO resource_roles VALUES ('3b78297e-d41a-4911-84ef-e68a66f73440', 'ResourceOperator', true, '2026-10-22 01:22:31.617466+00');
INSERT INTO resource_roles VALUES ('6007eec6-9c72-4670-9222-59093c573681', 'ResourceViewer', true, '2026-10-22 01:22:31.617466+00');


--
-- Data for Name: resource_schemas; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO resource_schemas VALUES ('ad0acccf-1d89-46ba-9569-3ee348c67f65', 'GenericMiddleware', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'disabled', '2026-10-22 01:22:31.617466+00', '通用中间件', '无法归入具体类型的中间件或平台服务。', 'lucide:Workflow');
INSERT INTO resource_schemas VALUES ('97df70dc-7873-43a8-a1b4-4bb71515c6b3', 'Elasticsearch', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"url": {"type": "string", "title": "服务 URL", "minLength": 1}, "tls_insecure": {"type": "boolean", "title": "跳过 TLS 校验", "default": false}, "timeout_seconds": {"type": "integer", "title": "超时时间（秒）", "default": 10, "maximum": 300, "minimum": 1}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:31.617466+00', 'ElasticSearch', 'Elasticsearch resource with unified Direct/Agent access.', 'iconify:simple-icons:elasticsearch');
INSERT INTO resource_schemas VALUES ('15eeb9c9-9176-4cab-8efd-b7ad841ce10f', 'MCPServer', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'active', '2026-10-22 01:22:31.617466+00', 'MCP 服务', '提供受控工具能力的 MCP 服务。', 'lucide:Waypoints');
INSERT INTO resource_schemas VALUES ('700df82f-4aca-4624-a6ea-138881c1863b', 'Tempo', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'active', '2026-10-22 01:22:31.617466+00', 'Tempo', 'Tempo 链路追踪服务连接配置。', 'lucide:Activity');
INSERT INTO resource_schemas VALUES ('21af89e3-2e89-4b18-9574-7df4d6c7c459', 'Jaeger', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'active', '2026-10-22 01:22:31.617466+00', 'Jaeger', 'Jaeger 链路追踪服务连接配置。', 'lucide:Activity');
INSERT INTO resource_schemas VALUES ('cbaac967-24cb-4a03-8f71-250e2c8365ba', 'Elastic', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'active', '2026-10-22 01:22:31.617466+00', 'Elastic Observability', 'Elastic 可观测性服务连接配置。', 'iconify:simple-icons:elasticsearch');
INSERT INTO resource_schemas VALUES ('70bfed51-bd3a-4bf2-8b43-30fb68054868', 'Pod', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'disabled', '2026-10-22 01:22:31.617466+00', '', '', 'lucide:Circle');
INSERT INTO resource_schemas VALUES ('48c80d07-04e0-4ba8-8743-03986df4c4f4', 'Endpoint', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'disabled', '2026-10-22 01:22:31.617466+00', '服务端点', '业务应用对外提供的访问端点。', 'lucide:ArrowLeftRight');
INSERT INTO resource_schemas VALUES ('52de74a8-a528-41cb-ad07-117ab1a28559', 'Prometheus', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "required": ["url"], "properties": {"url": {"type": "string", "title": "服务 URL", "format": "uri"}, "token": {"type": "string", "title": "访问 Token", "sensitive": true}, "password": {"type": "string", "title": "密码", "sensitive": true}, "username": {"type": "string", "title": "用户名", "sensitive": true}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:31.617466+00', 'Prometheus', 'Prometheus 指标查询和告警来源。', 'lucide:Activity');
INSERT INTO resource_schemas VALUES ('1eeeff00-3fb1-4caa-bb9f-799def2404df', 'Loki', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "required": ["url"], "properties": {"url": {"type": "string", "title": "服务 URL", "format": "uri"}, "token": {"type": "string", "title": "访问 Token", "sensitive": true}, "password": {"type": "string", "title": "密码", "sensitive": true}, "username": {"type": "string", "title": "用户名", "sensitive": true}, "tenant_id": {"type": "string", "title": "租户 ID"}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:31.617466+00', 'Loki', 'Loki 日志查询来源。', 'lucide:Activity');
INSERT INTO resource_schemas VALUES ('840402b4-fd25-4e7a-8c1e-5e4b659d4ffb', 'MCPServer', 2, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "required": ["transport"], "properties": {"url": {"type": "string", "title": "服务 URL"}, "command": {"type": "string", "title": "启动命令"}, "transport": {"enum": ["stdio", "streamable_http"], "type": "string", "title": "传输方式"}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:31.617466+00', 'MCP 服务', '提供受控工具能力的 MCP 服务；运行时接入在 T14 实施。', 'lucide:Waypoints');
INSERT INTO resource_schemas VALUES ('7bcb5e90-0238-4554-b6ea-9b789c648291', 'Artifact', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"url": {"type": "string", "title": "仓库 URL"}, "token": {"type": "string", "title": "访问 Token", "sensitive": true}, "password": {"type": "string", "title": "密码", "sensitive": true}, "provider": {"enum": ["oci", "harbor", "docker", "maven", "npm", "generic"], "type": "string", "title": "仓库类型"}, "username": {"type": "string", "title": "用户名", "sensitive": true}, "namespace": {"type": "string", "title": "命名空间"}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:31.617466+00', 'Artifact', 'Artifact repository resource.', 'lucide:Database');
INSERT INTO resource_schemas VALUES ('6189420f-4b89-4541-a544-59cd10010fed', 'MCPServer', 3, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "required": ["transport", "url"], "properties": {"url": {"type": "string", "title": "Server 地址", "format": "uri"}, "transport": {"enum": ["streamable_http", "sse"], "type": "string", "title": "传输方式"}, "tool_allowlist": {"type": "array", "items": {"type": "string", "maxLength": 128}, "title": "工具白名单"}, "request_headers": {"type": "object", "title": "请求 Header", "additionalProperties": {"type": "string"}}, "timeout_seconds": {"type": "integer", "title": "超时时间（秒）", "default": 120, "maximum": 600, "minimum": 1}, "max_response_bytes": {"type": "integer", "title": "响应体大小限制（字节）", "default": 4194304, "maximum": 16777216, "minimum": 1}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:31.617466+00', 'MCP 服务', '通过 Streamable HTTP 或 SSE 接入 MCP 服务。Token 加密后保存在资源本身；工具白名单支持通配符，空白表示不限制。', 'lucide:Waypoints');
INSERT INTO resource_schemas VALUES ('27625acc-687a-447c-9169-68982682b1cb', 'KnowledgeBase', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "required": ["documents"], "properties": {"documents": {"type": "array", "items": {"type": "object", "required": ["id", "content"], "properties": {"id": {"type": "string", "maxLength": 200, "minLength": 1}, "title": {"type": "string", "maxLength": 500}, "content": {"type": "string", "maxLength": 200000, "minLength": 1}, "metadata": {"type": "object"}, "source_uri": {"type": "string", "maxLength": 2000}}, "additionalProperties": false}, "maxItems": 10000}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:35.732849+00', '知识库', '按 Scope 管理、可检索并提供引用的知识文档集合。', 'lucide:BookOpen');
INSERT INTO resource_schemas VALUES ('733f64fd-8296-4002-98a3-f938c1c5d081', 'OceanBase', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"host": {"type": "string", "title": "主机", "minLength": 1}, "mode": {"enum": ["mysql", "oracle"], "type": "string", "title": "兼容模式", "default": "mysql"}, "port": {"type": "integer", "title": "端口", "default": 2881, "maximum": 65535, "minimum": 1}, "database": {"type": "string", "title": "数据库名"}, "service_name": {"type": "string", "title": "Service Name"}, "timeout_seconds": {"type": "integer", "title": "超时时间（秒）", "default": 10, "maximum": 300, "minimum": 1}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:36.368656+00', 'OceanBase', 'OceanBase database resource with unified MySQL and Oracle mode access.', 'lucide:Database');
INSERT INTO resource_schemas VALUES ('c4531680-54f3-4d8a-96ba-b8bc76bca495', 'Host', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'active', '2026-10-22 01:22:36.368656+00', 'Host', 'Host connection resource.', 'lucide:Server');
INSERT INTO resource_schemas VALUES ('dec79f55-80ae-4b19-a9a9-154a8de4856a', 'Docker', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'active', '2026-10-22 01:22:36.368656+00', 'Docker', 'Docker Engine resource.', 'iconify:simple-icons:docker');
INSERT INTO resource_schemas VALUES ('0409542e-5b66-446a-b282-60f5a9f2046c', 'TongRDS', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'active', '2026-10-22 01:22:36.368656+00', 'TongRDS', 'TongRDS database resource.', 'lucide:Database');
INSERT INTO resource_schemas VALUES ('b8614ea0-ddd8-4366-b0b3-ac4daf6c5827', 'Datadog', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'active', '2026-10-22 01:22:31.617466+00', 'Datadog', 'Datadog 可观测性服务连接配置。', 'lucide:Activity');
INSERT INTO resource_schemas VALUES ('19f2e289-a042-4071-81bb-581e7a30ad50', 'NotificationChannel', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'active', '2026-10-22 01:22:31.617466+00', '通知渠道', 'Webhook、邮件或其他通知目标。', 'lucide:Bell');
INSERT INTO resource_schemas VALUES ('de85d322-5744-4ae6-a843-30dd3106fab5', 'Runbook', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'active', '2026-10-22 01:22:31.617466+00', '运行手册', '可供运维流程引用的运行手册。', 'lucide:BookOpen');
INSERT INTO resource_schemas VALUES ('cdbbbc42-cd09-47c9-af20-9815b6cc521c', 'Namespace', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'disabled', '2026-10-22 01:22:31.617466+00', '', '', 'lucide:Circle');
INSERT INTO resource_schemas VALUES ('9fe57350-a502-49c5-a610-4071329e3f3b', 'Node', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'disabled', '2026-10-22 01:22:31.617466+00', '', '', 'lucide:Circle');
INSERT INTO resource_schemas VALUES ('2c7f38be-a5b9-4d61-aa90-98859aebb4a6', 'Workload', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'disabled', '2026-10-22 01:22:31.617466+00', '', '', 'lucide:Circle');
INSERT INTO resource_schemas VALUES ('46ee0a19-3759-4650-a31c-245d9f0f2edc', 'Service', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'disabled', '2026-10-22 01:22:31.617466+00', '', '', 'lucide:Circle');
INSERT INTO resource_schemas VALUES ('0b0d572a-f707-4edc-bb7f-afdc8cdbb554', 'Ingress', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'disabled', '2026-10-22 01:22:31.617466+00', '', '', 'lucide:Circle');
INSERT INTO resource_schemas VALUES ('ce4780eb-602c-4263-9a15-4a845e87d45f', 'Model', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'disabled', '2026-10-22 01:22:31.617466+00', '', '', 'lucide:Circle');
INSERT INTO resource_schemas VALUES ('758d1deb-b625-4899-995e-d5bbb5bb1ff9', 'GenericAPI', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "additionalProperties": true}', 'disabled', '2026-10-22 01:22:31.617466+00', '通用 API', '可通过 HTTP 访问的外部 API。', 'lucide:ArrowLeftRight');
INSERT INTO resource_schemas VALUES ('9e58deb0-bb72-4a72-9873-eb6828f2b90c', 'Redis', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"host": {"type": "string", "title": "主机", "minLength": 1}, "port": {"type": "integer", "title": "端口", "default": 6379, "maximum": 65535, "minimum": 1}, "database": {"type": "integer", "title": "数据库编号", "default": 0, "minimum": 0}, "timeout_seconds": {"type": "integer", "title": "超时时间（秒）", "default": 10, "maximum": 300, "minimum": 1}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:31.617466+00', 'Redis', 'Redis resource with unified Direct/Agent read-only tools.', 'iconify:simple-icons:redis');
INSERT INTO resource_schemas VALUES ('60d47c8f-b618-4b8c-853c-34b28099d0cd', 'Kafka', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"tls": {"type": "boolean", "title": "启用 TLS", "default": false}, "brokers": {"type": "array", "items": {"type": "string", "minLength": 1}, "title": "Broker 地址", "minItems": 1}, "timeout_seconds": {"type": "integer", "title": "超时时间（秒）", "default": 10, "maximum": 300, "minimum": 1}, "tls_server_name": {"type": "string", "title": "TLS Server Name"}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:31.617466+00', 'Kafka', 'Kafka resource with unified Direct/Agent access.', 'iconify:simple-icons:apache-kafka');
INSERT INTO resource_schemas VALUES ('97eaf9fc-7736-45d8-94ff-2d6e5d1aa5a9', 'Repository', 2, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"url": {"type": "string", "title": "Git 仓库 URL"}, "path": {"type": "string", "title": "Bundle 工作副本路径"}, "provider": {"type": "string", "title": "代码托管平台"}, "s3_bucket": {"type": "string", "title": "S3 Bucket"}, "s3_prefix": {"type": "string", "title": "S3 Prefix"}, "s3_endpoint": {"type": "string", "title": "S3 Endpoint"}, "storage_key": {"type": "string", "title": "Bundle 存储键", "readOnly": true}, "default_branch": {"type": "string", "title": "默认分支"}, "storage_backend": {"enum": ["local", "postgres", "s3"], "type": "string", "title": "Bundle 存储后端"}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:31.617466+00', 'Repository', '支持实时 Git 仓库和上传 Bundle 仓库的代码上下文资源。', 'lucide:FolderGit2');
INSERT INTO resource_schemas VALUES ('4553e23c-dedf-42b7-81c2-beab20cb7ba6', 'Kubernetes', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"server": {"type": "string"}, "connection_mode": {"enum": ["kubeconfig", "endpoint"], "type": "string"}, "skip_tls_verify": {"type": "boolean"}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:31.617466+00', 'Kubernetes', 'Kubernetes cluster resource.', 'iconify:simple-icons:kubernetes');
INSERT INTO resource_schemas VALUES ('61fc92cd-9c8e-42f5-b09e-26cf2448cd39', 'Workflow', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "required": ["version", "nodes", "edges", "enabled"], "properties": {"edges": {"type": "array", "items": {"type": "object", "required": ["from", "to"], "properties": {"to": {"type": "string"}, "from": {"type": "string"}, "condition": {"type": "string", "maxLength": 2000}}, "additionalProperties": false}, "maxItems": 1000}, "nodes": {"type": "array", "items": {"type": "object", "required": ["id", "type", "name"], "properties": {"id": {"type": "string", "pattern": "^[A-Za-z][A-Za-z0-9_.-]{0,119}$"}, "name": {"type": "string", "maxLength": 200, "minLength": 1}, "type": {"enum": ["agent", "skill", "tool", "retrieval", "condition", "parallel", "approval"], "type": "string"}, "retry": {"type": "object", "properties": {"max_attempts": {"type": "integer", "maximum": 10, "minimum": 0}, "backoff_seconds": {"type": "integer", "maximum": 3600, "minimum": 0}}, "additionalProperties": false}, "config": {"type": "object"}, "timeout_seconds": {"type": "integer", "maximum": 3600, "minimum": 0}}, "additionalProperties": false}, "maxItems": 200, "minItems": 1}, "enabled": {"type": "boolean"}, "version": {"type": "integer", "minimum": 1}, "description": {"type": "string", "maxLength": 2000}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:35.732849+00', 'AI 工作流', '由 Agent、Skill、Tool、检索、条件、并行和审批节点组成的版本化 DAG。', 'lucide:Circle');
INSERT INTO resource_schemas VALUES ('d4af3cce-4312-449d-9a63-45a2986e45a9', 'PostgreSQL', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"host": {"type": "string", "title": "主机", "minLength": 1}, "port": {"type": "integer", "title": "端口", "default": 5432, "maximum": 65535, "minimum": 1}, "database": {"type": "string", "title": "数据库", "minLength": 1}, "timeout_seconds": {"type": "integer", "title": "超时时间（秒）", "default": 10, "maximum": 300, "minimum": 1}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:31.617466+00', 'PostgreSQL', 'PostgreSQL database resource with Direct/Agent access.', 'iconify:simple-icons:postgresql');
INSERT INTO resource_schemas VALUES ('ba44185e-363a-4ab7-b2c2-632b3dcb830b', 'Nacos', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"host": {"type": "string", "title": "主机", "minLength": 1}, "port": {"type": "integer", "title": "端口", "default": 8848, "maximum": 65535, "minimum": 1}, "scheme": {"enum": ["http", "https"], "type": "string", "title": "协议", "default": "http"}, "context_path": {"type": "string", "title": "上下文路径", "default": "/nacos"}, "timeout_seconds": {"type": "integer", "title": "超时时间（秒）", "default": 10, "maximum": 300, "minimum": 1}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:37.318307+00', 'Nacos', 'Nacos service registry and configuration center resource.', 'lucide:Network');
INSERT INTO resource_schemas VALUES ('faf6837b-bfbc-4236-b0ef-99952f68042c', 'MySQL', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"host": {"type": "string", "title": "主机", "minLength": 1}, "port": {"type": "integer", "title": "端口", "default": 3306, "maximum": 65535, "minimum": 1}, "database": {"type": "string", "title": "数据库", "minLength": 1}, "timeout_seconds": {"type": "integer", "title": "超时时间（秒）", "default": 10, "maximum": 300, "minimum": 1}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:36.368656+00', 'MySQL', 'MySQL database resource with Direct/Agent access.', 'lucide:Database');
INSERT INTO resource_schemas VALUES ('455d7d3f-53a2-4dfc-8214-ec60815edaf0', 'Oracle', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"sid": {"type": "string", "title": "SID"}, "tls": {"type": "boolean", "title": "启用 TCPS", "default": false}, "host": {"type": "string", "title": "主机", "minLength": 1}, "port": {"type": "integer", "title": "端口", "default": 1521, "maximum": 65535, "minimum": 1}, "service_name": {"type": "string", "title": "Service Name"}, "timeout_seconds": {"type": "integer", "title": "超时时间（秒）", "default": 10, "maximum": 300, "minimum": 1}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:36.368656+00', 'Oracle', 'Oracle database resource with unified Direct/Agent access.', 'lucide:Database');
INSERT INTO resource_schemas VALUES ('b03332f0-36e8-4e1d-93b7-8ddac0ceb302', 'RabbitMQ', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"url": {"type": "string", "title": "Management API URL", "minLength": 1}, "tls_insecure": {"type": "boolean", "title": "跳过 TLS 校验", "default": false}, "timeout_seconds": {"type": "integer", "title": "超时时间（秒）", "default": 10, "maximum": 300, "minimum": 1}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:36.368656+00', 'RabbitMQ', 'RabbitMQ Management API resource with unified Direct/Agent access.', 'iconify:simple-icons:rabbitmq');
INSERT INTO resource_schemas VALUES ('3d8ed7ab-62f3-4bc5-843e-e0907c6ae6c4', 'MinIO', 1, '{"type": "object", "$schema": "https://json-schema.org/draft/2020-12/schema", "properties": {"region": {"type": "string"}, "secure": {"type": "boolean", "default": false}, "endpoint": {"type": "string", "minLength": 1}, "timeout_seconds": {"type": "integer", "default": 10, "maximum": 300, "minimum": 1}}, "additionalProperties": false}', 'active', '2026-10-22 01:22:38.285744+00', 'MinIO', 'MinIO object storage resource with unified Direct/Agent access.', 'lucide:Database');


--
-- Data for Name: resource_sync_states; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: resources; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: role_bindings; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: role_permissions; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'audit:read');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'inspection:execute');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'inspection:manage');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'diagnosis:read');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'diagnosis:start');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'relation:manage');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'resource:use');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'resource:delete');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'resource:update');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'resource:create');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'resource:read');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'project:manage');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'team:manage');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'organization:read');
INSERT INTO role_permissions VALUES ('85c46e68-001e-4988-8cae-ab2d11ee3d3e', 'inspection:execute');
INSERT INTO role_permissions VALUES ('85c46e68-001e-4988-8cae-ab2d11ee3d3e', 'diagnosis:read');
INSERT INTO role_permissions VALUES ('85c46e68-001e-4988-8cae-ab2d11ee3d3e', 'diagnosis:start');
INSERT INTO role_permissions VALUES ('85c46e68-001e-4988-8cae-ab2d11ee3d3e', 'resource:use');
INSERT INTO role_permissions VALUES ('85c46e68-001e-4988-8cae-ab2d11ee3d3e', 'resource:read');
INSERT INTO role_permissions VALUES ('85c46e68-001e-4988-8cae-ab2d11ee3d3e', 'organization:read');
INSERT INTO role_permissions VALUES ('e9b6da21-9b77-4774-ba82-3259fc845000', 'diagnosis:read');
INSERT INTO role_permissions VALUES ('e9b6da21-9b77-4774-ba82-3259fc845000', 'resource:read');
INSERT INTO role_permissions VALUES ('e9b6da21-9b77-4774-ba82-3259fc845000', 'organization:read');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'relation:manage');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'resource:use');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'resource:delete');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'resource:update');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'resource:create');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'resource:read');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'project:manage');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'team:manage');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'organization:read');
INSERT INTO role_permissions VALUES ('3770afba-f895-4192-a5dd-9aa364f9070c', 'inspection:execute');
INSERT INTO role_permissions VALUES ('3770afba-f895-4192-a5dd-9aa364f9070c', 'diagnosis:read');
INSERT INTO role_permissions VALUES ('3770afba-f895-4192-a5dd-9aa364f9070c', 'diagnosis:start');
INSERT INTO role_permissions VALUES ('3770afba-f895-4192-a5dd-9aa364f9070c', 'resource:use');
INSERT INTO role_permissions VALUES ('3770afba-f895-4192-a5dd-9aa364f9070c', 'resource:read');
INSERT INTO role_permissions VALUES ('3770afba-f895-4192-a5dd-9aa364f9070c', 'organization:read');
INSERT INTO role_permissions VALUES ('8ce59528-658a-499e-b63d-28dad1596b0d', 'diagnosis:read');
INSERT INTO role_permissions VALUES ('8ce59528-658a-499e-b63d-28dad1596b0d', 'resource:read');
INSERT INTO role_permissions VALUES ('8ce59528-658a-499e-b63d-28dad1596b0d', 'organization:read');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'relation:manage');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'resource:use');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'resource:delete');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'resource:update');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'resource:create');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'resource:read');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'project:manage');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'organization:read');
INSERT INTO role_permissions VALUES ('74b4d25e-f72f-413e-9d25-4671056028ea', 'inspection:execute');
INSERT INTO role_permissions VALUES ('74b4d25e-f72f-413e-9d25-4671056028ea', 'diagnosis:read');
INSERT INTO role_permissions VALUES ('74b4d25e-f72f-413e-9d25-4671056028ea', 'diagnosis:start');
INSERT INTO role_permissions VALUES ('74b4d25e-f72f-413e-9d25-4671056028ea', 'resource:use');
INSERT INTO role_permissions VALUES ('74b4d25e-f72f-413e-9d25-4671056028ea', 'resource:read');
INSERT INTO role_permissions VALUES ('74b4d25e-f72f-413e-9d25-4671056028ea', 'organization:read');
INSERT INTO role_permissions VALUES ('4ef9298b-c770-4823-9cee-54becdbbf9d5', 'diagnosis:read');
INSERT INTO role_permissions VALUES ('4ef9298b-c770-4823-9cee-54becdbbf9d5', 'resource:read');
INSERT INTO role_permissions VALUES ('4ef9298b-c770-4823-9cee-54becdbbf9d5', 'organization:read');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'member:grant');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'member:grant');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'member:grant');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'inspection:execute');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'diagnosis:read');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'diagnosis:start');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'inspection:execute');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'diagnosis:read');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'diagnosis:start');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'persona:use');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'persona:manage');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'persona:read');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'skill:execute');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'skill:manage');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'skill:read');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'provider:use');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'provider:manage');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'provider:read');
INSERT INTO role_permissions VALUES ('85c46e68-001e-4988-8cae-ab2d11ee3d3e', 'persona:use');
INSERT INTO role_permissions VALUES ('85c46e68-001e-4988-8cae-ab2d11ee3d3e', 'persona:read');
INSERT INTO role_permissions VALUES ('85c46e68-001e-4988-8cae-ab2d11ee3d3e', 'skill:execute');
INSERT INTO role_permissions VALUES ('85c46e68-001e-4988-8cae-ab2d11ee3d3e', 'skill:read');
INSERT INTO role_permissions VALUES ('85c46e68-001e-4988-8cae-ab2d11ee3d3e', 'provider:use');
INSERT INTO role_permissions VALUES ('85c46e68-001e-4988-8cae-ab2d11ee3d3e', 'provider:read');
INSERT INTO role_permissions VALUES ('e9b6da21-9b77-4774-ba82-3259fc845000', 'persona:read');
INSERT INTO role_permissions VALUES ('e9b6da21-9b77-4774-ba82-3259fc845000', 'skill:read');
INSERT INTO role_permissions VALUES ('e9b6da21-9b77-4774-ba82-3259fc845000', 'provider:read');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'persona:use');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'persona:manage');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'persona:read');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'skill:execute');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'skill:manage');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'skill:read');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'provider:use');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'provider:manage');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'provider:read');
INSERT INTO role_permissions VALUES ('3770afba-f895-4192-a5dd-9aa364f9070c', 'persona:use');
INSERT INTO role_permissions VALUES ('3770afba-f895-4192-a5dd-9aa364f9070c', 'persona:read');
INSERT INTO role_permissions VALUES ('3770afba-f895-4192-a5dd-9aa364f9070c', 'skill:execute');
INSERT INTO role_permissions VALUES ('3770afba-f895-4192-a5dd-9aa364f9070c', 'skill:read');
INSERT INTO role_permissions VALUES ('3770afba-f895-4192-a5dd-9aa364f9070c', 'provider:use');
INSERT INTO role_permissions VALUES ('3770afba-f895-4192-a5dd-9aa364f9070c', 'provider:read');
INSERT INTO role_permissions VALUES ('8ce59528-658a-499e-b63d-28dad1596b0d', 'persona:read');
INSERT INTO role_permissions VALUES ('8ce59528-658a-499e-b63d-28dad1596b0d', 'skill:read');
INSERT INTO role_permissions VALUES ('8ce59528-658a-499e-b63d-28dad1596b0d', 'provider:read');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'persona:use');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'persona:manage');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'persona:read');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'skill:execute');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'skill:manage');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'skill:read');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'provider:use');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'provider:manage');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'provider:read');
INSERT INTO role_permissions VALUES ('74b4d25e-f72f-413e-9d25-4671056028ea', 'persona:use');
INSERT INTO role_permissions VALUES ('74b4d25e-f72f-413e-9d25-4671056028ea', 'persona:read');
INSERT INTO role_permissions VALUES ('74b4d25e-f72f-413e-9d25-4671056028ea', 'skill:execute');
INSERT INTO role_permissions VALUES ('74b4d25e-f72f-413e-9d25-4671056028ea', 'skill:read');
INSERT INTO role_permissions VALUES ('74b4d25e-f72f-413e-9d25-4671056028ea', 'provider:use');
INSERT INTO role_permissions VALUES ('74b4d25e-f72f-413e-9d25-4671056028ea', 'provider:read');
INSERT INTO role_permissions VALUES ('4ef9298b-c770-4823-9cee-54becdbbf9d5', 'persona:read');
INSERT INTO role_permissions VALUES ('4ef9298b-c770-4823-9cee-54becdbbf9d5', 'skill:read');
INSERT INTO role_permissions VALUES ('4ef9298b-c770-4823-9cee-54becdbbf9d5', 'provider:read');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'engine:use');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'engine:manage');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'engine:read');
INSERT INTO role_permissions VALUES ('85c46e68-001e-4988-8cae-ab2d11ee3d3e', 'engine:use');
INSERT INTO role_permissions VALUES ('85c46e68-001e-4988-8cae-ab2d11ee3d3e', 'engine:read');
INSERT INTO role_permissions VALUES ('e9b6da21-9b77-4774-ba82-3259fc845000', 'engine:read');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'engine:use');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'engine:manage');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'engine:read');
INSERT INTO role_permissions VALUES ('3770afba-f895-4192-a5dd-9aa364f9070c', 'engine:use');
INSERT INTO role_permissions VALUES ('3770afba-f895-4192-a5dd-9aa364f9070c', 'engine:read');
INSERT INTO role_permissions VALUES ('8ce59528-658a-499e-b63d-28dad1596b0d', 'engine:read');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'engine:use');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'engine:manage');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'engine:read');
INSERT INTO role_permissions VALUES ('74b4d25e-f72f-413e-9d25-4671056028ea', 'engine:use');
INSERT INTO role_permissions VALUES ('74b4d25e-f72f-413e-9d25-4671056028ea', 'engine:read');
INSERT INTO role_permissions VALUES ('4ef9298b-c770-4823-9cee-54becdbbf9d5', 'engine:read');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'notification:read');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'notification:manage');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'notification:test');
INSERT INTO role_permissions VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'notification:retry');
INSERT INTO role_permissions VALUES ('85c46e68-001e-4988-8cae-ab2d11ee3d3e', 'notification:read');
INSERT INTO role_permissions VALUES ('85c46e68-001e-4988-8cae-ab2d11ee3d3e', 'notification:test');
INSERT INTO role_permissions VALUES ('e9b6da21-9b77-4774-ba82-3259fc845000', 'notification:read');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'notification:read');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'notification:manage');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'notification:test');
INSERT INTO role_permissions VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'notification:retry');
INSERT INTO role_permissions VALUES ('3770afba-f895-4192-a5dd-9aa364f9070c', 'notification:read');
INSERT INTO role_permissions VALUES ('3770afba-f895-4192-a5dd-9aa364f9070c', 'notification:test');
INSERT INTO role_permissions VALUES ('8ce59528-658a-499e-b63d-28dad1596b0d', 'notification:read');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'notification:read');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'notification:manage');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'notification:test');
INSERT INTO role_permissions VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'notification:retry');
INSERT INTO role_permissions VALUES ('74b4d25e-f72f-413e-9d25-4671056028ea', 'notification:read');
INSERT INTO role_permissions VALUES ('74b4d25e-f72f-413e-9d25-4671056028ea', 'notification:test');
INSERT INTO role_permissions VALUES ('4ef9298b-c770-4823-9cee-54becdbbf9d5', 'notification:read');


--
-- Data for Name: roles; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO roles VALUES ('a6448a92-25b1-4191-84e3-86e8a013d59a', 'PlatformAdmin', 'platform', true, '2026-10-22 01:22:31.617466+00');
INSERT INTO roles VALUES ('85c46e68-001e-4988-8cae-ab2d11ee3d3e', 'PlatformOperator', 'platform', true, '2026-10-22 01:22:31.617466+00');
INSERT INTO roles VALUES ('e9b6da21-9b77-4774-ba82-3259fc845000', 'PlatformViewer', 'platform', true, '2026-10-22 01:22:31.617466+00');
INSERT INTO roles VALUES ('a532826a-6fc9-457a-b6b9-0cdde6d40a10', 'TeamAdmin', 'team', true, '2026-10-22 01:22:31.617466+00');
INSERT INTO roles VALUES ('3770afba-f895-4192-a5dd-9aa364f9070c', 'TeamOperator', 'team', true, '2026-10-22 01:22:31.617466+00');
INSERT INTO roles VALUES ('8ce59528-658a-499e-b63d-28dad1596b0d', 'TeamViewer', 'team', true, '2026-10-22 01:22:31.617466+00');
INSERT INTO roles VALUES ('ce4a0d04-44de-49b6-b4d0-03b8b4b0f03d', 'ProjectAdmin', 'project', true, '2026-10-22 01:22:31.617466+00');
INSERT INTO roles VALUES ('74b4d25e-f72f-413e-9d25-4671056028ea', 'ProjectOperator', 'project', true, '2026-10-22 01:22:31.617466+00');
INSERT INTO roles VALUES ('4ef9298b-c770-4823-9cee-54becdbbf9d5', 'ProjectViewer', 'project', true, '2026-10-22 01:22:31.617466+00');


--
-- Data for Name: scope_ai_provider_bindings; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: scope_defaults; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: scopes; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO scopes VALUES ('2817fc61-c924-4760-84a7-06a0d3f8995f', 'default', 'platform', NULL, 'active', '2026-10-22 01:22:31.617466+00', '2026-10-22 01:22:31.617466+00', NULL);


--
-- Data for Name: sessions; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: skill_scope_defaults; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: skill_versions; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO skill_versions VALUES ('9a254bbd-28d2-4147-8886-f47c75fa3097', '61e943a6-07db-4b4b-b893-b6f1cca9d17a', 2, '{"name": "Kubernetes-工作负载诊断", "description": "Kubernetes 工作负载、Pod、事件、探针和发布状态的只读诊断。", "instruction": "仅调用声明的 Kubernetes 只读工具。输出 JSON，且必须包含 facts、findings、evidence、hypotheses、confidence 和 recommendations；将事实、异常和待验证假设分开。", "target_kinds": ["Application", "Kubernetes"]}', '{"type": "object", "required": ["facts", "findings", "evidence", "hypotheses", "confidence", "recommendations"], "properties": {"facts": {"type": "object"}, "evidence": {"type": "array"}, "findings": {"type": "array"}, "confidence": {"type": "number", "maximum": 1, "minimum": 0}, "hypotheses": {"type": "array"}, "recommendations": {"type": "array"}}, "additionalProperties": false}', '{"type": "object", "required": ["facts", "findings", "evidence", "hypotheses", "confidence", "recommendations"], "properties": {"facts": {"type": "object"}, "evidence": {"type": "array"}, "findings": {"type": "array"}, "confidence": {"type": "number", "maximum": 1, "minimum": 0}, "hypotheses": {"type": "array"}, "recommendations": {"type": "array"}}, "additionalProperties": false}', '[{"name": "connector_kubernetes_read", "description": "受限读取 Kubernetes 对象。", "input_schema": {"type": "object", "required": ["target_resource_id", "resource"], "properties": {"name": {"type": "string"}, "limit": {"type": "integer"}, "resource": {"type": "string"}, "namespace": {"type": "string"}, "label_selector": {"type": "string"}, "target_resource_id": {"type": "string"}}, "additionalProperties": false}}]', 'read_only', 'published', NULL, '2026-10-22 01:22:31.617466+00', '2026-10-22 01:22:31.617466+00');
INSERT INTO skill_versions VALUES ('563e2b5b-3fc3-4046-af8b-eb293bd660ca', 'b8a85717-7d66-4d0a-b1ff-f4375aba1ee1', 2, '{"name": "Kafka-健康诊断", "description": "Kafka Broker、Topic、分区、ISR、消费组和积压的只读诊断。", "instruction": "只调用 Kafka 固定诊断快照；不得请求管理操作。输出 JSON，且必须包含 facts、findings、evidence、hypotheses、confidence 和 recommendations。", "target_kinds": ["Kafka"]}', '{"type": "object", "required": ["facts", "findings", "evidence", "hypotheses", "confidence", "recommendations"], "properties": {"facts": {"type": "object"}, "evidence": {"type": "array"}, "findings": {"type": "array"}, "confidence": {"type": "number", "maximum": 1, "minimum": 0}, "hypotheses": {"type": "array"}, "recommendations": {"type": "array"}}, "additionalProperties": false}', '{"type": "object", "required": ["facts", "findings", "evidence", "hypotheses", "confidence", "recommendations"], "properties": {"facts": {"type": "object"}, "evidence": {"type": "array"}, "findings": {"type": "array"}, "confidence": {"type": "number", "maximum": 1, "minimum": 0}, "hypotheses": {"type": "array"}, "recommendations": {"type": "array"}}, "additionalProperties": false}', '[{"name": "connector_kafka_inspect", "description": "读取固定 Kafka 健康快照。", "input_schema": {"type": "object", "required": ["target_resource_id"], "properties": {"target_resource_id": {"type": "string"}}, "additionalProperties": false}}]', 'read_only', 'published', NULL, '2026-10-22 01:22:31.617466+00', '2026-10-22 01:22:31.617466+00');
INSERT INTO skill_versions VALUES ('0539e357-4073-4de0-8b33-adfe0a5da7eb', '61e943a6-07db-4b4b-b893-b6f1cca9d17a', 1, '{"name": "Kubernetes-工作负载诊断", "description": "Kubernetes 工作负载、Pod、事件、探针和发布状态的只读诊断。", "instruction": "仅调用声明的 Kubernetes 只读工具。输出 JSON，且必须包含 facts、findings、evidence、hypotheses、confidence 和 recommendations；将事实、异常和待验证假设分开。", "target_kinds": ["Application", "Kubernetes"]}', '{"type": "object", "required": ["facts", "findings", "evidence", "hypotheses", "confidence", "recommendations"], "properties": {"facts": {"type": "object"}, "evidence": {"type": "array"}, "findings": {"type": "array"}, "confidence": {"type": "number", "maximum": 1, "minimum": 0}, "hypotheses": {"type": "array"}, "recommendations": {"type": "array"}}, "additionalProperties": false}', '{"type": "object", "additionalProperties": true}', '[{"name": "connector_kubernetes_read", "description": "受限读取 Kubernetes 对象。", "input_schema": {"type": "object", "required": ["target_resource_id", "resource"], "properties": {"name": {"type": "string"}, "limit": {"type": "integer"}, "resource": {"type": "string"}, "namespace": {"type": "string"}, "label_selector": {"type": "string"}, "target_resource_id": {"type": "string"}}, "additionalProperties": false}}]', 'read_only', 'disabled', NULL, '2026-10-22 01:22:31.617466+00', '2026-10-22 01:22:31.617466+00');
INSERT INTO skill_versions VALUES ('66d8c5d1-5c24-472c-90b1-b85c0826a8a8', 'e79aa588-b389-4ed9-88a1-d93b2f69cd7f', 1, '{"name": "PostgreSQL-健康诊断", "description": "PostgreSQL 连接、会话、慢查询、锁、复制和容量的只读诊断。", "instruction": "只调用 PostgreSQL 固定诊断快照；不得请求写 SQL。输出 JSON，且必须包含 facts、findings、evidence、hypotheses、confidence 和 recommendations。", "target_kinds": ["PostgreSQL"]}', '{"type": "object", "required": ["facts", "findings", "evidence", "hypotheses", "confidence", "recommendations"], "properties": {"facts": {"type": "object"}, "evidence": {"type": "array"}, "findings": {"type": "array"}, "confidence": {"type": "number", "maximum": 1, "minimum": 0}, "hypotheses": {"type": "array"}, "recommendations": {"type": "array"}}, "additionalProperties": false}', '{"type": "object", "additionalProperties": true}', '[{"name": "connector_postgresql_inspect", "description": "读取固定 PostgreSQL 健康快照。", "input_schema": {"type": "object", "required": ["target_resource_id"], "properties": {"target_resource_id": {"type": "string"}}, "additionalProperties": false}}]', 'read_only', 'disabled', NULL, '2026-10-22 01:22:31.617466+00', '2026-10-22 01:22:31.617466+00');
INSERT INTO skill_versions VALUES ('7324e5ee-8796-42cc-939a-26e840f1fe25', 'd2b66149-b2bc-41a4-a9b3-23d6f94835a2', 2, '{"name": "Redis-健康诊断", "description": "Redis 内存、热 Key 能力、慢命令、客户端和复制的只读诊断。", "instruction": "只调用 Redis 固定诊断快照；不得请求写命令。输出 JSON，且必须包含 facts、findings、evidence、hypotheses、confidence 和 recommendations。", "target_kinds": ["Redis"]}', '{"type": "object", "required": ["facts", "findings", "evidence", "hypotheses", "confidence", "recommendations"], "properties": {"facts": {"type": "object"}, "evidence": {"type": "array"}, "findings": {"type": "array"}, "confidence": {"type": "number", "maximum": 1, "minimum": 0}, "hypotheses": {"type": "array"}, "recommendations": {"type": "array"}}, "additionalProperties": false}', '{"type": "object", "required": ["facts", "findings", "evidence", "hypotheses", "confidence", "recommendations"], "properties": {"facts": {"type": "object"}, "evidence": {"type": "array"}, "findings": {"type": "array"}, "confidence": {"type": "number", "maximum": 1, "minimum": 0}, "hypotheses": {"type": "array"}, "recommendations": {"type": "array"}}, "additionalProperties": false}', '[{"name": "connector_redis_inspect", "description": "读取固定 Redis 健康快照。", "input_schema": {"type": "object", "required": ["target_resource_id"], "properties": {"target_resource_id": {"type": "string"}}, "additionalProperties": false}}]', 'read_only', 'disabled', NULL, '2026-10-22 01:22:31.617466+00', '2026-10-22 01:22:31.617466+00');
INSERT INTO skill_versions VALUES ('dfbb6aac-f7fe-46ef-936c-f9392944aec3', 'd2b66149-b2bc-41a4-a9b3-23d6f94835a2', 1, '{"name": "Redis-健康诊断", "description": "Redis 内存、热 Key 能力、慢命令、客户端和复制的只读诊断。", "instruction": "只调用 Redis 固定诊断快照；不得请求写命令。输出 JSON，且必须包含 facts、findings、evidence、hypotheses、confidence 和 recommendations。", "target_kinds": ["Redis"]}', '{"type": "object", "required": ["facts", "findings", "evidence", "hypotheses", "confidence", "recommendations"], "properties": {"facts": {"type": "object"}, "evidence": {"type": "array"}, "findings": {"type": "array"}, "confidence": {"type": "number", "maximum": 1, "minimum": 0}, "hypotheses": {"type": "array"}, "recommendations": {"type": "array"}}, "additionalProperties": false}', '{"type": "object", "additionalProperties": true}', '[{"name": "connector_redis_inspect", "description": "读取固定 Redis 健康快照。", "input_schema": {"type": "object", "required": ["target_resource_id"], "properties": {"target_resource_id": {"type": "string"}}, "additionalProperties": false}}]', 'read_only', 'disabled', NULL, '2026-10-22 01:22:31.617466+00', '2026-10-22 01:22:31.617466+00');
INSERT INTO skill_versions VALUES ('7a3dbfcc-0ea2-44e8-9eaf-d1aace3e2fb9', 'b8a85717-7d66-4d0a-b1ff-f4375aba1ee1', 1, '{"name": "Kafka-健康诊断", "description": "Kafka Broker、Topic、分区、ISR、消费组和积压的只读诊断。", "instruction": "只调用 Kafka 固定诊断快照；不得请求管理操作。输出 JSON，且必须包含 facts、findings、evidence、hypotheses、confidence 和 recommendations。", "target_kinds": ["Kafka"]}', '{"type": "object", "required": ["facts", "findings", "evidence", "hypotheses", "confidence", "recommendations"], "properties": {"facts": {"type": "object"}, "evidence": {"type": "array"}, "findings": {"type": "array"}, "confidence": {"type": "number", "maximum": 1, "minimum": 0}, "hypotheses": {"type": "array"}, "recommendations": {"type": "array"}}, "additionalProperties": false}', '{"type": "object", "additionalProperties": true}', '[{"name": "connector_kafka_inspect", "description": "读取固定 Kafka 健康快照。", "input_schema": {"type": "object", "required": ["target_resource_id"], "properties": {"target_resource_id": {"type": "string"}}, "additionalProperties": false}}]', 'read_only', 'disabled', NULL, '2026-10-22 01:22:31.617466+00', '2026-10-22 01:22:31.617466+00');
INSERT INTO skill_versions VALUES ('902b4b8a-1592-4c15-b724-0bbd259c029f', 'e79aa588-b389-4ed9-88a1-d93b2f69cd7f', 3, '{"name": "PostgreSQL-健康诊断", "description": "PostgreSQL 连接、会话、慢查询、锁、复制和容量的只读诊断。", "instruction": "只调用 PostgreSQL 固定诊断快照；不得请求写 SQL。输出 JSON，且必须包含 facts、findings、evidence、hypotheses、confidence 和 recommendations。", "target_kinds": ["PostgreSQL"]}', '{"type": "object", "required": ["facts", "findings", "evidence", "hypotheses", "confidence", "recommendations"], "properties": {"facts": {"type": "object"}, "evidence": {"type": "array"}, "findings": {"type": "array"}, "confidence": {"type": "number", "maximum": 1, "minimum": 0}, "hypotheses": {"type": "array"}, "recommendations": {"type": "array"}}, "additionalProperties": false}', '{"type": "object", "required": ["facts", "findings", "evidence", "hypotheses", "confidence", "recommendations"], "properties": {"facts": {"type": "object"}, "evidence": {"type": "array"}, "findings": {"type": "array"}, "confidence": {"type": "number", "maximum": 1, "minimum": 0}, "hypotheses": {"type": "array"}, "recommendations": {"type": "array"}}, "additionalProperties": false}', '[{"name": "postgresql_health", "description": "检查 PostgreSQL 健康"}, {"name": "postgresql_sessions", "description": "读取活跃会话"}, {"name": "postgresql_long_running_queries", "description": "读取长时间运行查询"}, {"name": "postgresql_locks", "description": "读取等待锁"}, {"name": "postgresql_replication", "description": "读取复制状态"}, {"name": "postgresql_capacity", "description": "读取数据库容量"}, {"name": "postgresql_tables", "description": "读取用户表统计"}, {"name": "postgresql_performance", "description": "读取性能与配置"}, {"name": "postgresql_vacuum", "description": "读取 VACUUM 配置"}, {"name": "postgresql_extensions", "description": "读取扩展信息"}, {"name": "postgresql_database_info", "description": "读取数据库概要"}, {"name": "postgresql_table_columns", "description": "读取指定用户表列信息"}]', 'read_only', 'published', NULL, '2026-10-22 01:22:37.04565+00', '2026-10-22 01:22:37.04565+00');
INSERT INTO skill_versions VALUES ('4b2a696d-47e8-40be-80f9-98dc6da38815', 'e79aa588-b389-4ed9-88a1-d93b2f69cd7f', 2, '{"name": "PostgreSQL-健康诊断", "description": "PostgreSQL 连接、会话、慢查询、锁、复制和容量的只读诊断。", "instruction": "只调用 PostgreSQL 固定诊断快照；不得请求写 SQL。输出 JSON，且必须包含 facts、findings、evidence、hypotheses、confidence 和 recommendations。", "target_kinds": ["PostgreSQL"]}', '{"type": "object", "required": ["facts", "findings", "evidence", "hypotheses", "confidence", "recommendations"], "properties": {"facts": {"type": "object"}, "evidence": {"type": "array"}, "findings": {"type": "array"}, "confidence": {"type": "number", "maximum": 1, "minimum": 0}, "hypotheses": {"type": "array"}, "recommendations": {"type": "array"}}, "additionalProperties": false}', '{"type": "object", "required": ["facts", "findings", "evidence", "hypotheses", "confidence", "recommendations"], "properties": {"facts": {"type": "object"}, "evidence": {"type": "array"}, "findings": {"type": "array"}, "confidence": {"type": "number", "maximum": 1, "minimum": 0}, "hypotheses": {"type": "array"}, "recommendations": {"type": "array"}}, "additionalProperties": false}', '[{"name": "connector_postgresql_inspect", "description": "读取固定 PostgreSQL 健康快照。", "input_schema": {"type": "object", "required": ["target_resource_id"], "properties": {"target_resource_id": {"type": "string"}}, "additionalProperties": false}}]', 'read_only', 'disabled', NULL, '2026-10-22 01:22:31.617466+00', '2026-10-22 01:22:31.617466+00');
INSERT INTO skill_versions VALUES ('19c4aa07-6739-4a30-a55f-4262c0d0a15c', 'd2b66149-b2bc-41a4-a9b3-23d6f94835a2', 3, '{"name": "Redis-健康诊断", "description": "Redis 内存、热 Key 能力、慢命令、客户端和复制的只读诊断。", "instruction": "只调用 Redis 固定诊断快照；不得请求写命令。输出 JSON，且必须包含 facts、findings、evidence、hypotheses、confidence 和 recommendations。", "target_kinds": ["Redis"]}', '{"type": "object", "required": ["facts", "findings", "evidence", "hypotheses", "confidence", "recommendations"], "properties": {"facts": {"type": "object"}, "evidence": {"type": "array"}, "findings": {"type": "array"}, "confidence": {"type": "number", "maximum": 1, "minimum": 0}, "hypotheses": {"type": "array"}, "recommendations": {"type": "array"}}, "additionalProperties": false}', '{"type": "object", "required": ["facts", "findings", "evidence", "hypotheses", "confidence", "recommendations"], "properties": {"facts": {"type": "object"}, "evidence": {"type": "array"}, "findings": {"type": "array"}, "confidence": {"type": "number", "maximum": 1, "minimum": 0}, "hypotheses": {"type": "array"}, "recommendations": {"type": "array"}}, "additionalProperties": false}', '[{"name": "redis_health", "description": "检查 Redis 健康"}, {"name": "redis_memory", "description": "读取内存统计"}, {"name": "redis_clients", "description": "读取客户端统计"}, {"name": "redis_replication", "description": "读取复制状态"}, {"name": "redis_slowlog", "description": "读取慢日志"}, {"name": "redis_database_info", "description": "读取数据库信息"}]', 'read_only', 'published', NULL, '2026-10-22 01:22:37.179198+00', '2026-10-22 01:22:37.179198+00');
INSERT INTO skill_versions VALUES ('6bc5787c-da55-4e66-b50f-d7c8e2709f2a', 'f789ea56-a16d-4b3b-a288-8071cd9c20aa', 1, '{"name": "MySQL-健康诊断", "description": "MySQL 数据库状态、性能、配置和结构的只读诊断。", "instruction": "调用 MySQL 固定只读工具；不要请求或建议写 SQL。", "target_kinds": ["MySQL"]}', '{"type": "object", "required": ["facts", "findings", "evidence", "hypotheses", "confidence", "recommendations"], "properties": {"facts": {"type": "object"}, "evidence": {"type": "array"}, "findings": {"type": "array"}, "confidence": {"type": "number", "maximum": 1, "minimum": 0}, "hypotheses": {"type": "array"}, "recommendations": {"type": "array"}}, "additionalProperties": false}', '{"type": "object", "additionalProperties": true}', '[{"name": "mysql_health", "description": "检查 MySQL 健康"}, {"name": "mysql_status", "description": "读取 MySQL 状态"}, {"name": "mysql_performance", "description": "读取性能与配置"}, {"name": "mysql_tables", "description": "读取用户表信息"}, {"name": "mysql_table_columns", "description": "读取指定表列信息"}, {"name": "mysql_table_structure", "description": "读取指定表结构和索引"}, {"name": "mysql_database_info", "description": "读取数据库概要"}]', 'read_only', 'published', NULL, '2026-10-22 01:22:37.601013+00', '2026-10-22 01:22:37.601013+00');
INSERT INTO skill_versions VALUES ('afd50095-8671-4ddf-927f-fdccb6a9b87c', 'b8a85717-7d66-4d0a-b1ff-f4375aba1ee1', 3, '{"name": "Kafka-健康诊断", "description": "Kafka Broker、Topic、分区、ISR、消费组和积压的只读诊断。", "instruction": "调用 Kafka 固定只读工具；不要请求或建议管理操作。", "target_kinds": ["Kafka"]}', '{}', '{}', '[{"name": "kafka_health"}, {"name": "kafka_brokers"}, {"name": "kafka_topics"}, {"name": "kafka_consumer_groups"}, {"name": "kafka_consumer_lag"}, {"name": "kafka_cluster_info"}, {"name": "kafka_topic_partitions"}]', 'read_only', 'published', NULL, '2026-10-22 01:22:37.72899+00', '2026-10-22 01:22:37.72899+00');
INSERT INTO skill_versions VALUES ('e35ec8d3-caf4-42d9-a118-9755896b089f', '1eb3ad07-eb37-4674-b513-46b2dd73a5b3', 1, '{"name": "Elasticsearch-健康诊断", "description": "Elasticsearch 集群、节点、索引、分片和配置的只读诊断。", "instruction": "调用 Elasticsearch 固定只读工具；不要请求管理操作。", "target_kinds": ["Elasticsearch"]}', '{}', '{}', '[{"name": "elasticsearch_health"}, {"name": "elasticsearch_cluster_info"}, {"name": "elasticsearch_nodes"}, {"name": "elasticsearch_indices"}, {"name": "elasticsearch_shards"}, {"name": "elasticsearch_settings"}, {"name": "elasticsearch_index_mapping"}]', 'read_only', 'published', NULL, '2026-10-22 01:22:37.850777+00', '2026-10-22 01:22:37.850777+00');
INSERT INTO skill_versions VALUES ('4c75896d-72e4-44ce-a309-1c962c905db5', 'ce262bcf-ce9a-408d-8c03-30f9c3a24eb9', 1, '{"name": "Oracle-健康诊断", "description": "Oracle 数据库状态、性能、配置与表结构的只读诊断。", "instruction": "调用 Oracle 固定只读工具；不要请求或建议写入 SQL。", "target_kinds": ["Oracle"]}', '{}', '{}', '[{"name": "oracle_health"}, {"name": "oracle_status"}, {"name": "oracle_performance"}, {"name": "oracle_database_info"}, {"name": "oracle_tables"}, {"name": "oracle_table_columns"}, {"name": "oracle_table_structure"}]', 'read_only', 'published', NULL, '2026-10-22 01:22:38.017579+00', '2026-10-22 01:22:38.017579+00');
INSERT INTO skill_versions VALUES ('4f94c00a-a11e-41a8-9ce8-f62a2a00d732', 'fb20a40a-986c-40c8-b025-0595f1feb3fb', 1, '{"name": "RabbitMQ-健康诊断", "description": "RabbitMQ 集群、节点、队列、交换机、连接和消费者只读诊断。", "instruction": "调用 RabbitMQ Management API 固定只读工具；不要执行写操作。", "target_kinds": ["RabbitMQ"]}', '{}', '{}', '[{"name": "rabbitmq_health"}, {"name": "rabbitmq_overview"}, {"name": "rabbitmq_nodes"}, {"name": "rabbitmq_queues"}, {"name": "rabbitmq_exchanges"}, {"name": "rabbitmq_connections"}, {"name": "rabbitmq_channels"}, {"name": "rabbitmq_consumers"}, {"name": "rabbitmq_vhosts"}]', 'read_only', 'published', NULL, '2026-10-22 01:22:38.145502+00', '2026-10-22 01:22:38.145502+00');
INSERT INTO skill_versions VALUES ('ec326f2a-82bb-4095-a98d-6c29ce70df95', 'b1354c01-f99c-4995-8f9b-645fe1248066', 1, '{"name": "MinIO-健康诊断", "description": "MinIO 连接、存储桶、对象和桶配置的只读诊断。", "instruction": "调用 MinIO 固定只读工具；不要执行写入、删除或管理操作。", "target_kinds": ["MinIO"]}', '{}', '{}', '[{"name": "minio_health"}, {"name": "minio_buckets"}, {"name": "minio_bucket_objects"}, {"name": "minio_object_stat"}, {"name": "minio_bucket_versioning"}, {"name": "minio_bucket_lifecycle"}]', 'read_only', 'published', NULL, '2026-10-22 01:22:38.411951+00', '2026-10-22 01:22:38.411951+00');
INSERT INTO skill_versions VALUES ('9c696f02-b296-4bc1-91b2-3a2d70d66345', 'fd0e8fcd-7837-4cb3-959f-b7d0e047a943', 1, '{"name": "OceanBase-健康诊断", "description": "OceanBase MySQL/Oracle 模式的状态、性能、配置与表结构只读诊断。", "instruction": "调用 OceanBase 固定只读工具；不要请求或建议写入 SQL。", "target_kinds": ["OceanBase"]}', '{}', '{}', '[{"name": "oceanbase_health"}, {"name": "oceanbase_status"}, {"name": "oceanbase_performance"}, {"name": "oceanbase_database_info"}, {"name": "oceanbase_tables"}, {"name": "oceanbase_table_columns"}, {"name": "oceanbase_table_structure"}]', 'read_only', 'published', NULL, '2026-10-22 01:22:42.94243+00', '2026-10-22 01:22:42.94243+00');


--
-- Data for Name: skills; Type: TABLE DATA; Schema: public; Owner: -
--

INSERT INTO skills VALUES ('61e943a6-07db-4b4b-b893-b6f1cca9d17a', 'default', '2817fc61-c924-4760-84a7-06a0d3f8995f', 'Kubernetes-工作负载诊断', 'kubernetes-', 'diagnosis', '[]', 'OpsKeeper builtin', 'active', '2026-10-22 01:22:31.617466+00', '2026-10-22 01:22:31.617466+00', NULL);
INSERT INTO skills VALUES ('e79aa588-b389-4ed9-88a1-d93b2f69cd7f', 'default', '2817fc61-c924-4760-84a7-06a0d3f8995f', 'PostgreSQL-健康诊断', 'postgresql-', 'diagnosis', '[]', 'OpsKeeper builtin', 'active', '2026-10-22 01:22:31.617466+00', '2026-10-22 01:22:31.617466+00', NULL);
INSERT INTO skills VALUES ('d2b66149-b2bc-41a4-a9b3-23d6f94835a2', 'default', '2817fc61-c924-4760-84a7-06a0d3f8995f', 'Redis-健康诊断', 'redis-', 'diagnosis', '[]', 'OpsKeeper builtin', 'active', '2026-10-22 01:22:31.617466+00', '2026-10-22 01:22:31.617466+00', NULL);
INSERT INTO skills VALUES ('b8a85717-7d66-4d0a-b1ff-f4375aba1ee1', 'default', '2817fc61-c924-4760-84a7-06a0d3f8995f', 'Kafka-健康诊断', 'kafka-', 'diagnosis', '[]', 'OpsKeeper builtin', 'active', '2026-10-22 01:22:31.617466+00', '2026-10-22 01:22:31.617466+00', NULL);
INSERT INTO skills VALUES ('f789ea56-a16d-4b3b-a288-8071cd9c20aa', 'default', '2817fc61-c924-4760-84a7-06a0d3f8995f', 'MySQL-健康诊断', 'mysql-', 'diagnosis', '[]', 'OpsKeeper builtin', 'active', '2026-10-22 01:22:37.601013+00', '2026-10-22 01:22:37.601013+00', NULL);
INSERT INTO skills VALUES ('1eb3ad07-eb37-4674-b513-46b2dd73a5b3', 'default', '2817fc61-c924-4760-84a7-06a0d3f8995f', 'Elasticsearch-健康诊断', 'elasticsearch-', 'diagnosis', '[]', 'OpsKeeper builtin', 'active', '2026-10-22 01:22:37.850777+00', '2026-10-22 01:22:37.850777+00', NULL);
INSERT INTO skills VALUES ('ce262bcf-ce9a-408d-8c03-30f9c3a24eb9', 'default', '2817fc61-c924-4760-84a7-06a0d3f8995f', 'Oracle-健康诊断', 'oracle-', 'diagnosis', '[]', 'OpsKeeper builtin', 'active', '2026-10-22 01:22:38.017579+00', '2026-10-22 01:22:38.017579+00', NULL);
INSERT INTO skills VALUES ('fb20a40a-986c-40c8-b025-0595f1feb3fb', 'default', '2817fc61-c924-4760-84a7-06a0d3f8995f', 'RabbitMQ-健康诊断', 'rabbitmq-', 'diagnosis', '[]', 'OpsKeeper builtin', 'active', '2026-10-22 01:22:38.145502+00', '2026-10-22 01:22:38.145502+00', NULL);
INSERT INTO skills VALUES ('b1354c01-f99c-4995-8f9b-645fe1248066', 'default', '2817fc61-c924-4760-84a7-06a0d3f8995f', 'MinIO-健康诊断', 'minio-', 'diagnosis', '[]', 'OpsKeeper builtin', 'active', '2026-10-22 01:22:38.411951+00', '2026-10-22 01:22:38.411951+00', NULL);
INSERT INTO skills VALUES ('fd0e8fcd-7837-4cb3-959f-b7d0e047a943', 'default', '2817fc61-c924-4760-84a7-06a0d3f8995f', 'OceanBase-健康诊断', 'oceanbase-health-diagnosis', 'diagnosis', '[]', 'OpsKeeper builtin', 'active', '2026-10-22 01:22:42.94243+00', '2026-10-22 01:22:42.94243+00', NULL);


--
-- Data for Name: teams; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: -
--



--
-- Name: diagnosis_events_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('diagnosis_events_id_seq', 1, false);


--
-- Name: inspection_health_snapshots_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('inspection_health_snapshots_id_seq', 1, false);


--
-- Name: inspection_run_steps_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('inspection_run_steps_id_seq', 1, false);


--
-- Name: notification_delivery_attempts_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('notification_delivery_attempts_id_seq', 1, false);


--
-- Name: persona_versions agent_profile_versions_agent_profile_resource_id_version_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY persona_versions
    ADD CONSTRAINT agent_profile_versions_agent_profile_resource_id_version_key UNIQUE (persona_id, version);


--
-- Name: persona_versions agent_profile_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY persona_versions
    ADD CONSTRAINT agent_profile_versions_pkey PRIMARY KEY (id);


--
-- Name: engine_execution_events ai_execution_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY engine_execution_events
    ADD CONSTRAINT ai_execution_events_pkey PRIMARY KEY (execution_id, sequence);


--
-- Name: engine_execution_tool_calls ai_execution_tool_calls_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY engine_execution_tool_calls
    ADD CONSTRAINT ai_execution_tool_calls_pkey PRIMARY KEY (execution_id, sequence);


--
-- Name: ai_workflow_runs ai_workflow_runs_execution_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_workflow_runs
    ADD CONSTRAINT ai_workflow_runs_execution_id_key UNIQUE (execution_id);


--
-- Name: ai_workflow_runs ai_workflow_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_workflow_runs
    ADD CONSTRAINT ai_workflow_runs_pkey PRIMARY KEY (id);


--
-- Name: application_dependencies application_dependencies_application_id_target_resource_id__key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY application_dependencies
    ADD CONSTRAINT application_dependencies_application_id_target_resource_id__key UNIQUE (application_id, target_resource_id, dependency_kind);


--
-- Name: application_dependencies application_dependencies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY application_dependencies
    ADD CONSTRAINT application_dependencies_pkey PRIMARY KEY (id);


--
-- Name: application_instances application_instances_application_id_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY application_instances
    ADD CONSTRAINT application_instances_application_id_name_key UNIQUE (application_id, name);


--
-- Name: application_instances application_instances_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY application_instances
    ADD CONSTRAINT application_instances_pkey PRIMARY KEY (id);


--
-- Name: applications applications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY applications
    ADD CONSTRAINT applications_pkey PRIMARY KEY (id);


--
-- Name: applications applications_project_id_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY applications
    ADD CONSTRAINT applications_project_id_code_key UNIQUE (project_id, code);


--
-- Name: audit_events audit_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY audit_events
    ADD CONSTRAINT audit_events_pkey PRIMARY KEY (id);


--
-- Name: audit_retention_runs audit_retention_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY audit_retention_runs
    ADD CONSTRAINT audit_retention_runs_pkey PRIMARY KEY (id);


--
-- Name: authorization_revision authorization_revision_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY authorization_revision
    ADD CONSTRAINT authorization_revision_pkey PRIMARY KEY (id);


--
-- Name: cache_entries cache_entries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY cache_entries
    ADD CONSTRAINT cache_entries_pkey PRIMARY KEY (cache_key);


--
-- Name: diagnosis_causal_chains diagnosis_causal_chains_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_causal_chains
    ADD CONSTRAINT diagnosis_causal_chains_pkey PRIMARY KEY (id);


--
-- Name: diagnosis_causal_chains diagnosis_causal_chains_run_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_causal_chains
    ADD CONSTRAINT diagnosis_causal_chains_run_id_key UNIQUE (run_id);


--
-- Name: diagnosis_causal_chains diagnosis_causal_chains_session_id_version_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_causal_chains
    ADD CONSTRAINT diagnosis_causal_chains_session_id_version_key UNIQUE (session_id, version);


--
-- Name: diagnosis_events diagnosis_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_events
    ADD CONSTRAINT diagnosis_events_pkey PRIMARY KEY (id);


--
-- Name: diagnosis_evidence diagnosis_evidence_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_evidence
    ADD CONSTRAINT diagnosis_evidence_pkey PRIMARY KEY (id);


--
-- Name: diagnosis_hypotheses diagnosis_hypotheses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_hypotheses
    ADD CONSTRAINT diagnosis_hypotheses_pkey PRIMARY KEY (id);


--
-- Name: diagnosis_messages diagnosis_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_messages
    ADD CONSTRAINT diagnosis_messages_pkey PRIMARY KEY (id);


--
-- Name: diagnosis_plan_steps diagnosis_plan_steps_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_plan_steps
    ADD CONSTRAINT diagnosis_plan_steps_pkey PRIMARY KEY (id);


--
-- Name: diagnosis_plan_steps diagnosis_plan_steps_plan_id_sequence_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_plan_steps
    ADD CONSTRAINT diagnosis_plan_steps_plan_id_sequence_key UNIQUE (plan_id, sequence);


--
-- Name: diagnosis_plans diagnosis_plans_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_plans
    ADD CONSTRAINT diagnosis_plans_pkey PRIMARY KEY (id);


--
-- Name: diagnosis_plans diagnosis_plans_session_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_plans
    ADD CONSTRAINT diagnosis_plans_session_id_key UNIQUE (session_id);


--
-- Name: diagnosis_reports diagnosis_reports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_reports
    ADD CONSTRAINT diagnosis_reports_pkey PRIMARY KEY (id);


--
-- Name: diagnosis_reports diagnosis_reports_session_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_reports
    ADD CONSTRAINT diagnosis_reports_session_id_key UNIQUE (session_id);


--
-- Name: diagnosis_runs diagnosis_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_runs
    ADD CONSTRAINT diagnosis_runs_pkey PRIMARY KEY (id);


--
-- Name: diagnosis_runs diagnosis_runs_session_id_sequence_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_runs
    ADD CONSTRAINT diagnosis_runs_session_id_sequence_key UNIQUE (session_id, sequence);


--
-- Name: diagnosis_sessions diagnosis_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_sessions
    ADD CONSTRAINT diagnosis_sessions_pkey PRIMARY KEY (id);


--
-- Name: diagnosis_targets diagnosis_targets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_targets
    ADD CONSTRAINT diagnosis_targets_pkey PRIMARY KEY (session_id, resource_id);


--
-- Name: engine_capability_terms engine_capability_terms_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY engine_capability_terms
    ADD CONSTRAINT engine_capability_terms_pkey PRIMARY KEY (term);


--
-- Name: engine_scope_bindings engine_scope_bindings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY engine_scope_bindings
    ADD CONSTRAINT engine_scope_bindings_pkey PRIMARY KEY (scope_id, tag);


--
-- Name: engines engines_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY engines
    ADD CONSTRAINT engines_pkey PRIMARY KEY (id);


--
-- Name: group_members group_members_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY group_members
    ADD CONSTRAINT group_members_pkey PRIMARY KEY (group_id, user_id);


--
-- Name: groups groups_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY groups
    ADD CONSTRAINT groups_pkey PRIMARY KEY (id);


--
-- Name: inspection_findings inspection_findings_id_scope_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_findings
    ADD CONSTRAINT inspection_findings_id_scope_unique UNIQUE (id, scope_id);


--
-- Name: inspection_findings inspection_findings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_findings
    ADD CONSTRAINT inspection_findings_pkey PRIMARY KEY (id);


--
-- Name: inspection_findings inspection_findings_policy_id_identity_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_findings
    ADD CONSTRAINT inspection_findings_policy_id_identity_key_key UNIQUE (policy_id, identity_key);


--
-- Name: inspection_health_snapshots inspection_health_snapshots_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_health_snapshots
    ADD CONSTRAINT inspection_health_snapshots_pkey PRIMARY KEY (id);


--
-- Name: inspection_jobs inspection_jobs_idempotency_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_jobs
    ADD CONSTRAINT inspection_jobs_idempotency_key_key UNIQUE (idempotency_key);


--
-- Name: inspection_jobs inspection_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_jobs
    ADD CONSTRAINT inspection_jobs_pkey PRIMARY KEY (id);


--
-- Name: inspection_jobs inspection_jobs_run_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_jobs
    ADD CONSTRAINT inspection_jobs_run_id_key UNIQUE (run_id);


--
-- Name: inspection_policies inspection_policies_id_scope_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_policies
    ADD CONSTRAINT inspection_policies_id_scope_unique UNIQUE (id, scope_id);


--
-- Name: inspection_policies inspection_policies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_policies
    ADD CONSTRAINT inspection_policies_pkey PRIMARY KEY (id);


--
-- Name: inspection_policy_notification_rules inspection_policy_notification_r_policy_id_rule_id_scope_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_policy_notification_rules
    ADD CONSTRAINT inspection_policy_notification_r_policy_id_rule_id_scope_id_key UNIQUE (policy_id, rule_id, scope_id);


--
-- Name: inspection_policy_notification_rules inspection_policy_notification_rules_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_policy_notification_rules
    ADD CONSTRAINT inspection_policy_notification_rules_pkey PRIMARY KEY (policy_id, rule_id);


--
-- Name: inspection_policy_targets inspection_policy_targets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_policy_targets
    ADD CONSTRAINT inspection_policy_targets_pkey PRIMARY KEY (policy_id, resource_id);


--
-- Name: inspection_run_steps inspection_run_steps_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_run_steps
    ADD CONSTRAINT inspection_run_steps_pkey PRIMARY KEY (id);


--
-- Name: inspection_run_steps inspection_run_steps_run_id_sequence_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_run_steps
    ADD CONSTRAINT inspection_run_steps_run_id_sequence_key UNIQUE (run_id, sequence);


--
-- Name: inspection_runs inspection_runs_id_scope_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_runs
    ADD CONSTRAINT inspection_runs_id_scope_unique UNIQUE (id, scope_id);


--
-- Name: inspection_runs inspection_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_runs
    ADD CONSTRAINT inspection_runs_pkey PRIMARY KEY (id);


--
-- Name: inspection_runs inspection_runs_policy_id_window_start_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_runs
    ADD CONSTRAINT inspection_runs_policy_id_window_start_key UNIQUE (policy_id, window_start);


--
-- Name: mcp_server_snapshots mcp_server_snapshots_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY mcp_server_snapshots
    ADD CONSTRAINT mcp_server_snapshots_pkey PRIMARY KEY (id);


--
-- Name: notification_channel_test_limits notification_channel_test_limits_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_channel_test_limits
    ADD CONSTRAINT notification_channel_test_limits_pkey PRIMARY KEY (channel_id);


--
-- Name: notification_channel_versions notification_channel_versions_channel_id_version_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_channel_versions
    ADD CONSTRAINT notification_channel_versions_channel_id_version_key UNIQUE (channel_id, version);


--
-- Name: notification_channel_versions notification_channel_versions_id_scope_id_channel_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_channel_versions
    ADD CONSTRAINT notification_channel_versions_id_scope_id_channel_id_key UNIQUE (id, scope_id, channel_id);


--
-- Name: notification_channel_versions notification_channel_versions_id_scope_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_channel_versions
    ADD CONSTRAINT notification_channel_versions_id_scope_id_key UNIQUE (id, scope_id);


--
-- Name: notification_channel_versions notification_channel_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_channel_versions
    ADD CONSTRAINT notification_channel_versions_pkey PRIMARY KEY (id);


--
-- Name: notification_channels notification_channels_id_scope_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_channels
    ADD CONSTRAINT notification_channels_id_scope_unique UNIQUE (id, scope_id);


--
-- Name: notification_channels notification_channels_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_channels
    ADD CONSTRAINT notification_channels_pkey PRIMARY KEY (id);


--
-- Name: notification_deliveries notification_deliveries_id_scope_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_deliveries
    ADD CONSTRAINT notification_deliveries_id_scope_unique UNIQUE (id, scope_id);


--
-- Name: notification_deliveries notification_deliveries_idempotency_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_deliveries
    ADD CONSTRAINT notification_deliveries_idempotency_key_key UNIQUE (idempotency_key);


--
-- Name: notification_deliveries notification_deliveries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_deliveries
    ADD CONSTRAINT notification_deliveries_pkey PRIMARY KEY (id);


--
-- Name: notification_delivery_attempts notification_delivery_attempts_delivery_id_attempt_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_delivery_attempts
    ADD CONSTRAINT notification_delivery_attempts_delivery_id_attempt_key UNIQUE (delivery_id, attempt);


--
-- Name: notification_delivery_attempts notification_delivery_attempts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_delivery_attempts
    ADD CONSTRAINT notification_delivery_attempts_pkey PRIMARY KEY (id);


--
-- Name: notification_events notification_events_event_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_events
    ADD CONSTRAINT notification_events_event_key_key UNIQUE (event_key);


--
-- Name: notification_events notification_events_id_scope_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_events
    ADD CONSTRAINT notification_events_id_scope_id_key UNIQUE (id, scope_id);


--
-- Name: notification_events notification_events_id_scope_id_policy_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_events
    ADD CONSTRAINT notification_events_id_scope_id_policy_id_key UNIQUE (id, scope_id, policy_id);


--
-- Name: notification_events notification_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_events
    ADD CONSTRAINT notification_events_pkey PRIMARY KEY (id);


--
-- Name: notification_rule_cooldowns notification_rule_cooldowns_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_rule_cooldowns
    ADD CONSTRAINT notification_rule_cooldowns_pkey PRIMARY KEY (rule_id, cooldown_key);


--
-- Name: notification_rule_routes notification_rule_routes_id_scope_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_rule_routes
    ADD CONSTRAINT notification_rule_routes_id_scope_id_key UNIQUE (id, scope_id);


--
-- Name: notification_rule_routes notification_rule_routes_id_scope_id_rule_id_channel_id_cha_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_rule_routes
    ADD CONSTRAINT notification_rule_routes_id_scope_id_rule_id_channel_id_cha_key UNIQUE (id, scope_id, rule_id, channel_id, channel_version_id, template_version_id);


--
-- Name: notification_rule_routes notification_rule_routes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_rule_routes
    ADD CONSTRAINT notification_rule_routes_pkey PRIMARY KEY (id);


--
-- Name: notification_rules notification_rules_id_scope_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_rules
    ADD CONSTRAINT notification_rules_id_scope_id_key UNIQUE (id, scope_id);


--
-- Name: notification_rules notification_rules_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_rules
    ADD CONSTRAINT notification_rules_pkey PRIMARY KEY (id);


--
-- Name: notification_template_versions notification_template_versions_id_scope_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_template_versions
    ADD CONSTRAINT notification_template_versions_id_scope_id_key UNIQUE (id, scope_id);


--
-- Name: notification_template_versions notification_template_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_template_versions
    ADD CONSTRAINT notification_template_versions_pkey PRIMARY KEY (id);


--
-- Name: notification_template_versions notification_template_versions_template_id_version_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_template_versions
    ADD CONSTRAINT notification_template_versions_template_id_version_key UNIQUE (template_id, version);


--
-- Name: notification_templates notification_templates_id_scope_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_templates
    ADD CONSTRAINT notification_templates_id_scope_id_key UNIQUE (id, scope_id);


--
-- Name: notification_templates notification_templates_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_templates
    ADD CONSTRAINT notification_templates_pkey PRIMARY KEY (id);


--
-- Name: personas personas_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY personas
    ADD CONSTRAINT personas_pkey PRIMARY KEY (id);


--
-- Name: platforms platforms_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY platforms
    ADD CONSTRAINT platforms_code_key UNIQUE (code);


--
-- Name: platforms platforms_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY platforms
    ADD CONSTRAINT platforms_pkey PRIMARY KEY (id);


--
-- Name: platforms platforms_scope_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY platforms
    ADD CONSTRAINT platforms_scope_id_key UNIQUE (scope_id);


--
-- Name: projects projects_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY projects
    ADD CONSTRAINT projects_pkey PRIMARY KEY (id);


--
-- Name: projects projects_scope_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY projects
    ADD CONSTRAINT projects_scope_id_key UNIQUE (scope_id);


--
-- Name: projects projects_team_id_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY projects
    ADD CONSTRAINT projects_team_id_code_key UNIQUE (team_id, code);


--
-- Name: provider_scope_bindings provider_scope_bindings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY provider_scope_bindings
    ADD CONSTRAINT provider_scope_bindings_pkey PRIMARY KEY (scope_id, tag);


--
-- Name: providers providers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY providers
    ADD CONSTRAINT providers_pkey PRIMARY KEY (id);


--
-- Name: repository_bundles repository_bundles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY repository_bundles
    ADD CONSTRAINT repository_bundles_pkey PRIMARY KEY (resource_id);


--
-- Name: resource_connection_checks resource_connection_checks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_connection_checks
    ADD CONSTRAINT resource_connection_checks_pkey PRIMARY KEY (id);


--
-- Name: resource_relations resource_relations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_relations
    ADD CONSTRAINT resource_relations_pkey PRIMARY KEY (id);


--
-- Name: resource_relations resource_relations_source_resource_id_target_resource_id_re_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_relations
    ADD CONSTRAINT resource_relations_source_resource_id_target_resource_id_re_key UNIQUE (source_resource_id, target_resource_id, relation_type);


--
-- Name: resource_role_bindings resource_role_bindings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_role_bindings
    ADD CONSTRAINT resource_role_bindings_pkey PRIMARY KEY (id);


--
-- Name: resource_role_bindings resource_role_bindings_subject_type_subject_id_role_id_reso_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_role_bindings
    ADD CONSTRAINT resource_role_bindings_subject_type_subject_id_role_id_reso_key UNIQUE (subject_type, subject_id, role_id, resource_id);


--
-- Name: resource_role_permissions resource_role_permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_role_permissions
    ADD CONSTRAINT resource_role_permissions_pkey PRIMARY KEY (role_id, permission);


--
-- Name: resource_roles resource_roles_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_roles
    ADD CONSTRAINT resource_roles_name_key UNIQUE (name);


--
-- Name: resource_roles resource_roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_roles
    ADD CONSTRAINT resource_roles_pkey PRIMARY KEY (id);


--
-- Name: resource_schemas resource_schemas_kind_version_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_schemas
    ADD CONSTRAINT resource_schemas_kind_version_key UNIQUE (kind, version);


--
-- Name: resource_schemas resource_schemas_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_schemas
    ADD CONSTRAINT resource_schemas_pkey PRIMARY KEY (id);


--
-- Name: resource_sync_states resource_sync_states_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_sync_states
    ADD CONSTRAINT resource_sync_states_pkey PRIMARY KEY (resource_id);


--
-- Name: resources resources_id_scope_unique; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resources
    ADD CONSTRAINT resources_id_scope_unique UNIQUE (id, scope_id);


--
-- Name: resources resources_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resources
    ADD CONSTRAINT resources_pkey PRIMARY KEY (id);


--
-- Name: role_bindings role_bindings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY role_bindings
    ADD CONSTRAINT role_bindings_pkey PRIMARY KEY (id);


--
-- Name: role_bindings role_bindings_subject_type_subject_id_role_id_scope_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY role_bindings
    ADD CONSTRAINT role_bindings_subject_type_subject_id_role_id_scope_id_key UNIQUE (subject_type, subject_id, role_id, scope_id);


--
-- Name: role_permissions role_permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY role_permissions
    ADD CONSTRAINT role_permissions_pkey PRIMARY KEY (role_id, permission);


--
-- Name: roles roles_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY roles
    ADD CONSTRAINT roles_name_key UNIQUE (name);


--
-- Name: roles roles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY roles
    ADD CONSTRAINT roles_pkey PRIMARY KEY (id);


--
-- Name: scope_ai_provider_bindings scope_ai_provider_bindings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY scope_ai_provider_bindings
    ADD CONSTRAINT scope_ai_provider_bindings_pkey PRIMARY KEY (scope_id, tag, provider_resource_id);


--
-- Name: scope_defaults scope_defaults_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY scope_defaults
    ADD CONSTRAINT scope_defaults_pkey PRIMARY KEY (scope_id, default_key);


--
-- Name: scopes scopes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY scopes
    ADD CONSTRAINT scopes_pkey PRIMARY KEY (id);


--
-- Name: sessions sessions_access_token_hash_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY sessions
    ADD CONSTRAINT sessions_access_token_hash_key UNIQUE (access_token_hash);


--
-- Name: sessions sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY sessions
    ADD CONSTRAINT sessions_pkey PRIMARY KEY (id);


--
-- Name: sessions sessions_refresh_token_hash_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY sessions
    ADD CONSTRAINT sessions_refresh_token_hash_key UNIQUE (refresh_token_hash);


--
-- Name: skill_scope_defaults skill_scope_defaults_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY skill_scope_defaults
    ADD CONSTRAINT skill_scope_defaults_pkey PRIMARY KEY (scope_id);


--
-- Name: skill_versions skill_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY skill_versions
    ADD CONSTRAINT skill_versions_pkey PRIMARY KEY (id);


--
-- Name: skill_versions skill_versions_skill_resource_id_version_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY skill_versions
    ADD CONSTRAINT skill_versions_skill_resource_id_version_key UNIQUE (skill_id, version);


--
-- Name: skills skills_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY skills
    ADD CONSTRAINT skills_pkey PRIMARY KEY (id);


--
-- Name: teams teams_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY teams
    ADD CONSTRAINT teams_pkey PRIMARY KEY (id);


--
-- Name: teams teams_scope_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY teams
    ADD CONSTRAINT teams_scope_id_key UNIQUE (scope_id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: ai_workflow_runs_scope_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ai_workflow_runs_scope_idx ON ai_workflow_runs USING btree (scope_id, created_at DESC);


--
-- Name: ai_workflow_runs_status_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ai_workflow_runs_status_idx ON ai_workflow_runs USING btree (status, updated_at);


--
-- Name: ai_workflow_runs_workflow_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ai_workflow_runs_workflow_idx ON ai_workflow_runs USING btree (workflow_resource_id, created_at DESC);


--
-- Name: application_dependencies_application_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX application_dependencies_application_idx ON application_dependencies USING btree (application_id);


--
-- Name: application_dependencies_target_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX application_dependencies_target_idx ON application_dependencies USING btree (target_resource_id);


--
-- Name: application_instances_application_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX application_instances_application_idx ON application_instances USING btree (application_id);


--
-- Name: application_instances_target_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX application_instances_target_idx ON application_instances USING btree (target_resource_id);


--
-- Name: applications_project_external_uid_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX applications_project_external_uid_idx ON applications USING btree (project_id, external_uid) WHERE ((deleted_at IS NULL) AND (external_uid <> ''::text));


--
-- Name: applications_project_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX applications_project_idx ON applications USING btree (project_id) WHERE (deleted_at IS NULL);


--
-- Name: audit_events_actor_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX audit_events_actor_idx ON audit_events USING btree (actor_user_id, created_at DESC);


--
-- Name: audit_events_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX audit_events_created_at_idx ON audit_events USING btree (created_at DESC);


--
-- Name: audit_events_scope_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX audit_events_scope_idx ON audit_events USING btree (scope_id, created_at DESC);


--
-- Name: cache_entries_expiry_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX cache_entries_expiry_idx ON cache_entries USING btree (expires_at);


--
-- Name: diagnosis_causal_chains_session_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX diagnosis_causal_chains_session_idx ON diagnosis_causal_chains USING btree (session_id, version DESC);


--
-- Name: diagnosis_events_session_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX diagnosis_events_session_idx ON diagnosis_events USING btree (session_id, id);


--
-- Name: diagnosis_evidence_run_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX diagnosis_evidence_run_idx ON diagnosis_evidence USING btree (run_id, created_at, id);


--
-- Name: diagnosis_evidence_session_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX diagnosis_evidence_session_idx ON diagnosis_evidence USING btree (session_id, created_at, id);


--
-- Name: diagnosis_hypotheses_session_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX diagnosis_hypotheses_session_idx ON diagnosis_hypotheses USING btree (session_id, created_at, id);


--
-- Name: diagnosis_messages_session_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX diagnosis_messages_session_idx ON diagnosis_messages USING btree (session_id, created_at, id);


--
-- Name: diagnosis_runs_session_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX diagnosis_runs_session_idx ON diagnosis_runs USING btree (session_id, sequence);


--
-- Name: diagnosis_sessions_actor_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX diagnosis_sessions_actor_idx ON diagnosis_sessions USING btree (actor_user_id, created_at DESC);


--
-- Name: diagnosis_sessions_application_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX diagnosis_sessions_application_idx ON diagnosis_sessions USING btree (application_id) WHERE (application_id IS NOT NULL);


--
-- Name: diagnosis_sessions_scope_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX diagnosis_sessions_scope_idx ON diagnosis_sessions USING btree (scope_id, created_at DESC);


--
-- Name: engine_capability_terms_lower_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX engine_capability_terms_lower_unique ON engine_capability_terms USING btree (lower(term));


--
-- Name: engine_execution_events_order_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX engine_execution_events_order_idx ON engine_execution_events USING btree (execution_id, sequence);


--
-- Name: engine_execution_tool_calls_provider_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX engine_execution_tool_calls_provider_idx ON engine_execution_tool_calls USING btree (provider_id);


--
-- Name: engine_execution_tool_calls_resource_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX engine_execution_tool_calls_resource_idx ON engine_execution_tool_calls USING btree (resource_id);


--
-- Name: engine_scope_bindings_engine_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX engine_scope_bindings_engine_idx ON engine_scope_bindings USING btree (engine_id);


--
-- Name: engines_scope_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX engines_scope_idx ON engines USING btree (scope_id) WHERE (deleted_at IS NULL);


--
-- Name: engines_scope_name_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX engines_scope_name_unique ON engines USING btree (scope_id, lower(name)) WHERE (deleted_at IS NULL);


--
-- Name: group_members_user_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX group_members_user_idx ON group_members USING btree (user_id, group_id);


--
-- Name: groups_scope_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX groups_scope_idx ON groups USING btree (scope_id) WHERE (deleted_at IS NULL);


--
-- Name: groups_scope_name_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX groups_scope_name_unique ON groups USING btree (scope_id, lower(name)) WHERE (deleted_at IS NULL);


--
-- Name: inspection_findings_scope_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX inspection_findings_scope_idx ON inspection_findings USING btree (policy_id, status, last_observed_at DESC);


--
-- Name: inspection_health_snapshots_target_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX inspection_health_snapshots_target_idx ON inspection_health_snapshots USING btree (target_resource_id, collected_at DESC);


--
-- Name: inspection_jobs_claim_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX inspection_jobs_claim_idx ON inspection_jobs USING btree (status, available_at) WHERE (status = ANY (ARRAY['queued'::text, 'leased'::text]));


--
-- Name: inspection_policies_agent_profile_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX inspection_policies_agent_profile_idx ON inspection_policies USING btree (persona_id) WHERE ((persona_id IS NOT NULL) AND (deleted_at IS NULL));


--
-- Name: inspection_policies_scope_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX inspection_policies_scope_idx ON inspection_policies USING btree (scope_id) WHERE (deleted_at IS NULL);


--
-- Name: inspection_policies_scope_name_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX inspection_policies_scope_name_unique ON inspection_policies USING btree (scope_id, lower(name)) WHERE (deleted_at IS NULL);


--
-- Name: inspection_runs_policy_window_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX inspection_runs_policy_window_idx ON inspection_runs USING btree (policy_id, window_start DESC);


--
-- Name: inspection_runs_scope_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX inspection_runs_scope_created_idx ON inspection_runs USING btree (scope_id, created_at DESC);


--
-- Name: mcp_server_snapshots_server_created_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX mcp_server_snapshots_server_created_idx ON mcp_server_snapshots USING btree (server_resource_id, created_at DESC);


--
-- Name: notification_channels_scope_name_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX notification_channels_scope_name_unique ON notification_channels USING btree (scope_id, lower(name)) WHERE (deleted_at IS NULL);


--
-- Name: notification_deliveries_claim_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX notification_deliveries_claim_idx ON notification_deliveries USING btree (status, available_at) WHERE (status = ANY (ARRAY['queued'::text, 'delivering'::text]));


--
-- Name: notification_deliveries_event_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX notification_deliveries_event_idx ON notification_deliveries USING btree (event_id) WHERE (event_id IS NOT NULL);


--
-- Name: notification_events_run_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX notification_events_run_idx ON notification_events USING btree (run_id) WHERE (run_id IS NOT NULL);


--
-- Name: notification_events_scope_occurred_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX notification_events_scope_occurred_idx ON notification_events USING btree (scope_id, occurred_at DESC);


--
-- Name: notification_rule_cooldowns_last_queued_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX notification_rule_cooldowns_last_queued_idx ON notification_rule_cooldowns USING btree (last_queued_at);


--
-- Name: notification_rule_routes_active_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX notification_rule_routes_active_unique ON notification_rule_routes USING btree (rule_id, channel_id, template_version_id) WHERE (retired_at IS NULL);


--
-- Name: notification_rules_scope_name_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX notification_rules_scope_name_unique ON notification_rules USING btree (scope_id, lower(name)) WHERE (deleted_at IS NULL);


--
-- Name: notification_templates_scope_name_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX notification_templates_scope_name_unique ON notification_templates USING btree (scope_id, lower(name)) WHERE (deleted_at IS NULL);


--
-- Name: persona_versions_persona_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX persona_versions_persona_idx ON persona_versions USING btree (persona_id, version DESC);


--
-- Name: persona_versions_published_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX persona_versions_published_idx ON persona_versions USING btree (persona_id, version DESC) WHERE (status = 'published'::text);


--
-- Name: personas_scope_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX personas_scope_idx ON personas USING btree (scope_id) WHERE (deleted_at IS NULL);


--
-- Name: personas_scope_name_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX personas_scope_name_unique ON personas USING btree (scope_id, lower(name)) WHERE (deleted_at IS NULL);


--
-- Name: personas_scope_identifier_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX personas_scope_identifier_unique ON personas USING btree (scope_id, lower(identifier)) WHERE (deleted_at IS NULL);


--
-- Name: projects_platform_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX projects_platform_id_idx ON projects USING btree (platform_id) WHERE (deleted_at IS NULL);


--
-- Name: projects_team_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX projects_team_id_idx ON projects USING btree (team_id) WHERE (deleted_at IS NULL);


--
-- Name: provider_scope_bindings_provider_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX provider_scope_bindings_provider_idx ON provider_scope_bindings USING btree (provider_id);


--
-- Name: providers_scope_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX providers_scope_idx ON providers USING btree (scope_id) WHERE (deleted_at IS NULL);


--
-- Name: providers_scope_name_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX providers_scope_name_unique ON providers USING btree (scope_id, lower(name)) WHERE (deleted_at IS NULL);


--
-- Name: resource_connection_checks_latest_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX resource_connection_checks_latest_idx ON resource_connection_checks USING btree (resource_id, checked_at DESC, id DESC);


--
-- Name: resource_relations_source_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX resource_relations_source_idx ON resource_relations USING btree (source_resource_id);


--
-- Name: resource_relations_target_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX resource_relations_target_idx ON resource_relations USING btree (target_resource_id);


--
-- Name: resource_role_bindings_resource_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX resource_role_bindings_resource_idx ON resource_role_bindings USING btree (resource_id);


--
-- Name: resource_role_bindings_subject_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX resource_role_bindings_subject_idx ON resource_role_bindings USING btree (subject_type, subject_id);


--
-- Name: resource_schemas_kind_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX resource_schemas_kind_idx ON resource_schemas USING btree (kind, version DESC) WHERE (status = 'active'::text);


--
-- Name: resources_agent_ref_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX resources_agent_ref_idx ON resources USING btree (agent_ref) WHERE (deleted_at IS NULL);


--
-- Name: resources_external_identity_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX resources_external_identity_unique ON resources USING btree (scope_id, kind, external_uid, source_resource_id) WHERE ((deleted_at IS NULL) AND (external_uid <> ''::text) AND (source_resource_id <> ''::text));


--
-- Name: resources_kind_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX resources_kind_idx ON resources USING btree (kind) WHERE (deleted_at IS NULL);


--
-- Name: resources_labels_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX resources_labels_idx ON resources USING gin (labels) WHERE (deleted_at IS NULL);


--
-- Name: resources_scope_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX resources_scope_idx ON resources USING btree (scope_id) WHERE (deleted_at IS NULL);


--
-- Name: resources_scope_name_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX resources_scope_name_unique ON resources USING btree (scope_id, lower(name)) WHERE (deleted_at IS NULL);


--
-- Name: resources_subtype_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX resources_subtype_idx ON resources USING btree (subtype) WHERE (deleted_at IS NULL);


--
-- Name: role_bindings_scope_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX role_bindings_scope_idx ON role_bindings USING btree (scope_id);


--
-- Name: role_bindings_subject_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX role_bindings_subject_idx ON role_bindings USING btree (subject_type, subject_id);


--
-- Name: scope_ai_provider_bindings_provider_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX scope_ai_provider_bindings_provider_idx ON scope_ai_provider_bindings USING btree (provider_resource_id);


--
-- Name: scope_ai_provider_one_tagged_provider; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX scope_ai_provider_one_tagged_provider ON scope_ai_provider_bindings USING btree (scope_id, tag);


--
-- Name: scopes_parent_scope_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX scopes_parent_scope_id_idx ON scopes USING btree (parent_scope_id) WHERE (deleted_at IS NULL);


--
-- Name: scopes_tenant_type_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX scopes_tenant_type_idx ON scopes USING btree (tenant_id, scope_type) WHERE (deleted_at IS NULL);


--
-- Name: sessions_access_lookup_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sessions_access_lookup_idx ON sessions USING btree (access_token_hash) WHERE (revoked_at IS NULL);


--
-- Name: sessions_refresh_lookup_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sessions_refresh_lookup_idx ON sessions USING btree (refresh_token_hash) WHERE (revoked_at IS NULL);


--
-- Name: sessions_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX sessions_user_id_idx ON sessions USING btree (user_id, created_at DESC);


--
-- Name: skill_versions_skill_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX skill_versions_skill_idx ON skill_versions USING btree (skill_id, version DESC);


--
-- Name: skills_scope_identifier_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX skills_scope_identifier_unique ON skills USING btree (scope_id, lower(identifier)) WHERE (deleted_at IS NULL);


--
-- Name: skills_scope_name_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX skills_scope_name_unique ON skills USING btree (scope_id, lower(name)) WHERE (deleted_at IS NULL);


--
-- Name: skills_scope_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX skills_scope_idx ON skills USING btree (scope_id) WHERE (deleted_at IS NULL);


--
-- Name: teams_platform_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX teams_platform_id_idx ON teams USING btree (platform_id) WHERE (deleted_at IS NULL);


--
-- Name: users_email_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX users_email_unique ON users USING btree (lower(email)) WHERE (deleted_at IS NULL);


--
-- Name: users_phone_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX users_phone_unique ON users USING btree (phone) WHERE ((deleted_at IS NULL) AND (phone IS NOT NULL) AND (phone <> ''::text));


--
-- Name: users_username_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX users_username_unique ON users USING btree (lower(username)) WHERE (deleted_at IS NULL);


--
-- Name: ai_workflow_runs ai_workflow_runs_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER ai_workflow_runs_updated_at BEFORE UPDATE ON ai_workflow_runs FOR EACH ROW EXECUTE FUNCTION ai_workflow_runs_touch();


--
-- Name: application_dependencies application_dependencies_validate_target; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER application_dependencies_validate_target BEFORE INSERT OR UPDATE OF application_id, target_resource_id ON application_dependencies FOR EACH ROW EXECUTE FUNCTION validate_application_target();


--
-- Name: application_instances application_instances_validate_runtime; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER application_instances_validate_runtime BEFORE INSERT OR UPDATE OF application_id, target_resource_id, selector ON application_instances FOR EACH ROW EXECUTE FUNCTION validate_application_runtime();


--
-- Name: application_instances application_instances_validate_target; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER application_instances_validate_target BEFORE INSERT OR UPDATE OF application_id, target_resource_id ON application_instances FOR EACH ROW EXECUTE FUNCTION validate_application_target();


--
-- Name: audit_events audit_events_append_only; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER audit_events_append_only BEFORE DELETE OR UPDATE ON audit_events FOR EACH ROW EXECUTE FUNCTION protect_audit_rows();


--
-- Name: audit_retention_runs audit_retention_runs_append_only; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER audit_retention_runs_append_only BEFORE DELETE OR UPDATE ON audit_retention_runs FOR EACH ROW EXECUTE FUNCTION protect_audit_rows();


--
-- Name: engine_scope_bindings engine_scope_bindings_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER engine_scope_bindings_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON engine_scope_bindings FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: engine_scope_bindings engine_scope_bindings_validate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER engine_scope_bindings_validate BEFORE INSERT OR UPDATE OF scope_id, engine_id ON engine_scope_bindings FOR EACH ROW EXECUTE FUNCTION validate_engine_scope_binding();


--
-- Name: engines engines_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER engines_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON engines FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: group_members group_members_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER group_members_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON group_members FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: groups groups_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER groups_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON groups FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: inspection_policy_notification_rules inspection_policy_notification_rules_validate_scope; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER inspection_policy_notification_rules_validate_scope BEFORE INSERT OR UPDATE OF scope_id, rule_scope_id, rule_id ON inspection_policy_notification_rules FOR EACH ROW EXECUTE FUNCTION validate_policy_notification_rule_scope();


--
-- Name: notification_channels notification_channels_prevent_share_revocation; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER notification_channels_prevent_share_revocation BEFORE UPDATE OF share_with_children ON notification_channels FOR EACH ROW EXECUTE FUNCTION reject_notification_share_revocation();


--
-- Name: notification_deliveries notification_deliveries_route_immutable; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER notification_deliveries_route_immutable BEFORE UPDATE ON notification_deliveries FOR EACH ROW EXECUTE FUNCTION reject_notification_route_snapshot_mutation();


--
-- Name: notification_delivery_attempts notification_delivery_attempts_append_only; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER notification_delivery_attempts_append_only BEFORE DELETE OR UPDATE ON notification_delivery_attempts FOR EACH ROW EXECUTE FUNCTION reject_notification_attempt_mutation();


--
-- Name: notification_events notification_events_immutable; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER notification_events_immutable BEFORE DELETE OR UPDATE ON notification_events FOR EACH ROW EXECUTE FUNCTION reject_notification_event_mutation();


--
-- Name: notification_rule_routes notification_rule_routes_validate_child_sharing_insert; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER notification_rule_routes_validate_child_sharing_insert BEFORE INSERT ON notification_rule_routes FOR EACH ROW EXECUTE FUNCTION validate_notification_route_child_sharing();


--
-- Name: notification_rule_routes notification_rule_routes_validate_child_sharing_update; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER notification_rule_routes_validate_child_sharing_update BEFORE UPDATE OF scope_id, rule_id, channel_id, template_version_id ON notification_rule_routes FOR EACH ROW EXECUTE FUNCTION validate_notification_route_child_sharing();


--
-- Name: notification_rules notification_rules_prevent_share_revocation; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER notification_rules_prevent_share_revocation BEFORE UPDATE OF share_with_children ON notification_rules FOR EACH ROW EXECUTE FUNCTION reject_notification_share_revocation();


--
-- Name: notification_template_versions notification_template_versions_immutable; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER notification_template_versions_immutable BEFORE UPDATE ON notification_template_versions FOR EACH ROW EXECUTE FUNCTION prevent_published_notification_template_changes();


--
-- Name: notification_templates notification_templates_prevent_share_revocation; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER notification_templates_prevent_share_revocation BEFORE UPDATE OF share_with_children ON notification_templates FOR EACH ROW EXECUTE FUNCTION reject_notification_share_revocation();


--
-- Name: personas personas_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER personas_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON personas FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: platforms platforms_validate_scope; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER platforms_validate_scope BEFORE INSERT OR UPDATE OF scope_id ON platforms FOR EACH ROW EXECUTE FUNCTION validate_organization_scope();


--
-- Name: projects projects_validate_scope; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER projects_validate_scope BEFORE INSERT OR UPDATE OF scope_id, platform_id, team_id ON projects FOR EACH ROW EXECUTE FUNCTION validate_organization_scope();


--
-- Name: provider_scope_bindings provider_scope_bindings_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER provider_scope_bindings_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON provider_scope_bindings FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: provider_scope_bindings provider_scope_bindings_validate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER provider_scope_bindings_validate BEFORE INSERT OR UPDATE OF scope_id, provider_id ON provider_scope_bindings FOR EACH ROW EXECUTE FUNCTION validate_provider_scope_binding();


--
-- Name: providers providers_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER providers_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON providers FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: resource_relations resource_relations_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER resource_relations_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON resource_relations FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: resource_relations resource_relations_validate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER resource_relations_validate BEFORE INSERT OR UPDATE OF source_resource_id, target_resource_id ON resource_relations FOR EACH ROW EXECUTE FUNCTION validate_resource_relation();


--
-- Name: resource_role_bindings resource_role_bindings_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER resource_role_bindings_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON resource_role_bindings FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: resource_role_bindings resource_role_bindings_validate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER resource_role_bindings_validate BEFORE INSERT OR UPDATE OF subject_type, subject_id ON resource_role_bindings FOR EACH ROW EXECUTE FUNCTION validate_resource_role_binding();


--
-- Name: resource_role_permissions resource_role_permissions_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER resource_role_permissions_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON resource_role_permissions FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: resource_roles resource_roles_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER resource_roles_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON resource_roles FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: resources resources_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER resources_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON resources FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: resources resources_validate_scope_move; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER resources_validate_scope_move BEFORE UPDATE OF scope_id ON resources FOR EACH ROW EXECUTE FUNCTION validate_resource_scope_move();


--
-- Name: role_bindings role_bindings_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER role_bindings_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON role_bindings FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: role_bindings role_bindings_validate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER role_bindings_validate BEFORE INSERT OR UPDATE OF subject_id, role_id, scope_id ON role_bindings FOR EACH ROW EXECUTE FUNCTION validate_role_binding();


--
-- Name: role_permissions role_permissions_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER role_permissions_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON role_permissions FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: roles roles_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER roles_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON roles FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: scope_ai_provider_bindings scope_ai_provider_bindings_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER scope_ai_provider_bindings_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON scope_ai_provider_bindings FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: scope_ai_provider_bindings scope_ai_provider_bindings_updated_at; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER scope_ai_provider_bindings_updated_at BEFORE UPDATE ON scope_ai_provider_bindings FOR EACH ROW EXECUTE FUNCTION scope_ai_provider_bindings_touch();


--
-- Name: scope_ai_provider_bindings scope_ai_provider_bindings_validate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER scope_ai_provider_bindings_validate BEFORE INSERT OR UPDATE OF scope_id, provider_resource_id ON scope_ai_provider_bindings FOR EACH ROW EXECUTE FUNCTION validate_scope_ai_provider_binding();


--
-- Name: scope_defaults scope_defaults_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER scope_defaults_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON scope_defaults FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: scope_defaults scope_defaults_validate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER scope_defaults_validate BEFORE INSERT OR UPDATE OF scope_id, resource_id ON scope_defaults FOR EACH ROW EXECUTE FUNCTION validate_scope_default();


--
-- Name: scopes scopes_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER scopes_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON scopes FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: scopes scopes_validate_parent; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER scopes_validate_parent BEFORE INSERT OR UPDATE OF tenant_id, scope_type, parent_scope_id ON scopes FOR EACH ROW EXECUTE FUNCTION validate_scope_parent();


--
-- Name: skill_scope_defaults skill_scope_defaults_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER skill_scope_defaults_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON skill_scope_defaults FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: skill_scope_defaults skill_scope_defaults_validate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER skill_scope_defaults_validate BEFORE INSERT OR UPDATE ON skill_scope_defaults FOR EACH ROW EXECUTE FUNCTION validate_skill_scope_default();


--
-- Name: skill_versions skill_versions_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER skill_versions_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON skill_versions FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: skill_versions skill_versions_validate; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER skill_versions_validate BEFORE INSERT OR UPDATE ON skill_versions FOR EACH ROW EXECUTE FUNCTION validate_skill_version();


--
-- Name: skills skills_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER skills_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON skills FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: teams teams_validate_scope; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER teams_validate_scope BEFORE INSERT OR UPDATE OF scope_id, platform_id ON teams FOR EACH ROW EXECUTE FUNCTION validate_organization_scope();


--
-- Name: users users_authorization_revision; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER users_authorization_revision AFTER INSERT OR DELETE OR UPDATE ON users FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();


--
-- Name: persona_versions agent_profile_versions_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY persona_versions
    ADD CONSTRAINT agent_profile_versions_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;


--
-- Name: ai_workflow_runs ai_workflow_runs_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_workflow_runs
    ADD CONSTRAINT ai_workflow_runs_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;


--
-- Name: ai_workflow_runs ai_workflow_runs_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_workflow_runs
    ADD CONSTRAINT ai_workflow_runs_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id) ON DELETE CASCADE;


--
-- Name: ai_workflow_runs ai_workflow_runs_workflow_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY ai_workflow_runs
    ADD CONSTRAINT ai_workflow_runs_workflow_resource_id_fkey FOREIGN KEY (workflow_resource_id) REFERENCES resources(id) ON DELETE CASCADE;


--
-- Name: application_dependencies application_dependencies_application_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY application_dependencies
    ADD CONSTRAINT application_dependencies_application_id_fkey FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE CASCADE;


--
-- Name: application_dependencies application_dependencies_target_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY application_dependencies
    ADD CONSTRAINT application_dependencies_target_resource_id_fkey FOREIGN KEY (target_resource_id) REFERENCES resources(id);


--
-- Name: application_instances application_instances_application_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY application_instances
    ADD CONSTRAINT application_instances_application_id_fkey FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE CASCADE;


--
-- Name: application_instances application_instances_target_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY application_instances
    ADD CONSTRAINT application_instances_target_resource_id_fkey FOREIGN KEY (target_resource_id) REFERENCES resources(id);


--
-- Name: applications applications_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY applications
    ADD CONSTRAINT applications_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id);


--
-- Name: audit_events audit_events_actor_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY audit_events
    ADD CONSTRAINT audit_events_actor_user_id_fkey FOREIGN KEY (actor_user_id) REFERENCES users(id) ON DELETE SET NULL;


--
-- Name: audit_events audit_events_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY audit_events
    ADD CONSTRAINT audit_events_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id) ON DELETE SET NULL;


--
-- Name: diagnosis_causal_chains diagnosis_causal_chains_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_causal_chains
    ADD CONSTRAINT diagnosis_causal_chains_run_id_fkey FOREIGN KEY (run_id) REFERENCES diagnosis_runs(id) ON DELETE CASCADE;


--
-- Name: diagnosis_causal_chains diagnosis_causal_chains_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_causal_chains
    ADD CONSTRAINT diagnosis_causal_chains_session_id_fkey FOREIGN KEY (session_id) REFERENCES diagnosis_sessions(id) ON DELETE CASCADE;


--
-- Name: diagnosis_events diagnosis_events_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_events
    ADD CONSTRAINT diagnosis_events_session_id_fkey FOREIGN KEY (session_id) REFERENCES diagnosis_sessions(id) ON DELETE CASCADE;


--
-- Name: diagnosis_evidence diagnosis_evidence_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_evidence
    ADD CONSTRAINT diagnosis_evidence_run_id_fkey FOREIGN KEY (run_id) REFERENCES diagnosis_runs(id) ON DELETE SET NULL;


--
-- Name: diagnosis_evidence diagnosis_evidence_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_evidence
    ADD CONSTRAINT diagnosis_evidence_session_id_fkey FOREIGN KEY (session_id) REFERENCES diagnosis_sessions(id) ON DELETE CASCADE;


--
-- Name: diagnosis_evidence diagnosis_evidence_source_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_evidence
    ADD CONSTRAINT diagnosis_evidence_source_resource_id_fkey FOREIGN KEY (source_resource_id) REFERENCES resources(id) ON DELETE RESTRICT;


--
-- Name: diagnosis_evidence diagnosis_evidence_target_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_evidence
    ADD CONSTRAINT diagnosis_evidence_target_resource_id_fkey FOREIGN KEY (target_resource_id) REFERENCES resources(id) ON DELETE RESTRICT;


--
-- Name: diagnosis_hypotheses diagnosis_hypotheses_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_hypotheses
    ADD CONSTRAINT diagnosis_hypotheses_session_id_fkey FOREIGN KEY (session_id) REFERENCES diagnosis_sessions(id) ON DELETE CASCADE;


--
-- Name: diagnosis_messages diagnosis_messages_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_messages
    ADD CONSTRAINT diagnosis_messages_session_id_fkey FOREIGN KEY (session_id) REFERENCES diagnosis_sessions(id) ON DELETE CASCADE;


--
-- Name: diagnosis_plan_steps diagnosis_plan_steps_plan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_plan_steps
    ADD CONSTRAINT diagnosis_plan_steps_plan_id_fkey FOREIGN KEY (plan_id) REFERENCES diagnosis_plans(id) ON DELETE CASCADE;


--
-- Name: diagnosis_plans diagnosis_plans_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_plans
    ADD CONSTRAINT diagnosis_plans_session_id_fkey FOREIGN KEY (session_id) REFERENCES diagnosis_sessions(id) ON DELETE CASCADE;


--
-- Name: diagnosis_reports diagnosis_reports_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_reports
    ADD CONSTRAINT diagnosis_reports_session_id_fkey FOREIGN KEY (session_id) REFERENCES diagnosis_sessions(id) ON DELETE CASCADE;


--
-- Name: diagnosis_runs diagnosis_runs_question_message_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_runs
    ADD CONSTRAINT diagnosis_runs_question_message_id_fkey FOREIGN KEY (question_message_id) REFERENCES diagnosis_messages(id) ON DELETE SET NULL;


--
-- Name: diagnosis_runs diagnosis_runs_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_runs
    ADD CONSTRAINT diagnosis_runs_session_id_fkey FOREIGN KEY (session_id) REFERENCES diagnosis_sessions(id) ON DELETE CASCADE;


--
-- Name: diagnosis_sessions diagnosis_sessions_actor_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_sessions
    ADD CONSTRAINT diagnosis_sessions_actor_user_id_fkey FOREIGN KEY (actor_user_id) REFERENCES users(id) ON DELETE SET NULL;


--
-- Name: diagnosis_sessions diagnosis_sessions_application_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_sessions
    ADD CONSTRAINT diagnosis_sessions_application_id_fkey FOREIGN KEY (application_id) REFERENCES applications(id) ON DELETE SET NULL;


--
-- Name: diagnosis_sessions diagnosis_sessions_provider_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_sessions
    ADD CONSTRAINT diagnosis_sessions_provider_id_fkey FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE SET NULL;


--
-- Name: diagnosis_sessions diagnosis_sessions_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_sessions
    ADD CONSTRAINT diagnosis_sessions_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id) ON DELETE RESTRICT;


--
-- Name: diagnosis_targets diagnosis_targets_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_targets
    ADD CONSTRAINT diagnosis_targets_resource_id_fkey FOREIGN KEY (resource_id) REFERENCES resources(id) ON DELETE RESTRICT;


--
-- Name: diagnosis_targets diagnosis_targets_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY diagnosis_targets
    ADD CONSTRAINT diagnosis_targets_session_id_fkey FOREIGN KEY (session_id) REFERENCES diagnosis_sessions(id) ON DELETE CASCADE;


--
-- Name: engine_capability_terms engine_capability_terms_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY engine_capability_terms
    ADD CONSTRAINT engine_capability_terms_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;


--
-- Name: engine_scope_bindings engine_scope_bindings_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY engine_scope_bindings
    ADD CONSTRAINT engine_scope_bindings_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;


--
-- Name: engine_scope_bindings engine_scope_bindings_engine_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY engine_scope_bindings
    ADD CONSTRAINT engine_scope_bindings_engine_id_fkey FOREIGN KEY (engine_id) REFERENCES engines(id) ON DELETE CASCADE;


--
-- Name: engine_scope_bindings engine_scope_bindings_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY engine_scope_bindings
    ADD CONSTRAINT engine_scope_bindings_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id) ON DELETE CASCADE;


--
-- Name: engines engines_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY engines
    ADD CONSTRAINT engines_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id) ON DELETE CASCADE;


--
-- Name: group_members group_members_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY group_members
    ADD CONSTRAINT group_members_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;


--
-- Name: group_members group_members_group_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY group_members
    ADD CONSTRAINT group_members_group_id_fkey FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE;


--
-- Name: group_members group_members_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY group_members
    ADD CONSTRAINT group_members_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;


--
-- Name: groups groups_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY groups
    ADD CONSTRAINT groups_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: inspection_findings inspection_findings_last_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_findings
    ADD CONSTRAINT inspection_findings_last_run_id_fkey FOREIGN KEY (last_run_id) REFERENCES inspection_runs(id) ON DELETE SET NULL;


--
-- Name: inspection_findings inspection_findings_policy_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_findings
    ADD CONSTRAINT inspection_findings_policy_id_fkey FOREIGN KEY (policy_id) REFERENCES inspection_policies(id) ON DELETE RESTRICT;


--
-- Name: inspection_findings inspection_findings_scope_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_findings
    ADD CONSTRAINT inspection_findings_scope_fk FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: inspection_findings inspection_findings_target_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_findings
    ADD CONSTRAINT inspection_findings_target_resource_id_fkey FOREIGN KEY (target_resource_id) REFERENCES resources(id) ON DELETE RESTRICT;


--
-- Name: inspection_findings inspection_findings_target_scope_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_findings
    ADD CONSTRAINT inspection_findings_target_scope_fk FOREIGN KEY (target_resource_id, scope_id) REFERENCES resources(id, scope_id) ON DELETE RESTRICT;


--
-- Name: inspection_health_snapshots inspection_health_snapshots_policy_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_health_snapshots
    ADD CONSTRAINT inspection_health_snapshots_policy_id_fkey FOREIGN KEY (policy_id) REFERENCES inspection_policies(id) ON DELETE RESTRICT;


--
-- Name: inspection_health_snapshots inspection_health_snapshots_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_health_snapshots
    ADD CONSTRAINT inspection_health_snapshots_run_id_fkey FOREIGN KEY (run_id) REFERENCES inspection_runs(id) ON DELETE CASCADE;


--
-- Name: inspection_health_snapshots inspection_health_snapshots_target_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_health_snapshots
    ADD CONSTRAINT inspection_health_snapshots_target_resource_id_fkey FOREIGN KEY (target_resource_id) REFERENCES resources(id) ON DELETE RESTRICT;


--
-- Name: inspection_jobs inspection_jobs_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_jobs
    ADD CONSTRAINT inspection_jobs_run_id_fkey FOREIGN KEY (run_id) REFERENCES inspection_runs(id) ON DELETE CASCADE;


--
-- Name: inspection_policies inspection_policies_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_policies
    ADD CONSTRAINT inspection_policies_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;


--
-- Name: inspection_policies inspection_policies_persona_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_policies
    ADD CONSTRAINT inspection_policies_persona_id_fkey FOREIGN KEY (persona_id) REFERENCES personas(id) ON DELETE SET NULL;


--
-- Name: inspection_policies inspection_policies_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_policies
    ADD CONSTRAINT inspection_policies_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: inspection_policy_notification_rules inspection_policy_notification_rules_policy_id_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_policy_notification_rules
    ADD CONSTRAINT inspection_policy_notification_rules_policy_id_scope_id_fkey FOREIGN KEY (policy_id, scope_id) REFERENCES inspection_policies(id, scope_id) ON DELETE CASCADE;


--
-- Name: inspection_policy_notification_rules inspection_policy_notification_rules_rule_owner_scope_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_policy_notification_rules
    ADD CONSTRAINT inspection_policy_notification_rules_rule_owner_scope_fk FOREIGN KEY (rule_id, rule_scope_id) REFERENCES notification_rules(id, scope_id) ON DELETE CASCADE;


--
-- Name: inspection_policy_notification_rules inspection_policy_notification_rules_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_policy_notification_rules
    ADD CONSTRAINT inspection_policy_notification_rules_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: inspection_policy_targets inspection_policy_targets_policy_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_policy_targets
    ADD CONSTRAINT inspection_policy_targets_policy_id_fkey FOREIGN KEY (policy_id) REFERENCES inspection_policies(id) ON DELETE CASCADE;


--
-- Name: inspection_policy_targets inspection_policy_targets_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_policy_targets
    ADD CONSTRAINT inspection_policy_targets_resource_id_fkey FOREIGN KEY (resource_id) REFERENCES resources(id) ON DELETE RESTRICT;


--
-- Name: inspection_run_steps inspection_run_steps_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_run_steps
    ADD CONSTRAINT inspection_run_steps_run_id_fkey FOREIGN KEY (run_id) REFERENCES inspection_runs(id) ON DELETE CASCADE;


--
-- Name: inspection_runs inspection_runs_policy_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_runs
    ADD CONSTRAINT inspection_runs_policy_id_fkey FOREIGN KEY (policy_id) REFERENCES inspection_policies(id) ON DELETE RESTRICT;


--
-- Name: inspection_runs inspection_runs_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY inspection_runs
    ADD CONSTRAINT inspection_runs_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: mcp_server_snapshots mcp_server_snapshots_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY mcp_server_snapshots
    ADD CONSTRAINT mcp_server_snapshots_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: mcp_server_snapshots mcp_server_snapshots_server_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY mcp_server_snapshots
    ADD CONSTRAINT mcp_server_snapshots_server_resource_id_fkey FOREIGN KEY (server_resource_id) REFERENCES resources(id) ON DELETE RESTRICT;


--
-- Name: notification_channel_test_limits notification_channel_test_limits_channel_id_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_channel_test_limits
    ADD CONSTRAINT notification_channel_test_limits_channel_id_scope_id_fkey FOREIGN KEY (channel_id, scope_id) REFERENCES notification_channels(id, scope_id) ON DELETE CASCADE;


--
-- Name: notification_channel_test_limits notification_channel_test_limits_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_channel_test_limits
    ADD CONSTRAINT notification_channel_test_limits_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: notification_channel_versions notification_channel_versions_channel_id_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_channel_versions
    ADD CONSTRAINT notification_channel_versions_channel_id_scope_id_fkey FOREIGN KEY (channel_id, scope_id) REFERENCES notification_channels(id, scope_id) ON DELETE RESTRICT;


--
-- Name: notification_channel_versions notification_channel_versions_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_channel_versions
    ADD CONSTRAINT notification_channel_versions_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: notification_channels notification_channels_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_channels
    ADD CONSTRAINT notification_channels_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: notification_deliveries notification_deliveries_channel_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_deliveries
    ADD CONSTRAINT notification_deliveries_channel_id_fkey FOREIGN KEY (channel_id) REFERENCES notification_channels(id) ON DELETE RESTRICT;


--
-- Name: notification_deliveries notification_deliveries_channel_scope_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_deliveries
    ADD CONSTRAINT notification_deliveries_channel_scope_fk FOREIGN KEY (channel_id, rule_scope_id) REFERENCES notification_channels(id, scope_id) ON DELETE RESTRICT;


--
-- Name: notification_deliveries notification_deliveries_channel_version_scope_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_deliveries
    ADD CONSTRAINT notification_deliveries_channel_version_scope_fk FOREIGN KEY (channel_version_id, rule_scope_id) REFERENCES notification_channel_versions(id, scope_id) ON DELETE RESTRICT;


--
-- Name: notification_deliveries notification_deliveries_event_policy_scope_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_deliveries
    ADD CONSTRAINT notification_deliveries_event_policy_scope_fk FOREIGN KEY (event_id, scope_id, policy_id) REFERENCES notification_events(id, scope_id, policy_id) ON DELETE RESTRICT;


--
-- Name: notification_deliveries notification_deliveries_event_scope_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_deliveries
    ADD CONSTRAINT notification_deliveries_event_scope_fk FOREIGN KEY (event_id, scope_id) REFERENCES notification_events(id, scope_id) ON DELETE RESTRICT;


--
-- Name: notification_deliveries notification_deliveries_finding_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_deliveries
    ADD CONSTRAINT notification_deliveries_finding_id_fkey FOREIGN KEY (finding_id) REFERENCES inspection_findings(id) ON DELETE SET NULL;


--
-- Name: notification_deliveries notification_deliveries_policy_rule_scope_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_deliveries
    ADD CONSTRAINT notification_deliveries_policy_rule_scope_fk FOREIGN KEY (policy_id, rule_id, scope_id) REFERENCES inspection_policy_notification_rules(policy_id, rule_id, scope_id) ON DELETE RESTRICT;


--
-- Name: notification_deliveries notification_deliveries_route_scope_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_deliveries
    ADD CONSTRAINT notification_deliveries_route_scope_fk FOREIGN KEY (route_id, rule_scope_id, rule_id, channel_id, channel_version_id, template_version_id) REFERENCES notification_rule_routes(id, scope_id, rule_id, channel_id, channel_version_id, template_version_id) ON DELETE RESTRICT;


--
-- Name: notification_deliveries notification_deliveries_rule_scope_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_deliveries
    ADD CONSTRAINT notification_deliveries_rule_scope_fk FOREIGN KEY (rule_id, rule_scope_id) REFERENCES notification_rules(id, scope_id) ON DELETE RESTRICT;


--
-- Name: notification_deliveries notification_deliveries_run_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_deliveries
    ADD CONSTRAINT notification_deliveries_run_id_fkey FOREIGN KEY (run_id) REFERENCES inspection_runs(id) ON DELETE SET NULL;


--
-- Name: notification_deliveries notification_deliveries_scope_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_deliveries
    ADD CONSTRAINT notification_deliveries_scope_fk FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: notification_deliveries notification_deliveries_template_version_scope_fk; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_deliveries
    ADD CONSTRAINT notification_deliveries_template_version_scope_fk FOREIGN KEY (template_version_id, rule_scope_id) REFERENCES notification_template_versions(id, scope_id) ON DELETE RESTRICT;


--
-- Name: notification_delivery_attempts notification_delivery_attempts_delivery_id_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_delivery_attempts
    ADD CONSTRAINT notification_delivery_attempts_delivery_id_scope_id_fkey FOREIGN KEY (delivery_id, scope_id) REFERENCES notification_deliveries(id, scope_id) ON DELETE RESTRICT;


--
-- Name: notification_delivery_attempts notification_delivery_attempts_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_delivery_attempts
    ADD CONSTRAINT notification_delivery_attempts_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: notification_events notification_events_finding_id_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_events
    ADD CONSTRAINT notification_events_finding_id_scope_id_fkey FOREIGN KEY (finding_id, scope_id) REFERENCES inspection_findings(id, scope_id) ON DELETE RESTRICT;


--
-- Name: notification_events notification_events_policy_id_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_events
    ADD CONSTRAINT notification_events_policy_id_scope_id_fkey FOREIGN KEY (policy_id, scope_id) REFERENCES inspection_policies(id, scope_id) ON DELETE RESTRICT;


--
-- Name: notification_events notification_events_run_id_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_events
    ADD CONSTRAINT notification_events_run_id_scope_id_fkey FOREIGN KEY (run_id, scope_id) REFERENCES inspection_runs(id, scope_id) ON DELETE RESTRICT;


--
-- Name: notification_events notification_events_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_events
    ADD CONSTRAINT notification_events_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: notification_events notification_events_target_resource_id_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_events
    ADD CONSTRAINT notification_events_target_resource_id_scope_id_fkey FOREIGN KEY (target_resource_id, scope_id) REFERENCES resources(id, scope_id) ON DELETE RESTRICT;


--
-- Name: notification_rule_cooldowns notification_rule_cooldowns_rule_id_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_rule_cooldowns
    ADD CONSTRAINT notification_rule_cooldowns_rule_id_scope_id_fkey FOREIGN KEY (rule_id, scope_id) REFERENCES notification_rules(id, scope_id) ON DELETE CASCADE;


--
-- Name: notification_rule_cooldowns notification_rule_cooldowns_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_rule_cooldowns
    ADD CONSTRAINT notification_rule_cooldowns_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: notification_rule_routes notification_rule_routes_channel_id_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_rule_routes
    ADD CONSTRAINT notification_rule_routes_channel_id_scope_id_fkey FOREIGN KEY (channel_id, scope_id) REFERENCES notification_channels(id, scope_id) ON DELETE RESTRICT;


--
-- Name: notification_rule_routes notification_rule_routes_channel_version_id_scope_id_chann_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_rule_routes
    ADD CONSTRAINT notification_rule_routes_channel_version_id_scope_id_chann_fkey FOREIGN KEY (channel_version_id, scope_id, channel_id) REFERENCES notification_channel_versions(id, scope_id, channel_id) ON DELETE RESTRICT;


--
-- Name: notification_rule_routes notification_rule_routes_rule_id_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_rule_routes
    ADD CONSTRAINT notification_rule_routes_rule_id_scope_id_fkey FOREIGN KEY (rule_id, scope_id) REFERENCES notification_rules(id, scope_id) ON DELETE CASCADE;


--
-- Name: notification_rule_routes notification_rule_routes_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_rule_routes
    ADD CONSTRAINT notification_rule_routes_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: notification_rule_routes notification_rule_routes_template_version_id_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_rule_routes
    ADD CONSTRAINT notification_rule_routes_template_version_id_scope_id_fkey FOREIGN KEY (template_version_id, scope_id) REFERENCES notification_template_versions(id, scope_id) ON DELETE RESTRICT;


--
-- Name: notification_rules notification_rules_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_rules
    ADD CONSTRAINT notification_rules_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;


--
-- Name: notification_rules notification_rules_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_rules
    ADD CONSTRAINT notification_rules_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: notification_template_versions notification_template_versions_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_template_versions
    ADD CONSTRAINT notification_template_versions_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;


--
-- Name: notification_template_versions notification_template_versions_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_template_versions
    ADD CONSTRAINT notification_template_versions_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: notification_template_versions notification_template_versions_template_id_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_template_versions
    ADD CONSTRAINT notification_template_versions_template_id_scope_id_fkey FOREIGN KEY (template_id, scope_id) REFERENCES notification_templates(id, scope_id) ON DELETE RESTRICT;


--
-- Name: notification_templates notification_templates_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_templates
    ADD CONSTRAINT notification_templates_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;


--
-- Name: notification_templates notification_templates_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY notification_templates
    ADD CONSTRAINT notification_templates_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: persona_versions persona_versions_persona_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY persona_versions
    ADD CONSTRAINT persona_versions_persona_id_fkey FOREIGN KEY (persona_id) REFERENCES personas(id) ON DELETE RESTRICT;


--
-- Name: personas personas_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY personas
    ADD CONSTRAINT personas_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: platforms platforms_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY platforms
    ADD CONSTRAINT platforms_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: projects projects_platform_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY projects
    ADD CONSTRAINT projects_platform_id_fkey FOREIGN KEY (platform_id) REFERENCES platforms(id);


--
-- Name: projects projects_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY projects
    ADD CONSTRAINT projects_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: projects projects_team_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY projects
    ADD CONSTRAINT projects_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id);


--
-- Name: provider_scope_bindings provider_scope_bindings_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY provider_scope_bindings
    ADD CONSTRAINT provider_scope_bindings_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;


--
-- Name: provider_scope_bindings provider_scope_bindings_provider_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY provider_scope_bindings
    ADD CONSTRAINT provider_scope_bindings_provider_id_fkey FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE;


--
-- Name: provider_scope_bindings provider_scope_bindings_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY provider_scope_bindings
    ADD CONSTRAINT provider_scope_bindings_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id) ON DELETE CASCADE;


--
-- Name: providers providers_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY providers
    ADD CONSTRAINT providers_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: repository_bundles repository_bundles_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY repository_bundles
    ADD CONSTRAINT repository_bundles_resource_id_fkey FOREIGN KEY (resource_id) REFERENCES resources(id) ON DELETE CASCADE;


--
-- Name: resource_connection_checks resource_connection_checks_checked_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_connection_checks
    ADD CONSTRAINT resource_connection_checks_checked_by_fkey FOREIGN KEY (checked_by) REFERENCES users(id) ON DELETE SET NULL;


--
-- Name: resource_connection_checks resource_connection_checks_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_connection_checks
    ADD CONSTRAINT resource_connection_checks_resource_id_fkey FOREIGN KEY (resource_id) REFERENCES resources(id) ON DELETE CASCADE;


--
-- Name: resource_relations resource_relations_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_relations
    ADD CONSTRAINT resource_relations_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;


--
-- Name: resource_relations resource_relations_source_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_relations
    ADD CONSTRAINT resource_relations_source_resource_id_fkey FOREIGN KEY (source_resource_id) REFERENCES resources(id) ON DELETE CASCADE;


--
-- Name: resource_relations resource_relations_target_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_relations
    ADD CONSTRAINT resource_relations_target_resource_id_fkey FOREIGN KEY (target_resource_id) REFERENCES resources(id) ON DELETE CASCADE;


--
-- Name: resource_role_bindings resource_role_bindings_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_role_bindings
    ADD CONSTRAINT resource_role_bindings_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;


--
-- Name: resource_role_bindings resource_role_bindings_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_role_bindings
    ADD CONSTRAINT resource_role_bindings_resource_id_fkey FOREIGN KEY (resource_id) REFERENCES resources(id) ON DELETE CASCADE;


--
-- Name: resource_role_bindings resource_role_bindings_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_role_bindings
    ADD CONSTRAINT resource_role_bindings_role_id_fkey FOREIGN KEY (role_id) REFERENCES resource_roles(id) ON DELETE CASCADE;


--
-- Name: resource_role_permissions resource_role_permissions_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_role_permissions
    ADD CONSTRAINT resource_role_permissions_role_id_fkey FOREIGN KEY (role_id) REFERENCES resource_roles(id) ON DELETE CASCADE;


--
-- Name: resource_sync_states resource_sync_states_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resource_sync_states
    ADD CONSTRAINT resource_sync_states_resource_id_fkey FOREIGN KEY (resource_id) REFERENCES resources(id) ON DELETE CASCADE;


--
-- Name: resources resources_agent_ref_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resources
    ADD CONSTRAINT resources_agent_ref_fkey FOREIGN KEY (agent_ref) REFERENCES resources(id) ON DELETE RESTRICT;


--
-- Name: resources resources_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY resources
    ADD CONSTRAINT resources_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: role_bindings role_bindings_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY role_bindings
    ADD CONSTRAINT role_bindings_role_id_fkey FOREIGN KEY (role_id) REFERENCES roles(id);


--
-- Name: role_bindings role_bindings_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY role_bindings
    ADD CONSTRAINT role_bindings_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: role_permissions role_permissions_role_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY role_permissions
    ADD CONSTRAINT role_permissions_role_id_fkey FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE;


--
-- Name: scope_ai_provider_bindings scope_ai_provider_bindings_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY scope_ai_provider_bindings
    ADD CONSTRAINT scope_ai_provider_bindings_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;


--
-- Name: scope_ai_provider_bindings scope_ai_provider_bindings_provider_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY scope_ai_provider_bindings
    ADD CONSTRAINT scope_ai_provider_bindings_provider_resource_id_fkey FOREIGN KEY (provider_resource_id) REFERENCES resources(id) ON DELETE CASCADE;


--
-- Name: scope_ai_provider_bindings scope_ai_provider_bindings_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY scope_ai_provider_bindings
    ADD CONSTRAINT scope_ai_provider_bindings_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id) ON DELETE CASCADE;


--
-- Name: scope_defaults scope_defaults_resource_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY scope_defaults
    ADD CONSTRAINT scope_defaults_resource_id_fkey FOREIGN KEY (resource_id) REFERENCES resources(id) ON DELETE RESTRICT;


--
-- Name: scope_defaults scope_defaults_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY scope_defaults
    ADD CONSTRAINT scope_defaults_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id) ON DELETE CASCADE;


--
-- Name: scopes scopes_parent_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY scopes
    ADD CONSTRAINT scopes_parent_scope_id_fkey FOREIGN KEY (parent_scope_id) REFERENCES scopes(id);


--
-- Name: sessions sessions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY sessions
    ADD CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id);


--
-- Name: skill_scope_defaults skill_scope_defaults_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY skill_scope_defaults
    ADD CONSTRAINT skill_scope_defaults_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;


--
-- Name: skill_scope_defaults skill_scope_defaults_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY skill_scope_defaults
    ADD CONSTRAINT skill_scope_defaults_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id) ON DELETE CASCADE;


--
-- Name: skill_scope_defaults skill_scope_defaults_skill_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY skill_scope_defaults
    ADD CONSTRAINT skill_scope_defaults_skill_id_fkey FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE RESTRICT;


--
-- Name: skill_scope_defaults skill_scope_defaults_skill_version_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY skill_scope_defaults
    ADD CONSTRAINT skill_scope_defaults_skill_version_id_fkey FOREIGN KEY (skill_version_id) REFERENCES skill_versions(id) ON DELETE RESTRICT;


--
-- Name: skill_versions skill_versions_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY skill_versions
    ADD CONSTRAINT skill_versions_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;


--
-- Name: skill_versions skill_versions_skill_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY skill_versions
    ADD CONSTRAINT skill_versions_skill_id_fkey FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE RESTRICT;


--
-- Name: skills skills_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY skills
    ADD CONSTRAINT skills_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- Name: teams teams_platform_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY teams
    ADD CONSTRAINT teams_platform_id_fkey FOREIGN KEY (platform_id) REFERENCES platforms(id);


--
-- Name: teams teams_scope_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY teams
    ADD CONSTRAINT teams_scope_id_fkey FOREIGN KEY (scope_id) REFERENCES scopes(id);


--
-- PostgreSQL database dump complete
--

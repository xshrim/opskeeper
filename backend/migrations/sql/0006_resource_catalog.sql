CREATE TABLE resource_schemas (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind text NOT NULL CHECK (length(btrim(kind)) BETWEEN 1 AND 120),
    version integer NOT NULL CHECK (version > 0),
    schema jsonb NOT NULL CHECK (jsonb_typeof(schema) = 'object'),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (kind, version)
);

CREATE INDEX resource_schemas_kind_idx ON resource_schemas(kind, version DESC) WHERE status = 'active';

CREATE TABLE resource_credentials (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    purpose text NOT NULL DEFAULT '' CHECK (length(purpose) <= 500),
    ciphertext bytea NOT NULL CHECK (octet_length(ciphertext) > 0),
    encryption_algorithm text NOT NULL DEFAULT 'AES-256-GCM',
    key_version text NOT NULL DEFAULT 'local-v1',
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX resource_credentials_scope_name_unique
    ON resource_credentials(scope_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX resource_credentials_scope_idx
    ON resource_credentials(scope_id) WHERE deleted_at IS NULL;

CREATE OR REPLACE FUNCTION validate_resource_credential_record_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    scope_status text;
BEGIN
    SELECT status INTO scope_status FROM scopes WHERE id = NEW.scope_id AND deleted_at IS NULL;
    IF scope_status IS DISTINCT FROM 'active' THEN
        RAISE EXCEPTION 'credential scope is inactive or missing' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER resource_credentials_validate_scope
BEFORE INSERT OR UPDATE OF scope_id ON resource_credentials
FOR EACH ROW EXECUTE FUNCTION validate_resource_credential_record_scope();

CREATE TABLE resources (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id text NOT NULL DEFAULT 'default',
    scope_id uuid NOT NULL REFERENCES scopes(id),
    kind text NOT NULL CHECK (length(btrim(kind)) BETWEEN 1 AND 120),
    schema_version integer NOT NULL DEFAULT 1 CHECK (schema_version > 0),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 200),
    external_uid text NOT NULL DEFAULT '' CHECK (length(external_uid) <= 512),
    source_resource_id text NOT NULL DEFAULT '' CHECK (length(source_resource_id) <= 512),
    labels jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(labels) = 'object'),
    config jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(config) = 'object'),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'unknown')),
    credential_id uuid REFERENCES resource_credentials(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX resources_scope_kind_name_unique
    ON resources(scope_id, kind, lower(name)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX resources_external_identity_unique
    ON resources(scope_id, kind, external_uid, source_resource_id)
    WHERE deleted_at IS NULL AND external_uid <> '' AND source_resource_id <> '';
CREATE INDEX resources_scope_idx ON resources(scope_id) WHERE deleted_at IS NULL;
CREATE INDEX resources_kind_idx ON resources(kind) WHERE deleted_at IS NULL;
CREATE INDEX resources_labels_idx ON resources USING gin(labels) WHERE deleted_at IS NULL;

CREATE TABLE resource_sync_states (
    resource_id uuid PRIMARY KEY REFERENCES resources(id) ON DELETE CASCADE,
    state text NOT NULL DEFAULT 'never' CHECK (state IN ('never', 'running', 'succeeded', 'failed')),
    last_started_at timestamptz,
    last_completed_at timestamptz,
    last_error text NOT NULL DEFAULT '' CHECK (length(last_error) <= 2000),
    external_version text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE resource_relations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    target_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    relation_type text NOT NULL CHECK (relation_type IN ('contains', 'deployed_on', 'depends_on', 'observed_by', 'exposes', 'uses_provider', 'uses_skill', 'served_by_mcp')),
    attributes jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(attributes) = 'object'),
    discovery_source text NOT NULL DEFAULT 'manual' CHECK (discovery_source IN ('manual', 'discovery', 'import')),
    confidence numeric(5,4) NOT NULL DEFAULT 1 CHECK (confidence >= 0 AND confidence <= 1),
    confirmed boolean NOT NULL DEFAULT true,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source_resource_id, target_resource_id, relation_type)
);

CREATE INDEX resource_relations_source_idx ON resource_relations(source_resource_id);
CREATE INDEX resource_relations_target_idx ON resource_relations(target_resource_id);

CREATE TABLE scope_defaults (
    scope_id uuid NOT NULL REFERENCES scopes(id) ON DELETE CASCADE,
    default_key text NOT NULL CHECK (length(btrim(default_key)) BETWEEN 1 AND 120),
    resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (scope_id, default_key)
);

CREATE OR REPLACE FUNCTION resource_scope_contains(ancestor_id uuid, descendant_id uuid)
RETURNS boolean
LANGUAGE sql
STABLE
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

CREATE OR REPLACE FUNCTION validate_resource_credential_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    credential_scope uuid;
    scope_status text;
BEGIN
    SELECT status INTO scope_status FROM scopes WHERE id = NEW.scope_id AND deleted_at IS NULL;
    IF scope_status IS DISTINCT FROM 'active' THEN
        RAISE EXCEPTION 'resource scope is inactive or missing' USING ERRCODE = '23514';
    END IF;
    IF NEW.credential_id IS NULL THEN
        RETURN NEW;
    END IF;
    SELECT scope_id INTO credential_scope
      FROM resource_credentials
     WHERE id = NEW.credential_id AND deleted_at IS NULL;
    IF credential_scope IS NULL OR NOT resource_scope_contains(credential_scope, NEW.scope_id) THEN
        RAISE EXCEPTION 'resource credential is outside resource scope' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER resources_validate_credential_scope
BEFORE INSERT OR UPDATE OF scope_id, credential_id ON resources
FOR EACH ROW EXECUTE FUNCTION validate_resource_credential_scope();

CREATE OR REPLACE FUNCTION validate_resource_relation()
RETURNS trigger
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

CREATE TRIGGER resource_relations_validate
BEFORE INSERT OR UPDATE OF source_resource_id, target_resource_id ON resource_relations
FOR EACH ROW EXECUTE FUNCTION validate_resource_relation();

CREATE OR REPLACE FUNCTION validate_resource_scope_move()
RETURNS trigger
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

CREATE TRIGGER resources_validate_scope_move
BEFORE UPDATE OF scope_id ON resources
FOR EACH ROW EXECUTE FUNCTION validate_resource_scope_move();

CREATE OR REPLACE FUNCTION validate_scope_default()
RETURNS trigger
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

CREATE TRIGGER scope_defaults_validate
BEFORE INSERT OR UPDATE OF scope_id, resource_id ON scope_defaults
FOR EACH ROW EXECUTE FUNCTION validate_scope_default();

INSERT INTO resource_schemas (kind, version, schema)
SELECT kind, 1, '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb
  FROM unnest(ARRAY[
      'KubernetesCluster', 'Namespace', 'Node', 'Workload', 'Pod', 'Service', 'Ingress',
      'BusinessApplication', 'Endpoint', 'CronApplication', 'PostgreSQL', 'Redis', 'Kafka',
      'Elasticsearch', 'GenericMiddleware', 'LLMProvider', 'Model', 'MCPServer', 'Skill',
      'Prometheus', 'Loki', 'Tempo', 'Jaeger', 'Elastic', 'Datadog', 'GenericAPI',
      'Credential', 'NotificationChannel', 'Runbook', 'ArtifactStore'
  ]) AS kinds(kind)
ON CONFLICT (kind, version) DO NOTHING;

CREATE TRIGGER resources_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON resources
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER resource_credentials_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON resource_credentials
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER resource_relations_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON resource_relations
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER scope_defaults_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON scope_defaults
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

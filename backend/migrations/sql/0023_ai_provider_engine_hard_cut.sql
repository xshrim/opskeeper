-- Hard cut to AIProvider + AIEngine. Legacy endpoint resources and default
-- tables are intentionally removed; this installation does not preserve old
-- AI catalog data.
DROP TABLE IF EXISTS ai_endpoint_scope_defaults CASCADE;
DROP TABLE IF EXISTS llm_scope_defaults CASCADE;

DO $$
DECLARE item record; ids uuid[];
BEGIN
  SELECT coalesce(array_agg(id), '{}'::uuid[]) INTO ids FROM resources WHERE kind = 'AIEndpoint';
  FOR item IN
    SELECT ns.nspname AS schema_name, cls.relname AS table_name, att.attname AS column_name
      FROM pg_constraint con
      JOIN pg_class cls ON cls.oid = con.conrelid
      JOIN pg_namespace ns ON ns.oid = cls.relnamespace
      JOIN pg_class ref ON ref.oid = con.confrelid
      JOIN LATERAL unnest(con.conkey) WITH ORDINALITY key(attnum, ord) ON true
      JOIN LATERAL unnest(con.confkey) WITH ORDINALITY refkey(attnum, ord) ON refkey.ord = key.ord
      JOIN pg_attribute att ON att.attrelid = cls.oid AND att.attnum = key.attnum
     WHERE con.contype = 'f' AND ref.oid = 'resources'::regclass
       AND cls.oid <> 'resources'::regclass AND array_length(con.conkey, 1) = 1
  LOOP
    EXECUTE format('DELETE FROM %I.%I WHERE %I = ANY($1)', item.schema_name, item.table_name, item.column_name) USING ids;
  END LOOP;
  DELETE FROM resources WHERE id = ANY(ids);
END $$;
DELETE FROM resource_schemas WHERE kind = 'AIEndpoint';
DELETE FROM role_permissions WHERE permission IN ('endpoint:manage', 'model:manage');
DELETE FROM resource_role_permissions WHERE permission IN ('endpoint:manage', 'model:manage');

ALTER TABLE diagnosis_sessions
    ADD COLUMN IF NOT EXISTS provider_resource_id uuid REFERENCES resources(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS model_name text;

UPDATE resource_schemas
SET display_name = '模型服务商',
    description = '模型服务连接、凭据和模型目录。',
    schema = '{
      "$schema":"https://json-schema.org/draft/2020-12/schema",
      "type":"object","additionalProperties":false,
      "required":["provider_type","base_url","models","enabled"],
      "properties":{
        "provider_type":{"type":"string"},"protocol":{"type":"string"},
        "base_url":{"type":"string","format":"uri"},"timeout_seconds":{"type":"integer"},
        "max_concurrency":{"type":"integer"},"rate_limit_per_minute":{"type":"integer"},
        "enabled":{"type":"boolean"},"default_model":{"type":"string"},
        "models":{"type":"array","minItems":1,"items":{"type":"object","additionalProperties":false,
          "required":["name","context_window_tokens","capabilities","enabled"],
          "properties":{"name":{"type":"string"},"context_window_tokens":{"type":"integer","minimum":1},
            "max_output_tokens":{"type":"integer"},"temperature":{"type":"number","minimum":0,"maximum":2},
            "temperature_mutable":{"type":"boolean"},"capabilities":{"type":"array","items":{"type":"string"}},
            "enabled":{"type":"boolean"},"priority":{"type":"integer"}}}}
      }}'::jsonb
WHERE kind = 'AIProvider';

CREATE TABLE IF NOT EXISTS scope_ai_provider_bindings (
    scope_id uuid NOT NULL REFERENCES scopes(id) ON DELETE CASCADE,
    provider_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    tag text NOT NULL CHECK (tag IN ('default', 'diagnosis', 'inspection', 'workflow')),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (scope_id, tag, provider_resource_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS scope_ai_provider_one_tagged_provider
    ON scope_ai_provider_bindings(scope_id, tag);
CREATE INDEX IF NOT EXISTS scope_ai_provider_bindings_provider_idx
    ON scope_ai_provider_bindings(provider_resource_id);

CREATE OR REPLACE FUNCTION validate_scope_ai_provider_binding()
RETURNS trigger LANGUAGE plpgsql AS $$
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
DROP TRIGGER IF EXISTS scope_ai_provider_bindings_validate ON scope_ai_provider_bindings;
CREATE TRIGGER scope_ai_provider_bindings_validate
BEFORE INSERT OR UPDATE OF scope_id, provider_resource_id ON scope_ai_provider_bindings
FOR EACH ROW EXECUTE FUNCTION validate_scope_ai_provider_binding();

CREATE OR REPLACE FUNCTION scope_ai_provider_bindings_touch()
RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN NEW.updated_at = now(); RETURN NEW; END $$;
DROP TRIGGER IF EXISTS scope_ai_provider_bindings_updated_at ON scope_ai_provider_bindings;
CREATE TRIGGER scope_ai_provider_bindings_updated_at
BEFORE UPDATE ON scope_ai_provider_bindings
FOR EACH ROW EXECUTE FUNCTION scope_ai_provider_bindings_touch();
DROP TRIGGER IF EXISTS scope_ai_provider_bindings_authorization_revision ON scope_ai_provider_bindings;
CREATE TRIGGER scope_ai_provider_bindings_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON scope_ai_provider_bindings
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

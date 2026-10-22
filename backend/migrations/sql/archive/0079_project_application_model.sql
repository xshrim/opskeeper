-- Projects contain business identity only. Runtime provenance belongs to the
-- applications and their bound instances.
DROP INDEX IF EXISTS projects_external_source_unique;
ALTER TABLE projects
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS source_resource_id,
    DROP COLUMN IF EXISTS external_uid,
    DROP COLUMN IF EXISTS source_config,
    DROP COLUMN IF EXISTS last_synced_at;

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS description text NOT NULL DEFAULT '';

UPDATE projects
   SET description = COALESCE(NULLIF(description, ''), labels->>'description', ''),
       labels = labels - 'description'
 WHERE labels ? 'description';

ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS runtime_kind text;

UPDATE applications
   SET runtime_kind = COALESCE(
       (SELECT i.runtime_kind
          FROM application_instances i
         WHERE i.application_id = applications.id
         ORDER BY i.created_at
         LIMIT 1),
       CASE WHEN source = 'kubernetes' THEN 'cloud_native' ELSE 'virtual_machine' END
   )
 WHERE runtime_kind IS NULL;

ALTER TABLE applications
    ALTER COLUMN status SET DEFAULT 'active',
    ALTER COLUMN runtime_kind SET DEFAULT 'virtual_machine',
    ALTER COLUMN runtime_kind SET NOT NULL,
    ADD CONSTRAINT applications_runtime_kind_check
      CHECK (runtime_kind IN ('virtual_machine', 'containerized', 'cloud_native'));

DROP TRIGGER IF EXISTS application_instances_validate_runtime ON application_instances;
DROP FUNCTION IF EXISTS validate_application_runtime();

CREATE FUNCTION validate_application_runtime() RETURNS trigger LANGUAGE plpgsql AS $$
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
  IF app_runtime = 'virtual_machine' AND COALESCE(NEW.selector->>'process_keyword', '') = '' THEN
    RAISE EXCEPTION 'Host instances require selector.process_keyword' USING ERRCODE = '23514';
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

CREATE TRIGGER application_instances_validate_runtime
BEFORE INSERT OR UPDATE OF application_id, target_resource_id, selector ON application_instances
FOR EACH ROW EXECUTE FUNCTION validate_application_runtime();

ALTER TABLE applications DROP COLUMN IF EXISTS source;

-- The old AI catalog is not retained. Remove every row that references an old
-- AI resource before deleting the resources themselves. The catalog is
-- referenced by several optional subsystems, so discover the foreign keys
-- rather than maintaining a fragile hand-written dependency list.
DO $$
DECLARE
  item record;
  ids uuid[];
BEGIN
  SELECT coalesce(array_agg(id), '{}'::uuid[]) INTO ids
    FROM resources WHERE kind IN ('AIProvider', 'AIEndpoint');
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
DELETE FROM resources
 WHERE kind IN ('AIProvider', 'AIEndpoint');

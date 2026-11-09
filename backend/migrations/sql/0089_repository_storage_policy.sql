UPDATE resource_schemas
SET schema = jsonb_set(
  jsonb_set(
    jsonb_set(
      jsonb_set(
        jsonb_set(
          schema,
          '{properties,storage_backend,readOnly}',
          'true'::jsonb,
          true
        ),
        '{properties,path,readOnly}',
        'true'::jsonb,
        true
      ),
      '{properties,s3_endpoint,readOnly}',
      'true'::jsonb,
      true
    ),
    '{properties,s3_bucket,readOnly}',
    'true'::jsonb,
    true
  ),
  '{properties,s3_prefix,readOnly}',
  'true'::jsonb,
  true
)
WHERE kind = 'Repository'
  AND schema ? 'properties';

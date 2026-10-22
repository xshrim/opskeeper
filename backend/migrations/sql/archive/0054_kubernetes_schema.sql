UPDATE resource_schemas
SET schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"connection_mode":{"type":"string","enum":["kubeconfig","endpoint"]},"server":{"type":"string"},"skip_tls_verify":{"type":"boolean"}}}'::jsonb
WHERE kind = 'Kubernetes' AND version = 1;

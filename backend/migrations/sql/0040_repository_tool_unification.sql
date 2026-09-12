UPDATE resource_schemas
SET version = 2,
    schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"url":{"title":"Git 仓库 URL","type":"string"},"default_branch":{"title":"默认分支","type":"string"},"path":{"title":"Bundle 工作副本路径","type":"string"},"storage_backend":{"title":"Bundle 存储后端","type":"string","enum":["local","s3"]},"storage_key":{"title":"Bundle 存储键","type":"string","readOnly":true},"s3_endpoint":{"title":"S3 Endpoint","type":"string"},"s3_bucket":{"title":"S3 Bucket","type":"string"},"s3_prefix":{"title":"S3 Prefix","type":"string"},"provider":{"title":"代码托管平台","type":"string"}}}'::jsonb,
    description = '支持实时 Git 仓库和上传 Bundle 仓库的代码上下文资源。', status = 'active'
WHERE kind = 'Repository';

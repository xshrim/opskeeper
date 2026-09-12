UPDATE resource_schemas SET version = 1,
schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"url":{"title":"仓库 URL","type":"string"},"provider":{"title":"代码托管平台","type":"string","enum":["git","github","gitlab","gitea","bitbucket"]},"default_branch":{"title":"默认分支","type":"string"},"username":{"title":"用户名","type":"string","sensitive":true},"token":{"title":"访问 Token","type":"string","sensitive":true},"ssh_private_key":{"title":"SSH 私钥","type":"string","sensitive":true}}}'::jsonb
WHERE kind = 'Repository';

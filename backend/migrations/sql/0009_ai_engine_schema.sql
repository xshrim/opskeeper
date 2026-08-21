INSERT INTO resource_schemas (kind, version, schema, status, display_name, description, icon)
VALUES (
    'AIEngine',
    1,
    '{
      "$schema":"https://json-schema.org/draft/2020-12/schema",
      "type":"object",
      "additionalProperties":false,
      "required":["profile","strategy","endpoints"],
      "properties":{
        "profile":{
          "title":"输入媒介",
          "type":"string",
          "enum":["single_modal","multi_modal"]
        },
        "strategy":{
          "title":"路由策略",
          "type":"string",
          "enum":["priority"]
        },
        "fallback_on":{
          "title":"故障切换条件",
          "type":"array",
          "items":{"type":"string","enum":["timeout","rate_limit","server_error"]}
        },
        "endpoints":{
          "title":"模型连接",
          "type":"array",
          "minItems":1,
          "items":{
            "type":"object",
            "additionalProperties":false,
            "required":["provider_type","base_url","model_name","priority","enabled"],
            "properties":{
              "provider_type":{"type":"string"},
              "base_url":{"type":"string","format":"uri"},
              "model_name":{"type":"string"},
              "credential_ref":{"type":"string"},
              "context_window":{"type":"integer"},
              "capabilities":{"type":"array","items":{"type":"string"}},
              "timeout_seconds":{"type":"integer"},
              "priority":{"type":"integer"},
              "enabled":{"type":"boolean"}
            }
          }
        }
      }
    }'::jsonb,
    'active',
    'AI 引擎',
    '聚合一个或多个模型连接，并按统一能力和故障转移策略提供模型调用入口。',
    'llm'
)
ON CONFLICT (kind, version) DO UPDATE
SET schema = EXCLUDED.schema,
    status = EXCLUDED.status,
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    icon = EXCLUDED.icon;

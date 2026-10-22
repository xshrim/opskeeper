UPDATE resource_schemas
SET schema = '{
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
    }'::jsonb
WHERE kind = 'AIEngine'
  AND version = 1;

package resource

import (
	"fmt"
	"reflect"
)

func validateConfig(config map[string]any, schema Schema) error {
	if config == nil {
		config = map[string]any{}
	}
	root := schema.Schema
	if rootType, ok := root["type"].(string); ok && rootType != "object" {
		return invalid(fmt.Sprintf("schema for %s must describe an object", schema.Kind))
	}
	if required, ok := root["required"].([]any); ok {
		for _, raw := range required {
			key, ok := raw.(string)
			if ok {
				if _, exists := config[key]; !exists {
					return invalid(fmt.Sprintf("config.%s is required", key))
				}
			}
		}
	}
	properties, _ := root["properties"].(map[string]any)
	additionalProperties, hasAdditional := root["additionalProperties"].(bool)
	for key, value := range config {
		definition, exists := properties[key]
		if !exists {
			if hasAdditional && !additionalProperties {
				return invalid(fmt.Sprintf("config.%s is not allowed by schema", key))
			}
			continue
		}
		property, _ := definition.(map[string]any)
		if expected, ok := property["type"].(string); ok && !matchesJSONType(value, expected) {
			return invalid(fmt.Sprintf("config.%s must be %s", key, expected))
		}
	}
	return nil
}

func matchesJSONType(value any, expected string) bool {
	switch expected {
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "number":
		kind := reflect.ValueOf(value).Kind()
		switch kind {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64:
			return true
		default:
			return false
		}
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		kind := reflect.ValueOf(value).Kind()
		return kind == reflect.Array || kind == reflect.Slice
	case "null":
		return value == nil
	default:
		return true
	}
}

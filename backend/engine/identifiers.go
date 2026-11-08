package engine

import (
	"fmt"
	"strings"
)

var locatorArgumentFields = map[string]struct{}{
	"container_id": {}, "container_name": {}, "namespace": {}, "workload_kind": {},
	"workload_name": {}, "pod": {}, "process_command_line": {},
}

func isPlaceholderLocator(field, value string) bool {
	field = strings.ToLower(strings.TrimSpace(field))
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	if _, ok := locatorArgumentFields[field]; !ok {
		return false
	}
	normalized := strings.Trim(strings.ToLower(value), "{}[]<>`\"'")
	return normalized == field || normalized == strings.TrimSuffix(field, "_id") || strings.HasPrefix(normalized, "your_") || strings.HasPrefix(normalized, "the_")
}

// ApplyLocatorHints fills obvious model placeholders from server-owned
// application bindings. A real model-provided locator always wins.
func ApplyLocatorHints(arguments map[string]any, hints map[string]any) map[string]any {
	result := make(map[string]any, len(arguments)+len(hints))
	for key, value := range arguments {
		result[key] = value
	}
	for key, hint := range hints {
		if hint == nil || strings.TrimSpace(fmt.Sprint(hint)) == "" {
			continue
		}
		current, exists := result[key]
		if !exists {
			result[key] = hint
			continue
		}
		if text, ok := current.(string); ok && isPlaceholderLocator(key, text) {
			result[key] = hint
		}
	}
	if text, ok := result["container_id"].(string); ok && isPlaceholderLocator("container_id", text) {
		if _, hasName := hints["container_name"]; hasName {
			delete(result, "container_id")
		}
	}
	return NormalizeDockerLocatorArguments(result)
}

// NormalizeDockerLocatorArguments keeps Docker names in container_name. A
// model often copies the human-readable name from docker_containers into the
// similarly named container_id field; Docker accepts both values, but keeping
// the fields canonical makes the call auditable and prevents a name from being
// mistaken for a database or engine ID by later layers.
func NormalizeDockerLocatorArguments(arguments map[string]any) map[string]any {
	result := make(map[string]any, len(arguments))
	for key, value := range arguments {
		result[key] = value
	}
	id, idOK := result["container_id"].(string)
	if !idOK || strings.TrimSpace(id) == "" || looksLikeDockerContainerID(id) {
		return result
	}
	name, nameOK := result["container_name"].(string)
	if !nameOK || strings.TrimSpace(name) == "" {
		result["container_name"] = id
	}
	delete(result, "container_id")
	return result
}

func looksLikeDockerContainerID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 12 && len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

func RejectPlaceholderLocatorArguments(arguments map[string]any) error {
	for key, value := range arguments {
		if text, ok := value.(string); ok && isPlaceholderLocator(key, text) {
			return fmt.Errorf("%s must be a concrete external object locator", key)
		}
	}
	return nil
}

// RejectInternalIdentifier protects external tool boundaries from database
// identities. Resource and application UUIDs are valid values inside the
// service, but are not valid substitutes for a Docker/Kubernetes/Host object
// locator.
func RejectInternalIdentifier(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if isUUID(strings.TrimSpace(value)) {
		return fmt.Errorf("%s must be an external object locator, not an internal database ID", field)
	}
	return nil
}

func isUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

// RejectModelInternalArguments blocks model-supplied arguments that use
// service identity fields as external tool inputs. These fields are never
// needed by a model-facing resource operation; routing already knows the
// selected resource on the server.
func RejectModelInternalArguments(arguments map[string]any) error {
	var visit func(map[string]any) error
	visit = func(values map[string]any) error {
		for key, value := range values {
			if isModelInternalIDField(key) && value != nil && strings.TrimSpace(fmt.Sprint(value)) != "" {
				return fmt.Errorf("%s is an internal database identifier and cannot be passed to a resource tool", key)
			}
			switch nested := value.(type) {
			case map[string]any:
				if err := visit(nested); err != nil {
					return err
				}
			case []any:
				for _, item := range nested {
					if child, ok := item.(map[string]any); ok {
						if err := visit(child); err != nil {
							return err
						}
					}
				}
			}
		}
		return nil
	}
	return visit(arguments)
}

// StripModelInternalIDs returns a recursively copied value with service and
// database identity fields removed. It is intended for model prompts only;
// server-side request objects must remain unchanged for authorization/routing.
func StripModelInternalIDs(value any) any {
	switch typed := value.(type) {
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, StripModelInternalIDs(item))
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if isModelInternalIDField(key) {
				continue
			}
			result[key] = StripModelInternalIDs(item)
		}
		return result
	default:
		return value
	}
}

func isModelInternalIDField(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	_, ok := modelInternalIDFields[key]
	return ok
}

var modelInternalIDFields = map[string]struct{}{
	"id": {}, "application_id": {}, "application_ids": {}, "project_id": {}, "project_ids": {},
	"resource_id": {}, "resource_ids": {}, "instance_id": {}, "instance_ids": {},
	"dependency_id": {}, "dependency_ids": {}, "relation_id": {}, "relation_ids": {},
	"scope_id": {}, "session_id": {}, "run_id": {}, "execution_id": {}, "actor_user_id": {},
	"provider_id": {}, "skill_id": {}, "persona_id": {}, "workflow_id": {},
	"target_resource_id": {}, "target_resource_ids": {}, "source_resource_id": {}, "evidence_id": {}, "evidence_ids": {},
	"call_id": {},
}

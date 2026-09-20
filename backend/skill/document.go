package skill

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	skillIdentifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{2,119}$`)
	skillVersionPattern    = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)
)

// ParseDocument converts all supported source formats into one validated
// document. The normalized JSON is the value that should be persisted and
// used by runners; Raw is retained for display and round-tripping.
func ParseDocument(format string, raw []byte) (ParsedDocument, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format != BodyFormatJSON && format != BodyFormatYAML && format != BodyFormatMarkdown {
		return ParsedDocument{}, invalid("Skill body format must be json, yaml or markdown")
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return ParsedDocument{}, invalid("Skill document must not be empty")
	}

	var value map[string]any
	var err error
	if format == BodyFormatMarkdown {
		value, err = parseMarkdownDocument(string(raw))
	} else {
		value, err = parseStructuredDocument(format, raw)
	}
	if err != nil {
		return ParsedDocument{}, invalid(fmt.Sprintf("invalid Skill document: %v", err))
	}
	document, err := documentFromMap(value)
	if err != nil {
		return ParsedDocument{}, err
	}
	normalized, err := json.Marshal(document)
	if err != nil {
		return ParsedDocument{}, fmt.Errorf("normalize Skill document: %w", err)
	}
	digest := sha256.Sum256(normalized)
	return ParsedDocument{
		Format:        format,
		Raw:           string(raw),
		Document:      document,
		Normalized:    normalized,
		ContentSHA256: hex.EncodeToString(digest[:]),
	}, nil
}

func ValidateDocument(document Document) error {
	return validateDocument(document)
}

func parseStructuredDocument(format string, raw []byte) (map[string]any, error) {
	var value any
	if format == BodyFormatJSON {
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
	} else if err := yaml.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return objectValue(value)
}

func parseMarkdownDocument(raw string) (map[string]any, error) {
	text := strings.ReplaceAll(strings.TrimPrefix(raw, "\ufeff"), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return nil, fmt.Errorf("markdown Skill documents require YAML front matter")
	}
	closing := -1
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			closing = index
			break
		}
	}
	if closing < 0 {
		return nil, fmt.Errorf("markdown Skill front matter is not closed")
	}
	value, err := parseStructuredDocument(BodyFormatYAML, []byte(strings.Join(lines[1:closing], "\n")))
	if err != nil {
		return nil, err
	}
	body := strings.TrimSpace(strings.Join(lines[closing+1:], "\n"))
	if body == "" {
		return nil, fmt.Errorf("markdown Skill body is required")
	}
	spec, ok := value["spec"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("spec must be an object")
	}
	if _, exists := spec["body"]; exists {
		return nil, fmt.Errorf("markdown body must be outside front matter")
	}
	spec["body"] = body
	value["spec"] = spec
	return value, nil
}

func objectValue(value any) (map[string]any, error) {
	converted, err := yamlObject(value)
	if err != nil {
		return nil, err
	}
	object, ok := converted.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("document root must be an object")
	}
	return object, nil
}

func yamlObject(value any) (any, error) {
	switch item := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(item))
		for key, child := range item {
			converted, err := yamlObject(child)
			if err != nil {
				return nil, err
			}
			result[key] = converted
		}
		return result, nil
	case map[any]any:
		result := make(map[string]any, len(item))
		for key, child := range item {
			name, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("object keys must be strings")
			}
			converted, err := yamlObject(child)
			if err != nil {
				return nil, err
			}
			result[name] = converted
		}
		return result, nil
	case []any:
		result := make([]any, len(item))
		for index, child := range item {
			converted, err := yamlObject(child)
			if err != nil {
				return nil, err
			}
			result[index] = converted
		}
		return result, nil
	default:
		return value, nil
	}
}

func documentFromMap(value map[string]any) (Document, error) {
	if err := rejectUnknown(value, "document", map[string]bool{"api_version": true, "kind": true, "metadata": true, "spec": true}); err != nil {
		return Document{}, err
	}
	metadata, ok := value["metadata"].(map[string]any)
	if !ok {
		return Document{}, invalid("Skill metadata must be an object")
	}
	if err := rejectUnknown(metadata, "metadata", map[string]bool{"name": true, "identifier": true, "version": true, "category": true, "tags": true, "maintainer": true}); err != nil {
		return Document{}, err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return Document{}, fmt.Errorf("encode Skill document: %w", err)
	}
	var document Document
	if err := json.Unmarshal(raw, &document); err != nil {
		return Document{}, fmt.Errorf("decode Skill document: %w", err)
	}
	if err := validateDocument(document); err != nil {
		return Document{}, err
	}
	return document, nil
}

func validateDocument(document Document) error {
	if document.APIVersion != DocumentAPIVersion {
		return invalid("Skill document api_version must be opskeeper.skill/v1")
	}
	if document.Kind != DocumentKind {
		return invalid("Skill document kind must be Skill")
	}
	if err := validateMetadata(document.Metadata); err != nil {
		return err
	}
	if document.Spec == nil {
		return invalid("Skill document spec is required")
	}
	if err := rejectUnknown(document.Spec, "spec", map[string]bool{"description": true, "input_schema": true, "output_schema": true, "body": true, "tools": true, "dependencies": true}); err != nil {
		return err
	}
	description, ok := document.Spec["description"].(string)
	if !ok || strings.TrimSpace(description) == "" || len([]rune(description)) > 4000 {
		return invalid("Skill spec.description must be 1 to 4000 characters")
	}
	for _, name := range []string{"input_schema", "output_schema"} {
		schema, ok := document.Spec[name].(map[string]any)
		if !ok {
			return invalid("Skill spec." + name + " must be a JSON object schema")
		}
		if schemaType, exists := schema["type"]; exists && schemaType != "object" {
			return invalid("Skill spec." + name + " root type must be object")
		}
	}
	if !validBody(document.Spec["body"]) {
		return invalid("Skill spec.body must be a non-empty string or object")
	}
	if tools, exists := document.Spec["tools"]; exists {
		if err := validateList(tools, "tools"); err != nil {
			return err
		}
	}
	if dependencies, exists := document.Spec["dependencies"]; exists {
		if err := validateList(dependencies, "dependencies"); err != nil {
			return err
		}
	}
	return nil
}

func validateMetadata(metadata Metadata) error {
	if strings.TrimSpace(metadata.Name) == "" || len([]rune(metadata.Name)) > 200 {
		return invalid("Skill metadata.name is required and must be at most 200 characters")
	}
	if !skillIdentifierPattern.MatchString(metadata.Identifier) {
		return invalid("Skill metadata.identifier is invalid")
	}
	if !skillVersionPattern.MatchString(metadata.Version) {
		return invalid("Skill metadata.version must be semantic version")
	}
	switch metadata.Category {
	case CategoryDiagnosis, CategoryMonitoring, CategoryOptimization, CategoryMaintenance:
	default:
		return invalid("Skill metadata.category is invalid")
	}
	if strings.TrimSpace(metadata.Maintainer) == "" {
		return invalid("Skill metadata.maintainer is required")
	}
	if len(metadata.Tags) > 50 {
		return invalid("Skill metadata.tags may contain at most 50 items")
	}
	seen := make(map[string]struct{}, len(metadata.Tags))
	for _, tag := range metadata.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || len([]rune(tag)) > 80 {
			return invalid("Skill metadata.tags must contain non-empty values of at most 80 characters")
		}
		if _, exists := seen[tag]; exists {
			return invalid("Skill metadata.tags must be unique")
		}
		seen[tag] = struct{}{}
	}
	return nil
}

func validBody(value any) bool {
	switch item := value.(type) {
	case string:
		return strings.TrimSpace(item) != ""
	case map[string]any:
		return len(item) > 0
	default:
		return false
	}
}

func validateList(value any, field string) error {
	items, ok := value.([]any)
	if !ok {
		return invalid("Skill spec." + field + " must be an array")
	}
	for _, item := range items {
		if _, ok := item.(map[string]any); !ok {
			return invalid("Skill spec." + field + " items must be objects")
		}
	}
	return nil
}

func rejectUnknown(value map[string]any, field string, allowed map[string]bool) error {
	for key := range value {
		if !allowed[key] {
			return invalid("Skill " + field + " contains unsupported field " + key)
		}
	}
	return nil
}

package resource

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func validateConfig(config map[string]any, schema Schema) error {
	if config == nil {
		config = map[string]any{}
	}
	raw, err := json.Marshal(schema.Schema)
	if err != nil {
		return invalid(fmt.Sprintf("schema for %s is invalid: %v", schema.Kind, err))
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return invalid(fmt.Sprintf("schema for %s is invalid: %v", schema.Kind, err))
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("resource-schema.json", document); err != nil {
		return invalid(fmt.Sprintf("schema for %s is invalid: %v", schema.Kind, err))
	}
	compiled, err := compiler.Compile("resource-schema.json")
	if err != nil {
		return invalid(fmt.Sprintf("schema for %s is invalid: %v", schema.Kind, err))
	}
	if err := compiled.Validate(config); err != nil {
		message := strings.TrimSpace(err.Error())
		message = strings.TrimPrefix(message, "jsonschema validation failed with ")
		message = strings.ReplaceAll(message, "file:///home/xcadmin/git/opskeeper/backend/resource/resource-schema.json#", "")
		return invalid(fmt.Sprintf("config does not match %s schema: %s", schema.Kind, message))
	}
	return nil
}

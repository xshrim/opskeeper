package resource

import (
	"bytes"
	"encoding/json"
	"fmt"

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
		return invalid(fmt.Sprintf("config does not match %s schema: %v", schema.Kind, err))
	}
	return nil
}

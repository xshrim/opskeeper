package notification

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"text/template/parse"
)

const MaxRenderedTemplateBytes = 64 << 10

type TemplateVariable struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

type TemplateDraft struct {
	Format          string             `json:"format"`
	TitleTemplate   string             `json:"title_template"`
	BodyTemplate    string             `json:"body_template"`
	PayloadTemplate json.RawMessage    `json:"payload_template,omitempty"`
	Variables       []TemplateVariable `json:"variables"`
}

type RenderedTemplate struct {
	Title   string          `json:"title,omitempty"`
	Body    string          `json:"body,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

var allowedTemplateVariables = map[string]bool{
	"event_type": true, "severity": true, "rule_name": true, "policy_name": true,
	"resource_name": true, "scope_name": true, "finding_summary": true,
	"first_observed_at": true, "last_observed_at": true, "run_url": true, "opskeeper_url": true,
}

func ValidateTemplate(draft TemplateDraft) (TemplateDraft, string, error) {
	draft.Format = strings.TrimSpace(draft.Format)
	if draft.Format != "text" && draft.Format != "markdown" && draft.Format != "json" {
		return TemplateDraft{}, "", invalidTemplate("unsupported template format")
	}
	if len(draft.Variables) > len(allowedTemplateVariables) {
		return TemplateDraft{}, "", invalidTemplate("too many template variables")
	}
	declared := make(map[string]TemplateVariable, len(draft.Variables))
	for _, variable := range draft.Variables {
		variable.Name = strings.TrimSpace(variable.Name)
		if !allowedTemplateVariables[variable.Name] || (variable.Type != "" && variable.Type != "string") {
			return TemplateDraft{}, "", invalidTemplate("unknown template variable or type")
		}
		if _, exists := declared[variable.Name]; exists {
			return TemplateDraft{}, "", invalidTemplate("duplicate template variable")
		}
		variable.Type = "string"
		declared[variable.Name] = variable
	}
	if strings.TrimSpace(draft.Format) == "json" {
		if len(draft.PayloadTemplate) == 0 || !json.Valid(draft.PayloadTemplate) {
			return TemplateDraft{}, "", invalidTemplate("JSON template must contain a valid JSON payload")
		}
		var payload any
		if err := json.Unmarshal(draft.PayloadTemplate, &payload); err != nil {
			return TemplateDraft{}, "", invalidTemplate("JSON template payload is invalid")
		}
		if err := validateTemplateValue(payload, declared); err != nil {
			return TemplateDraft{}, "", err
		}
		draft.PayloadTemplate, _ = json.Marshal(payload)
		draft.TitleTemplate, draft.BodyTemplate = "", ""
	} else {
		if len(draft.PayloadTemplate) != 0 {
			return TemplateDraft{}, "", invalidTemplate("payload template is only valid for JSON format")
		}
		if strings.TrimSpace(draft.TitleTemplate) == "" && strings.TrimSpace(draft.BodyTemplate) == "" {
			return TemplateDraft{}, "", invalidTemplate("template title or body is required")
		}
		if err := validateTemplateSource(draft.TitleTemplate, declared); err != nil {
			return TemplateDraft{}, "", err
		}
		if err := validateTemplateSource(draft.BodyTemplate, declared); err != nil {
			return TemplateDraft{}, "", err
		}
	}
	canonical, err := json.Marshal(draft)
	if err != nil {
		return TemplateDraft{}, "", err
	}
	hash := sha256.Sum256(canonical)
	return draft, hex.EncodeToString(hash[:]), nil
}

func RenderTemplate(draft TemplateDraft, values map[string]string) (RenderedTemplate, error) {
	validated, _, err := ValidateTemplate(draft)
	if err != nil {
		return RenderedTemplate{}, err
	}
	declared := make(map[string]TemplateVariable, len(validated.Variables))
	for _, variable := range validated.Variables {
		declared[variable.Name] = variable
	}
	for key := range values {
		if _, exists := declared[key]; !exists {
			return RenderedTemplate{}, invalidTemplate("render input contains an undeclared variable")
		}
	}
	data := make(map[string]string, len(declared))
	for name, variable := range declared {
		value, exists := values[name]
		if variable.Required && !exists {
			return RenderedTemplate{}, invalidTemplate("required template variable is missing")
		}
		data[name] = value
	}
	var rendered RenderedTemplate
	if validated.Format == "json" {
		var payload any
		if err := json.Unmarshal(validated.PayloadTemplate, &payload); err != nil {
			return RenderedTemplate{}, invalidTemplate("JSON template payload is invalid")
		}
		payload, err = renderValue(payload, data)
		if err != nil {
			return RenderedTemplate{}, err
		}
		rendered.Payload, err = json.Marshal(payload)
	} else {
		rendered.Title, err = renderString(validated.TitleTemplate, data)
		if err == nil {
			rendered.Body, err = renderString(validated.BodyTemplate, data)
		}
	}
	if err != nil {
		return RenderedTemplate{}, err
	}
	size := len(rendered.Title) + len(rendered.Body) + len(rendered.Payload)
	if size > MaxRenderedTemplateBytes {
		return RenderedTemplate{}, invalidTemplate("rendered template exceeds the size limit")
	}
	return rendered, nil
}

func RenderForProvider(provider string, draft TemplateDraft, values map[string]string) ([]byte, error) {
	descriptor, ok := DefaultProviderRegistry().Get(provider)
	if !ok || !descriptor.Supported {
		return nil, invalidTemplate("provider does not support template delivery")
	}
	rendered, err := RenderTemplate(draft, values)
	if err != nil {
		return nil, err
	}
	var payload []byte
	if draft.Format == "json" {
		payload = rendered.Payload
	} else {
		payload, err = json.Marshal(struct {
			Title string `json:"title"`
			Body  string `json:"body"`
		}{Title: rendered.Title, Body: rendered.Body})
		if err != nil {
			return nil, err
		}
	}
	if len(payload) > descriptor.MaxPayload {
		return nil, invalidTemplate("rendered template exceeds provider payload limit")
	}
	return payload, nil
}

func validateTemplateValue(value any, declared map[string]TemplateVariable) error {
	switch typed := value.(type) {
	case string:
		return validateTemplateSource(typed, declared)
	case []any:
		for _, item := range typed {
			if err := validateTemplateValue(item, declared); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, item := range typed {
			if err := validateTemplateValue(item, declared); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateTemplateSource(source string, declared map[string]TemplateVariable) error {
	parsed, err := template.New("notification").Option("missingkey=error").Parse(source)
	if err != nil {
		return invalidTemplate("template syntax is invalid")
	}
	for _, parsedTemplate := range parsed.Templates() {
		for _, node := range parsedTemplate.Tree.Root.Nodes {
			action, ok := node.(*parse.ActionNode)
			if !ok {
				switch node.(type) {
				case *parse.TextNode, *parse.CommentNode:
					continue
				default:
					return invalidTemplate("template control actions are not allowed")
				}
			}
			if len(action.Pipe.Decl) != 0 || len(action.Pipe.Cmds) != 1 || len(action.Pipe.Cmds[0].Args) != 1 {
				return invalidTemplate("only direct variable placeholders are allowed")
			}
			field, ok := action.Pipe.Cmds[0].Args[0].(*parse.FieldNode)
			if !ok || len(field.Ident) != 1 {
				return invalidTemplate("only direct variable placeholders are allowed")
			}
			if _, ok := declared[field.Ident[0]]; !ok {
				return invalidTemplate("template references an undeclared variable")
			}
		}
	}
	return nil
}

func renderString(source string, values map[string]string) (string, error) {
	parsed, err := template.New("notification").Option("missingkey=error").Parse(source)
	if err != nil {
		return "", invalidTemplate("template syntax is invalid")
	}
	var output bytes.Buffer
	if err := parsed.Execute(&output, values); err != nil {
		return "", invalidTemplate("template could not be rendered")
	}
	return output.String(), nil
}

func renderValue(value any, values map[string]string) (any, error) {
	switch typed := value.(type) {
	case string:
		return renderString(typed, values)
	case []any:
		for index, item := range typed {
			rendered, err := renderValue(item, values)
			if err != nil {
				return nil, err
			}
			typed[index] = rendered
		}
	case map[string]any:
		for key, item := range typed {
			rendered, err := renderValue(item, values)
			if err != nil {
				return nil, err
			}
			typed[key] = rendered
		}
	}
	return value, nil
}

func invalidTemplate(message string) error {
	return fmt.Errorf("invalid notification template: %s", message)
}

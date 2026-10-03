package notification

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderTemplateRequiresDeclaredVariablesAndIsDeterministic(t *testing.T) {
	draft := TemplateDraft{
		Format:        "markdown",
		TitleTemplate: "{{.severity}}: {{.rule_name}}",
		BodyTemplate:  "{{.finding_summary}}",
		Variables: []TemplateVariable{
			{Name: "severity", Required: true},
			{Name: "rule_name", Required: true},
			{Name: "finding_summary"},
		},
	}
	first, hash, err := ValidateTemplate(draft)
	if err != nil {
		t.Fatal(err)
	}
	second, secondHash, err := ValidateTemplate(draft)
	if err != nil || hash != secondHash || first.Format != second.Format {
		t.Fatalf("validation is not stable: hash=%q second=%q err=%v", hash, secondHash, err)
	}
	rendered, err := RenderTemplate(draft, map[string]string{
		"severity": "critical", "rule_name": "Public endpoint", "finding_summary": "exposed",
	})
	if err != nil || rendered.Title != "critical: Public endpoint" || rendered.Body != "exposed" {
		t.Fatalf("RenderTemplate() = %#v, %v", rendered, err)
	}
	if _, err := RenderTemplate(draft, map[string]string{"severity": "critical"}); err == nil {
		t.Fatal("missing required variable was accepted")
	}
	if _, err := RenderTemplate(draft, map[string]string{"severity": "critical", "rule_name": "x", "extra": "y"}); err == nil {
		t.Fatal("undeclared render variable was accepted")
	}
}

func TestValidateTemplateRejectsCodeAndUndeclaredVariables(t *testing.T) {
	base := TemplateDraft{Format: "text", BodyTemplate: "{{.finding_summary}}", Variables: []TemplateVariable{{Name: "finding_summary"}}}
	for _, source := range []string{
		"{{.finding_summary | printf \"%s\"}}",
		"{{if .finding_summary}}yes{{end}}",
		"{{.unknown}}",
		"{{define \"extra\"}}{{.unknown}}{{end}}{{template \"extra\"}}",
	} {
		base.BodyTemplate = source
		if _, _, err := ValidateTemplate(base); err == nil {
			t.Errorf("ValidateTemplate accepted %q", source)
		}
	}
}

func TestRenderJSONTemplateEscapesValues(t *testing.T) {
	draft := TemplateDraft{
		Format:          "json",
		PayloadTemplate: json.RawMessage(`{"message":"{{.finding_summary}}"}`),
		Variables:       []TemplateVariable{{Name: "finding_summary", Required: true}},
	}
	rendered, err := RenderTemplate(draft, map[string]string{"finding_summary": `"quoted"`})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]string
	if err := json.Unmarshal(rendered.Payload, &payload); err != nil {
		t.Fatalf("rendered invalid JSON %s: %v", rendered.Payload, err)
	}
	if payload["message"] != `"quoted"` {
		t.Fatalf("message = %q", payload["message"])
	}
}

func TestRenderForProviderConvertsTextAndRejectsUnsupportedProvider(t *testing.T) {
	draft := TemplateDraft{Format: "text", TitleTemplate: "Alert", BodyTemplate: "{{.finding_summary}}", Variables: []TemplateVariable{{Name: "finding_summary", Required: true}}}
	payload, err := RenderForProvider("webhook", draft, map[string]string{"finding_summary": "example"})
	if err != nil {
		t.Fatal(err)
	}
	var message map[string]string
	if err := json.Unmarshal(payload, &message); err != nil || message["title"] != "Alert" || message["body"] != "example" {
		t.Fatalf("provider payload = %s, %v", payload, err)
	}
	if _, err := RenderForProvider("whatsapp", draft, map[string]string{"finding_summary": "example"}); err == nil {
		t.Fatal("unsupported provider accepted a template")
	}
}

func TestRenderTemplateEnforcesOutputLimit(t *testing.T) {
	draft := TemplateDraft{Format: "text", BodyTemplate: "{{.finding_summary}}", Variables: []TemplateVariable{{Name: "finding_summary"}}}
	if _, err := RenderTemplate(draft, map[string]string{"finding_summary": strings.Repeat("x", MaxRenderedTemplateBytes+1)}); err == nil {
		t.Fatal("oversized output was accepted")
	}
}

package skill

import (
	"bytes"
	"testing"
)

const testDocumentBody = `
api_version: opskeeper.skill/v1
kind: Skill
metadata:
  name: PostgreSQL health
  identifier: postgresql-health
  version: 1.2.3
  category: diagnosis
  tags: [database, readonly]
  maintainer: user-123
spec:
  description: Inspect PostgreSQL health without changing the resource.
  input_schema:
    type: object
    properties:
      target_resource_id: {type: string}
  output_schema:
    type: object
    required: [findings]
  body: Inspect the resource and report evidence.
`

func TestParseDocumentNormalizesJSONYAMLAndMarkdown(t *testing.T) {
	yamlSource := []byte(testDocumentBody)
	jsonSource := []byte(`{"api_version":"opskeeper.skill/v1","kind":"Skill","metadata":{"name":"PostgreSQL health","identifier":"postgresql-health","version":"1.2.3","category":"diagnosis","tags":["database","readonly"],"maintainer":"user-123"},"spec":{"description":"Inspect PostgreSQL health without changing the resource.","input_schema":{"type":"object","properties":{"target_resource_id":{"type":"string"}}},"output_schema":{"type":"object","required":["findings"]},"body":"Inspect the resource and report evidence."}}`)
	markdownSource := []byte(`---
api_version: opskeeper.skill/v1
kind: Skill
metadata:
  name: PostgreSQL health
  identifier: postgresql-health
  version: 1.2.3
  category: diagnosis
  tags: [database, readonly]
  maintainer: user-123
spec:
  description: Inspect PostgreSQL health without changing the resource.
  input_schema:
    type: object
    properties:
      target_resource_id: {type: string}
  output_schema:
    type: object
    required: [findings]
---
Inspect the resource and report evidence.
`)

	parsedJSON, err := ParseDocument(BodyFormatJSON, jsonSource)
	if err != nil {
		t.Fatal(err)
	}
	parsedYAML, err := ParseDocument(BodyFormatYAML, yamlSource)
	if err != nil {
		t.Fatal(err)
	}
	parsedMarkdown, err := ParseDocument(BodyFormatMarkdown, markdownSource)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(parsedJSON.Normalized, parsedYAML.Normalized) || !bytes.Equal(parsedJSON.Normalized, parsedMarkdown.Normalized) {
		t.Fatalf("formats did not normalize equally:\njson=%s\nyaml=%s\nmarkdown=%s", parsedJSON.Normalized, parsedYAML.Normalized, parsedMarkdown.Normalized)
	}
	if parsedJSON.ContentSHA256 != parsedYAML.ContentSHA256 || parsedJSON.ContentSHA256 != parsedMarkdown.ContentSHA256 {
		t.Fatal("equivalent documents must have the same content hash")
	}
}

func TestParseDocumentRejectsInvalidBoundary(t *testing.T) {
	valid := []byte(testDocumentBody)
	tests := []struct {
		name   string
		format string
		raw    []byte
	}{
		{"markdown without front matter", BodyFormatMarkdown, []byte("# no front matter")},
		{"non object root", BodyFormatJSON, []byte(`[1,2,3]`)},
		{"invalid category", BodyFormatYAML, bytes.Replace(valid, []byte("category: diagnosis"), []byte("category: unknown"), 1)},
		{"invalid identifier", BodyFormatYAML, bytes.Replace(valid, []byte("identifier: postgresql-health"), []byte("identifier: Team/Postgres"), 1)},
		{"unknown field", BodyFormatYAML, append(valid, []byte("unexpected: true\n")...)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseDocument(test.format, test.raw); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestParseDocumentHashIsStable(t *testing.T) {
	first, err := ParseDocument(BodyFormatYAML, []byte(testDocumentBody))
	if err != nil {
		t.Fatal(err)
	}
	second, err := ParseDocument(BodyFormatYAML, []byte(testDocumentBody))
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentSHA256 == "" || first.ContentSHA256 != second.ContentSHA256 {
		t.Fatalf("unexpected unstable hash: %q and %q", first.ContentSHA256, second.ContentSHA256)
	}
}

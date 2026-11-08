package engine

import (
	"strings"
	"testing"
)

func TestStripModelInternalIDs(t *testing.T) {
	value := map[string]any{
		"application_id": "app-db-id",
		"name":           "Orders",
		"binding": map[string]any{
			"container_name": "orders-api",
			"resource_id":    "resource-db-id",
		},
	}
	got := StripModelInternalIDs(value).(map[string]any)
	if _, ok := got["application_id"]; ok {
		t.Fatal("application_id leaked into model value")
	}
	binding := got["binding"].(map[string]any)
	if binding["container_name"] != "orders-api" {
		t.Fatalf("external locator was removed: %#v", binding)
	}
	if _, ok := binding["resource_id"]; ok {
		t.Fatal("resource_id leaked into model value")
	}
}

func TestRejectInternalIdentifier(t *testing.T) {
	if err := RejectInternalIdentifier("container_id", "251683f4-d022-4153-886a-2eec581bf110"); err == nil {
		t.Fatal("UUID must be rejected as an external locator")
	}
	if err := RejectInternalIdentifier("container_name", "orders-api"); err != nil {
		t.Fatalf("external name rejected: %v", err)
	}
}

func TestApplyLocatorHintsReplacesModelPlaceholders(t *testing.T) {
	got := ApplyLocatorHints(map[string]any{
		"container_id":   "container_id",
		"container_name": "container_name",
	}, map[string]any{"container_name": "orders-api"})
	if _, exists := got["container_id"]; exists {
		t.Fatalf("placeholder container_id should be removed: %#v", got)
	}
	if got["container_name"] != "orders-api" {
		t.Fatalf("container name hint = %#v", got)
	}

	got = ApplyLocatorHints(map[string]any{"container_name": "billing-api"}, map[string]any{"container_name": "orders-api"})
	if got["container_name"] != "billing-api" {
		t.Fatalf("real model locator was overwritten: %#v", got)
	}
}

func TestNormalizeDockerLocatorArgumentsMovesNamesOutOfContainerID(t *testing.T) {
	got := NormalizeDockerLocatorArguments(map[string]any{"container_id": "opskeeper-postgres"})
	if _, exists := got["container_id"]; exists {
		t.Fatalf("container name must not remain in container_id: %#v", got)
	}
	if got["container_name"] != "opskeeper-postgres" {
		t.Fatalf("container name = %#v", got["container_name"])
	}

	got = NormalizeDockerLocatorArguments(map[string]any{"container_id": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"})
	if got["container_id"] == nil || got["container_name"] != nil {
		t.Fatalf("real Docker ID must remain an ID: %#v", got)
	}

	got = ApplyLocatorHints(map[string]any{"container_id": "opskeeper-postgres"}, map[string]any{"container_name": "opskeeper-postgres"})
	if _, exists := got["container_id"]; exists || got["container_name"] != "opskeeper-postgres" {
		t.Fatalf("application binding must canonicalize the name field: %#v", got)
	}
}

func TestRejectPlaceholderLocatorArguments(t *testing.T) {
	if err := RejectPlaceholderLocatorArguments(map[string]any{"container_name": "container_name"}); err == nil {
		t.Fatal("placeholder locator must be rejected")
	}
	if err := RejectPlaceholderLocatorArguments(map[string]any{"container_name": "orders-api"}); err != nil {
		t.Fatalf("concrete locator rejected: %v", err)
	}
}

func TestExecutionPromptOmitsContextResourceIDs(t *testing.T) {
	prompt, err := executionPrompt(Request{
		Input: map[string]any{"application_id": "app-db-id", "question": "inspect"},
		ResolvedContext: &ResolvedContext{Facts: []ContextFact{{
			ResourceID: "resource-db-id", Kind: "Docker", Summary: map[string]any{"resource_id": "resource-db-id", "name": "Docker"},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(prompt, "app-db-id") || strings.Contains(prompt, "resource-db-id") || strings.Contains(prompt, "application_id") {
		t.Fatalf("internal IDs leaked into execution prompt: %s", prompt)
	}
	if !strings.Contains(prompt, "inspect") || !strings.Contains(prompt, "Docker") {
		t.Fatalf("safe model input was lost: %s", prompt)
	}
}

func TestToolAliasesDoNotExposeResourceIDs(t *testing.T) {
	bindings := buildToolBindings([]ToolDefinition{
		{Name: "docker_container_inspect", ResourceID: "251683f4-d022-4153-886a-2eec581bf110", ResourceName: "Orders Docker"},
		{Name: "docker_container_inspect", ResourceID: "another-resource-id", ResourceName: "Billing Docker"},
	})
	if len(bindings) != 2 || strings.Contains(bindings[0].modelName, "251683f4") || strings.Contains(bindings[1].modelName, "another-resource-id") {
		t.Fatalf("tool aliases leaked resource IDs: %#v", bindings)
	}
	if !strings.Contains(bindings[0].modelName, "Orders_Docker") {
		t.Fatalf("tool alias should retain safe resource name: %#v", bindings[0].modelName)
	}
}

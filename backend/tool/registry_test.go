package tool

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestRegistryKeepsMinimalDefinitionAndInvokesWithAdapterConnection(t *testing.T) {
	registry := NewRegistry()
	tool := ToolFunc{
		Def: Definition{
			Name:        "docker_info",
			Description: "Read Docker Engine information.",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		Fn: func(_ context.Context, invocation Invocation) (Result, error) {
			if invocation.Connection != "direct-client" {
				t.Fatalf("connection = %#v", invocation.Connection)
			}
			return Result{Value: map[string]any{"id": "docker-1"}}, nil
		},
	}
	if err := registry.Register(tool); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	definitions := registry.List()
	if len(definitions) != 1 || definitions[0].Name != "docker_info" {
		t.Fatalf("definitions = %#v", definitions)
	}
	result, err := registry.Invoke(context.Background(), "docker_info", Invocation{Arguments: map[string]any{"unused": true}, Connection: "direct-client"})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !reflect.DeepEqual(result.Value, map[string]any{"id": "docker-1"}) {
		t.Fatalf("result = %#v", result)
	}
}

func TestRegistryRejectsDuplicateAndInvalidDefinitions(t *testing.T) {
	registry := NewRegistry()
	tool := ToolFunc{Def: Definition{Name: "docker_info"}, Fn: func(context.Context, Invocation) (Result, error) { return Result{}, nil }}
	if err := registry.Register(tool); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	if !errors.Is(registry.Register(tool), ErrToolExists) {
		t.Fatalf("duplicate Register() did not return ErrToolExists")
	}
	invalid := ToolFunc{Def: Definition{Name: "docker info"}, Fn: tool.Fn}
	if !errors.Is(registry.Register(invalid), ErrInvalidTool) {
		t.Fatalf("invalid Register() did not return ErrInvalidTool")
	}
	badSchema := ToolFunc{Def: Definition{Name: "docker_images", InputSchema: json.RawMessage("{")}, Fn: tool.Fn}
	if !errors.Is(registry.Register(badSchema), ErrInvalidTool) {
		t.Fatalf("bad schema Register() did not return ErrInvalidTool")
	}
}

func TestRegistryCopiesInputAndNormalizesErrors(t *testing.T) {
	registry := NewRegistry()
	argumentsSeen := make(chan map[string]any, 1)
	if err := registry.Register(ToolFunc{
		Def: Definition{Name: "docker_images"},
		Fn: func(_ context.Context, invocation Invocation) (Result, error) {
			argumentsSeen <- invocation.Arguments
			return Result{Partial: true, Warnings: []Warning{{Code: "fallback", Message: "used fallback"}}}, nil
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	arguments := map[string]any{"all": true}
	result, err := registry.Invoke(context.Background(), "docker_images", Invocation{Arguments: arguments})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	arguments["all"] = false
	if got := (<-argumentsSeen)["all"]; got != true {
		t.Fatalf("tool observed mutated arguments: %#v", got)
	}
	if !result.Partial || len(result.Warnings) != 1 {
		t.Fatalf("result metadata = %#v", result)
	}
	_, err = registry.Invoke(context.Background(), "missing", Invocation{})
	if CodeOf(err) != CodeToolUnavailable {
		t.Fatalf("missing tool code = %q, err = %v", CodeOf(err), err)
	}
}

func TestCodeOfContextCancellation(t *testing.T) {
	if CodeOf(context.Canceled) != CodeCancelled {
		t.Fatalf("CodeOf(context.Canceled) = %q", CodeOf(context.Canceled))
	}
	if CodeOf(NewError(CodeUnavailable, "connect Docker", context.DeadlineExceeded)) != CodeUnavailable {
		t.Fatalf("wrapped error code = %q", CodeOf(NewError(CodeUnavailable, "connect Docker", context.DeadlineExceeded)))
	}
}

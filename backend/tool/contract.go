// Package tool contains protocol-neutral resource tools.
//
// It deliberately does not depend on AIEngine, MCP, HTTP handlers, or the
// resource catalog. Adapters provide connection context and translate the
// result to their own protocol.
package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Definition is the public business contract of a resource tool. Runtime
// binding data such as resource IDs, adapter source, permissions, and
// credentials live outside this contract.
type Definition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// Invocation contains model-controlled business arguments and adapter-owned
// connection context. Connection is intentionally opaque here; each resource
// package can pass its own typed client or connection value.
type Invocation struct {
	Arguments  map[string]any `json:"arguments,omitempty"`
	Connection any            `json:"-"`
}

// Result is the protocol-neutral execution result. Value is the unchanged
// business output that MCP and Direct adapters encode for their callers.
type Result struct {
	Value    any       `json:"value,omitempty"`
	Partial  bool      `json:"partial,omitempty"`
	Warnings []Warning `json:"warnings,omitempty"`
}

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Tool interface {
	Definition() Definition
	Invoke(context.Context, Invocation) (Result, error)
}

type ToolFunc struct {
	Def Definition
	Fn  func(context.Context, Invocation) (Result, error)
}

func (t ToolFunc) Definition() Definition { return t.Def }

func (t ToolFunc) Invoke(ctx context.Context, invocation Invocation) (Result, error) {
	if t.Fn == nil {
		return Result{}, NewError(CodeInternal, "invoke tool", errors.New("tool function is nil"))
	}
	return t.Fn(ctx, invocation)
}

type ErrorCode string

const (
	CodeConfiguration    ErrorCode = "configuration"
	CodeAuthentication   ErrorCode = "authentication"
	CodeUnavailable      ErrorCode = "unavailable"
	CodeToolUnavailable  ErrorCode = "tool_unavailable"
	CodePermissionDenied ErrorCode = "permission_denied"
	CodeInvalidArgument  ErrorCode = "invalid_argument"
	CodeResponseLimited  ErrorCode = "response_limited"
	CodeCancelled        ErrorCode = "cancelled"
	CodeInternal         ErrorCode = "internal"
)

// Error is the common resource-tool error envelope. Adapters may wrap their
// protocol-specific error while preserving Code and Temporary for callers.
type Error struct {
	Code      ErrorCode
	Operation string
	Temporary bool
	Err       error
}

func NewError(code ErrorCode, operation string, err error) error {
	if err == nil {
		err = errors.New(string(code))
	}
	return &Error{Code: code, Operation: strings.TrimSpace(operation), Err: err}
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Operation == "" {
		return e.Err.Error()
	}
	return e.Operation + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func CodeOf(err error) ErrorCode {
	if err == nil {
		return ""
	}
	var resourceErr *Error
	if errors.As(err, &resourceErr) && resourceErr != nil {
		return resourceErr.Code
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return CodeCancelled
	}
	return CodeInternal
}

var (
	ErrInvalidTool  = errors.New("invalid resource tool")
	ErrToolExists   = errors.New("resource tool already registered")
	ErrToolNotFound = errors.New("resource tool not found")
)

func invalidTool(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidTool, strings.TrimSpace(message))
}

func cloneSchema(schema json.RawMessage) json.RawMessage {
	if len(schema) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), schema...)
}

func cloneArguments(arguments map[string]any) map[string]any {
	if len(arguments) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(arguments))
	for key, value := range arguments {
		cloned[key] = value
	}
	return cloned
}

package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

func TestChatCompletionsModelTextUsageAndToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization header was not set")
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "test-model" {
			t.Errorf("model = %#v", body["model"])
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"model":"test-model","choices":[{"finish_reason":"tool_calls","message":{"content":"checking","tool_calls":[{"id":"call-1","type":"function","function":{"name":"lookup","arguments":"{\"resource_id\":\"r1\"}"}}]}}],"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}}`)
	}))
	defer server.Close()
	client, err := NewChatCompletionsModel(ChatCompletionsConfig{APIKey: "secret", BaseURL: server.URL + "/v1", ModelName: "test-model", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	request := &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText("inspect", genai.RoleUser)}, Config: &genai.GenerateContentConfig{Tools: []*genai.Tool{{FunctionDeclarations: []*genai.FunctionDeclaration{{Name: "lookup", Description: "look up a resource", ParametersJsonSchema: map[string]any{"type": "object", "properties": map[string]any{"resource_id": map[string]any{"type": "string"}}}}}}}}}
	var response *model.LLMResponse
	for item, generateErr := range client.GenerateContent(context.Background(), request, false) {
		if generateErr != nil {
			t.Fatal(generateErr)
		}
		response = item
	}
	if response == nil || response.UsageMetadata == nil || response.UsageMetadata.TotalTokenCount != 18 {
		t.Fatalf("response usage = %#v", response)
	}
	if len(response.Content.Parts) != 2 || response.Content.Parts[1].FunctionCall == nil || response.Content.Parts[1].FunctionCall.Name != "lookup" {
		t.Fatalf("response content = %#v", response.Content)
	}
}

func TestChatCompletionsModelStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		flusher := writer.(http.Flusher)
		for _, line := range []string{
			`data: {"choices":[{"finish_reason":"","delta":{"content":"hel"}}]}`,
			`data: {"choices":[{"finish_reason":"stop","delta":{"content":"lo"}}]}`,
			`data: {"choices":[],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`,
			`data: [DONE]`,
		} {
			fmt.Fprintln(writer, line)
			fmt.Fprintln(writer)
			flusher.Flush()
		}
	}))
	defer server.Close()
	client, err := NewChatCompletionsModel(ChatCompletionsConfig{BaseURL: server.URL, ModelName: "test-model", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	var partial strings.Builder
	var final *model.LLMResponse
	for item, generateErr := range client.GenerateContent(context.Background(), &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText("hi", genai.RoleUser)}}, true) {
		if generateErr != nil {
			t.Fatal(generateErr)
		}
		if item.Partial {
			for _, part := range item.Content.Parts {
				partial.WriteString(part.Text)
			}
		} else {
			final = item
		}
	}
	if partial.String() != "hello" {
		t.Errorf("partial text = %q", partial.String())
	}
	if final == nil || textParts(final.Content) != "hello" || final.UsageMetadata.TotalTokenCount != 3 {
		t.Fatalf("final response = %#v", final)
	}
}

func TestChatCompletionsUpstreamFailureDoesNotExposeBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(writer, `{"error":"token=do-not-expose"}`)
	}))
	defer server.Close()
	client, err := NewChatCompletionsModel(ChatCompletionsConfig{BaseURL: server.URL, ModelName: "test-model", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	for _, generateErr := range client.GenerateContent(context.Background(), &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText("hi", genai.RoleUser)}}, false) {
		if generateErr == nil {
			t.Fatal("expected upstream error")
		}
		if strings.Contains(generateErr.Error(), "do-not-expose") || generateErr.Error() != "Chat Completions returned status 401" {
			t.Fatalf("unsafe upstream error = %q", generateErr)
		}
	}
}

func TestUpstreamErrorMessage(t *testing.T) {
	if got := upstreamErrorMessage([]byte(`{"error":{"message":"model does not exist"}}`)); got != "model does not exist" {
		t.Fatalf("provider message = %q", got)
	}
	if got := upstreamErrorMessage([]byte(`{"error":"token=do-not-expose"}`)); got != "" {
		t.Fatalf("credential leaked through provider message = %q", got)
	}
	if got := upstreamErrorMessage([]byte(`{"message":"invalid API key"}`)); got != "invalid API key" {
		t.Fatalf("safe API key reason = %q", got)
	}
}

func TestAPIKeyFromCredential(t *testing.T) {
	if got := apiKeyFromCredential([]byte(`{"token":" structured-secret "}`)); got != "structured-secret" {
		t.Fatalf("structured API key = %q", got)
	}
	if got := apiKeyFromCredential([]byte("legacy-secret")); got != "legacy-secret" {
		t.Fatalf("legacy API key = %q", got)
	}
}

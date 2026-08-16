// SPDX-FileCopyrightText: 2026 OpsKeeper contributors
// SPDX-FileCopyrightText: 2026 Alby Hernández <hola@achetronic.com>
// SPDX-License-Identifier: Apache-2.0

// The ADK model adapter in this file is a focused implementation derived from
// the Chat Completions adapter in achetronic/adk-utils-go. It intentionally
// supports text, streaming, usage and function calls only.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"strings"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

var _ model.LLM = (*ChatCompletionsModel)(nil)

type ChatCompletionsConfig struct {
	APIKey, BaseURL, ModelName string
	HTTPClient                 *http.Client
}

type ChatCompletionsModel struct {
	apiKey, endpoint, name string
	client                 *http.Client
}

func NewChatCompletionsModel(config ChatCompletionsConfig) (*ChatCompletionsModel, error) {
	base, err := url.Parse(strings.TrimRight(config.BaseURL, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, invalid("OpenAI-compatible base URL is invalid")
	}
	if strings.TrimSpace(config.ModelName) == "" {
		return nil, invalid("OpenAI-compatible model name is required")
	}
	client := config.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return &ChatCompletionsModel{apiKey: config.APIKey, endpoint: strings.TrimRight(base.String(), "/") + "/chat/completions", name: config.ModelName, client: client}, nil
}

func (m *ChatCompletionsModel) Name() string { return m.name }

func (m *ChatCompletionsModel) GenerateContent(ctx context.Context, request *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		payload, err := m.buildRequest(request, stream)
		if err != nil {
			yield(nil, err)
			return
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			yield(nil, fmt.Errorf("encode Chat Completions request: %w", err))
			return
		}
		httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint, bytes.NewReader(encoded))
		if err != nil {
			yield(nil, fmt.Errorf("create Chat Completions request: %w", err))
			return
		}
		httpRequest.Header.Set("Content-Type", "application/json")
		httpRequest.Header.Set("Accept", "application/json")
		if m.apiKey != "" {
			httpRequest.Header.Set("Authorization", "Bearer "+m.apiKey)
		}
		response, err := m.client.Do(httpRequest)
		if err != nil {
			yield(nil, fmt.Errorf("call Chat Completions: %w", err))
			return
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 32<<10))
			yield(nil, fmt.Errorf("Chat Completions returned status %d", response.StatusCode))
			return
		}
		if stream {
			m.decodeStream(response.Body, yield)
			return
		}
		var wire chatResponse
		if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&wire); err != nil {
			yield(nil, fmt.Errorf("decode Chat Completions response: %w", err))
			return
		}
		result, err := convertChatResponse(wire)
		yield(result, err)
	}
}

type chatRequest struct {
	Model         string         `json:"model"`
	Messages      []chatMessage  `json:"messages"`
	Tools         []chatTool     `json:"tools,omitempty"`
	ToolChoice    string         `json:"tool_choice,omitempty"`
	Stream        bool           `json:"stream,omitempty"`
	StreamOptions map[string]any `json:"stream_options,omitempty"`
	Temperature   *float32       `json:"temperature,omitempty"`
	TopP          *float32       `json:"top_p,omitempty"`
	MaxTokens     int32          `json:"max_tokens,omitempty"`
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Name       string         `json:"name,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type chatToolCall struct {
	Index    int          `json:"index,omitempty"`
	ID       string       `json:"id"`
	Type     string       `json:"type,omitempty"`
	Function functionCall `json:"function"`
}

type functionCall struct{ Name, Arguments string }

func (f *functionCall) UnmarshalJSON(data []byte) error {
	var raw struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	f.Name, f.Arguments = raw.Name, raw.Arguments
	return nil
}

func (f functionCall) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}{f.Name, f.Arguments})
}

func (m *ChatCompletionsModel) buildRequest(request *model.LLMRequest, stream bool) (chatRequest, error) {
	if request == nil {
		return chatRequest{}, errors.New("LLM request is required")
	}
	wire := chatRequest{Model: m.name, Stream: stream}
	if stream {
		wire.StreamOptions = map[string]any{"include_usage": true}
	}
	if request.Config != nil {
		wire.Temperature, wire.TopP, wire.MaxTokens = request.Config.Temperature, request.Config.TopP, request.Config.MaxOutputTokens
		if request.Config.SystemInstruction != nil {
			if text := textParts(request.Config.SystemInstruction); text != "" {
				wire.Messages = append(wire.Messages, chatMessage{Role: "system", Content: text})
			}
		}
		for _, group := range request.Config.Tools {
			if group == nil {
				continue
			}
			for _, declaration := range group.FunctionDeclarations {
				if declaration == nil {
					continue
				}
				parameters := schemaMap(declaration.ParametersJsonSchema)
				if parameters == nil {
					parameters = schemaMap(declaration.Parameters)
				}
				if parameters == nil {
					parameters = map[string]any{"type": "object", "properties": map[string]any{}}
				}
				wire.Tools = append(wire.Tools, chatTool{Type: "function", Function: chatFunction{Name: declaration.Name, Description: declaration.Description, Parameters: parameters}})
			}
		}
		if len(wire.Tools) > 0 {
			wire.ToolChoice = "auto"
		}
	}
	for _, content := range request.Contents {
		messages, err := contentMessages(content)
		if err != nil {
			return chatRequest{}, err
		}
		wire.Messages = append(wire.Messages, messages...)
	}
	return wire, nil
}

func contentMessages(content *genai.Content) ([]chatMessage, error) {
	if content == nil {
		return nil, nil
	}
	role := "user"
	if content.Role == genai.RoleModel {
		role = "assistant"
	}
	message := chatMessage{Role: role}
	var result []chatMessage
	for _, part := range content.Parts {
		if part == nil {
			continue
		}
		if part.Text != "" {
			message.Content += part.Text
		}
		if part.FunctionCall != nil {
			arguments, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				return nil, fmt.Errorf("encode function arguments: %w", err)
			}
			message.Role = "assistant"
			message.ToolCalls = append(message.ToolCalls, chatToolCall{ID: normalizeToolCallID(part.FunctionCall.ID), Type: "function", Function: functionCall{Name: part.FunctionCall.Name, Arguments: string(arguments)}})
		}
		if part.FunctionResponse != nil {
			if message.Content != "" || len(message.ToolCalls) > 0 {
				result = append(result, message)
				message = chatMessage{Role: role}
			}
			encoded, err := json.Marshal(part.FunctionResponse.Response)
			if err != nil {
				return nil, fmt.Errorf("encode function response: %w", err)
			}
			result = append(result, chatMessage{Role: "tool", ToolCallID: normalizeToolCallID(part.FunctionResponse.ID), Name: part.FunctionResponse.Name, Content: string(encoded)})
		}
	}
	if message.Content != "" || len(message.ToolCalls) > 0 {
		result = append(result, message)
	}
	return result, nil
}

type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   string         `json:"content"`
			ToolCalls []chatToolCall `json:"tool_calls"`
		} `json:"message"`
		Delta struct {
			Content   string         `json:"content"`
			ToolCalls []chatToolCall `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int32 `json:"prompt_tokens"`
		CompletionTokens int32 `json:"completion_tokens"`
		TotalTokens      int32 `json:"total_tokens"`
	} `json:"usage"`
}

func convertChatResponse(wire chatResponse) (*model.LLMResponse, error) {
	if len(wire.Choices) == 0 {
		return nil, errors.New("Chat Completions response contains no choices")
	}
	choice := wire.Choices[0]
	content := &genai.Content{Role: genai.RoleModel}
	if choice.Message.Content != "" {
		content.Parts = append(content.Parts, &genai.Part{Text: choice.Message.Content})
	}
	for _, call := range choice.Message.ToolCalls {
		args := map[string]any{}
		if strings.TrimSpace(call.Function.Arguments) != "" {
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				return nil, fmt.Errorf("decode function arguments: %w", err)
			}
		}
		content.Parts = append(content.Parts, &genai.Part{FunctionCall: &genai.FunctionCall{ID: call.ID, Name: call.Function.Name, Args: args}})
	}
	return &model.LLMResponse{Content: content, ModelVersion: wire.Model, FinishReason: finishReason(choice.FinishReason), UsageMetadata: usageMetadata(wire), TurnComplete: true}, nil
}

func (m *ChatCompletionsModel) decodeStream(reader io.Reader, yield func(*model.LLMResponse, error) bool) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), 2<<20)
	var content strings.Builder
	calls := map[int]chatToolCall{}
	var usage chatResponse
	finish := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk chatResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			yield(nil, fmt.Errorf("decode Chat Completions stream: %w", err))
			return
		}
		if chunk.Usage.TotalTokens > 0 || chunk.Usage.PromptTokens > 0 {
			usage.Usage = chunk.Usage
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		if choice.FinishReason != "" {
			finish = choice.FinishReason
		}
		if choice.Delta.Content != "" {
			content.WriteString(choice.Delta.Content)
			if !yield(&model.LLMResponse{Content: genai.NewContentFromText(choice.Delta.Content, genai.RoleModel), Partial: true}, nil) {
				return
			}
		}
		for _, delta := range choice.Delta.ToolCalls {
			current := calls[delta.Index]
			if delta.ID != "" {
				current.ID = delta.ID
			}
			if delta.Function.Name != "" {
				current.Function.Name += delta.Function.Name
			}
			current.Function.Arguments += delta.Function.Arguments
			calls[delta.Index] = current
		}
	}
	if err := scanner.Err(); err != nil {
		yield(nil, fmt.Errorf("read Chat Completions stream: %w", err))
		return
	}
	final := chatResponse{Model: m.name, Usage: usage.Usage}
	final.Choices = append(final.Choices, struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   string         `json:"content"`
			ToolCalls []chatToolCall `json:"tool_calls"`
		} `json:"message"`
		Delta struct {
			Content   string         `json:"content"`
			ToolCalls []chatToolCall `json:"tool_calls"`
		} `json:"delta"`
	}{FinishReason: finish})
	final.Choices[0].Message.Content = content.String()
	for index := 0; index < len(calls); index++ {
		final.Choices[0].Message.ToolCalls = append(final.Choices[0].Message.ToolCalls, calls[index])
	}
	response, err := convertChatResponse(final)
	if response != nil {
		response.Partial = false
		response.TurnComplete = true
	}
	yield(response, err)
}

func schemaMap(value any) map[string]any {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var result map[string]any
	if json.Unmarshal(encoded, &result) != nil {
		return nil
	}
	return result
}

func textParts(content *genai.Content) string {
	var values []string
	if content != nil {
		for _, part := range content.Parts {
			if part != nil && part.Text != "" {
				values = append(values, part.Text)
			}
		}
	}
	return strings.Join(values, "\n")
}

func usageMetadata(wire chatResponse) *genai.GenerateContentResponseUsageMetadata {
	return &genai.GenerateContentResponseUsageMetadata{PromptTokenCount: wire.Usage.PromptTokens, CandidatesTokenCount: wire.Usage.CompletionTokens, TotalTokenCount: wire.Usage.TotalTokens}
}

func finishReason(reason string) genai.FinishReason {
	switch reason {
	case "stop":
		return genai.FinishReasonStop
	case "length":
		return genai.FinishReasonMaxTokens
	default:
		return genai.FinishReasonUnspecified
	}
}

func normalizeToolCallID(id string) string {
	if len(id) <= 40 {
		return id
	}
	return id[:40]
}

package aiengine

import (
	"context"
	"iter"
	"strings"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

const emptyModelResponseRetries = 1

// retryingModel hides provider keep-alive/role/usage chunks from ADK and
// retries one model request when the provider closes without any meaningful
// content. It operates inside one ADK model turn, so tools are never replayed.
type retryingModel struct {
	delegate model.LLM
	onRetry  func() error
}

func (m *retryingModel) Name() string { return m.delegate.Name() }

func (m *retryingModel) GenerateContent(ctx context.Context, request *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		for attempt := 0; attempt <= emptyModelResponseRetries; attempt++ {
			meaningful := false
			sawFinal := false
			var partialParts []*genai.Part
			partialHasFunctionCall := false
			var pendingWhitespace []*model.LLMResponse
			for response, err := range m.delegate.GenerateContent(ctx, request, stream) {
				if err != nil {
					if !meaningful && attempt < emptyModelResponseRetries && isRetryableEmptyModelError(err) {
						meaningful = false
						break
					}
					yield(nil, err)
					return
				}
				if response == nil {
					continue
				}
				if response.ErrorCode != "" || response.ErrorMessage != "" {
					// Provider/model errors are terminal and must not be hidden by
					// an empty-response retry.
					if !yield(response, nil) {
						return
					}
					return
				}
				if modelResponseHasMeaningfulContent(response) {
					meaningful = true
					if !response.Partial {
						sawFinal = true
					}
					if response.Partial {
						appendPartialParts(response, &partialParts, &partialHasFunctionCall)
						for _, pending := range pendingWhitespace {
							if !yield(pending, nil) {
								return
							}
						}
						pendingWhitespace = nil
					}
					if !yield(response, nil) {
						return
					}
					continue
				}
				// Empty provider chunks are intentionally ignored. Once a turn has
				// useful content, later empty usage/final chunks are still safe to
				// ignore and cannot terminate the visible answer early.
				if response.Partial && modelResponseHasText(response) {
					appendPartialParts(response, &partialParts, &partialHasFunctionCall)
					if meaningful {
						if !yield(response, nil) {
							return
						}
					} else {
						pendingWhitespace = append(pendingWhitespace, response)
					}
				}
			}
			if stream && meaningful && !sawFinal && len(partialParts) > 0 && !partialHasFunctionCall {
				if !yield(&model.LLMResponse{
					Content:      &genai.Content{Role: genai.RoleModel, Parts: partialParts},
					TurnComplete: true,
				}, nil) {
					return
				}
			}
			if meaningful || attempt == emptyModelResponseRetries || ctx.Err() != nil {
				return
			}
			if m.onRetry != nil {
				if err := m.onRetry(); err != nil {
					yield(nil, err)
					return
				}
			}
		}
	}
}

func appendPartialParts(response *model.LLMResponse, parts *[]*genai.Part, hasFunctionCall *bool) {
	if response == nil || response.Content == nil {
		return
	}
	for _, part := range response.Content.Parts {
		if part == nil {
			continue
		}
		*hasFunctionCall = *hasFunctionCall || part.FunctionCall != nil
		if part.Text == "" || part.Thought {
			continue
		}
		copy := *part
		*parts = append(*parts, &copy)
	}
}

func modelResponseHasMeaningfulContent(response *model.LLMResponse) bool {
	if response == nil || response.Content == nil {
		return false
	}
	for _, part := range response.Content.Parts {
		if part == nil {
			continue
		}
		if (!part.Thought && strings.TrimSpace(part.Text) != "") ||
			part.FunctionCall != nil ||
			part.FunctionResponse != nil ||
			part.CodeExecutionResult != nil ||
			part.ExecutableCode != nil ||
			part.FileData != nil ||
			part.InlineData != nil ||
			part.ToolCall != nil ||
			part.ToolResponse != nil ||
			part.AudioTranscription != nil {
			return true
		}
	}
	return false
}

func modelResponseHasText(response *model.LLMResponse) bool {
	if response == nil || response.Content == nil {
		return false
	}
	for _, part := range response.Content.Parts {
		if part != nil && !part.Thought && part.Text != "" {
			return true
		}
	}
	return false
}

func isRetryableEmptyModelError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	for _, marker := range []string{
		"empty response",
		"empty final response",
		"no output items",
		"no text or tool content",
		"contains no choices",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

var _ model.LLM = (*retryingModel)(nil)

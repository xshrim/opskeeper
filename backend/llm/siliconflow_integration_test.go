//go:build integration

package llm

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/genai"
)

func TestSiliconFlowThroughADKRunner(t *testing.T) {
	apiKey := os.Getenv("OPSK_TEST_LLM_API_KEY")
	if apiKey == "" {
		t.Skip("OPSK_TEST_LLM_API_KEY is not set")
	}
	baseURL := os.Getenv("OPSK_TEST_LLM_BASE_URL")
	modelName := os.Getenv("OPSK_TEST_LLM_MODEL")
	if baseURL == "" || modelName == "" {
		t.Fatal("OPSK_TEST_LLM_BASE_URL and OPSK_TEST_LLM_MODEL must be set")
	}
	client, err := NewChatCompletionsModel(ChatCompletionsConfig{APIKey: apiKey, BaseURL: baseURL, ModelName: modelName})
	if err != nil {
		t.Fatal(err)
	}
	agentRoot, err := llmagent.New(llmagent.Config{Name: "siliconflow_check", Model: client, Instruction: "请使用一句简短中文回答。"})
	if err != nil {
		t.Fatal(err)
	}
	adkRunner, err := runner.NewInMemory("opskeeper-provider-check", agentRoot)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	for _, streaming := range []agent.StreamingMode{agent.StreamingModeNone, agent.StreamingModeSSE} {
		var output strings.Builder
		var tokens int32
		for event, runErr := range adkRunner.Run(ctx, "acceptance-user", string(streaming), genai.NewContentFromText("你好，请自我介绍一下。", genai.RoleUser), agent.RunConfig{StreamingMode: streaming}) {
			if runErr != nil {
				t.Fatalf("mode %s: %v", streaming, runErr)
			}
			if event == nil {
				continue
			}
			if event.Content != nil && !event.Partial {
				for _, part := range event.Content.Parts {
					output.WriteString(part.Text)
				}
			}
			if event.UsageMetadata != nil {
				tokens += event.UsageMetadata.TotalTokenCount
			}
		}
		if strings.TrimSpace(output.String()) == "" {
			t.Fatalf("mode %s returned no output", streaming)
		}
		if tokens == 0 {
			t.Fatalf("mode %s returned no token usage", streaming)
		}
		t.Logf("mode=%s output_chars=%d total_tokens=%d", streaming, len([]rune(output.String())), tokens)
	}
}

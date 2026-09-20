package llm

import "testing"

func TestNormalizeProviderConfigRemovesDeprecatedImageTag(t *testing.T) {
	config := ProviderConfig{Models: []ProviderModel{{
		Tags:         []string{"text", "image", "vision"},
		Capabilities: []string{"image"},
	}}}

	normalizeProviderConfig(&config)
	model := config.Models[0]
	if got := model.Tags; len(got) != 2 || got[0] != "text" || got[1] != "vision" {
		t.Fatalf("normalized model tags = %#v, want text and vision", got)
	}
	if got := model.Capabilities; len(got) != 2 || got[0] != "text" || got[1] != "vision" {
		t.Fatalf("normalized model capabilities = %#v, want text and vision", got)
	}
	for _, tag := range ModelTags {
		if tag == "image" {
			t.Fatal("deprecated image tag is still accepted")
		}
	}
}

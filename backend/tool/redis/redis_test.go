package redis

import "testing"

func TestToolsAreFixedAndReadOnly(t *testing.T) {
	got := ListTools()
	if len(got) != 6 {
		t.Fatalf("tool count = %d", len(got))
	}
	for _, tool := range got {
		if tool.Name == "redis_command" || tool.Name == "redis_eval" {
			t.Fatalf("unsafe tool exposed: %s", tool.Name)
		}
	}
	schema := InputSchema(nil)
	if schema["additionalProperties"] != false {
		t.Fatal("schema allows arbitrary properties")
	}
}

func TestParseInfo(t *testing.T) {
	values := parseInfo("# Memory\nused_memory:42\nredis_version:7.2.0\n")
	if values["used_memory"] != int64(42) || values["redis_version"] != "7.2.0" {
		t.Fatalf("unexpected values: %#v", values)
	}
}

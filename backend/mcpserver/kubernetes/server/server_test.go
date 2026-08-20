package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToolsUsePrimitiveSchemasAndNoOutputSchemas(t *testing.T) {
	handler, err := New(Config{Address: "127.0.0.1:8812"})
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewUnstartedServer(handler)
	httpServer.Listener = listener
	httpServer.Start()
	defer httpServer.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "schema-test-client", Version: "test"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: httpServer.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tools) != len(toolCatalog) {
		t.Fatalf("tools=%d want=%d", len(result.Tools), len(toolCatalog))
	}
	for _, tool := range result.Tools {
		encoded, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		text := string(encoded)
		for _, forbidden := range []string{"$ref", "$defs", "allOf", "oneOf", "anyOf"} {
			if strings.Contains(text, forbidden) {
				t.Errorf("tool %q has %s: %s", tool.Name, forbidden, text)
			}
		}
		if tool.OutputSchema != nil {
			t.Errorf("tool %q unexpectedly has output schema", tool.Name)
		}
	}
}

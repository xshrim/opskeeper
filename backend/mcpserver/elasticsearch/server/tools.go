package server

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	es "opskeeper/backend/tool/elasticsearch"
)

type input struct{ es.ConnectionInput }
type indexInput struct {
	es.ConnectionInput
	Index string `json:"index"`
}

func RegisterTools(s *mcp.Server) {
	for _, t := range es.ListTools() {
		_ = t
	}
	mcp.AddTool(s, &mcp.Tool{Name: "elasticsearch_health", Description: "Read Elasticsearch cluster health.", InputSchema: es.InputSchema(nil)}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, any, error) {
		x, e := es.New(in.ConnectionInput)
		if e != nil {
			return nil, nil, e
		}
		o, e := x.Health(c)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "elasticsearch_cluster_info", Description: "Read Elasticsearch cluster metadata.", InputSchema: es.InputSchema(nil)}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, any, error) {
		x, e := es.New(in.ConnectionInput)
		if e != nil {
			return nil, nil, e
		}
		o, e := x.ClusterInfo(c)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "elasticsearch_nodes", Description: "List Elasticsearch nodes and roles.", InputSchema: es.InputSchema(nil)}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, any, error) {
		x, e := es.New(in.ConnectionInput)
		if e != nil {
			return nil, nil, e
		}
		o, e := x.Nodes(c)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "elasticsearch_indices", Description: "List user indices and metrics.", InputSchema: es.InputSchema(nil)}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, any, error) {
		x, e := es.New(in.ConnectionInput)
		if e != nil {
			return nil, nil, e
		}
		o, e := x.Indices(c)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "elasticsearch_shards", Description: "Read shard allocation summary.", InputSchema: es.InputSchema(nil)}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, any, error) {
		x, e := es.New(in.ConnectionInput)
		if e != nil {
			return nil, nil, e
		}
		o, e := x.Shards(c)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "elasticsearch_settings", Description: "Read cluster settings.", InputSchema: es.InputSchema(nil)}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, any, error) {
		x, e := es.New(in.ConnectionInput)
		if e != nil {
			return nil, nil, e
		}
		o, e := x.Settings(c)
		return nil, o, e
	})
	mcp.AddTool(s, &mcp.Tool{Name: "elasticsearch_index_mapping", Description: "Read one index mapping.", InputSchema: es.InputSchema(map[string]any{"index": map[string]any{"type": "string"}})}, func(c context.Context, _ *mcp.CallToolRequest, in indexInput) (*mcp.CallToolResult, any, error) {
		x, e := es.New(in.ConnectionInput)
		if e != nil {
			return nil, nil, e
		}
		o, e := x.IndexMapping(c, in.Index)
		return nil, o, e
	})
}

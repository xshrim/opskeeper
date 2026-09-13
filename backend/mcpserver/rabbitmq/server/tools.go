package server

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	rmq "opskeeper/backend/tool/rabbitmq"
)

type input struct{ rmq.ConnectionInput }

func RegisterTools(s *mcp.Server) {
	for _, t := range rmq.ListTools() {
		name := t.Name
		mcp.AddTool(s, &mcp.Tool{Name: name, Description: t.Description, InputSchema: rmq.InputSchema(nil)}, func(c context.Context, _ *mcp.CallToolRequest, in input) (*mcp.CallToolResult, any, error) {
			switch name {
			case "rabbitmq_health":
				o, e := rmq.Health(c, in.ConnectionInput)
				return nil, o, e
			case "rabbitmq_overview":
				o, e := rmq.Overview(c, in.ConnectionInput)
				return nil, o, e
			case "rabbitmq_nodes":
				o, e := rmq.Nodes(c, in.ConnectionInput)
				return nil, o, e
			case "rabbitmq_queues":
				o, e := rmq.Queues(c, in.ConnectionInput)
				return nil, o, e
			case "rabbitmq_exchanges":
				o, e := rmq.Exchanges(c, in.ConnectionInput)
				return nil, o, e
			case "rabbitmq_connections":
				o, e := rmq.Connections(c, in.ConnectionInput)
				return nil, o, e
			case "rabbitmq_channels":
				o, e := rmq.Channels(c, in.ConnectionInput)
				return nil, o, e
			case "rabbitmq_consumers":
				o, e := rmq.Consumers(c, in.ConnectionInput)
				return nil, o, e
			case "rabbitmq_vhosts":
				o, e := rmq.Vhosts(c, in.ConnectionInput)
				return nil, o, e
			}
			return nil, nil, nil
		})
	}
}

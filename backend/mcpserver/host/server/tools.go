package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	host "opskeeper/backend/tool/host"
)

type HostInfoInput = host.InfoInput
type HostMetricsInput = host.MetricsInput
type HostProcessesInput = host.ProcessesInput
type HostFileLogsInput = host.FileLogsInput
type HostInfoOutput = host.InfoOutput
type HostMetricsOutput = host.MetricsOutput
type HostProcessesOutput = host.ProcessesOutput
type HostFileLogsOutput = host.FileLogsOutput
type HostHealthOutput = host.HealthOutput

func RegisterTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "host_info", Description: "Read Linux host identity and system information.", InputSchema: host.InputSchema(nil)}, func(ctx context.Context, _ *mcp.CallToolRequest, input HostInfoInput) (*mcp.CallToolResult, any, error) {
		output, err := host.Info(ctx, input)
		return nil, output, err
	})
	mcp.AddTool(s, &mcp.Tool{Name: "host_metrics", Description: "Read Linux host metrics as structured JSON.", InputSchema: host.InputSchema(map[string]any{"sample_seconds": map[string]any{"type": "integer", "minimum": 0, "maximum": host.MaxSampleSeconds}})}, func(ctx context.Context, _ *mcp.CallToolRequest, input HostMetricsInput) (*mcp.CallToolResult, any, error) {
		output, err := host.Metrics(ctx, input)
		return nil, output, err
	})
	mcp.AddTool(s, &mcp.Tool{Name: "host_processes", Description: "Read information about selected Linux processes.", InputSchema: host.InputSchema(map[string]any{"pid": map[string]any{"type": "integer", "minimum": 1}, "keyword": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": host.MaxProcessLimit}, "sample_seconds": map[string]any{"type": "integer", "minimum": 0, "maximum": host.MaxSampleSeconds}})}, func(ctx context.Context, _ *mcp.CallToolRequest, input HostProcessesInput) (*mcp.CallToolResult, any, error) {
		output, err := host.Processes(ctx, input)
		return nil, output, err
	})
	mcp.AddTool(s, &mcp.Tool{Name: "host_file_logs", Description: "Read bounded logs from a Linux host file.", InputSchema: host.InputSchema(map[string]any{"path": map[string]any{"type": "string", "description": "Absolute path on the selected Linux host."}, "tail": map[string]any{"type": "string"}, "since": map[string]any{"type": "string"}, "until": map[string]any{"type": "string"}, "keyword": map[string]any{"type": "string"}, "timestamps": map[string]any{"type": "boolean"}})}, func(ctx context.Context, _ *mcp.CallToolRequest, input HostFileLogsInput) (*mcp.CallToolResult, any, error) {
		output, err := host.FileLogs(ctx, input)
		return nil, output, err
	})
	mcp.AddTool(s, &mcp.Tool{Name: "host_health", Description: "Check Linux host health and availability.", InputSchema: host.InputSchema(nil)}, func(ctx context.Context, _ *mcp.CallToolRequest, input host.ConnectionInput) (*mcp.CallToolResult, any, error) {
		output, err := host.Health(ctx, input)
		return nil, output, err
	})
}

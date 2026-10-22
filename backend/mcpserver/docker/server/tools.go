package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	docker "opskeeper/backend/tool/docker"
)

const maxOutputLogLines = 200

type DockerInfoInput = docker.DockerInfoInput
type ListImagesInput = docker.ListImagesInput
type ListContainersInput = docker.ListContainersInput
type ContainerLogsInput = docker.ContainerLogsInput
type ContainerFileInput = docker.ContainerFileInput
type ContainerInspectInput = docker.ContainerInspectInput
type ContainerStatsInput = docker.ContainerStatsInput
type LogsOutput = docker.LogsOutput
type FileOutput = docker.FileOutput
type DockerInfoOutput = docker.DockerInfoOutput
type ImagesOutput = docker.ImagesOutput
type ContainersOutput = docker.ContainersOutput
type DockerInfoDTO = docker.DockerInfoDTO
type ImageDTO = docker.ImageDTO
type ContainerDTO = docker.ContainerDTO
type ContainerInspectDTO = docker.ContainerInspectDTO
type ContainerStatsDTO = docker.ContainerStatsDTO

func RegisterTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{Name: "docker_info", Description: "Read Docker Engine information.", InputSchema: docker.InputSchema(nil)}, func(ctx context.Context, _ *mcp.CallToolRequest, input DockerInfoInput) (*mcp.CallToolResult, any, error) {
		output, err := docker.Info(ctx, input)
		return nil, output, err
	})
	mcp.AddTool(s, &mcp.Tool{Name: "docker_images", Description: "List Docker images.", InputSchema: docker.InputSchema(map[string]any{
		"all":     map[string]any{"type": "boolean", "description": "Include intermediate and dangling images."},
		"filters": map[string]any{"type": "string", "description": "Optional comma-separated Docker filters in key:value or key=value format, for example label=app:web,label:com.example.env=prod,reference=nginx:latest. Each item is split at its first separator."},
	})}, func(ctx context.Context, _ *mcp.CallToolRequest, input ListImagesInput) (*mcp.CallToolResult, any, error) {
		output, err := docker.Images(ctx, input)
		return nil, output, err
	})
	mcp.AddTool(s, &mcp.Tool{Name: "docker_containers", Description: "List Docker containers.", InputSchema: docker.InputSchema(map[string]any{
		"all":     map[string]any{"type": "boolean", "description": "Include stopped containers."},
		"limit":   map[string]any{"type": "integer", "description": "Maximum number of containers to return; 0 uses the Docker default and the maximum is 500."},
		"filters": map[string]any{"type": "string", "description": "Optional comma-separated Docker filters in key:value or key=value format, for example status=running,label:app=web. Each item is split at its first separator."},
	})}, func(ctx context.Context, _ *mcp.CallToolRequest, input ListContainersInput) (*mcp.CallToolResult, any, error) {
		output, err := docker.Containers(ctx, input)
		return nil, output, err
	})
	mcp.AddTool(s, &mcp.Tool{Name: "docker_container_logs", Description: "Read bounded, non-following logs from a Docker container.", InputSchema: docker.InputSchema(map[string]any{
		"container_id":   map[string]any{"type": "string", "description": "Container ID or name; takes precedence over container_name."},
		"container_name": map[string]any{"type": "string", "description": "Container name when container_id is omitted."},
		"tail":           map[string]any{"type": "string", "description": "Number of lines requested from the end; defaults to 1000. Keyword filtering is applied first, then output is limited to the most recent 200 lines."},
		"since":          map[string]any{"type": "string", "description": "Only show logs since this timestamp or duration."},
		"until":          map[string]any{"type": "string", "description": "Only show logs until this timestamp."},
		"keyword":        map[string]any{"type": "string", "description": "Case-insensitive keyword filter. Separate terms with & to require all terms on the same line, or | to match any term; AND binds tighter than OR (for example, error&timeout|fatal). Returns each matching line plus up to 3 lines before and after it, without duplicate lines when contexts overlap."},
		"timestamps":     map[string]any{"type": "boolean", "description": "Include timestamps in log lines."},
		"details":        map[string]any{"type": "boolean", "description": "Include extra Docker log attributes."},
	})}, func(ctx context.Context, _ *mcp.CallToolRequest, input ContainerLogsInput) (*mcp.CallToolResult, any, error) {
		output, err := docker.ContainerLogs(ctx, input)
		return nil, output, err
	})
	mcp.AddTool(s, &mcp.Tool{Name: "docker_container_file", Description: "Read a bounded regular file from a Docker container.", InputSchema: docker.InputSchema(map[string]any{
		"container_id":   map[string]any{"type": "string", "description": "Container ID or name; takes precedence over container_name."},
		"container_name": map[string]any{"type": "string", "description": "Container name when container_id is omitted."},
		"path":           map[string]any{"type": "string", "description": "Absolute path of a regular file inside the container."},
	})}, func(ctx context.Context, _ *mcp.CallToolRequest, input docker.ContainerFileInput) (*mcp.CallToolResult, any, error) {
		output, err := docker.ContainerFile(ctx, input)
		return nil, output, err
	})
	mcp.AddTool(s, &mcp.Tool{Name: "docker_container_inspect", Description: "Inspect a Docker container.", InputSchema: docker.InputSchema(map[string]any{
		"container_id":   map[string]any{"type": "string", "description": "Container ID or name; takes precedence over container_name."},
		"container_name": map[string]any{"type": "string", "description": "Container name when container_id is omitted."},
	})}, func(ctx context.Context, _ *mcp.CallToolRequest, input ContainerInspectInput) (*mcp.CallToolResult, any, error) {
		output, err := docker.ContainerInspect(ctx, input)
		return nil, output, err
	})
	mcp.AddTool(s, &mcp.Tool{Name: "docker_container_stats", Description: "Read one snapshot of Docker container statistics.", InputSchema: docker.InputSchema(map[string]any{
		"container_id":   map[string]any{"type": "string", "description": "Container ID or name; takes precedence over container_name."},
		"container_name": map[string]any{"type": "string", "description": "Container name when container_id is omitted."},
	})}, func(ctx context.Context, _ *mcp.CallToolRequest, input ContainerStatsInput) (*mcp.CallToolResult, any, error) {
		output, err := docker.ContainerStats(ctx, input)
		return nil, output, err
	})
}

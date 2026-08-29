package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"opskeeper/backend/mcpserver/docker/client"
)

const (
	maxLogBytes   = 256 * 1024
	maxStatsBytes = 2 * 1024 * 1024
	maxListLimit  = 500
)

type DockerInfoInput struct{ client.ConnectionInput }

type ListImagesInput struct {
	client.ConnectionInput
	All     bool   `json:"all,omitempty" jsonschema:"Include intermediate and dangling images."`
	Filters string `json:"filters,omitempty" jsonschema:"Optional Docker image filters in key:value,key:value format."`
}

type ListContainersInput struct {
	client.ConnectionInput
	All     bool   `json:"all,omitempty" jsonschema:"Include stopped containers."`
	Limit   int    `json:"limit,omitempty" jsonschema:"Maximum number of containers to return. Maximum 500."`
	Filters string `json:"filters,omitempty" jsonschema:"Optional Docker container filters in key:value,key:value format."`
}

type ContainerLogsInput struct {
	client.ConnectionInput
	ContainerID   string `json:"container_id,omitempty" jsonschema:"Container ID or name. Optional when container_name is provided."`
	ContainerName string `json:"container_name,omitempty" jsonschema:"Container name. Used when container_id is not provided."`
	Tail          string `json:"tail,omitempty" jsonschema:"Number of lines from the end. Defaults to 300."`
	Since         string `json:"since,omitempty" jsonschema:"Show logs since this timestamp or duration."`
	Until         string `json:"until,omitempty" jsonschema:"Show logs until this timestamp."`
	Keyword       string `json:"keyword,omitempty" jsonschema:"Only return log lines containing this keyword, case-insensitive."`
	Timestamps    bool   `json:"timestamps,omitempty" jsonschema:"Include timestamps."`
	Details       bool   `json:"details,omitempty" jsonschema:"Include extra log attributes."`
}

type ContainerInspectInput struct {
	client.ConnectionInput
	ContainerID   string `json:"container_id,omitempty" jsonschema:"Container ID or name. Optional when container_name is provided."`
	ContainerName string `json:"container_name,omitempty" jsonschema:"Container name. Used when container_id is not provided."`
}

type ContainerStatsInput struct {
	client.ConnectionInput
	ContainerID   string `json:"container_id,omitempty" jsonschema:"Container ID or name. Optional when container_name is provided."`
	ContainerName string `json:"container_name,omitempty" jsonschema:"Container name. Used when container_id is not provided."`
}

type LogsOutput struct {
	ContainerID string `json:"container_id"`
	Logs        string `json:"logs"`
	Truncated   bool   `json:"truncated"`
}

type DockerInfoOutput struct {
	Info DockerInfoDTO `json:"info"`
}

func RegisterTools(s *mcp.Server) {
	// Register wrappers with an `any` output type. The SDK then omits the
	// automatically inferred outputSchema while still returning the typed value
	// as structuredContent and text, avoiding complex client-side schema JIT.
	mcp.AddTool(s, &mcp.Tool{Name: "docker_info", Description: "Read Docker Engine information.", InputSchema: toolInputSchema(nil)}, dockerInfoTool)
	mcp.AddTool(s, &mcp.Tool{Name: "docker_images", Description: "List Docker images.", InputSchema: toolInputSchema(map[string]any{
		"all":     map[string]any{"type": "boolean"},
		"filters": map[string]any{"type": "string"},
	})}, listImagesTool)
	mcp.AddTool(s, &mcp.Tool{Name: "docker_containers", Description: "List Docker containers.", InputSchema: toolInputSchema(map[string]any{
		"all":     map[string]any{"type": "boolean"},
		"limit":   map[string]any{"type": "integer"},
		"filters": map[string]any{"type": "string"},
	})}, listContainersTool)
	mcp.AddTool(s, &mcp.Tool{Name: "docker_container_logs", Description: "Read bounded, non-following logs from a Docker container.", InputSchema: toolInputSchema(map[string]any{
		"container_id":   map[string]any{"type": "string"},
		"container_name": map[string]any{"type": "string"},
		"tail":           map[string]any{"type": "string"},
		"since":          map[string]any{"type": "string"},
		"until":          map[string]any{"type": "string"},
		"keyword":        map[string]any{"type": "string"},
		"timestamps":     map[string]any{"type": "boolean"},
		"details":        map[string]any{"type": "boolean"},
	})}, containerLogsTool)
	mcp.AddTool(s, &mcp.Tool{Name: "docker_container_inspect", Description: "Inspect a Docker container.", InputSchema: toolInputSchema(map[string]any{
		"container_id":   map[string]any{"type": "string"},
		"container_name": map[string]any{"type": "string"},
	})}, containerInspectTool)
	mcp.AddTool(s, &mcp.Tool{Name: "docker_container_stats", Description: "Read one snapshot of Docker container statistics.", InputSchema: toolInputSchema(map[string]any{
		"container_id":   map[string]any{"type": "string"},
		"container_name": map[string]any{"type": "string"},
	})}, containerStatsTool)
}

func toolInputSchema(extra map[string]any) map[string]any {
	properties := map[string]any{
		"docker_host":            map[string]any{"type": "string"},
		"docker_ca":              map[string]any{"type": "string"},
		"docker_cert":            map[string]any{"type": "string"},
		"docker_key":             map[string]any{"type": "string"},
		"docker_server_name":     map[string]any{"type": "string"},
		"docker_skip_tls_verify": map[string]any{"type": "boolean", "default": false},
	}
	for name, schema := range extra {
		properties[name] = schema
	}
	return map[string]any{"type": "object", "properties": properties}
}

func dockerInfoTool(ctx context.Context, req *mcp.CallToolRequest, input DockerInfoInput) (*mcp.CallToolResult, any, error) {
	return dockerInfo(ctx, req, input)
}

func listImagesTool(ctx context.Context, req *mcp.CallToolRequest, input ListImagesInput) (*mcp.CallToolResult, any, error) {
	return listImages(ctx, req, input)
}

func listContainersTool(ctx context.Context, req *mcp.CallToolRequest, input ListContainersInput) (*mcp.CallToolResult, any, error) {
	return listContainers(ctx, req, input)
}

func containerLogsTool(ctx context.Context, req *mcp.CallToolRequest, input ContainerLogsInput) (*mcp.CallToolResult, any, error) {
	return containerLogs(ctx, req, input)
}

func containerInspectTool(ctx context.Context, req *mcp.CallToolRequest, input ContainerInspectInput) (*mcp.CallToolResult, any, error) {
	return containerInspect(ctx, req, input)
}

func containerStatsTool(ctx context.Context, req *mcp.CallToolRequest, input ContainerStatsInput) (*mcp.CallToolResult, any, error) {
	return containerStats(ctx, req, input)
}

func dockerInfo(ctx context.Context, _ *mcp.CallToolRequest, input DockerInfoInput) (*mcp.CallToolResult, DockerInfoOutput, error) {
	cli, err := client.Open(input.ConnectionInput)
	if err != nil {
		return nil, DockerInfoOutput{}, err
	}
	defer cli.Close()
	info, err := cli.Info(ctx)
	return nil, DockerInfoOutput{Info: toDockerInfo(info)}, err
}

func listImages(ctx context.Context, _ *mcp.CallToolRequest, input ListImagesInput) (*mcp.CallToolResult, ImagesOutput, error) {
	dockerFilters, err := toFilters(input.Filters)
	if err != nil {
		return nil, ImagesOutput{}, err
	}
	cli, err := client.Open(input.ConnectionInput)
	if err != nil {
		return nil, ImagesOutput{}, err
	}
	defer cli.Close()
	items, err := cli.ImageList(ctx, image.ListOptions{All: input.All, Filters: dockerFilters})
	images := make([]ImageDTO, 0, len(items))
	for _, item := range items {
		images = append(images, toImageDTO(item))
	}
	return nil, ImagesOutput{Images: images}, err
}

func listContainers(ctx context.Context, _ *mcp.CallToolRequest, input ListContainersInput) (*mcp.CallToolResult, ContainersOutput, error) {
	dockerFilters, err := toFilters(input.Filters)
	if err != nil {
		return nil, ContainersOutput{}, err
	}
	cli, err := client.Open(input.ConnectionInput)
	if err != nil {
		return nil, ContainersOutput{}, err
	}
	defer cli.Close()
	limit := input.Limit
	if limit < 0 {
		return nil, ContainersOutput{}, fmt.Errorf("limit must not be negative")
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	items, err := cli.ContainerList(ctx, container.ListOptions{All: input.All, Limit: limit, Filters: dockerFilters})
	containers := make([]ContainerDTO, 0, len(items))
	for _, item := range items {
		containers = append(containers, toContainerDTO(item))
	}
	return nil, ContainersOutput{Containers: containers}, err
}

func containerLogs(ctx context.Context, _ *mcp.CallToolRequest, input ContainerLogsInput) (*mcp.CallToolResult, LogsOutput, error) {
	id, err := resolveContainerIdentifier(input.ContainerID, input.ContainerName)
	if err != nil {
		return nil, LogsOutput{}, err
	}
	cli, err := client.Open(input.ConnectionInput)
	if err != nil {
		return nil, LogsOutput{}, err
	}
	defer cli.Close()
	tail := input.Tail
	if strings.TrimSpace(tail) == "" {
		tail = "300"
	}
	reader, err := cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true, ShowStderr: true, Since: input.Since, Until: input.Until,
		Timestamps: input.Timestamps, Follow: false, Tail: tail, Details: input.Details,
	})
	if err != nil {
		return nil, LogsOutput{}, err
	}
	defer reader.Close()
	raw, err := io.ReadAll(io.LimitReader(reader, maxLogBytes+1))
	if err != nil {
		return nil, LogsOutput{}, err
	}
	truncated := len(raw) > maxLogBytes
	if truncated {
		raw = raw[:maxLogBytes]
	}
	logs := decodeLogs(raw)
	logs = filterLogLines(logs, input.Keyword)
	return nil, LogsOutput{ContainerID: id, Logs: logs, Truncated: truncated}, nil
}

func containerInspect(ctx context.Context, _ *mcp.CallToolRequest, input ContainerInspectInput) (*mcp.CallToolResult, ContainerInspectDTO, error) {
	id, err := resolveContainerIdentifier(input.ContainerID, input.ContainerName)
	if err != nil {
		return nil, ContainerInspectDTO{}, err
	}
	cli, err := client.Open(input.ConnectionInput)
	if err != nil {
		return nil, ContainerInspectDTO{}, err
	}
	defer cli.Close()
	inspect, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return nil, ContainerInspectDTO{}, err
	}
	return nil, toInspectDTO(inspect), nil
}

func containerStats(ctx context.Context, _ *mcp.CallToolRequest, input ContainerStatsInput) (*mcp.CallToolResult, ContainerStatsDTO, error) {
	id, err := resolveContainerIdentifier(input.ContainerID, input.ContainerName)
	if err != nil {
		return nil, ContainerStatsDTO{}, err
	}
	cli, err := client.Open(input.ConnectionInput)
	if err != nil {
		return nil, ContainerStatsDTO{}, err
	}
	defer cli.Close()
	reader, err := cli.ContainerStatsOneShot(ctx, id)
	if err != nil {
		return nil, ContainerStatsDTO{}, err
	}
	defer reader.Body.Close()
	var stats container.StatsResponse
	if err := json.NewDecoder(io.LimitReader(reader.Body, maxStatsBytes)).Decode(&stats); err != nil {
		return nil, ContainerStatsDTO{}, fmt.Errorf("decode container stats: %w", err)
	}
	return nil, toStatsDTO(stats), nil
}

func resolveContainerIdentifier(containerID, containerName string) (string, error) {
	if id := strings.TrimSpace(containerID); id != "" {
		return id, nil
	}
	if name := strings.TrimSpace(containerName); name != "" {
		return name, nil
	}
	return "", fmt.Errorf("container_id or container_name is required")
}

func toFilters(raw string) (filters.Args, error) {
	args := filters.NewArgs()
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return args, nil
	}
	for _, item := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(item, ":")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !ok || key == "" || value == "" {
			return filters.Args{}, fmt.Errorf("invalid filter %q: expected key:value", strings.TrimSpace(item))
		}
		args.Add(key, value)
	}
	return args, nil
}

func decodeLogs(raw []byte) string {
	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, bytes.NewReader(raw)); err == nil {
		return stdout.String() + stderr.String()
	}
	return string(raw)
}

// filterLogLines applies keyword matching after Docker has applied its time
// range and tail selection. Matching is case-insensitive and preserves each
// selected line's original newline delimiter.
func filterLogLines(logs, keyword string) string {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || logs == "" {
		return logs
	}
	foldedKeyword := strings.ToLower(keyword)
	var filtered strings.Builder
	for _, line := range strings.SplitAfter(logs, "\n") {
		if strings.Contains(strings.ToLower(strings.TrimSuffix(line, "\n")), foldedKeyword) {
			filtered.WriteString(line)
		}
	}
	return filtered.String()
}

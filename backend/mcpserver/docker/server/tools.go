package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	dockerapi "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"opskeeper/backend/mcpserver/docker/client"
)

const (
	// Read enough raw data for 200 verbose log lines (including stack traces)
	// before keyword filtering and the final line-based limit are applied.
	maxLogBytes         = 2 * 1024 * 1024
	maxStatsBytes       = 2 * 1024 * 1024
	maxListLimit        = 500
	defaultLogTail      = 1000
	maxOutputLogLines   = 200
	keywordContextLines = 3
)

type DockerInfoInput struct{ client.ConnectionInput }

type ListImagesInput struct {
	client.ConnectionInput
	All     bool   `json:"all,omitempty" jsonschema:"Include intermediate and dangling images."`
	Filters string `json:"filters,omitempty" jsonschema:"Optional Docker image filters in key:value or key=value format."`
}

type ListContainersInput struct {
	client.ConnectionInput
	All     bool   `json:"all,omitempty" jsonschema:"Include stopped containers."`
	Limit   int    `json:"limit,omitempty" jsonschema:"Maximum number of containers to return. Maximum 500."`
	Filters string `json:"filters,omitempty" jsonschema:"Optional Docker container filters in key:value or key=value format."`
}

type ContainerLogsInput struct {
	client.ConnectionInput
	ContainerID   string `json:"container_id,omitempty" jsonschema:"Container ID or name. Optional when container_name is provided."`
	ContainerName string `json:"container_name,omitempty" jsonschema:"Container name. Used when container_id is not provided."`
	Tail          string `json:"tail,omitempty" jsonschema:"Number of lines requested from the end. Defaults to 1000; keyword filtering is applied before the final output is limited to the most recent 200 lines."`
	Since         string `json:"since,omitempty" jsonschema:"Show logs since this timestamp or duration."`
	Until         string `json:"until,omitempty" jsonschema:"Show logs until this timestamp."`
	Keyword       string `json:"keyword,omitempty" jsonschema:"Case-insensitive keyword filter. Returns each matching line plus up to 3 lines before and after it; overlapping context is shown only once."`
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
	ContainerID        string                            `json:"container_id"`
	Logs               string                            `json:"logs"`
	Truncated          bool                              `json:"truncated"`
	ConnectionFallback *client.ConnectionFallbackWarning `json:"connection_fallback,omitempty"`
}

type DockerInfoOutput struct {
	Info               DockerInfoDTO                     `json:"info"`
	ConnectionFallback *client.ConnectionFallbackWarning `json:"connection_fallback,omitempty"`
}

func RegisterTools(s *mcp.Server) {
	// Register wrappers with an `any` output type. The SDK then omits the
	// automatically inferred outputSchema while still returning the typed value
	// as structuredContent and text, avoiding complex client-side schema JIT.
	mcp.AddTool(s, &mcp.Tool{Name: "docker_info", Description: "Read Docker Engine information.", InputSchema: toolInputSchema(nil)}, dockerInfoTool)
	mcp.AddTool(s, &mcp.Tool{Name: "docker_images", Description: "List Docker images.", InputSchema: toolInputSchema(map[string]any{
		"all":     map[string]any{"type": "boolean", "description": "Include intermediate and dangling images."},
		"filters": map[string]any{"type": "string", "description": "Optional comma-separated Docker filters in key:value or key=value format, for example label=app:web,label:com.example.env=prod,reference=nginx:latest. Each item is split at its first separator."},
	})}, listImagesTool)
	mcp.AddTool(s, &mcp.Tool{Name: "docker_containers", Description: "List Docker containers.", InputSchema: toolInputSchema(map[string]any{
		"all":     map[string]any{"type": "boolean", "description": "Include stopped containers."},
		"limit":   map[string]any{"type": "integer", "description": "Maximum number of containers to return; 0 uses the Docker default and the maximum is 500."},
		"filters": map[string]any{"type": "string", "description": "Optional comma-separated Docker filters in key:value or key=value format, for example status=running,label:app=web. Each item is split at its first separator."},
	})}, listContainersTool)
	mcp.AddTool(s, &mcp.Tool{Name: "docker_container_logs", Description: "Read bounded, non-following logs from a Docker container.", InputSchema: toolInputSchema(map[string]any{
		"container_id":   map[string]any{"type": "string", "description": "Container ID or name; takes precedence over container_name."},
		"container_name": map[string]any{"type": "string", "description": "Container name when container_id is omitted."},
		"tail":           map[string]any{"type": "string", "description": "Number of lines requested from the end; defaults to 1000. Keyword filtering is applied first, then output is limited to the most recent 200 lines."},
		"since":          map[string]any{"type": "string", "description": "Only show logs since this timestamp or duration."},
		"until":          map[string]any{"type": "string", "description": "Only show logs until this timestamp."},
		"keyword":        map[string]any{"type": "string", "description": "Case-insensitive keyword filter; returns each matching line plus up to 3 lines before and after it, without duplicate lines when contexts overlap."},
		"timestamps":     map[string]any{"type": "boolean", "description": "Include timestamps in log lines."},
		"details":        map[string]any{"type": "boolean", "description": "Include extra Docker log attributes."},
	})}, containerLogsTool)
	mcp.AddTool(s, &mcp.Tool{Name: "docker_container_inspect", Description: "Inspect a Docker container.", InputSchema: toolInputSchema(map[string]any{
		"container_id":   map[string]any{"type": "string", "description": "Container ID or name; takes precedence over container_name."},
		"container_name": map[string]any{"type": "string", "description": "Container name when container_id is omitted."},
	})}, containerInspectTool)
	mcp.AddTool(s, &mcp.Tool{Name: "docker_container_stats", Description: "Read one snapshot of Docker container statistics.", InputSchema: toolInputSchema(map[string]any{
		"container_id":   map[string]any{"type": "string", "description": "Container ID or name; takes precedence over container_name."},
		"container_name": map[string]any{"type": "string", "description": "Container name when container_id is omitted."},
	})}, containerStatsTool)
}

func toolInputSchema(extra map[string]any) map[string]any {
	properties := map[string]any{
		"docker_host":            map[string]any{"type": "string", "description": "Optional Docker daemon URL (unix:///var/run/docker.sock, http[s]://host:port). Connection failures fall back to the default connection."},
		"docker_ca":              map[string]any{"type": "string", "description": "Optional path to a CA PEM file for an HTTPS Docker daemon."},
		"docker_cert":            map[string]any{"type": "string", "description": "Optional path to the Docker client certificate PEM file; use together with docker_key."},
		"docker_key":             map[string]any{"type": "string", "description": "Optional path to the Docker client private key PEM file; use together with docker_cert."},
		"docker_server_name":     map[string]any{"type": "string", "description": "Optional TLS server name override."},
		"docker_skip_tls_verify": map[string]any{"type": "boolean", "default": false, "description": "Skip TLS certificate verification for explicitly trusted development endpoints."},
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
	var info DockerInfoDTO
	warning, err := client.WithFallback(ctx, input.ConnectionInput, func(cli *dockerapi.Client) error {
		value, callErr := cli.Info(ctx)
		if callErr == nil {
			info = toDockerInfo(value)
		}
		return callErr
	})
	return nil, DockerInfoOutput{Info: info, ConnectionFallback: warning}, err
}

func listImages(ctx context.Context, _ *mcp.CallToolRequest, input ListImagesInput) (*mcp.CallToolResult, ImagesOutput, error) {
	dockerFilters, err := toFilters(input.Filters)
	if err != nil {
		return nil, ImagesOutput{}, err
	}
	var items []image.Summary
	warning, err := client.WithFallback(ctx, input.ConnectionInput, func(cli *dockerapi.Client) error {
		var callErr error
		items, callErr = cli.ImageList(ctx, image.ListOptions{All: input.All, Filters: dockerFilters})
		return callErr
	})
	images := make([]ImageDTO, 0, len(items))
	for _, item := range items {
		images = append(images, toImageDTO(item))
	}
	return nil, ImagesOutput{Images: images, ConnectionFallback: warning}, err
}

func listContainers(ctx context.Context, _ *mcp.CallToolRequest, input ListContainersInput) (*mcp.CallToolResult, ContainersOutput, error) {
	dockerFilters, err := toFilters(input.Filters)
	if err != nil {
		return nil, ContainersOutput{}, err
	}
	limit := input.Limit
	if limit < 0 {
		return nil, ContainersOutput{}, fmt.Errorf("limit must not be negative")
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	var items []container.Summary
	warning, err := client.WithFallback(ctx, input.ConnectionInput, func(cli *dockerapi.Client) error {
		var callErr error
		items, callErr = cli.ContainerList(ctx, container.ListOptions{All: input.All, Limit: limit, Filters: dockerFilters})
		return callErr
	})
	containers := make([]ContainerDTO, 0, len(items))
	for _, item := range items {
		containers = append(containers, toContainerDTO(item))
	}
	return nil, ContainersOutput{Containers: containers, ConnectionFallback: warning}, err
}

func containerLogs(ctx context.Context, _ *mcp.CallToolRequest, input ContainerLogsInput) (*mcp.CallToolResult, LogsOutput, error) {
	id, err := resolveContainerIdentifier(input.ContainerID, input.ContainerName)
	if err != nil {
		return nil, LogsOutput{}, err
	}
	tail := input.Tail
	if strings.TrimSpace(tail) == "" {
		tail = strconv.Itoa(defaultLogTail)
	}
	var logs string
	var truncated bool
	warning, err := client.WithFallback(ctx, input.ConnectionInput, func(cli *dockerapi.Client) error {
		reader, callErr := cli.ContainerLogs(ctx, id, container.LogsOptions{
			ShowStdout: true, ShowStderr: true, Since: input.Since, Until: input.Until,
			Timestamps: input.Timestamps, Follow: false, Tail: tail, Details: input.Details,
		})
		if callErr != nil {
			return callErr
		}
		defer reader.Close()
		raw, callErr := io.ReadAll(io.LimitReader(reader, maxLogBytes+1))
		if callErr != nil {
			return callErr
		}
		truncated = len(raw) > maxLogBytes
		if truncated {
			raw = raw[:maxLogBytes]
		}
		// Docker applies the time range and requested tail first. Expand keyword
		// matches with context, then apply the final response line cap.
		logs = limitLogLines(filterLogLines(decodeLogs(raw), input.Keyword), maxOutputLogLines)
		return nil
	})
	return nil, LogsOutput{ContainerID: id, Logs: logs, Truncated: truncated, ConnectionFallback: warning}, err
}

func containerInspect(ctx context.Context, _ *mcp.CallToolRequest, input ContainerInspectInput) (*mcp.CallToolResult, ContainerInspectDTO, error) {
	id, err := resolveContainerIdentifier(input.ContainerID, input.ContainerName)
	if err != nil {
		return nil, ContainerInspectDTO{}, err
	}
	var output ContainerInspectDTO
	warning, err := client.WithFallback(ctx, input.ConnectionInput, func(cli *dockerapi.Client) error {
		inspect, callErr := cli.ContainerInspect(ctx, id)
		if callErr == nil {
			output = toInspectDTO(inspect)
		}
		return callErr
	})
	output.ConnectionFallback = warning
	return nil, output, err
}

func containerStats(ctx context.Context, _ *mcp.CallToolRequest, input ContainerStatsInput) (*mcp.CallToolResult, ContainerStatsDTO, error) {
	id, err := resolveContainerIdentifier(input.ContainerID, input.ContainerName)
	if err != nil {
		return nil, ContainerStatsDTO{}, err
	}
	var output ContainerStatsDTO
	warning, err := client.WithFallback(ctx, input.ConnectionInput, func(cli *dockerapi.Client) error {
		reader, callErr := cli.ContainerStatsOneShot(ctx, id)
		if callErr != nil {
			return callErr
		}
		defer reader.Body.Close()
		var stats container.StatsResponse
		if callErr := json.NewDecoder(io.LimitReader(reader.Body, maxStatsBytes)).Decode(&stats); callErr != nil {
			return fmt.Errorf("decode container stats: %w", callErr)
		}
		output = toStatsDTO(stats)
		return nil
	})
	output.ConnectionFallback = warning
	return nil, output, err
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
		item = strings.TrimSpace(item)
		separator := strings.IndexAny(item, ":=")
		ok := separator >= 0
		key, value := "", ""
		if ok {
			key, value = item[:separator], item[separator+1:]
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !ok || key == "" || value == "" {
			return filters.Args{}, fmt.Errorf("invalid filter %q: expected key:value or key=value", item)
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

// limitLogLines keeps the most recent maxLines while preserving each selected
// line's original newline delimiter.
func limitLogLines(logs string, maxLines int) string {
	if logs == "" || maxLines <= 0 {
		return ""
	}
	lines := strings.SplitAfter(logs, "\n")
	end := len(lines)
	if end > 0 && lines[end-1] == "" {
		end--
	}
	if end <= maxLines {
		return logs
	}
	return strings.Join(lines[end-maxLines:end], "")
}

// filterLogLines returns each case-insensitive keyword match and up to three
// surrounding lines. A marked-line set merges overlapping context windows so
// no line is displayed more than once.
func filterLogLines(logs, keyword string) string {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || logs == "" {
		return logs
	}
	foldedKeyword := strings.ToLower(keyword)
	lines := strings.SplitAfter(logs, "\n")
	selected := make([]bool, len(lines))
	for index, line := range lines {
		if !strings.Contains(strings.ToLower(strings.TrimRight(line, "\r\n")), foldedKeyword) {
			continue
		}
		start := index - keywordContextLines
		if start < 0 {
			start = 0
		}
		end := index + keywordContextLines + 1
		if end > len(lines) {
			end = len(lines)
		}
		for contextIndex := start; contextIndex < end; contextIndex++ {
			selected[contextIndex] = true
		}
	}
	var filtered strings.Builder
	for index, line := range lines {
		if selected[index] {
			filtered.WriteString(line)
		}
	}
	return filtered.String()
}

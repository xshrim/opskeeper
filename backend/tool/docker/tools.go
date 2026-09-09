package docker

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
	Keyword       string `json:"keyword,omitempty" jsonschema:"Case-insensitive keyword filter. Separate multiple terms with & for AND (all terms must occur on the same line) or | for OR (any term matches). AND binds tighter than OR. Each matching line is returned with up to 3 lines before and after it; overlapping context is shown only once."`
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

// InputSchema returns the stable MCP-facing schema for the Docker tool set.
// The schema is kept in the protocol-neutral package so Direct and MCP use
// the same public parameter contract.
func InputSchema(extra map[string]any) map[string]any {
	properties := map[string]any{
		"host":            map[string]any{"type": "string", "description": "Optional Docker daemon URL."},
		"timeout":         map[string]any{"description": "Docker request timeout in seconds or duration such as 30s. Defaults to 30s."},
		"tls_ca":          map[string]any{"type": "string", "description": "Optional CA PEM content encoded as Base64, or a path to a CA PEM file."},
		"tls_cert":        map[string]any{"type": "string", "description": "Optional client certificate PEM content encoded as Base64, or a path to a PEM file."},
		"tls_key":         map[string]any{"type": "string", "description": "Optional client private key PEM content encoded as Base64, or a path to a private key PEM file."},
		"skip_tls_verify": map[string]any{"type": "boolean", "default": false, "description": "Skip TLS certificate verification."},
	}
	for name, schema := range extra {
		properties[name] = schema
	}
	return map[string]any{"type": "object", "properties": properties}
}

func Info(ctx context.Context, input DockerInfoInput) (DockerInfoOutput, error) {
	var info DockerInfoDTO
	warning, err := client.WithFallback(ctx, input.ConnectionInput, func(cli *dockerapi.Client) error {
		value, callErr := cli.Info(ctx)
		if callErr == nil {
			info = toDockerInfo(value)
		}
		return callErr
	})
	return DockerInfoOutput{Info: info, ConnectionFallback: warning}, err
}

func Images(ctx context.Context, input ListImagesInput) (ImagesOutput, error) {
	dockerFilters, err := toFilters(input.Filters)
	if err != nil {
		return ImagesOutput{}, err
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
	return ImagesOutput{Images: images, ConnectionFallback: warning}, err
}

func Containers(ctx context.Context, input ListContainersInput) (ContainersOutput, error) {
	dockerFilters, err := toFilters(input.Filters)
	if err != nil {
		return ContainersOutput{}, err
	}
	limit := input.Limit
	if limit < 0 {
		return ContainersOutput{}, fmt.Errorf("limit must not be negative")
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
	return ContainersOutput{Containers: containers, ConnectionFallback: warning}, err
}

func ContainerLogs(ctx context.Context, input ContainerLogsInput) (LogsOutput, error) {
	id, err := resolveContainerIdentifier(input.ContainerID, input.ContainerName)
	if err != nil {
		return LogsOutput{}, err
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
	return LogsOutput{ContainerID: id, Logs: logs, Truncated: truncated, ConnectionFallback: warning}, err
}

func ContainerInspect(ctx context.Context, input ContainerInspectInput) (ContainerInspectDTO, error) {
	id, err := resolveContainerIdentifier(input.ContainerID, input.ContainerName)
	if err != nil {
		return ContainerInspectDTO{}, err
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
	return output, err
}

func ContainerStats(ctx context.Context, input ContainerStatsInput) (ContainerStatsDTO, error) {
	id, err := resolveContainerIdentifier(input.ContainerID, input.ContainerName)
	if err != nil {
		return ContainerStatsDTO{}, err
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
	return output, err
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

func ResolveContainerIdentifier(containerID, containerName string) (string, error) {
	return resolveContainerIdentifier(containerID, containerName)
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

func ParseFilters(raw string) (filters.Args, error) { return toFilters(raw) }

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

func LimitLogLines(logs string, maxLines int) string { return limitLogLines(logs, maxLines) }

// filterLogLines returns each line matching the keyword expression and up to
// three surrounding lines. Terms joined by & must all match the same line;
// terms joined by | form alternatives. AND binds tighter than OR, so
// "error&timeout|fatal" means (error AND timeout) OR fatal. A marked-line set
// merges overlapping context windows so no line is displayed more than once.
func filterLogLines(logs, keyword string) string {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || logs == "" {
		return logs
	}
	clauses := parseKeywordExpression(keyword)
	if len(clauses) == 0 {
		return logs
	}
	lines := strings.SplitAfter(logs, "\n")
	selected := make([]bool, len(lines))
	for index, line := range lines {
		foldedLine := strings.ToLower(strings.TrimRight(line, "\r\n"))
		if !matchesKeywordExpression(foldedLine, clauses) {
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

func FilterLogLines(logs, keyword string) string { return filterLogLines(logs, keyword) }

// parseKeywordExpression implements the documented keyword grammar. Empty
// terms are ignored so accidental repeated or trailing separators do not turn
// a useful filter into a match-nothing expression.
func parseKeywordExpression(keyword string) [][]string {
	groups := make([][]string, 0)
	for _, alternative := range strings.Split(keyword, "|") {
		terms := make([]string, 0)
		for _, term := range strings.Split(alternative, "&") {
			term = strings.ToLower(strings.TrimSpace(term))
			if term != "" {
				terms = append(terms, term)
			}
		}
		if len(terms) > 0 {
			groups = append(groups, terms)
		}
	}
	return groups
}

func ParseKeywordExpression(keyword string) [][]string { return parseKeywordExpression(keyword) }

func matchesKeywordExpression(line string, clauses [][]string) bool {
	for _, clause := range clauses {
		matches := true
		for _, term := range clause {
			if !strings.Contains(line, term) {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

func MatchesKeywordExpression(line string, clauses [][]string) bool {
	return matchesKeywordExpression(line, clauses)
}

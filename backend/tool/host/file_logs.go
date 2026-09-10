package host

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func FileLogs(ctx context.Context, input FileLogsInput) (FileLogsOutput, error) {
	ctx = contextOrBackground(ctx)
	path, err := safePath(input.Path)
	if err != nil {
		return FileLogsOutput{}, err
	}
	src, target, err := openSource(ctx, input.ConnectionInput)
	if err != nil {
		return FileLogsOutput{}, err
	}
	defer src.Close()
	raw, err := readLimited(ctx, src, path)
	if err != nil {
		return FileLogsOutput{}, fmt.Errorf("read log file: %w", err)
	}
	truncated := len(raw) >= MaxReadBytes
	lines := splitLogLines(string(raw))
	since, until, err := parseLogBounds(input.Since, input.Until)
	if err != nil {
		return FileLogsOutput{}, err
	}
	filtered := make([]string, 0, len(lines))
	keyword := strings.ToLower(strings.TrimSpace(input.Keyword))
	for _, line := range lines {
		if since != nil || until != nil {
			timestamp := lineTimestamp(line)
			if timestamp != nil && since != nil && timestamp.Before(*since) {
				continue
			}
			if timestamp != nil && until != nil && timestamp.After(*until) {
				continue
			}
		}
		if keyword != "" && !matchKeyword(line, keyword) {
			continue
		}
		filtered = append(filtered, line)
	}
	limit, err := parseTail(input.Tail)
	if err != nil {
		return FileLogsOutput{}, err
	}
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return FileLogsOutput{SchemaVersion: 1, CollectedAt: time.Now().UTC(), Target: target, Path: path, Logs: strings.Join(filtered, "\n"), Truncated: truncated, MatchedLines: len(filtered)}, nil
}

func parseTail(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultLogLines, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("%w: tail must be a non-negative integer", ErrInvalidArgument)
	}
	if value > 10000 {
		value = 10000
	}
	return value, nil
}
func splitLogLines(raw string) []string {
	raw = strings.TrimSuffix(raw, "\n")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\n")
}
func parseLogBounds(since, until string) (*time.Time, *time.Time, error) {
	parse := func(raw string) (*time.Time, error) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil, nil
		}
		if duration, err := time.ParseDuration(raw); err == nil {
			value := time.Now().Add(-duration)
			return &value, nil
		}
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, fmt.Errorf("%w: since/until must be RFC3339 or duration", ErrInvalidArgument)
		}
		return &value, nil
	}
	start, err := parse(since)
	if err != nil {
		return nil, nil, err
	}
	end, err := parse(until)
	if err != nil {
		return nil, nil, err
	}
	return start, end, nil
}
func lineTimestamp(line string) *time.Time {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}
	value := strings.Trim(fields[0], "[]")
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		parsed, err = time.Parse("2006-01-02 15:04:05", value)
	}
	if err != nil {
		return nil
	}
	return &parsed
}
func matchKeyword(line, expression string) bool {
	for _, clause := range strings.Split(expression, "|") {
		matched := true
		for _, term := range strings.Split(clause, "&") {
			term = strings.TrimSpace(term)
			if term != "" && !strings.Contains(strings.ToLower(line), term) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

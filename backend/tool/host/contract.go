// Package host exposes read-only Linux host tools shared by Direct resources
// and the Host MCP agent.
package host

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultTimeout      = 30 * time.Second
	MaxTimeout          = 5 * time.Minute
	MaxReadBytes        = 2 << 20
	MaxProcessLimit     = 100
	DefaultProcessLimit = 20
	MaxLogLines         = 200
	DefaultLogLines     = 1000
	MaxSampleSeconds    = 10
)

// ConnectionInput is intentionally shared by every Host tool. Sensitive
// fields are accepted by the MCP transport but are never included in output.
type ConnectionInput struct {
	Host           string `json:"host,omitempty"`
	Port           int    `json:"port,omitempty"`
	Username       string `json:"username,omitempty"`
	AuthMethod     string `json:"auth_method,omitempty"`
	Password       string `json:"password,omitempty"`
	PrivateKey     string `json:"private_key,omitempty"`
	Passphrase     string `json:"passphrase,omitempty"`
	KnownHosts     string `json:"known_hosts,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

type InfoInput struct{ ConnectionInput }
type MetricsInput struct {
	ConnectionInput
	SampleSeconds int `json:"sample_seconds,omitempty"`
}
type ProcessesInput struct {
	ConnectionInput
	PID     int    `json:"pid,omitempty"`
	Keyword string `json:"keyword,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}
type FileLogsInput struct {
	ConnectionInput
	Path       string `json:"path"`
	Tail       string `json:"tail,omitempty"`
	Since      string `json:"since,omitempty"`
	Until      string `json:"until,omitempty"`
	Keyword    string `json:"keyword,omitempty"`
	Timestamps bool   `json:"timestamps,omitempty"`
}

// InputSchema is the MCP-facing schema. Adapters remove connection fields
// from this schema when resource-owned connection settings are injected.
func InputSchema(extra map[string]any) map[string]any {
	properties := map[string]any{
		"host":            map[string]any{"type": "string", "description": "Optional Linux host. Empty selects the local host."},
		"port":            map[string]any{"type": "integer", "minimum": 1, "maximum": 65535, "description": "SSH port; defaults to 22."},
		"username":        map[string]any{"type": "string", "description": "SSH username."},
		"auth_method":     map[string]any{"type": "string", "enum": []string{"password", "key"}, "description": "SSH authentication method. Empty infers it from password or private_key."},
		"password":        map[string]any{"type": "string", "format": "password", "x-sensitive": true, "description": "SSH password."},
		"private_key":     map[string]any{"type": "string", "format": "password", "x-sensitive": true, "description": "PEM or Base64 encoded SSH private key."},
		"passphrase":      map[string]any{"type": "string", "format": "password", "x-sensitive": true, "description": "Private key passphrase."},
		"known_hosts":     map[string]any{"type": "string", "format": "password", "x-sensitive": true, "description": "Optional known_hosts file content. When empty, SSH host-key verification is skipped."},
		"timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": int(MaxTimeout / time.Second), "description": "Connection and read timeout in seconds."},
	}
	for name, schema := range extra {
		properties[name] = schema
	}
	return map[string]any{"type": "object", "properties": properties}
}

type InfoOutput struct {
	SchemaVersion int       `json:"schema_version"`
	CollectedAt   time.Time `json:"collected_at"`
	Target        Target    `json:"target"`
	Hostname      string    `json:"hostname,omitempty"`
	Kernel        string    `json:"kernel,omitempty"`
	Architecture  string    `json:"architecture,omitempty"`
	OS            OSInfo    `json:"os"`
	BootTime      time.Time `json:"boot_time,omitempty"`
	UptimeSeconds float64   `json:"uptime_seconds,omitempty"`
	CPUs          int       `json:"cpus,omitempty"`
	MemoryBytes   uint64    `json:"memory_bytes,omitempty"`
	Runtime       Runtime   `json:"runtime"`
	Partial       bool      `json:"partial"`
	Unavailable   []string  `json:"unavailable,omitempty"`
}

type Target struct {
	Mode     string `json:"mode"`
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Username string `json:"username,omitempty"`
}
type OSInfo struct {
	Name       string `json:"name,omitempty"`
	Version    string `json:"version,omitempty"`
	ID         string `json:"id,omitempty"`
	PrettyName string `json:"pretty_name,omitempty"`
}
type Runtime struct {
	Containerized bool   `json:"containerized,omitempty"`
	Virtualized   string `json:"virtualized,omitempty"`
}

type MetricsOutput struct {
	SchemaVersion int         `json:"schema_version"`
	CollectedAt   time.Time   `json:"collected_at"`
	Target        Target      `json:"target"`
	Host          HostSummary `json:"host"`
	Metrics       HostMetrics `json:"metrics"`
	Partial       bool        `json:"partial"`
	Unavailable   []string    `json:"unavailable,omitempty"`
}
type HostSummary struct {
	Hostname string `json:"hostname,omitempty"`
	Kernel   string `json:"kernel,omitempty"`
	Arch     string `json:"arch,omitempty"`
}
type HostMetrics struct {
	Load       LoadMetrics          `json:"load"`
	CPU        CPUMetrics           `json:"cpu"`
	Memory     MemoryMetrics        `json:"memory"`
	Swap       SwapMetrics          `json:"swap"`
	Filesystem []FilesystemMetric   `json:"filesystem,omitempty"`
	Disk       []DiskMetric         `json:"disk,omitempty"`
	Network    []NetworkMetric      `json:"network,omitempty"`
	Pressure   map[string]PSIMetric `json:"pressure,omitempty"`
	Processes  ProcessSummary       `json:"processes"`
}
type LoadMetrics struct {
	One     float64 `json:"one,omitempty"`
	Five    float64 `json:"five,omitempty"`
	Fifteen float64 `json:"fifteen,omitempty"`
}
type CPUMetrics struct {
	UsagePercent    float64            `json:"usage_percent,omitempty"`
	PerCPU          map[string]float64 `json:"per_cpu_percent,omitempty"`
	UserSeconds     float64            `json:"user_seconds,omitempty"`
	SystemSeconds   float64            `json:"system_seconds,omitempty"`
	IdleSeconds     float64            `json:"idle_seconds,omitempty"`
	ContextSwitches uint64             `json:"context_switches,omitempty"`
	Running         int                `json:"running,omitempty"`
	Blocked         int                `json:"blocked,omitempty"`
}
type MemoryMetrics struct {
	TotalBytes     uint64 `json:"total_bytes,omitempty"`
	FreeBytes      uint64 `json:"free_bytes,omitempty"`
	AvailableBytes uint64 `json:"available_bytes,omitempty"`
	BuffersBytes   uint64 `json:"buffers_bytes,omitempty"`
	CachedBytes    uint64 `json:"cached_bytes,omitempty"`
	UsedBytes      uint64 `json:"used_bytes,omitempty"`
}
type SwapMetrics struct {
	TotalBytes uint64 `json:"total_bytes,omitempty"`
	FreeBytes  uint64 `json:"free_bytes,omitempty"`
	UsedBytes  uint64 `json:"used_bytes,omitempty"`
}
type FilesystemMetric struct {
	Mountpoint  string  `json:"mountpoint,omitempty"`
	FSType      string  `json:"fs_type,omitempty"`
	TotalBytes  uint64  `json:"total_bytes,omitempty"`
	FreeBytes   uint64  `json:"free_bytes,omitempty"`
	UsedBytes   uint64  `json:"used_bytes,omitempty"`
	UsedPercent float64 `json:"used_percent,omitempty"`
}
type DiskMetric struct {
	Device          string `json:"device"`
	ReadsCompleted  uint64 `json:"reads_completed,omitempty"`
	WritesCompleted uint64 `json:"writes_completed,omitempty"`
	ReadBytes       uint64 `json:"read_bytes,omitempty"`
	WriteBytes      uint64 `json:"write_bytes,omitempty"`
	ReadTimeMillis  uint64 `json:"read_time_millis,omitempty"`
	WriteTimeMillis uint64 `json:"write_time_millis,omitempty"`
}
type NetworkMetric struct {
	Interface       string `json:"interface"`
	ReceiveBytes    uint64 `json:"receive_bytes,omitempty"`
	TransmitBytes   uint64 `json:"transmit_bytes,omitempty"`
	ReceivePackets  uint64 `json:"receive_packets,omitempty"`
	TransmitPackets uint64 `json:"transmit_packets,omitempty"`
	ReceiveErrors   uint64 `json:"receive_errors,omitempty"`
	TransmitErrors  uint64 `json:"transmit_errors,omitempty"`
	ReceiveDrops    uint64 `json:"receive_drops,omitempty"`
	TransmitDrops   uint64 `json:"transmit_drops,omitempty"`
}
type PSIMetric struct {
	SomeAvg10  float64 `json:"some_avg10,omitempty"`
	SomeAvg60  float64 `json:"some_avg60,omitempty"`
	SomeAvg300 float64 `json:"some_avg300,omitempty"`
	FullAvg10  float64 `json:"full_avg10,omitempty"`
	FullAvg60  float64 `json:"full_avg60,omitempty"`
	FullAvg300 float64 `json:"full_avg300,omitempty"`
}
type ProcessSummary struct {
	Total    int `json:"total,omitempty"`
	Running  int `json:"running,omitempty"`
	Sleeping int `json:"sleeping,omitempty"`
	Stopped  int `json:"stopped,omitempty"`
	Zombie   int `json:"zombie,omitempty"`
}

type ProcessesOutput struct {
	SchemaVersion int           `json:"schema_version"`
	CollectedAt   time.Time     `json:"collected_at"`
	Target        Target        `json:"target"`
	Processes     []ProcessInfo `json:"processes"`
	MatchCount    int           `json:"match_count"`
	Truncated     bool          `json:"truncated"`
	Partial       bool          `json:"partial"`
	Unavailable   []string      `json:"unavailable,omitempty"`
}
type ProcessInfo struct {
	PID             int       `json:"pid"`
	PPID            int       `json:"ppid,omitempty"`
	State           string    `json:"state,omitempty"`
	Name            string    `json:"name,omitempty"`
	Executable      string    `json:"executable,omitempty"`
	CWD             string    `json:"cwd,omitempty"`
	Root            string    `json:"root,omitempty"`
	UID             uint64    `json:"uid,omitempty"`
	User            string    `json:"user,omitempty"`
	StartTime       time.Time `json:"start_time,omitempty"`
	Threads         int       `json:"threads,omitempty"`
	FileDescriptors int       `json:"file_descriptors,omitempty"`
	CPUTimeSeconds  float64   `json:"cpu_time_seconds,omitempty"`
	CPUUsagePercent float64   `json:"cpu_usage_percent,omitempty"`
	RSSBytes        uint64    `json:"rss_bytes,omitempty"`
	VirtualBytes    uint64    `json:"virtual_bytes,omitempty"`
	MemoryPercent   float64   `json:"memory_percent,omitempty"`
	ReadBytes       uint64    `json:"read_bytes,omitempty"`
	WriteBytes      uint64    `json:"write_bytes,omitempty"`
	CommandLine     string    `json:"command_line,omitempty"`
}

type FileLogsOutput struct {
	SchemaVersion int       `json:"schema_version"`
	CollectedAt   time.Time `json:"collected_at"`
	Target        Target    `json:"target"`
	Path          string    `json:"path"`
	Logs          string    `json:"logs"`
	Truncated     bool      `json:"truncated"`
	MatchedLines  int       `json:"matched_lines,omitempty"`
}

var (
	ErrInvalidArgument = errors.New("invalid host tool argument")
	ErrSSHUnavailable  = errors.New("SSH connection unavailable")
)

func resolveInput(input ConnectionInput) (ConnectionInput, Target, error) {
	input.Host = strings.TrimSpace(input.Host)
	input.Username = strings.TrimSpace(input.Username)
	input.AuthMethod = strings.ToLower(strings.TrimSpace(input.AuthMethod))
	if input.Port < 0 || input.Port > 65535 {
		return ConnectionInput{}, Target{}, fmt.Errorf("%w: port must be between 1 and 65535", ErrInvalidArgument)
	}
	if input.TimeoutSeconds < 0 {
		return ConnectionInput{}, Target{}, fmt.Errorf("%w: timeout_seconds must be positive", ErrInvalidArgument)
	}
	if input.Host == "" {
		input.Host = envFirst("HOST_MCP_HOST", "HOST_MCP_SSH_HOST")
	}
	if input.Port == 0 {
		input.Port = envInt("HOST_MCP_PORT", "HOST_MCP_SSH_PORT")
	}
	if input.Username == "" {
		input.Username = envFirst("HOST_MCP_USERNAME", "HOST_MCP_SSH_USERNAME")
	}
	if input.AuthMethod == "" {
		input.AuthMethod = strings.ToLower(envFirst("HOST_MCP_AUTH_METHOD", "HOST_MCP_SSH_AUTH_METHOD"))
	}
	if input.Password == "" {
		input.Password = envFirst("HOST_MCP_PASSWORD", "HOST_MCP_SSH_PASSWORD")
	}
	if input.PrivateKey == "" {
		input.PrivateKey = envFirst("HOST_MCP_PRIVATE_KEY", "HOST_MCP_SSH_PRIVATE_KEY")
	}
	if input.Passphrase == "" {
		input.Passphrase = envFirst("HOST_MCP_PASSPHRASE", "HOST_MCP_SSH_PASSPHRASE")
	}
	if input.KnownHosts == "" {
		input.KnownHosts = envFirst("HOST_MCP_KNOWN_HOSTS", "HOST_MCP_SSH_KNOWN_HOSTS")
	}
	if input.TimeoutSeconds == 0 {
		input.TimeoutSeconds = envInt("HOST_MCP_TIMEOUT_SECONDS", "HOST_MCP_SSH_TIMEOUT_SECONDS")
	}
	if input.Port == 0 {
		input.Port = 22
	}
	if input.TimeoutSeconds == 0 {
		input.TimeoutSeconds = int(DefaultTimeout / time.Second)
	}
	if input.TimeoutSeconds > int(MaxTimeout/time.Second) {
		input.TimeoutSeconds = int(MaxTimeout / time.Second)
	}
	if input.Host == "" || strings.EqualFold(input.Host, "local") {
		return input, Target{Mode: "local", Host: "localhost"}, nil
	}
	if input.AuthMethod == "" {
		switch {
		case input.PrivateKey != "":
			input.AuthMethod = "key"
		case input.Password != "":
			input.AuthMethod = "password"
		}
	}
	if input.Username == "" {
		return ConnectionInput{}, Target{}, fmt.Errorf("%w: username is required for SSH", ErrInvalidArgument)
	}
	if input.AuthMethod != "password" && input.AuthMethod != "key" {
		return ConnectionInput{}, Target{}, fmt.Errorf("%w: auth_method must be password or key", ErrInvalidArgument)
	}
	if input.AuthMethod == "password" && input.Password == "" {
		return ConnectionInput{}, Target{}, fmt.Errorf("%w: password is required", ErrInvalidArgument)
	}
	if input.AuthMethod == "key" && input.PrivateKey == "" {
		return ConnectionInput{}, Target{}, fmt.Errorf("%w: private_key is required", ErrInvalidArgument)
	}
	return input, Target{Mode: "ssh", Host: input.Host, Port: input.Port, Username: input.Username}, nil
}

func envFirst(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}
func envInt(names ...string) int {
	value := envFirst(names...)
	if value == "" {
		return 0
	}
	parsed, _ := strconv.Atoi(value)
	return parsed
}

func timeoutContext(ctx context.Context, input ConnectionInput) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	seconds := input.TimeoutSeconds
	if seconds <= 0 {
		seconds = int(DefaultTimeout / time.Second)
	}
	if seconds > int(MaxTimeout/time.Second) {
		seconds = int(MaxTimeout / time.Second)
	}
	return context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
}

func safePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || !filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
		return "", fmt.Errorf("%w: path must be an absolute path", ErrInvalidArgument)
	}
	return filepath.Clean(path), nil
}

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

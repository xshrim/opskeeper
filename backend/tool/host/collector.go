package host

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

func Info(ctx context.Context, input InfoInput) (InfoOutput, error) {
	ctx = contextOrBackground(ctx)
	src, target, err := openSource(ctx, input.ConnectionInput)
	if err != nil {
		return InfoOutput{}, err
	}
	defer src.Close()
	now := time.Now().UTC()
	output := InfoOutput{SchemaVersion: 1, CollectedAt: now, Target: target, Runtime: Runtime{Virtualized: "unknown"}}
	output.Hostname, _ = readText(ctx, src, "/proc/sys/kernel/hostname")
	output.Kernel, _ = readText(ctx, src, "/proc/sys/kernel/osrelease")
	output.Architecture = runtime.GOARCH
	if machine, readErr := readText(ctx, src, "/proc/sys/kernel/arch"); readErr == nil && machine != "" {
		output.Architecture = machine
	}
	output.OS = parseOSRelease(mustRead(ctx, src, "/etc/os-release"))
	if uptime, readErr := parseUptime(mustRead(ctx, src, "/proc/uptime")); readErr == nil {
		output.UptimeSeconds = uptime
		output.BootTime = now.Add(-time.Duration(uptime * float64(time.Second)))
	}
	output.CPUs = cpuCount(mustRead(ctx, src, "/proc/stat"))
	output.MemoryBytes = memValue(mustRead(ctx, src, "/proc/meminfo"), "MemTotal") * 1024
	if value, readErr := readText(ctx, src, "/.dockerenv"); readErr == nil && value == "" {
		output.Runtime.Containerized = true
	}
	if output.Hostname == "" || output.Kernel == "" || output.CPUs == 0 || output.MemoryBytes == 0 {
		output.Partial = true
	}
	for _, item := range []struct {
		name  string
		value string
	}{
		{"hostname", output.Hostname}, {"kernel", output.Kernel}, {"os-release", output.OS.Name},
	} {
		if item.value == "" {
			output.Unavailable = append(output.Unavailable, item.name)
		}
	}
	return output, nil
}

func Metrics(ctx context.Context, input MetricsInput) (MetricsOutput, error) {
	ctx = contextOrBackground(ctx)
	src, target, err := openSource(ctx, input.ConnectionInput)
	if err != nil {
		return MetricsOutput{}, err
	}
	defer src.Close()
	sample := input.SampleSeconds
	if sample < 0 || sample > MaxSampleSeconds {
		return MetricsOutput{}, fmt.Errorf("%w: sample_seconds must be between 0 and %d", ErrInvalidArgument, MaxSampleSeconds)
	}
	first := snapshot(ctx, src)
	if sample > 0 {
		timer := time.NewTimer(time.Duration(sample) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return MetricsOutput{}, ctx.Err()
		case <-timer.C:
		}
	}
	second := first
	if sample > 0 {
		second = snapshot(ctx, src)
	}
	output := MetricsOutput{SchemaVersion: 1, CollectedAt: time.Now().UTC(), Target: target}
	output.Host = HostSummary{Hostname: first.hostname, Kernel: first.kernel, Arch: runtime.GOARCH}
	output.Metrics = buildMetrics(first, second, float64(max(sample, 1)))
	if first.hostname == "" || len(first.cpu) == 0 {
		output.Partial = true
	}
	output.Unavailable = append(output.Unavailable, first.unavailable...)
	output.Partial = output.Partial || len(output.Unavailable) > 0
	return output, nil
}

type snapshotData struct {
	hostname, kernel string
	load             [3]float64
	cpu              map[string]cpuTicks
	mem              map[string]uint64
	disk             []DiskMetric
	network          []NetworkMetric
	psi              map[string]PSIMetric
	proc             ProcessSummary
	fs               fsStat
	fsOK             bool
	unavailable      []string
}
type cpuTicks struct {
	user, system, idle, total, ctx uint64
	running, blocked               int
}

func snapshot(ctx context.Context, src source) snapshotData {
	data := snapshotData{cpu: map[string]cpuTicks{}, mem: map[string]uint64{}, psi: map[string]PSIMetric{}}
	data.hostname, _ = readText(ctx, src, "/proc/sys/kernel/hostname")
	data.kernel, _ = readText(ctx, src, "/proc/sys/kernel/osrelease")
	if raw, err := readLimited(ctx, src, "/proc/loadavg"); err == nil {
		fields := strings.Fields(string(raw))
		for i := 0; i < 3 && i < len(fields); i++ {
			data.load[i], _ = strconv.ParseFloat(fields[i], 64)
		}
	} else {
		data.unavailable = append(data.unavailable, "load")
	}
	if raw, err := readLimited(ctx, src, "/proc/stat"); err == nil {
		data.cpu = parseCPUTicks(string(raw))
	} else {
		data.unavailable = append(data.unavailable, "cpu")
	}
	if raw, err := readLimited(ctx, src, "/proc/meminfo"); err == nil {
		data.mem = parseMeminfo(string(raw))
	} else {
		data.unavailable = append(data.unavailable, "memory")
	}
	if raw, err := readLimited(ctx, src, "/proc/diskstats"); err == nil {
		data.disk = parseDiskstats(string(raw))
	} else {
		data.unavailable = append(data.unavailable, "disk")
	}
	if raw, err := readLimited(ctx, src, "/proc/net/dev"); err == nil {
		data.network = parseNetdev(string(raw))
	} else {
		data.unavailable = append(data.unavailable, "network")
	}
	for _, kind := range []string{"cpu", "memory", "io"} {
		if raw, err := readLimited(ctx, src, "/proc/pressure/"+kind); err == nil {
			data.psi[kind] = parsePSI(string(raw))
		}
	}
	data.proc = processSummary(ctx, src)
	if value, err := src.StatFS(ctx, "/"); err == nil {
		data.fs, data.fsOK = value, true
	} else {
		data.unavailable = append(data.unavailable, "filesystem")
	}
	return data
}

func buildMetrics(first, second snapshotData, seconds float64) HostMetrics {
	out := HostMetrics{Load: LoadMetrics{One: second.load[0], Five: second.load[1], Fifteen: second.load[2]}, Memory: memoryMetrics(second.mem), Swap: swapMetrics(second.mem), Disk: second.disk, Network: second.network, Pressure: second.psi, Processes: second.proc}
	out.CPU = cpuMetrics(first.cpu, second.cpu, seconds)
	if second.fsOK {
		used := second.fs.Total - second.fs.Available
		out.Filesystem = []FilesystemMetric{{Mountpoint: "/", FSType: "unknown", TotalBytes: second.fs.Total, FreeBytes: second.fs.Free, UsedBytes: used, UsedPercent: percent(float64(used), float64(second.fs.Total))}}
	}
	return out
}

func readText(ctx context.Context, src source, path string) (string, error) {
	raw, err := readLimited(ctx, src, path)
	return strings.TrimSpace(string(raw)), err
}
func mustRead(ctx context.Context, src source, path string) []byte {
	raw, _ := readLimited(ctx, src, path)
	return raw
}
func parseUptime(raw []byte) (float64, error) {
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return 0, fmt.Errorf("invalid uptime")
	}
	return strconv.ParseFloat(fields[0], 64)
}
func parseOSRelease(raw []byte) OSInfo {
	var out OSInfo
	for _, line := range strings.Split(string(raw), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), "\"")
		switch key {
		case "NAME":
			out.Name = value
		case "VERSION":
			out.Version = value
		case "ID":
			out.ID = value
		case "PRETTY_NAME":
			out.PrettyName = value
		}
	}
	return out
}
func cpuCount(raw []byte) int {
	count := 0
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && strings.HasPrefix(fields[0], "cpu") && fields[0] != "cpu" {
			count++
		}
	}
	return count
}
func parseCPUTicks(raw string) map[string]cpuTicks {
	out := map[string]cpuTicks{}
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || (fields[0] != "cpu" && !strings.HasPrefix(fields[0], "cpu")) {
			if fields[0] == "ctxt" && len(fields) > 1 {
				out["__ctxt"] = cpuTicks{ctx: parseUintField(fields[1])}
			}
			if fields[0] == "procs_running" && len(fields) > 1 {
				out["__running"] = cpuTicks{running: parseIntField(fields[1])}
			}
			if fields[0] == "procs_blocked" && len(fields) > 1 {
				out["__blocked"] = cpuTicks{blocked: parseIntField(fields[1])}
			}
			continue
		}
		var values [8]uint64
		for i := 1; i < len(fields) && i <= 8; i++ {
			values[i-1] = parseUintField(fields[i])
		}
		ticks := cpuTicks{user: values[0] + values[1], system: values[2] + values[5] + values[6] + values[7], idle: values[3] + values[4]}
		ticks.total = ticks.user + ticks.system + ticks.idle
		out[fields[0]] = ticks
	}
	return out
}
func parseMeminfo(raw string) map[string]uint64 {
	out := map[string]uint64{}
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(value)
		if len(fields) == 0 {
			continue
		}
		number := parseUintField(fields[0])
		if len(fields) > 1 && strings.EqualFold(fields[1], "kb") {
			number *= 1024
		}
		out[strings.TrimSpace(key)] = number
	}
	return out
}
func memValue(raw []byte, key string) uint64 { return parseMeminfo(string(raw))[key] }
func memoryMetrics(values map[string]uint64) MemoryMetrics {
	out := MemoryMetrics{TotalBytes: values["MemTotal"] * 1, FreeBytes: values["MemFree"], AvailableBytes: values["MemAvailable"], BuffersBytes: values["Buffers"], CachedBytes: values["Cached"]}
	out.UsedBytes = out.TotalBytes - out.AvailableBytes
	return out
}
func swapMetrics(values map[string]uint64) SwapMetrics {
	out := SwapMetrics{TotalBytes: values["SwapTotal"], FreeBytes: values["SwapFree"]}
	out.UsedBytes = out.TotalBytes - out.FreeBytes
	return out
}
func cpuMetrics(first, second map[string]cpuTicks, seconds float64) CPUMetrics {
	out := CPUMetrics{PerCPU: map[string]float64{}}
	total := second["cpu"]
	previous := first["cpu"]
	deltaTotal := total.total - previous.total
	deltaIdle := total.idle - previous.idle
	if deltaTotal > 0 {
		out.UsagePercent = percent(float64(deltaTotal-deltaIdle), float64(deltaTotal))
	}
	out.UserSeconds = float64(total.user) / 100
	out.SystemSeconds = float64(total.system) / 100
	out.IdleSeconds = float64(total.idle) / 100
	out.ContextSwitches = second["__ctxt"].ctx
	out.Running = second["__running"].running
	out.Blocked = second["__blocked"].blocked
	for name, ticks := range second {
		if name == "cpu" || strings.HasPrefix(name, "__") {
			continue
		}
		old := first[name]
		totalDelta := ticks.total - old.total
		idleDelta := ticks.idle - old.idle
		usage := 0.0
		if totalDelta > 0 {
			usage = percent(float64(totalDelta-idleDelta), float64(totalDelta))
		}
		out.PerCPU[strings.TrimPrefix(name, "cpu")] = usage
	}
	_ = seconds
	return out
}
func parseDiskstats(raw string) []DiskMetric {
	out := make([]DiskMetric, 0)
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 14 {
			continue
		}
		device := fields[2]
		if strings.HasPrefix(device, "loop") || strings.HasPrefix(device, "ram") {
			continue
		}
		out = append(out, DiskMetric{Device: device, ReadsCompleted: parseUintField(fields[3]), ReadBytes: parseUintField(fields[5]) * 512, ReadTimeMillis: parseUintField(fields[6]), WritesCompleted: parseUintField(fields[7]), WriteBytes: parseUintField(fields[9]) * 512, WriteTimeMillis: parseUintField(fields[10])})
	}
	return out
}
func parseNetdev(raw string) []NetworkMetric {
	out := make([]NetworkMetric, 0)
	for _, line := range strings.Split(raw, "\n") {
		name, values, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(values)
		if len(fields) < 16 {
			continue
		}
		out = append(out, NetworkMetric{Interface: strings.TrimSpace(name), ReceiveBytes: parseUintField(fields[0]), ReceivePackets: parseUintField(fields[1]), ReceiveErrors: parseUintField(fields[2]), ReceiveDrops: parseUintField(fields[3]), TransmitBytes: parseUintField(fields[8]), TransmitPackets: parseUintField(fields[9]), TransmitErrors: parseUintField(fields[10]), TransmitDrops: parseUintField(fields[11])})
	}
	return out
}
func parsePSI(raw string) PSIMetric {
	var out PSIMetric
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		values := map[string]*float64{"avg10": &out.SomeAvg10, "avg60": &out.SomeAvg60, "avg300": &out.SomeAvg300}
		target := values
		if fields[0] == "full" {
			target = map[string]*float64{"avg10": &out.FullAvg10, "avg60": &out.FullAvg60, "avg300": &out.FullAvg300}
		}
		for _, field := range fields[1:] {
			key, value, ok := strings.Cut(field, "=")
			if !ok || target[key] == nil {
				continue
			}
			*target[key], _ = strconv.ParseFloat(value, 64)
		}
	}
	return out
}
func processSummary(ctx context.Context, src source) ProcessSummary {
	paths, err := src.ReadDir(ctx, "/proc")
	if err != nil {
		return ProcessSummary{}
	}
	var out ProcessSummary
	for _, path := range paths {
		if _, err := strconv.Atoi(filepath.Base(path)); err != nil {
			continue
		}
		out.Total++
		state, _ := readText(ctx, src, filepath.Join(path, "status"))
		if strings.Contains(state, "State:\tR") {
			out.Running++
		} else if strings.Contains(state, "State:\tT") {
			out.Stopped++
		} else if strings.Contains(state, "State:\tZ") {
			out.Zombie++
		} else {
			out.Sleeping++
		}
	}
	return out
}
func percent(numerator, denominator float64) float64 {
	if denominator <= 0 {
		return 0
	}
	value := numerator / denominator * 100
	return math.Round(value*100) / 100
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func encodeJSON(value any) json.RawMessage { raw, _ := json.Marshal(value); return raw }
func scanLines(raw []byte) []string {
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	scanner.Buffer(make([]byte, 64*1024), MaxReadBytes)
	lines := make([]string, 0)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}
func sortStrings(values []string) { sort.Strings(values) }

package host

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type processExtras struct {
	Executable      string
	CWD             string
	Root            string
	FileDescriptors int
	ReadBytes       uint64
	WriteBytes      uint64
}

func Processes(ctx context.Context, input ProcessesInput) (ProcessesOutput, error) {
	ctx = contextOrBackground(ctx)
	if input.PID < 0 {
		return ProcessesOutput{}, fmt.Errorf("%w: pid must be positive", ErrInvalidArgument)
	}
	keyword := strings.TrimSpace(input.Keyword)
	if keyword != "" && len([]rune(keyword)) > 128 {
		return ProcessesOutput{}, fmt.Errorf("%w: keyword is too long", ErrInvalidArgument)
	}
	expression, err := parseKeywordExpression(keyword)
	if err != nil {
		return ProcessesOutput{}, fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	if input.Limit == 0 {
		input.Limit = DefaultProcessLimit
	}
	if input.Limit < 1 || input.Limit > MaxProcessLimit {
		return ProcessesOutput{}, fmt.Errorf("%w: limit must be between 1 and %d", ErrInvalidArgument, MaxProcessLimit)
	}
	if input.PID <= 0 && keyword == "" {
		_, target, resolveErr := resolveInput(input.ConnectionInput)
		if resolveErr != nil {
			return ProcessesOutput{}, resolveErr
		}
		return ProcessesOutput{SchemaVersion: 1, CollectedAt: time.Now().UTC(), Target: target, Processes: []ProcessInfo{}}, nil
	}
	src, target, err := openSource(ctx, input.ConnectionInput)
	if err != nil {
		return ProcessesOutput{}, err
	}
	defer src.Close()
	if reader, ok := src.(interface {
		ReadProcesses(context.Context, int) ([]byte, error)
	}); ok {
		if raw, readErr := reader.ReadProcesses(ctx, input.PID); readErr == nil {
			passwd, _ := readLimited(ctx, src, "/etc/passwd")
			output := ProcessesOutput{SchemaVersion: 1, CollectedAt: time.Now().UTC(), Target: target, Processes: make([]ProcessInfo, 0, input.Limit)}
			for _, process := range parsePSProcesses(raw, output.CollectedAt, passwd) {
				if keyword != "" && !expression.Match(processSearchText(process)) {
					continue
				}
				output.MatchCount++
				if len(output.Processes) < input.Limit {
					output.Processes = append(output.Processes, process)
				} else {
					output.Truncated = true
				}
			}
			if detailReader, ok := src.(interface {
				ReadProcessExtras(context.Context, []int) (map[int]processExtras, error)
			}); ok && len(output.Processes) > 0 {
				pids := make([]int, 0, len(output.Processes))
				for _, process := range output.Processes {
					pids = append(pids, process.PID)
				}
				if extras, detailErr := detailReader.ReadProcessExtras(ctx, pids); detailErr == nil {
					for i := range output.Processes {
						if extra, found := extras[output.Processes[i].PID]; found {
							applyProcessExtras(&output.Processes[i], extra)
						}
					}
				} else if ctx.Err() != nil {
					return ProcessesOutput{}, ctx.Err()
				}
			}
			return output, nil
		} else if ctx.Err() != nil {
			return ProcessesOutput{}, ctx.Err()
		}
	}
	paths := make([]string, 0)
	if input.PID > 0 {
		paths = append(paths, filepath.Join("/proc", fmt.Sprint(input.PID)))
	} else {
		paths, err = src.ReadDir(ctx, "/proc")
		if err != nil {
			return ProcessesOutput{}, err
		}
	}
	sort.Strings(paths)
	output := ProcessesOutput{SchemaVersion: 1, CollectedAt: time.Now().UTC(), Target: target, Processes: make([]ProcessInfo, 0, min(input.Limit, len(paths)))}
	passwd, _ := readLimited(ctx, src, "/etc/passwd")
	for _, path := range paths {
		pid := parseIntField(filepath.Base(path))
		if pid <= 0 {
			continue
		}
		process, readErr := readProcess(ctx, src, path, passwd)
		if readErr != nil {
			output.Partial = true
			output.Unavailable = append(output.Unavailable, fmt.Sprintf("pid:%d", pid))
			if input.PID > 0 {
				return output, nil
			}
			continue
		}
		if keyword != "" && !expression.Match(processSearchText(process)) {
			continue
		}
		output.MatchCount++
		if len(output.Processes) < input.Limit {
			output.Processes = append(output.Processes, process)
		} else {
			output.Truncated = true
		}
	}
	return output, nil
}

func parsePSProcesses(raw []byte, now time.Time, passwd []byte) []ProcessInfo {
	processes := make([]ProcessInfo, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 0 {
			continue
		}
		process := ProcessInfo{
			PID:          pid,
			PPID:         parseIntField(fields[1]),
			State:        fields[2],
			UID:          parseUintField(fields[3]),
			Threads:      parseIntField(fields[4]),
			RSSBytes:     parseUintField(fields[5]) * 1024,
			VirtualBytes: parseUintField(fields[6]) * 1024,
			Name:         fields[10],
			User:         lookupUser(parseUintField(fields[3]), passwd),
		}
		if elapsed, err := strconv.ParseInt(fields[7], 10, 64); err == nil && elapsed >= 0 {
			process.StartTime = now.Add(-time.Duration(elapsed) * time.Second)
		}
		process.CPUUsagePercent, _ = strconv.ParseFloat(strings.TrimSuffix(fields[8], "%"), 64)
		process.CPUTimeSeconds = parseCPUTime(fields[9])
		if len(fields) > 11 {
			process.CommandLine = redactCommandLine([]byte(strings.Join(fields[11:], " ")))
			if commandFields := strings.Fields(process.CommandLine); len(commandFields) > 0 {
				process.Executable = commandFields[0]
			}
		}
		if process.CommandLine == "" {
			process.CommandLine = process.Name
		}
		processes = append(processes, process)
	}
	return processes
}

func parseCPUTime(value string) float64 {
	parts := strings.Split(value, ":")
	if len(parts) != 3 {
		return 0
	}
	hours := parseUintField(parts[0])
	minutes := parseUintField(parts[1])
	seconds, _ := strconv.ParseFloat(parts[2], 64)
	return float64(hours*3600+minutes*60) + seconds
}

func applyProcessExtras(process *ProcessInfo, extras processExtras) {
	if extras.Executable != "" {
		process.Executable = extras.Executable
	}
	process.CWD = extras.CWD
	process.Root = extras.Root
	process.FileDescriptors = extras.FileDescriptors
	process.ReadBytes = extras.ReadBytes
	process.WriteBytes = extras.WriteBytes
}

func readProcess(ctx context.Context, src source, path string, passwd []byte) (ProcessInfo, error) {
	var output ProcessInfo
	output.PID = parseIntField(filepath.Base(path))
	statusRaw, err := readLimited(ctx, src, filepath.Join(path, "status"))
	if err != nil {
		return output, err
	}
	status := parseStatus(string(statusRaw))
	output.Name = status["Name"]
	output.State = status["State"]
	output.PPID = parseIntField(status["PPid"])
	output.UID = uint64(parseUintField(status["Uid"]))
	output.User = lookupUser(output.UID, passwd)
	output.Threads = parseIntField(status["Threads"])
	output.RSSBytes = parseUintField(status["VmRSS"]) * 1024
	output.VirtualBytes = parseUintField(status["VmSize"]) * 1024
	statRaw, _ := readLimited(ctx, src, filepath.Join(path, "stat"))
	if stat := parseProcStat(string(statRaw)); stat != nil {
		output.Name = firstNonEmpty(output.Name, stat.name)
		output.State = firstNonEmpty(output.State, stat.state)
		output.PPID = stat.ppid
		output.CPUTimeSeconds = float64(stat.utime+stat.stime) / 100
		output.StartTime = time.Now().Add(-time.Duration(stat.starttime/100) * time.Second)
	}
	output.Executable, _ = src.ReadLink(ctx, filepath.Join(path, "exe"))
	output.CWD, _ = src.ReadLink(ctx, filepath.Join(path, "cwd"))
	output.Root, _ = src.ReadLink(ctx, filepath.Join(path, "root"))
	output.CommandLine = redactCommandLine(mustRead(ctx, src, filepath.Join(path, "cmdline")))
	if raw, readErr := readLimited(ctx, src, filepath.Join(path, "io")); readErr == nil {
		output.ReadBytes = ioValue(string(raw), "read_bytes")
		output.WriteBytes = ioValue(string(raw), "write_bytes")
	}
	if entries, readErr := src.ReadDir(ctx, filepath.Join(path, "fd")); readErr == nil {
		output.FileDescriptors = len(entries)
	}
	return output, nil
}

type procStat struct {
	name, state                   string
	ppid, utime, stime, starttime int
}

func parseProcStat(raw string) *procStat {
	closeParen := strings.LastIndex(raw, ")")
	if closeParen < 0 {
		return nil
	}
	prefix := strings.TrimSpace(raw[:closeParen+1])
	open := strings.Index(prefix, "(")
	name := ""
	if open >= 0 {
		name = strings.TrimSpace(prefix[open+1 : closeParen])
	}
	fields := strings.Fields(raw[closeParen+1:])
	if len(fields) < 20 {
		return nil
	}
	return &procStat{name: name, state: fields[0], ppid: parseIntField(fields[1]), utime: parseIntField(fields[11]), stime: parseIntField(fields[12]), starttime: parseIntField(fields[19])}
}
func parseStatus(raw string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok {
			out[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return out
}
func ioValue(raw, key string) uint64 {
	for _, line := range strings.Split(raw, "\n") {
		name, value, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(name) == key {
			return parseUintField(value)
		}
	}
	return 0
}
func redactCommandLine(raw []byte) string {
	value := strings.TrimSpace(strings.ReplaceAll(string(raw), "\x00", " "))
	if value == "" {
		return ""
	}
	fields := strings.Fields(value)
	for i, field := range fields {
		lower := strings.ToLower(field)
		if strings.Contains(lower, "password") || strings.Contains(lower, "passwd") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "private_key") {
			if equal := strings.Index(field, "="); equal >= 0 {
				fields[i] = field[:equal+1] + "<redacted>"
			} else if i+1 < len(fields) {
				fields[i+1] = "<redacted>"
			}
		}
	}
	value = strings.Join(fields, " ")
	if len(value) > 512 {
		value = value[:512] + "…"
	}
	return value
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

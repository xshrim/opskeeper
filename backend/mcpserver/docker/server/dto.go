package server

import (
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/system"
	dockerclient "opskeeper/backend/mcpserver/docker/client"
)

// DockerInfoDTO is the stable public subset of Docker Engine information.
type DockerInfoDTO struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	ServerVersion     string   `json:"server_version"`
	OperatingSystem   string   `json:"operating_system"`
	Architecture      string   `json:"architecture"`
	KernelVersion     string   `json:"kernel_version"`
	Driver            string   `json:"storage_driver"`
	LoggingDriver     string   `json:"logging_driver"`
	CgroupDriver      string   `json:"cgroup_driver"`
	CgroupVersion     string   `json:"cgroup_version,omitempty"`
	DockerRootDir     string   `json:"docker_root_dir"`
	DefaultRuntime    string   `json:"default_runtime"`
	NCPU              int      `json:"cpu_count"`
	MemTotalBytes     int64    `json:"memory_total_bytes"`
	Containers        int      `json:"containers"`
	ContainersRunning int      `json:"containers_running"`
	ContainersPaused  int      `json:"containers_paused"`
	ContainersStopped int      `json:"containers_stopped"`
	Images            int      `json:"images"`
	MemoryLimit       bool     `json:"memory_limit"`
	SwapLimit         bool     `json:"swap_limit"`
	SecurityOptions   []string `json:"security_options,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`
}

// ImageDTO is the stable public subset of an image summary.
type ImageDTO struct {
	ID          string            `json:"id"`
	RepoTags    []string          `json:"repo_tags,omitempty"`
	RepoDigests []string          `json:"repo_digests,omitempty"`
	ParentID    string            `json:"parent_id,omitempty"`
	Created     int64             `json:"created_unix"`
	SizeBytes   int64             `json:"size_bytes"`
	SharedSize  int64             `json:"shared_size_bytes"`
	Containers  int64             `json:"containers"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type ImagesOutput struct {
	Images             []ImageDTO                              `json:"images"`
	ConnectionFallback *dockerclient.ConnectionFallbackWarning `json:"connection_fallback,omitempty"`
}

type PortDTO struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port,omitempty"`
	Type        string `json:"type"`
}

// ContainerDTO is the stable public subset of a container list item.
type ContainerDTO struct {
	ID          string            `json:"id"`
	Names       []string          `json:"names,omitempty"`
	Image       string            `json:"image"`
	ImageID     string            `json:"image_id"`
	Command     string            `json:"command"`
	Created     int64             `json:"created_unix"`
	Ports       []PortDTO         `json:"ports,omitempty"`
	SizeRWBytes int64             `json:"size_rw_bytes,omitempty"`
	SizeRootFS  int64             `json:"size_rootfs_bytes,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	State       string            `json:"state"`
	Status      string            `json:"status"`
	NetworkMode string            `json:"network_mode,omitempty"`
	Mounts      []MountDTO        `json:"mounts,omitempty"`
}

type ContainersOutput struct {
	Containers         []ContainerDTO                          `json:"containers"`
	ConnectionFallback *dockerclient.ConnectionFallbackWarning `json:"connection_fallback,omitempty"`
}

type MountDTO struct {
	Type        string `json:"type"`
	Name        string `json:"name,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination"`
	Driver      string `json:"driver,omitempty"`
	Mode        string `json:"mode,omitempty"`
	ReadWrite   bool   `json:"read_write"`
}

type ContainerStateDTO struct {
	Status     string `json:"status"`
	Running    bool   `json:"running"`
	Paused     bool   `json:"paused"`
	Restarting bool   `json:"restarting"`
	OOMKilled  bool   `json:"oom_killed"`
	Dead       bool   `json:"dead"`
	PID        int    `json:"pid"`
	ExitCode   int    `json:"exit_code"`
	Error      string `json:"error,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	Health     string `json:"health,omitempty"`
}

type ContainerConfigDTO struct {
	Hostname        string            `json:"hostname,omitempty"`
	User            string            `json:"user,omitempty"`
	Image           string            `json:"image"`
	WorkingDir      string            `json:"working_dir,omitempty"`
	Command         []string          `json:"command,omitempty"`
	Entrypoint      []string          `json:"entrypoint,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	EnvNames        []string          `json:"environment_names,omitempty"`
	TTY             bool              `json:"tty"`
	OpenStdin       bool              `json:"open_stdin"`
	NetworkDisabled bool              `json:"network_disabled"`
}

type NetworkDTO struct {
	NetworkID   string   `json:"network_id,omitempty"`
	EndpointID  string   `json:"endpoint_id,omitempty"`
	Gateway     string   `json:"gateway,omitempty"`
	IPAddress   string   `json:"ip_address,omitempty"`
	IPPrefixLen int      `json:"ip_prefix_len,omitempty"`
	IPv6Gateway string   `json:"ipv6_gateway,omitempty"`
	MacAddress  string   `json:"mac_address,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	DNSNames    []string `json:"dns_names,omitempty"`
}

type ContainerInspectDTO struct {
	ID                 string                                  `json:"id"`
	Name               string                                  `json:"name"`
	Created            string                                  `json:"created,omitempty"`
	Image              string                                  `json:"image"`
	Platform           string                                  `json:"platform,omitempty"`
	Driver             string                                  `json:"storage_driver,omitempty"`
	RestartCount       int                                     `json:"restart_count"`
	State              *ContainerStateDTO                      `json:"state,omitempty"`
	Config             *ContainerConfigDTO                     `json:"config,omitempty"`
	Mounts             []MountDTO                              `json:"mounts,omitempty"`
	Networks           map[string]NetworkDTO                   `json:"networks,omitempty"`
	ConnectionFallback *dockerclient.ConnectionFallbackWarning `json:"connection_fallback,omitempty"`
}

type DTOStatsCPU struct {
	TotalUsageNanoseconds    uint64 `json:"total_usage_nanoseconds"`
	SystemUsageNanoseconds   uint64 `json:"system_usage_nanoseconds,omitempty"`
	OnlineCPUs               uint32 `json:"online_cpus,omitempty"`
	UsageInKernelNanoseconds uint64 `json:"usage_in_kernel_nanoseconds,omitempty"`
	UsageInUserNanoseconds   uint64 `json:"usage_in_user_nanoseconds,omitempty"`
	ThrottledPeriods         uint64 `json:"throttled_periods,omitempty"`
	ThrottledTimeNanoseconds uint64 `json:"throttled_time_nanoseconds,omitempty"`
}

type StatsMemory struct {
	UsageBytes    uint64 `json:"usage_bytes,omitempty"`
	MaxUsageBytes uint64 `json:"max_usage_bytes,omitempty"`
	LimitBytes    uint64 `json:"limit_bytes,omitempty"`
	FailCount     uint64 `json:"fail_count,omitempty"`
}

type StatsNetwork struct {
	ReceivedBytes      uint64 `json:"received_bytes"`
	ReceivedPackets    uint64 `json:"received_packets"`
	ReceivedErrors     uint64 `json:"received_errors"`
	ReceivedDropped    uint64 `json:"received_dropped"`
	TransmittedBytes   uint64 `json:"transmitted_bytes"`
	TransmittedPackets uint64 `json:"transmitted_packets"`
	TransmittedErrors  uint64 `json:"transmitted_errors"`
	TransmittedDropped uint64 `json:"transmitted_dropped"`
}

type ContainerStatsDTO struct {
	ID                 string                                  `json:"id"`
	Name               string                                  `json:"name"`
	ReadAt             string                                  `json:"read_at"`
	PreviousReadAt     string                                  `json:"previous_read_at"`
	PIDs               uint64                                  `json:"pids"`
	CPU                DTOStatsCPU                             `json:"cpu"`
	Memory             StatsMemory                             `json:"memory"`
	Networks           map[string]StatsNetwork                 `json:"networks,omitempty"`
	ConnectionFallback *dockerclient.ConnectionFallbackWarning `json:"connection_fallback,omitempty"`
}

func toDockerInfo(info system.Info) DockerInfoDTO {
	return DockerInfoDTO{
		ID: info.ID, Name: info.Name, ServerVersion: info.ServerVersion,
		OperatingSystem: info.OperatingSystem, Architecture: info.Architecture,
		KernelVersion: info.KernelVersion, Driver: info.Driver, LoggingDriver: info.LoggingDriver,
		CgroupDriver: info.CgroupDriver, CgroupVersion: info.CgroupVersion, DockerRootDir: info.DockerRootDir,
		DefaultRuntime: info.DefaultRuntime, NCPU: info.NCPU, MemTotalBytes: info.MemTotal,
		Containers: info.Containers, ContainersRunning: info.ContainersRunning,
		ContainersPaused: info.ContainersPaused, ContainersStopped: info.ContainersStopped,
		Images: info.Images, MemoryLimit: info.MemoryLimit, SwapLimit: info.SwapLimit,
		SecurityOptions: append([]string(nil), info.SecurityOptions...), Warnings: append([]string(nil), info.Warnings...),
	}
}

func toImageDTO(item image.Summary) ImageDTO {
	return ImageDTO{ID: item.ID, RepoTags: item.RepoTags, RepoDigests: item.RepoDigests, ParentID: item.ParentID,
		Created: item.Created, SizeBytes: item.Size, SharedSize: item.SharedSize, Containers: item.Containers, Labels: item.Labels}
}

func toContainerDTO(item container.Summary) ContainerDTO {
	ports := make([]PortDTO, 0, len(item.Ports))
	for _, port := range item.Ports {
		ports = append(ports, PortDTO{IP: port.IP, PrivatePort: port.PrivatePort, PublicPort: port.PublicPort, Type: port.Type})
	}
	mounts := make([]MountDTO, 0, len(item.Mounts))
	for _, mount := range item.Mounts {
		mounts = append(mounts, MountDTO{Type: string(mount.Type), Name: mount.Name, Source: mount.Source, Destination: mount.Destination, Driver: mount.Driver, Mode: mount.Mode, ReadWrite: mount.RW})
	}
	return ContainerDTO{ID: item.ID, Names: item.Names, Image: item.Image, ImageID: item.ImageID, Command: item.Command,
		Created: item.Created, Ports: ports, SizeRWBytes: item.SizeRw, SizeRootFS: item.SizeRootFs, Labels: item.Labels,
		State: string(item.State), Status: item.Status, NetworkMode: item.HostConfig.NetworkMode, Mounts: mounts}
}

func toInspectDTO(item container.InspectResponse) ContainerInspectDTO {
	dto := ContainerInspectDTO{Networks: make(map[string]NetworkDTO)}
	if item.ContainerJSONBase != nil {
		base := item.ContainerJSONBase
		dto.ID, dto.Created, dto.Image, dto.Platform, dto.Driver, dto.RestartCount = base.ID, base.Created, base.Image, base.Platform, base.Driver, base.RestartCount
		dto.Name = base.Name
		if base.State != nil {
			dto.State = toStateDTO(base.State)
		}
	}
	if item.Config != nil {
		envNames := make([]string, 0, len(item.Config.Env))
		for _, value := range item.Config.Env {
			if index := strings.IndexByte(value, '='); index > 0 {
				envNames = append(envNames, value[:index])
			} else if value != "" {
				envNames = append(envNames, value)
			}
		}
		dto.Config = &ContainerConfigDTO{Hostname: item.Config.Hostname, User: item.Config.User, Image: item.Config.Image,
			WorkingDir: item.Config.WorkingDir, Command: append([]string(nil), item.Config.Cmd...), Entrypoint: append([]string(nil), item.Config.Entrypoint...),
			Labels: item.Config.Labels, EnvNames: envNames, TTY: item.Config.Tty, OpenStdin: item.Config.OpenStdin, NetworkDisabled: item.Config.NetworkDisabled}
	}
	for _, mount := range item.Mounts {
		dto.Mounts = append(dto.Mounts, MountDTO{Type: string(mount.Type), Name: mount.Name, Source: mount.Source, Destination: mount.Destination, Driver: mount.Driver, Mode: mount.Mode, ReadWrite: mount.RW})
	}
	if item.NetworkSettings != nil {
		for name, network := range item.NetworkSettings.Networks {
			if network == nil {
				continue
			}
			dto.Networks[name] = NetworkDTO{NetworkID: network.NetworkID, EndpointID: network.EndpointID, Gateway: network.Gateway,
				IPAddress: network.IPAddress, IPPrefixLen: network.IPPrefixLen, IPv6Gateway: network.IPv6Gateway, MacAddress: network.MacAddress,
				Aliases: network.Aliases, DNSNames: network.DNSNames}
		}
	}
	if len(dto.Networks) == 0 {
		dto.Networks = nil
	}
	return dto
}

func toStateDTO(state *container.State) *ContainerStateDTO {
	if state == nil {
		return nil
	}
	dto := &ContainerStateDTO{Status: string(state.Status), Running: state.Running, Paused: state.Paused, Restarting: state.Restarting,
		OOMKilled: state.OOMKilled, Dead: state.Dead, PID: state.Pid, ExitCode: state.ExitCode, Error: state.Error,
		StartedAt: state.StartedAt, FinishedAt: state.FinishedAt}
	if state.Health != nil {
		dto.Health = state.Health.Status
	}
	return dto
}

func toStatsDTO(stats container.StatsResponse) ContainerStatsDTO {
	dto := ContainerStatsDTO{ID: stats.ID, Name: stats.Name, ReadAt: stats.Read.Format(time.RFC3339Nano), PreviousReadAt: stats.PreRead.Format(time.RFC3339Nano),
		PIDs: stats.PidsStats.Current, CPU: DTOStatsCPU{TotalUsageNanoseconds: stats.CPUStats.CPUUsage.TotalUsage,
			SystemUsageNanoseconds: stats.CPUStats.SystemUsage, OnlineCPUs: stats.CPUStats.OnlineCPUs,
			UsageInKernelNanoseconds: stats.CPUStats.CPUUsage.UsageInKernelmode, UsageInUserNanoseconds: stats.CPUStats.CPUUsage.UsageInUsermode,
			ThrottledPeriods: stats.CPUStats.ThrottlingData.ThrottledPeriods, ThrottledTimeNanoseconds: stats.CPUStats.ThrottlingData.ThrottledTime},
		Memory: StatsMemory{UsageBytes: stats.MemoryStats.Usage, MaxUsageBytes: stats.MemoryStats.MaxUsage, LimitBytes: stats.MemoryStats.Limit, FailCount: stats.MemoryStats.Failcnt}}
	if len(stats.Networks) > 0 {
		dto.Networks = make(map[string]StatsNetwork, len(stats.Networks))
		for name, network := range stats.Networks {
			dto.Networks[name] = StatsNetwork{ReceivedBytes: network.RxBytes, ReceivedPackets: network.RxPackets, ReceivedErrors: network.RxErrors, ReceivedDropped: network.RxDropped,
				TransmittedBytes: network.TxBytes, TransmittedPackets: network.TxPackets, TransmittedErrors: network.TxErrors, TransmittedDropped: network.TxDropped}
		}
	}
	return dto
}

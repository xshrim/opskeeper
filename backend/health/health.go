package health

import (
	"context"
	"sync"
	"time"

	"opskeeper/backend/version"
)

type Check struct {
	Name string
	Run  func(context.Context) error
}

type CheckResult struct {
	Status    string `json:"status"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

type Report struct {
	Status    string                 `json:"status"`
	Service   string                 `json:"service"`
	Version   string                 `json:"version"`
	Commit    string                 `json:"commit"`
	BuildTime string                 `json:"build_time"`
	Timestamp time.Time              `json:"timestamp"`
	Checks    map[string]CheckResult `json:"checks,omitempty"`
}

type Service struct {
	name    string
	timeout time.Duration
	checks  []Check
}

func NewService(name string, timeout time.Duration, checks []Check) *Service {
	return &Service{name: name, timeout: timeout, checks: checks}
}

func (s *Service) Name() string {
	return s.name
}

func (s *Service) Readiness(ctx context.Context, build version.Info) Report {
	report := Report{
		Status:    "ready",
		Service:   s.name,
		Version:   build.Version,
		Commit:    build.Commit,
		BuildTime: build.BuildTime,
		Timestamp: time.Now().UTC(),
		Checks:    make(map[string]CheckResult, len(s.checks)),
	}

	var mutex sync.Mutex
	var waitGroup sync.WaitGroup
	for _, dependency := range s.checks {
		dependency := dependency
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			start := time.Now()
			checkCtx, cancel := context.WithTimeout(ctx, s.timeout)
			defer cancel()

			result := CheckResult{Status: "up"}
			if err := dependency.Run(checkCtx); err != nil {
				result.Status = "down"
				result.Error = err.Error()
			}
			result.LatencyMS = time.Since(start).Milliseconds()

			mutex.Lock()
			report.Checks[dependency.Name] = result
			mutex.Unlock()
		}()
	}
	waitGroup.Wait()

	for _, result := range report.Checks {
		if result.Status != "up" {
			report.Status = "not_ready"
			break
		}
	}
	return report
}

func Liveness(serviceName string, build version.Info) Report {
	return Report{
		Status:    "alive",
		Service:   serviceName,
		Version:   build.Version,
		Commit:    build.Commit,
		BuildTime: build.BuildTime,
		Timestamp: time.Now().UTC(),
	}
}

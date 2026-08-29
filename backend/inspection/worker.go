package inspection

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// DeterministicChecker is intentionally narrow. Implementations may inspect a
// connector, but must return facts/rules without involving a model.
type DeterministicChecker interface {
	Check(context.Context, string) ([]RuleResult, error)
}

// AIExplainer may enrich an already completed deterministic run. It never
// receives authority to alter RuleResult or HealthScore.
type AIExplainer interface {
	Explain(context.Context, Run, Policy, []Finding) error
}

type Worker struct {
	store     Store
	checker   DeterministicChecker
	explainer AIExplainer
	owner     string
	lease     time.Duration
}

func NewWorker(store Store, checker DeterministicChecker, explainer AIExplainer, owner string, lease time.Duration) *Worker {
	if lease <= 0 {
		lease = 45 * time.Second
	}
	return &Worker{store: store, checker: checker, explainer: explainer, owner: owner, lease: lease}
}

// RunOnce claims one durable job. A lost lease makes the run fail safely; a
// subsequent worker can resume it from the frozen run snapshot.
func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	if w == nil || w.store == nil {
		return false, errors.New("inspection worker store is unavailable")
	}
	job, claimed, err := w.store.ClaimJob(ctx, w.owner, w.lease)
	if err != nil || !claimed {
		return claimed, err
	}
	if err := w.run(ctx, job); err != nil {
		return true, w.store.FinishJob(ctx, job.ID, w.owner, err)
	}
	return true, w.store.FinishJob(ctx, job.ID, w.owner, nil)
}

func (w *Worker) run(ctx context.Context, job Job) error {
	run, policy, targets, err := w.store.GetRun(ctx, job.RunID)
	if err != nil {
		return err
	}
	if err := w.store.StartRun(ctx, run.ID); err != nil {
		return err
	}
	run.Status = RunRunning
	ctx, cancel := context.WithTimeout(ctx, policy.Timeout)
	defer cancel()
	stop := make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(stop) })
	go func() {
		ticker := time.NewTicker(w.lease / 3)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				ok, _ := w.store.Heartbeat(context.Background(), job.ID, w.owner, w.lease)
				if !ok {
					return
				}
			}
		}
	}()
	if w.checker == nil {
		return fmt.Errorf("deterministic checker is unavailable")
	}
	maxConcurrent := policy.MaxConcurrent
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	type targetResult struct {
		target string
		values []RuleResult
		err    error
	}
	semaphore := make(chan struct{}, maxConcurrent)
	completed := make(chan targetResult, len(targets))
	var checks sync.WaitGroup
	for _, target := range targets {
		target := target
		checks.Add(1)
		go func() {
			defer checks.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				completed <- targetResult{target: target, err: ctx.Err()}
				return
			}
			values, checkErr := w.checker.Check(ctx, target)
			completed <- targetResult{target: target, values: values, err: checkErr}
		}()
	}
	checks.Wait()
	close(completed)
	results := []RuleResult{}
	for result := range completed {
		values := result.values
		if result.err != nil {
			values = []RuleResult{{TargetResourceID: result.target, Rule: "connector.connectivity", Severity: "critical", Weight: 50, Message: "确定性连接检查失败：" + errorText(result.err)}}
		}
		for i := range values {
			if values[i].TargetResourceID == "" {
				values[i].TargetResourceID = result.target
			}
		}
		results = append(results, values...)
	}
	policy.TargetResourceIDs = targets
	findings, _, err := w.store.SaveResults(ctx, run, policy, results)
	if err != nil {
		return err
	}
	if len(findings) == 0 {
		return w.store.MarkLLMStatus(ctx, run.ID, "not_requested")
	}
	if w.explainer == nil {
		return w.store.MarkLLMStatus(ctx, run.ID, "degraded")
	}
	if err := w.explainer.Explain(ctx, run, policy, findings); err != nil {
		_ = w.store.MarkLLMStatus(ctx, run.ID, "degraded")
		return nil
	}
	return w.store.MarkLLMStatus(ctx, run.ID, "succeeded")
}

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
	leaseLost := make(chan struct{})
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(w.lease / 3)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				heartbeatCtx, heartbeatCancel := context.WithTimeout(ctx, min(w.lease/3, 5*time.Second))
				ok, err := w.store.Heartbeat(heartbeatCtx, job.ID, w.owner, w.lease)
				heartbeatCancel()
				if err != nil || !ok {
					cancel()
					close(leaseLost)
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
	completed := make(chan targetResult, len(targets))
	targetQueue := make(chan string)
	var checks sync.WaitGroup
	workers := min(maxConcurrent, len(targets))
	for range workers {
		checks.Add(1)
		go func() {
			defer checks.Done()
			for target := range targetQueue {
				values, checkErr := w.checker.Check(ctx, target)
				completed <- targetResult{target: target, values: values, err: checkErr}
			}
		}()
	}
	for _, target := range targets {
		select {
		case targetQueue <- target:
		case <-ctx.Done():
			break
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(targetQueue)
	checks.Wait()
	close(completed)
	if err := ctx.Err(); err != nil {
		select {
		case <-leaseLost:
			return fmt.Errorf("inspection job lease lost")
		default:
			return err
		}
	}
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
	if ok, err := w.store.Heartbeat(ctx, job.ID, w.owner, w.lease); err != nil || !ok {
		return fmt.Errorf("inspection job lease lost")
	}
	once.Do(func() { close(stop) })
	<-heartbeatDone
	var findings []Finding
	if guarded, ok := w.store.(interface {
		SaveResultsForJob(context.Context, string, string, Run, Policy, []RuleResult) ([]Finding, []HealthSnapshot, error)
	}); ok {
		findings, _, err = guarded.SaveResultsForJob(ctx, job.ID, w.owner, run, policy, results)
	} else {
		findings, _, err = w.store.SaveResults(ctx, run, policy, results)
	}
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

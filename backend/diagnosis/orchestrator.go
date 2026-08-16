package diagnosis

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"opskeeper/backend/connector"
	"opskeeper/backend/skill"
)

type SkillRunner interface {
	Run(context.Context, skill.RunInput) (skill.RunResult, error)
}

type Orchestrator struct {
	*Service
	runner  SkillRunner
	timeout time.Duration
}

func NewOrchestrator(service *Service, runner SkillRunner, timeout time.Duration) *Orchestrator {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	return &Orchestrator{Service: service, runner: runner, timeout: timeout}
}

func (o *Orchestrator) Start(ctx context.Context, input StartInput) (Session, error) {
	session, err := o.Service.Start(ctx, input)
	if err != nil {
		return Session{}, err
	}
	o.launch(context.WithoutCancel(ctx), session.ID)
	return session, nil
}

func (o *Orchestrator) Ask(ctx context.Context, sessionID, content string) (Message, error) {
	message, err := o.Service.Ask(ctx, sessionID, content)
	if err != nil {
		return Message{}, err
	}
	o.launch(context.WithoutCancel(ctx), strings.TrimSpace(sessionID))
	return message, nil
}

func (o *Orchestrator) launch(parent context.Context, sessionID string) {
	if o.runner == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(parent, o.timeout)
		defer cancel()
		o.run(ctx, sessionID)
	}()
}

func (o *Orchestrator) run(ctx context.Context, sessionID string) {
	session, claimed, err := o.store.ClaimRun(ctx, sessionID)
	if err != nil {
		return
	}
	if !claimed {
		return
	}
	_, _ = o.store.AppendEvent(ctx, session.ID, CreateEventInput{Type: "phase.changed", Payload: map[string]any{"phase": StatusPlanning}})
	plan, err := o.store.CreatePlan(ctx, session.ID, "围绕已授权目标收集只读证据并形成可追溯结论。", []PlanStep{
		{Phase: "plan", Status: "succeeded", Title: "确定诊断范围", Detail: "已固定会话 Scope 和目标资源。"},
		{Phase: "collect", Status: "running", Title: "采集受控证据", Detail: "仅允许已声明的 Skill 工具。"},
		{Phase: "verify", Status: "pending", Title: "核验证据链", Detail: "外部内容不可信；确定性结论必须引用本会话 Evidence。"},
		{Phase: "summarize", Status: "pending", Title: "归纳结论", Detail: "确定性结论必须附带 Evidence 引用。"},
	})
	if err != nil {
		o.fail(session.ID, "plan", err)
		return
	}
	_, _ = o.store.AppendEvent(ctx, session.ID, CreateEventInput{Type: "plan.created", Payload: map[string]any{"plan_id": plan.ID, "steps": len(plan.Steps)}})
	if _, err = o.store.SetStatus(ctx, session.ID, StatusCollecting); err != nil {
		return
	}
	_, _ = o.store.AppendEvent(ctx, session.ID, CreateEventInput{Type: "phase.changed", Payload: map[string]any{"phase": StatusCollecting}})
	targets, err := o.store.Targets(ctx, session.ID)
	if err != nil || len(targets) == 0 {
		o.fail(session.ID, "target", errors.New("diagnosis target is unavailable"))
		return
	}
	messages, err := o.store.Messages(ctx, session.ID, 20)
	if err != nil || len(messages) == 0 {
		o.fail(session.ID, "message", errors.New("diagnosis question is unavailable"))
		return
	}
	targetIDs := make([]string, 0, len(targets))
	for _, target := range targets {
		targetIDs = append(targetIDs, target.ResourceID)
	}
	conversation := make([]map[string]string, 0, len(messages))
	for _, message := range messages {
		conversation = append(conversation, map[string]string{"role": message.Role, "content": message.Content})
	}
	result, err := o.runner.Run(ctx, skill.RunInput{ActorID: dereference(session.ActorUserID), ScopeID: session.ScopeID, TargetResourceID: targets[0].ResourceID, Input: map[string]any{"question": messages[len(messages)-1].Content, "target_resource_ids": targetIDs, "conversation": conversation}, MaxToolCalls: 12, MaxTokens: 20000, MaxOutputBytes: 64 << 10, Timeout: o.timeout, Stream: true, EvidenceObserver: func(observed skill.ObservedEvidence) { o.captureEvidence(session.ID, observed) }})
	if err != nil {
		o.fail(session.ID, runnerErrorCode(err), err)
		return
	}
	for _, step := range plan.Steps {
		if step.Phase == "collect" {
			_, _ = o.store.UpdateStep(ctx, step.ID, "succeeded", "已完成受控 Skill 与 Tool 执行。")
		}
	}
	if _, err = o.store.SetStatus(ctx, session.ID, StatusAnalyzing); err != nil {
		return
	}
	_, _ = o.store.AppendEvent(ctx, session.ID, CreateEventInput{Type: "phase.changed", Payload: map[string]any{"phase": StatusAnalyzing}})
	evidence, err := o.store.Evidence(ctx, session.ID)
	if err != nil {
		o.fail(session.ID, "evidence", err)
		return
	}
	output := safeText(result.Output, 8000)
	_, _ = o.store.AppendMessage(ctx, session.ID, AppendMessageInput{Role: "assistant", Content: output})
	evidenceIDs := make([]string, 0, len(evidence))
	for _, item := range evidence {
		evidenceIDs = append(evidenceIDs, item.ID)
	}
	for _, step := range plan.Steps {
		if step.Phase == "verify" {
			detail := "已核验本会话中可引用的 Evidence。"
			status := "succeeded"
			if len(evidenceIDs) == 0 {
				status, detail = "skipped", "未采集到 Evidence；输出只能作为待验证假设。"
			}
			_, _ = o.store.UpdateStep(ctx, step.ID, status, detail)
		}
	}
	reportStatus, conclusion := "succeeded", output
	if len(evidenceIDs) == 0 {
		reportStatus, conclusion = "warning", "证据不足：Skill 未采集到可引用 Evidence，模型输出仅作为待验证假设。"
		_, _ = o.store.SaveHypothesis(ctx, Hypothesis{SessionID: session.ID, Statement: output, Status: "needs_verification", Confidence: 0})
	} else {
		_, _ = o.store.SaveHypothesis(ctx, Hypothesis{SessionID: session.ID, Statement: output, Status: "supported", Confidence: 0.5, EvidenceIDs: evidenceIDs})
	}
	recommendations, _ := json.Marshal([]string{"查看已引用 Evidence 的来源、时间窗口和原始内容后再执行任何变更。"})
	report, err := o.store.SaveReport(ctx, Report{SessionID: session.ID, Status: reportStatus, Conclusion: conclusion, Recommendations: recommendations, EvidenceIDs: evidenceIDs})
	if err != nil {
		o.fail(session.ID, "report", err)
		return
	}
	for _, step := range plan.Steps {
		if step.Phase == "summarize" {
			_, _ = o.store.UpdateStep(ctx, step.ID, "succeeded", "已生成带 Evidence ID 的报告。")
		}
	}
	_, _ = o.store.Finish(ctx, session.ID, StatusSucceeded, "", "")
	_, _ = o.store.AppendEvent(context.Background(), session.ID, CreateEventInput{Type: "report.ready", Payload: map[string]any{"report_id": report.ID, "evidence_ids": evidenceIDs, "status": report.Status}})
}

func (o *Orchestrator) captureEvidence(sessionID string, observed skill.ObservedEvidence) {
	evidence := observed.Evidence
	item, err := o.store.SaveEvidence(context.Background(), sessionID, CreateEvidenceInput{TargetResourceID: observed.TargetResourceID, SourceResourceID: evidence.SourceResourceID, Capability: string(evidence.Capability), CollectedAt: evidence.CollectedAt, WindowStart: windowStart(evidence.Window), WindowEnd: windowEnd(evidence.Window), Summary: mustJSON(evidence.Summary), Content: evidence.Data, Partial: evidence.Partial, Untrusted: true})
	if err == nil {
		_, _ = o.store.AppendEvent(context.Background(), sessionID, CreateEventInput{Type: "tool.completed", Payload: map[string]any{"tool": observed.ToolName, "target_resource_id": observed.TargetResourceID, "capability": item.Capability}})
		_, _ = o.store.AppendEvent(context.Background(), sessionID, CreateEventInput{Type: "evidence.collected", Payload: map[string]any{"evidence_id": item.ID, "source_resource_id": item.SourceResourceID, "capability": item.Capability, "partial": item.Partial}})
	}
}

func (o *Orchestrator) fail(sessionID, code string, cause error) {
	message := safeText(cause.Error(), 1000)
	_, _ = o.store.Finish(context.Background(), sessionID, StatusFailed, code, message)
	_, _ = o.store.AppendEvent(context.Background(), sessionID, CreateEventInput{Type: "diagnosis.failed", Payload: map[string]any{"code": code, "message": message}})
}

func windowStart(window *connector.Window) *time.Time {
	if window == nil {
		return nil
	}
	return &window.Start
}
func windowEnd(window *connector.Window) *time.Time {
	if window == nil {
		return nil
	}
	return &window.End
}
func mustJSON(value any) json.RawMessage {
	output, _ := json.Marshal(value)
	if len(output) == 0 {
		return json.RawMessage(`{}`)
	}
	return output
}
func dereference(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func runnerErrorCode(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "skill_runner"
}

var sensitiveText = regexp.MustCompile(`(?i)(bearer\s+|token|password|secret|api[_-]?key)[:=\s]+[^\s,;]+`)

func safeText(value string, maximum int) string {
	value = sensitiveText.ReplaceAllString(value, "$1[REDACTED]")
	if len(value) > maximum {
		return value[:maximum]
	}
	return value
}

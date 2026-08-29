package diagnosis

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"opskeeper/backend/aiengine"
	"opskeeper/backend/connector"
)

type Orchestrator struct {
	*Service
	engine  aiengine.Engine
	timeout time.Duration
}

func NewOrchestrator(service *Service, engine aiengine.Engine, timeout time.Duration) *Orchestrator {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	return &Orchestrator{Service: service, engine: engine, timeout: timeout}
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
	if o.engine == nil {
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
		{Phase: "collect", Status: "running", Title: "采集受控证据", Detail: "仅允许经过 AIEngine PolicyGateway 注册的只读工具。"},
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
	engineMessages := make([]aiengine.Message, 0, len(messages))
	for _, message := range messages {
		conversation = append(conversation, map[string]string{"role": message.Role, "content": message.Content})
		engineMessages = append(engineMessages, aiengine.Message{Role: message.Role, Content: message.Content})
	}
	// A diagnosis can be started without selecting a persisted Skill. The
	// built-in profile is passed directly to AIEngine; Skill is only an optional
	// prompt/tool/contract adapter for other execution scenarios.
	defaultProfile := &aiengine.AgentProfile{
		ResourceID:   "builtin:diagnosis-agent",
		ScopeID:      session.ScopeID,
		Name:         "故障诊断 Agent",
		Version:      1,
		Instruction:  "你是 OpsKeeper 故障诊断专家。围绕用户问题和授权目标资源进行只读分析；优先调用可用的受控工具获取证据，明确区分事实、推断和待验证假设。不要执行写操作，不要泄露凭据。输出简洁、可追溯的诊断结论和建议。",
		Capabilities: []string{"text", "tool_calling", "stream"},
		Enabled:      true,
	}
	result, err := o.engine.Execute(ctx, aiengine.Request{ActorID: dereference(session.ActorUserID), ScopeID: session.ScopeID, AIProviderResourceID: session.ProviderResourceID, ModelName: session.ModelName, Purpose: aiengine.PurposeDiagnosis, Profile: aiengine.ProfileDiagnosis, ResolvedAgentProfile: defaultProfile, Messages: engineMessages, Context: aiengine.ContextRequest{ResourceIDs: targetIDs}, Input: map[string]any{"question": messages[len(messages)-1].Content, "target_resource_ids": targetIDs, "conversation": conversation}, Budget: aiengine.Budget{MaxToolCalls: 12, MaxTokens: 20000, MaxOutputBytes: 64 << 10, Timeout: o.timeout}, Stream: true, EventSink: func(event aiengine.Event) error {
		_, _ = o.store.AppendEvent(context.Background(), session.ID, CreateEventInput{Type: event.Type, Payload: event.Payload})
		return nil
	}, ObservationSink: func(observed aiengine.ToolObservation) { o.captureObservation(session.ID, observed) }})
	if err != nil {
		o.fail(session.ID, runnerErrorCode(err), err)
		return
	}
	for _, step := range plan.Steps {
		if step.Phase == "collect" {
			_, _ = o.store.UpdateStep(ctx, step.ID, "succeeded", "已完成受控工具执行。")
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
		reportStatus, conclusion = "warning", "证据不足：本次执行未采集到可引用 Evidence，模型输出仅作为待验证假设。"
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

func (o *Orchestrator) captureEvidence(sessionID string, observed connector.Evidence, toolName, targetResourceID string) {
	item, err := o.store.SaveEvidence(context.Background(), sessionID, CreateEvidenceInput{TargetResourceID: targetResourceID, SourceResourceID: observed.SourceResourceID, Capability: string(observed.Capability), CollectedAt: observed.CollectedAt, WindowStart: windowStart(observed.Window), WindowEnd: windowEnd(observed.Window), Summary: mustJSON(observed.Summary), Content: observed.Data, Partial: observed.Partial, Untrusted: true})
	if err == nil {
		_, _ = o.store.AppendEvent(context.Background(), sessionID, CreateEventInput{Type: "tool.completed", Payload: map[string]any{"tool": toolName, "target_resource_id": targetResourceID, "capability": item.Capability}})
		_, _ = o.store.AppendEvent(context.Background(), sessionID, CreateEventInput{Type: "evidence.collected", Payload: map[string]any{"evidence_id": item.ID, "source_resource_id": item.SourceResourceID, "capability": item.Capability, "partial": item.Partial}})
	}
}

func (o *Orchestrator) captureObservation(sessionID string, observed aiengine.ToolObservation) {
	content := mustJSON(observed.Result.Output)
	item, err := o.store.SaveEvidence(context.Background(), sessionID, CreateEvidenceInput{TargetResourceID: observed.ResourceID, SourceResourceID: observed.ResourceID, Capability: observed.ToolName, CollectedAt: time.Now().UTC(), Summary: mustJSON(map[string]any{"tool": observed.ToolName}), Content: content, Untrusted: observed.Result.Untrusted})
	if err == nil {
		_, _ = o.store.AppendEvent(context.Background(), sessionID, CreateEventInput{Type: "evidence.collected", Payload: map[string]any{"evidence_id": item.ID, "source_resource_id": item.SourceResourceID, "capability": item.Capability, "untrusted": item.Untrusted}})
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
	return "ai_engine"
}

var sensitiveText = regexp.MustCompile(`(?i)(bearer\s+|token|password|secret|api[_-]?key)[:=\s]+[^\s,;]+`)

func safeText(value string, maximum int) string {
	value = sensitiveText.ReplaceAllString(value, "$1[REDACTED]")
	if len(value) > maximum {
		return value[:maximum]
	}
	return value
}

package diagnosis

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"
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

// Cancel stops the active AIEngine execution for a diagnosis session.
func (o *Orchestrator) Cancel(ctx context.Context, sessionID string) error {
	if o.engine == nil {
		return errors.New("AIEngine is unavailable")
	}
	return o.engine.Cancel(ctx, diagnosisExecutionID(strings.TrimSpace(sessionID)))
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
	o.appendEvent(ctx, session.ID, CreateEventInput{Type: "phase.changed", Payload: map[string]any{"phase": StatusPlanning}})
	plan, err := o.store.CreatePlan(ctx, session.ID, "由 AIEngine 统一处理对话；仅在用户选择并授权资源时加载上下文和调用工具。", []PlanStep{
		{Phase: "plan", Status: "succeeded", Title: "确定执行范围", Detail: "已固定会话 Scope、模型以及用户提交的上下文选择。"},
		{Phase: "collect", Status: "pending", Title: "按需加载上下文", Detail: "仅对已授权且已选择的资源注册只读工具；普通问题可不调用工具。"},
		{Phase: "verify", Status: "pending", Title: "核验工具结果", Detail: "工具返回内容作为不可信输入，回答应区分事实与推断。"},
		{Phase: "summarize", Status: "pending", Title: "生成回答", Detail: "由 AIEngine 按当前对话和可用上下文生成自然语言回答。"},
	})
	if err != nil {
		o.fail(session.ID, "plan", err)
		return
	}
	o.appendEvent(ctx, session.ID, CreateEventInput{Type: "plan.created", Payload: map[string]any{"plan_id": plan.ID, "steps": len(plan.Steps)}})
	if _, err = o.store.SetStatus(ctx, session.ID, StatusCollecting); err != nil {
		return
	}
	o.appendEvent(ctx, session.ID, CreateEventInput{Type: "phase.changed", Payload: map[string]any{"phase": StatusCollecting}})
	targets, err := o.store.Targets(ctx, session.ID)
	if err != nil {
		o.fail(session.ID, "target", errors.New("session context is unavailable"))
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
	instruction := "你是 OpsKeeper AI 助手。当前没有选择资源，请按用户意图进行自然、直接的普通对话，不要把问候或一般问题强行解释为故障诊断。"
	if len(targetIDs) > 0 {
		instruction = "你是 OpsKeeper AI 助手。用户选择了受控资源；仅在问题需要时调用可用的只读工具，并清楚区分工具事实、推断和待验证内容。"
	}
	var assistantMu sync.Mutex
	assistantPersisted := false
	var assistantPersistErr error
	var pendingAssistantCompleted *aiengine.Event
	var pendingTerminalEvents []aiengine.Event
	result, err := o.engine.Execute(ctx, aiengine.Request{ExecutionID: diagnosisExecutionID(session.ID), ActorID: dereference(session.ActorUserID), ScopeID: session.ScopeID, AIProviderResourceID: session.ProviderResourceID, ModelName: session.ModelName, Purpose: aiengine.PurposeDiagnosis, Profile: aiengine.ProfileInteractive, Instruction: instruction, Messages: engineMessages, Context: aiengine.ContextRequest{ResourceIDs: targetIDs}, Input: map[string]any{"question": messages[len(messages)-1].Content, "target_resource_ids": targetIDs, "conversation": conversation}, Budget: aiengine.Budget{MaxToolCalls: 12, MaxTokens: 20000, MaxOutputBytes: 64 << 10, Timeout: o.timeout}, Stream: true, EventSink: func(event aiengine.Event) error {
		if event.Type == "assistant.completed" {
			assistantMu.Lock()
			defer assistantMu.Unlock()
			if assistantPersisted {
				return nil
			}
			text, _ := event.Payload["text"].(string)
			if !assistantPersisted && assistantPersistErr == nil {
				// An adapter may emit an empty completion marker and only expose
				// the final text in Execute's Result. Hold the marker until the
				// fallback message has been persisted below.
				if strings.TrimSpace(text) == "" {
					copy := event
					pendingAssistantCompleted = &copy
					return nil
				}
				_, assistantPersistErr = o.store.AppendMessage(context.WithoutCancel(ctx), session.ID, AppendMessageInput{Role: "assistant", Content: safeText(text, 8000)})
				if assistantPersistErr == nil {
					assistantPersisted = true
					pendingAssistantCompleted = nil
				}
			}
			if assistantPersistErr != nil {
				return assistantPersistErr
			}
			return o.appendEvent(context.Background(), session.ID, CreateEventInput{Type: event.Type, Payload: event.Payload})
		}
		if event.Type == "execution.completed" || event.Type == "execution.failed" || event.Type == "execution.cancelled" {
			assistantMu.Lock()
			defer assistantMu.Unlock()
			if !assistantPersisted {
				pendingTerminalEvents = append(pendingTerminalEvents, event)
				return nil
			}
		}
		return o.appendEvent(context.Background(), session.ID, CreateEventInput{Type: event.Type, Payload: event.Payload})
	}, ObservationSink: func(observed aiengine.ToolObservation) { o.captureObservation(session.ID, observed) }})
	if err != nil {
		// A terminal AIEngine event can arrive before Execute returns. Flush it
		// before closing the diagnosis session so cancellation and failure are
		// visible to reconnecting clients even when no assistant answer exists.
		assistantMu.Lock()
		pendingTerminal := append([]aiengine.Event(nil), pendingTerminalEvents...)
		assistantMu.Unlock()
		for _, event := range pendingTerminal {
			o.appendEvent(context.Background(), session.ID, CreateEventInput{Type: event.Type, Payload: event.Payload})
		}
		code := runnerErrorCode(err)
		if result.Status == aiengine.StatusCancelled || code == "cancelled" || code == "timeout" {
			o.cancel(session.ID, code, err)
		} else {
			o.fail(session.ID, code, err)
		}
		return
	}
	if result.Status == aiengine.StatusFailed || result.Status == aiengine.StatusCancelled {
		assistantMu.Lock()
		pendingTerminal := append([]aiengine.Event(nil), pendingTerminalEvents...)
		assistantMu.Unlock()
		for _, event := range pendingTerminal {
			o.appendEvent(context.Background(), session.ID, CreateEventInput{Type: event.Type, Payload: event.Payload})
		}
		if result.Status == aiengine.StatusCancelled {
			o.cancel(session.ID, result.ErrorCode, errors.New(result.ErrorMessage))
		} else {
			o.fail(session.ID, result.ErrorCode, errors.New(result.ErrorMessage))
		}
		return
	}
	assistantMu.Lock()
	assistantError := assistantPersistErr
	assistantDone := assistantPersisted
	assistantMu.Unlock()
	if assistantError != nil {
		o.fail(session.ID, "assistant_message", assistantError)
		return
	}
	if strings.TrimSpace(result.Output) == "" {
		o.fail(session.ID, "assistant_message", errors.New("AIEngine returned an empty final response"))
		return
	}
	// Engines that do not expose lifecycle events (for example a minimal test
	// adapter) still receive the same persistence guarantee after Execute
	// returns. The production AgentRunner emits assistant.completed before it
	// returns, so this branch is normally not taken.
	if !assistantDone {
		if _, err := o.store.AppendMessage(context.WithoutCancel(ctx), session.ID, AppendMessageInput{Role: "assistant", Content: safeText(result.Output, 8000)}); err != nil {
			o.fail(session.ID, "assistant_message", err)
			return
		}
		assistantMu.Lock()
		assistantPersisted = true
		assistantMu.Unlock()
	}
	// Complete the event stream only after the assistant message is durable.
	// Replace an empty adapter marker with the final result so consumers never
	// observe a completion event without a corresponding answer.
	assistantMu.Lock()
	pendingAssistant := pendingAssistantCompleted
	pendingTerminal := append([]aiengine.Event(nil), pendingTerminalEvents...)
	assistantMu.Unlock()
	if pendingAssistant != nil {
		payload := pendingAssistant.Payload
		if payload == nil {
			payload = map[string]any{}
		} else {
			copied := make(map[string]any, len(payload)+1)
			for key, value := range payload {
				copied[key] = value
			}
			payload = copied
		}
		payload["text"] = safeText(result.Output, 8000)
		if err := o.appendEvent(context.Background(), session.ID, CreateEventInput{Type: "assistant.completed", Payload: payload}); err != nil {
			o.fail(session.ID, "assistant_event", err)
			return
		}
	}
	for _, event := range pendingTerminal {
		if err := o.appendEvent(context.Background(), session.ID, CreateEventInput{Type: event.Type, Payload: event.Payload}); err != nil {
			o.fail(session.ID, "execution_event", err)
			return
		}
	}
	for _, step := range plan.Steps {
		if step.Phase == "collect" {
			status, detail := "skipped", "本轮未选择上下文资源，直接完成对话。"
			if len(targetIDs) > 0 {
				status, detail = "succeeded", "已完成受控工具执行。"
				if result.ToolCallCount == 0 {
					status, detail = "skipped", "已加载上下文资源，但本轮回答未调用工具。"
				}
			}
			_, _ = o.store.UpdateStep(ctx, step.ID, status, detail)
		}
	}
	if _, err = o.store.SetStatus(ctx, session.ID, StatusAnalyzing); err != nil {
		return
	}
	o.appendEvent(ctx, session.ID, CreateEventInput{Type: "phase.changed", Payload: map[string]any{"phase": StatusAnalyzing}})
	evidence, err := o.store.Evidence(ctx, session.ID)
	if err != nil {
		o.fail(session.ID, "evidence", err)
		return
	}
	output := safeText(result.Output, 8000)
	evidenceIDs := make([]string, 0, len(evidence))
	for _, item := range evidence {
		evidenceIDs = append(evidenceIDs, item.ID)
	}
	for _, step := range plan.Steps {
		if step.Phase == "verify" {
			status, detail := "succeeded", "已核验本会话中可引用的 Evidence。"
			if len(targetIDs) == 0 {
				status, detail = "skipped", "本轮未选择上下文资源，不需要 Evidence 核验。"
			} else if len(evidenceIDs) == 0 {
				status, detail = "skipped", "未采集到 Evidence；涉及资源的结论需要进一步核验。"
			}
			_, _ = o.store.UpdateStep(ctx, step.ID, status, detail)
		}
	}
	reportStatus, conclusion := "succeeded", output
	if len(targetIDs) > 0 && len(evidenceIDs) == 0 {
		reportStatus = "warning"
		conclusion = output + "\n\n> 未采集到工具证据；涉及资源状态或内容的结论请进一步核验。"
		_, _ = o.store.SaveHypothesis(ctx, Hypothesis{SessionID: session.ID, Statement: output, Status: "needs_verification", Confidence: 0})
	} else if len(evidenceIDs) > 0 {
		_, _ = o.store.SaveHypothesis(ctx, Hypothesis{SessionID: session.ID, Statement: output, Status: "supported", Confidence: 0.5, EvidenceIDs: evidenceIDs})
	}
	recommendations := json.RawMessage(`[]`)
	if len(evidenceIDs) > 0 {
		recommendations, _ = json.Marshal([]string{"查看已引用 Evidence 的来源、时间窗口和原始内容后再执行任何变更。"})
	}
	report, err := o.store.SaveReport(ctx, Report{SessionID: session.ID, Status: reportStatus, Conclusion: conclusion, Recommendations: recommendations, EvidenceIDs: evidenceIDs})
	if err != nil {
		o.fail(session.ID, "report", err)
		return
	}
	for _, step := range plan.Steps {
		if step.Phase == "summarize" {
			detail := "已生成回答。"
			if len(evidenceIDs) > 0 {
				detail = "已生成带 Evidence ID 的回答。"
			}
			_, _ = o.store.UpdateStep(ctx, step.ID, "succeeded", detail)
		}
	}
	_, _ = o.store.Finish(ctx, session.ID, StatusSucceeded, "", "")
	o.appendEvent(context.Background(), session.ID, CreateEventInput{Type: "report.ready", Payload: map[string]any{"report_id": report.ID, "evidence_ids": evidenceIDs, "status": report.Status}})
}

func (o *Orchestrator) captureEvidence(sessionID string, observed connector.Evidence, toolName, targetResourceID string) {
	item, err := o.store.SaveEvidence(context.Background(), sessionID, CreateEvidenceInput{TargetResourceID: targetResourceID, SourceResourceID: observed.SourceResourceID, Capability: string(observed.Capability), CollectedAt: observed.CollectedAt, WindowStart: windowStart(observed.Window), WindowEnd: windowEnd(observed.Window), Summary: mustJSON(observed.Summary), Content: observed.Data, Partial: observed.Partial, Untrusted: true})
	if err == nil {
		o.appendEvent(context.Background(), sessionID, CreateEventInput{Type: "tool.completed", Payload: map[string]any{"tool": toolName, "target_resource_id": targetResourceID, "capability": item.Capability}})
		o.appendEvent(context.Background(), sessionID, CreateEventInput{Type: "evidence.collected", Payload: map[string]any{"evidence_id": item.ID, "source_resource_id": item.SourceResourceID, "capability": item.Capability, "partial": item.Partial}})
	}
}

func (o *Orchestrator) captureObservation(sessionID string, observed aiengine.ToolObservation) {
	content := mustJSON(observed.Result.Output)
	item, err := o.store.SaveEvidence(context.Background(), sessionID, CreateEvidenceInput{TargetResourceID: observed.ResourceID, SourceResourceID: observed.ResourceID, Capability: observed.ToolName, CollectedAt: time.Now().UTC(), Summary: mustJSON(map[string]any{"tool": observed.ToolName, "iteration": observed.Iteration, "call_id": observed.CallID}), Content: content, Untrusted: observed.Result.Untrusted})
	if err == nil {
		o.appendEvent(context.Background(), sessionID, CreateEventInput{Type: "evidence.collected", Payload: map[string]any{"evidence_id": item.ID, "source_resource_id": item.SourceResourceID, "capability": item.Capability, "untrusted": item.Untrusted}})
	}
}

func diagnosisExecutionID(sessionID string) string { return "diagnosis-" + sessionID }

func (o *Orchestrator) fail(sessionID, code string, cause error) {
	message := "AIEngine execution failed"
	if cause != nil {
		if value := safeText(cause.Error(), 1000); strings.TrimSpace(value) != "" {
			message = value
		}
	}
	_, _ = o.store.Finish(context.Background(), sessionID, StatusFailed, code, message)
	o.appendEvent(context.Background(), sessionID, CreateEventInput{Type: "diagnosis.failed", Payload: map[string]any{"code": code, "message": message}})
}

func (o *Orchestrator) cancel(sessionID, code string, cause error) {
	message := "AIEngine execution was cancelled"
	if cause != nil {
		if value := safeText(cause.Error(), 1000); strings.TrimSpace(value) != "" {
			message = value
		}
	}
	_, _ = o.store.Finish(context.Background(), sessionID, StatusCancelled, code, message)
	o.appendEvent(context.Background(), sessionID, CreateEventInput{Type: "diagnosis.cancelled", Payload: map[string]any{"code": code, "message": message}})
}

// appendEvent is the single event persistence path for an orchestration run.
// The AIEngine can invoke its EventSink and ObservationSink from different
// goroutines (for example when a model emits multiple tool calls in one
// turn), so serializing writes keeps database sequence IDs and the SSE
// timeline deterministic.
func (o *Orchestrator) appendEvent(ctx context.Context, sessionID string, input CreateEventInput) error {
	if o == nil || o.Service == nil {
		return nil
	}
	return o.Service.appendEvent(ctx, sessionID, input)
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
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	return "ai_engine"
}

var sensitiveText = regexp.MustCompile(`(?i)(\b(?:bearer|token|password|secret|credential|api[_-]?key|authorization|private[_-]?key|client[_-]?secret|access[_-]?key)\b\s*(?:[:=]\s*|\s+))(?:bearer\s+)?[^\s,;]+`)

func safeText(value string, maximum int) string {
	value = sensitiveText.ReplaceAllString(value, "$1[REDACTED]")
	if maximum > 0 && len([]rune(value)) > maximum {
		return string([]rune(value)[:maximum])
	}
	return value
}

package diagnosis

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"opskeeper/backend/aiengine"
)

const causalChainInstruction = `你是诊断证据编排器。只根据输入中给出的最终回答、Evidence 和公开分析摘要，输出一个紧凑的 JSON 因果论证图。不要调用工具，不要复述工具调用过程，不要输出隐藏思维过程。

节点只描述已知事实、明确推断、排除项或待核验项。关系方向必须从因到果。一个节点或关系如无 Evidence 引用，状态必须为 unverified。AI 分析摘要只能解释证据关系，不能替代 Evidence。只能引用输入中出现的 evidence_ids 和 observation_ids；不要编造 ID。

kind 只能是 cause、mechanism、effect、exclusion、unknown；status 只能是 confirmed、likely、unverified、refuted；relation 只能是 causes、contributes_to、explains、contradicts、rules_out。节点 ID 使用 n1、n2 等简短稳定字符串。最多 6 个节点和 6 条关系。`

var causalChainSchema = json.RawMessage(`{
  "type":"object","required":["summary","nodes","links"],
  "properties":{
    "summary":{"type":"string"},
    "nodes":{"type":"array"},
    "links":{"type":"array"}
  }
}`)

type causalChainDraft struct {
	Summary string       `json:"summary"`
	Nodes   []CausalNode `json:"nodes"`
	Links   []CausalLink `json:"links"`
}

func (o *Orchestrator) compileCausalChain(ctx context.Context, session Session, run Run, conclusion string, evidence []Evidence) (CausalChain, error) {
	input := map[string]any{
		"final_answer":                 conclusion,
		"evidence":                     causalEvidenceInput(evidence),
		"public_analysis_observations": causalObservationInput(ctx, o.store, session.ID, run.Sequence),
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return CausalChain{}, fmt.Errorf("encode causal chain input: %w", err)
	}
	result, err := o.engine.Execute(ctx, aiengine.Request{
		ExecutionID: diagnosisExecutionID(session.ID) + "-chain-" + run.ID,
		ActorID:     dereference(session.ActorUserID), ScopeID: session.ScopeID,
		AIProviderResourceID: session.ProviderResourceID, ModelName: session.ModelName,
		Purpose: aiengine.PurposeDiagnosis, Profile: aiengine.ProfileInteractive,
		Task:          "编排本轮诊断的精选因果证据链",
		Instruction:   causalChainInstruction,
		Messages:      []aiengine.Message{{Role: "user", Content: string(encoded)}},
		OutputSchema:  causalChainSchema,
		Budget:        aiengine.Budget{MaxIterations: 1, MaxToolCalls: 0, MaxTokens: 12000, MaxOutputBytes: 16 << 10, Timeout: o.timeout},
		RestrictTools: true,
	})
	if err == nil {
		var draft causalChainDraft
		if decodeErr := json.Unmarshal([]byte(result.Output), &draft); decodeErr == nil {
			if chain, valid := validateCausalDraft(session.ID, run.ID, draft, evidence); valid {
				return chain, nil
			}
		}
	}
	return fallbackCausalChain(session.ID, run.ID, conclusion, evidence), nil
}

func causalEvidenceInput(evidence []Evidence) []map[string]any {
	items := make([]map[string]any, 0, len(evidence))
	for _, item := range evidence {
		items = append(items, map[string]any{
			"id": item.ID, "run_id": item.RunID, "capability": item.Capability,
			"collected_at": item.CollectedAt, "summary": item.Summary,
			"partial": item.Partial, "untrusted": item.Untrusted,
		})
	}
	return items
}

func causalObservationInput(ctx context.Context, store Store, sessionID string, runSequence int) []map[string]string {
	events, err := store.EventsAfter(ctx, sessionID, 0, 5000)
	if err != nil {
		return nil
	}
	// Runs are ordered by their execution.started marker. Keep narrative
	// observations scoped to this answer, while allowing the curator to cite
	// older raw Evidence when a follow-up explicitly builds on it.
	start, end, executionCount := 0, len(events), 0
	for index, event := range events {
		if event.Type != "execution.started" {
			continue
		}
		executionCount++
		if executionCount == runSequence {
			start = index
			for next := index + 1; next < len(events); next++ {
				if events[next].Type == "execution.started" {
					end = next
					break
				}
			}
			break
		}
	}
	if executionCount == 0 {
		start, end = 0, len(events)
	}
	items := make([]map[string]string, 0)
	for _, event := range events[start:end] {
		if event.Type != "assistant.progress" {
			continue
		}
		var payload map[string]any
		if json.Unmarshal(event.Payload, &payload) != nil {
			continue
		}
		if strings.TrimSpace(fmt.Sprint(payload["kind"])) != "analysis" {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(payload["text"]))
		if text == "" {
			continue
		}
		items = append(items, map[string]string{"id": fmt.Sprintf("event-%d", event.ID), "text": safeText(text, 800)})
	}
	return items
}

func validateCausalDraft(sessionID, runID string, draft causalChainDraft, evidence []Evidence) (CausalChain, bool) {
	if strings.TrimSpace(draft.Summary) == "" || len(draft.Nodes) == 0 || len(draft.Nodes) > 6 || len(draft.Links) > 6 {
		return CausalChain{}, false
	}
	validEvidence := make(map[string]struct{}, len(evidence))
	for _, item := range evidence {
		validEvidence[item.ID] = struct{}{}
	}
	validNodes := map[string]struct{}{}
	for index := range draft.Nodes {
		node := &draft.Nodes[index]
		node.ID = strings.TrimSpace(node.ID)
		node.Statement = safeText(strings.TrimSpace(node.Statement), 800)
		if node.ID == "" || node.Statement == "" || !validNodeKind(node.Kind) || !validNodeStatus(node.Status) {
			return CausalChain{}, false
		}
		if _, exists := validNodes[node.ID]; exists {
			return CausalChain{}, false
		}
		validNodes[node.ID] = struct{}{}
		node.EvidenceIDs = validReferences(node.EvidenceIDs, validEvidence)
		node.ObservationIDs = validObservationIDs(node.ObservationIDs)
		if len(node.EvidenceIDs) == 0 && node.Status != "unverified" {
			return CausalChain{}, false
		}
		node.Confidence = normalizedConfidence(node.Confidence)
	}
	for index := range draft.Links {
		link := &draft.Links[index]
		if _, ok := validNodes[link.From]; !ok {
			return CausalChain{}, false
		}
		if _, ok := validNodes[link.To]; !ok {
			return CausalChain{}, false
		}
		if link.From == link.To || !validRelation(link.Relation) || !validNodeStatus(link.Status) {
			return CausalChain{}, false
		}
		link.Statement = safeText(strings.TrimSpace(link.Statement), 600)
		link.EvidenceIDs = validReferences(link.EvidenceIDs, validEvidence)
		link.ObservationIDs = validObservationIDs(link.ObservationIDs)
		if len(link.EvidenceIDs) == 0 && link.Status != "unverified" {
			return CausalChain{}, false
		}
		link.Confidence = normalizedConfidence(link.Confidence)
	}
	return CausalChain{SessionID: sessionID, RunID: runID, Status: "active", Summary: safeText(strings.TrimSpace(draft.Summary), 1600), Nodes: draft.Nodes, Links: draft.Links}, true
}

func fallbackCausalChain(sessionID, runID, conclusion string, evidence []Evidence) CausalChain {
	ids := make([]string, 0, len(evidence))
	for _, item := range evidence {
		if item.RunID == runID {
			ids = append(ids, item.ID)
		}
	}
	status := "unverified"
	confidence := 0.0
	chainStatus := "partial"
	if len(ids) > 0 {
		status = "likely"
		confidence = 0.4
	}
	return CausalChain{SessionID: sessionID, RunID: runID, Status: chainStatus, Summary: "本轮尚未形成可确认的完整因果链；以下结论仅保留可追溯来源。", Nodes: []CausalNode{{ID: "n1", Kind: "unknown", Statement: safeText(strings.TrimSpace(conclusion), 800), Status: status, Confidence: confidence, EvidenceIDs: ids}}}
}

func validReferences(ids []string, available map[string]struct{}) []string {
	result := []string{}
	seen := map[string]struct{}{}
	for _, id := range ids {
		if _, ok := available[id]; ok {
			if _, duplicate := seen[id]; !duplicate {
				result = append(result, id)
				seen[id] = struct{}{}
			}
		}
	}
	sort.Strings(result)
	return result
}
func validObservationIDs(ids []string) []string {
	result := []string{}
	seen := map[string]struct{}{}
	for _, id := range ids {
		if strings.HasPrefix(id, "event-") && strings.TrimPrefix(id, "event-") != "" {
			if _, duplicate := seen[id]; !duplicate {
				result = append(result, id)
				seen[id] = struct{}{}
			}
		}
	}
	sort.Strings(result)
	return result
}
func normalizedConfidence(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
func validNodeKind(value string) bool {
	return value == "cause" || value == "mechanism" || value == "effect" || value == "exclusion" || value == "unknown"
}
func validNodeStatus(value string) bool {
	return value == "confirmed" || value == "likely" || value == "unverified" || value == "refuted"
}
func validRelation(value string) bool {
	return value == "causes" || value == "contributes_to" || value == "explains" || value == "contradicts" || value == "rules_out"
}

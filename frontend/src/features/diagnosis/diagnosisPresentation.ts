import type { DiagnosisCausalChain, DiagnosisEvidence, DiagnosisMessage, DiagnosisSnapshot, Resource } from '../../lib/api';
import { diagnosisActionLabel } from './diagnosisUtils';
import { diagnosisEvidenceTimeline, diagnosisLiveTimeline } from './diagnosisTimelines';

export function diagnosisResourceName(resources: Resource[], targets: Resource[], resourceId?: string) {
  if (!resourceId) return '未标记资源';
  return resources.find((resource) => resource.id === resourceId)?.name
    ?? targets.find((resource) => resource.id === resourceId)?.name
    ?? resourceId;
}

export function isLastDiagnosisUser(snapshot: DiagnosisSnapshot | null, index: number) {
  const messages = snapshot?.messages ?? [];
  return index === messages.map((message) => message.role).lastIndexOf('user');
}

export function diagnosisHasPersistedNewAnswer(snapshot: DiagnosisSnapshot | null, baseline: number) {
  return Boolean(snapshot && snapshot.messages.filter((message) => message.role === 'assistant').length > baseline);
}

export function shouldShowEmptyDiagnosisAnswer(
  snapshot: DiagnosisSnapshot | null,
  answerCompleted: boolean,
  streamingText: string,
  generating: boolean
) {
  if (!snapshot || !answerCompleted || generating || streamingText.trim()) return false;
  // `assistant.completed` and `report.ready` arrive before a refresh may
  // expose the durable message. Only call it empty after the session itself
  // is durably succeeded and there is still no assistant message at all.
  return snapshot.session.status === 'succeeded' &&
    !snapshot.messages.some((message) => message.role === 'assistant');
}

export function diagnosisEvidenceSummary(evidence: DiagnosisEvidence) {
  const entries = Object.entries(evidence.summary ?? {}).slice(0, 3).map(([key, value]) => {
    const text = typeof value === 'string' ? value : JSON.stringify(value);
    return `${key}: ${text ?? ''}`;
  });
  return entries.join(' · ') || '已保存工具返回结果，可展开查看完整内容。';
}

export function diagnosisProcessText(snapshot: DiagnosisSnapshot | null) {
  if (!snapshot) return '';
  const lines: string[] = [];
  for (const item of diagnosisLiveTimeline(snapshot)) {
    if (item.kind === 'analysis') {
      if (item.text) lines.push(item.text);
      continue;
    }
    const actions = item.actions ?? [item];
    if (actions.length > 1) lines.push(`调用工具 · ${actions.length} 个动作 · ${item.status ?? '执行中'}`);
    for (const action of actions) {
      lines.push(`${diagnosisActionLabel(action)} · ${action.status ?? '执行中'} · ${action.duration ?? '—'}`);
      if (action.input) lines.push(`入参:\n${action.input}`);
      if (action.output) lines.push(`出参:\n${action.output}`);
    }
  }
  return lines.join('\n\n');
}

export function diagnosisEvidenceSourceTools(snapshot: DiagnosisSnapshot, evidence: DiagnosisEvidence) {
  const labels = diagnosisEvidenceTimeline(snapshot)
    .flatMap((turn) => turn.children ?? [])
    .flatMap((item) => item.kind === 'tool-group' ? item.children ?? [] : [item])
    .filter((item) => item.evidenceIds?.includes(evidence.id))
    .map((item) => item.tool ?? item.title);
  return [...new Set(labels)];
}

export function activeDiagnosisCausalChain(snapshot: DiagnosisSnapshot | null): DiagnosisCausalChain | null {
  if (!snapshot?.causal_chains?.length) return null;
  return snapshot.causal_chains.find((chain) => chain.status === 'active')
    ?? snapshot.causal_chains[0]
    ?? null;
}

export function decodeDiagnosisCode(encoded: string) {
  try {
    return decodeURIComponent(encoded);
  } catch {
    return encoded;
  }
}

export function diagnosisMessageClipboardText(
  message: DiagnosisMessage,
  processExpanded: boolean,
  snapshot: DiagnosisSnapshot | null
) {
  const answer = message.content ?? '';
  const process = processExpanded ? diagnosisProcessText(snapshot) : '';
  return process ? `${answer}\n\n执行过程\n\n${process}` : answer;
}

export function diagnosisStreamingClipboardText(text: string, snapshot: DiagnosisSnapshot | null) {
  return `${text}\n\n执行过程\n\n${diagnosisProcessText(snapshot)}`;
}

import { describe, expect, it } from 'vitest';
import type { DiagnosisSnapshot } from '../../lib/api';
import {
  shouldShowEmptyDiagnosisAnswer
} from './diagnosisPresentation';

const snapshot = {
  messages: [
    { id: 'old', role: 'assistant', content: 'old answer' },
    { id: 'new', role: 'assistant', content: 'new answer' }
  ]
} as DiagnosisSnapshot;

describe('diagnosis answer presentation', () => {
  it('waits for a durable succeeded snapshot before showing an empty answer', () => {
    expect(shouldShowEmptyDiagnosisAnswer({ ...snapshot, session: { ...snapshot.session, status: 'analyzing' } }, true, '', false)).toBe(false);
    expect(shouldShowEmptyDiagnosisAnswer({ ...snapshot, messages: [], session: { ...snapshot.session, status: 'succeeded' } }, true, '', false)).toBe(true);
    expect(shouldShowEmptyDiagnosisAnswer({ ...snapshot, session: { ...snapshot.session, status: 'succeeded' } }, true, '', false)).toBe(false);
  });
});

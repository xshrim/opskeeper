import { describe, expect, it } from 'vitest';
import {
  normalizeDiagnosisMarkdown,
  renderDiagnosisMarkdown
} from './diagnosisMarkdown';

describe('diagnosis markdown rendering', () => {
  it('renders a standard heading and paragraph without rewriting the source', () => {
    const source = '## 结论\n\n当前运行正常。';
    expect(normalizeDiagnosisMarkdown(source)).toBe(source);
    expect(renderDiagnosisMarkdown(source)).toContain('<h2>结论</h2>');
    expect(renderDiagnosisMarkdown(source)).toContain('<p>当前运行正常。</p>');
  });

  it('does not guess a heading from malformed provider text', () => {
    const source = '##结论当前运行正常。';
    expect(normalizeDiagnosisMarkdown(source)).toBe(source);
    expect(renderDiagnosisMarkdown(source)).not.toContain('<h2>');
    expect(renderDiagnosisMarkdown(source)).toContain('##结论当前运行正常。');
  });

  it('does not flash an empty code panel for an opening fence', () => {
    expect(renderDiagnosisMarkdown('```sql\n', true)).not.toContain(
      'diagnosis-code-wrap'
    );
    expect(renderDiagnosisMarkdown('```sql\nSELECT 1;', true)).toContain(
      'diagnosis-code-wrap'
    );
  });

  it('renders completed fenced code with its language', () => {
    const html = renderDiagnosisMarkdown('```sql\nSELECT 1;\n```');
    expect(html).toContain('diagnosis-code-wrap');
    expect(html).toContain('language-sql');
    expect(html).toContain('SELECT');
    expect(html).toContain('hljs-number">1</span>;');
    expect(html).toContain('data-code-copy="SELECT%201%3B"');
  });

  it('temporarily closes incomplete inline delimiters only while streaming', () => {
    expect(normalizeDiagnosisMarkdown('**重点', true)).toBe('**重点**');
    expect(normalizeDiagnosisMarkdown('`字段', true)).toBe('`字段`');
    expect(normalizeDiagnosisMarkdown('**重点')).toBe('**重点');
  });
});

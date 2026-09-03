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

  it('renders a completed heading even when the answer is still a prefix', () => {
    const prefix = '## 结论\n\n当前运行';
    const html = renderDiagnosisMarkdown(prefix);
    expect(html).toContain('<h2>结论</h2>');
    expect(html).toContain('<p>当前运行</p>');
  });

  it('does not guess a heading from malformed provider text', () => {
    const source = '##结论当前运行正常。';
    expect(normalizeDiagnosisMarkdown(source)).toBe(source);
    expect(renderDiagnosisMarkdown(source)).not.toContain('<h2>');
    expect(renderDiagnosisMarkdown(source)).toContain('##结论当前运行正常。');
  });

  it('keeps incomplete markdown source unchanged', () => {
    const source = '```sql\nSELECT 1;';
    expect(normalizeDiagnosisMarkdown(source)).toBe(source);
    expect(renderDiagnosisMarkdown(source)).toContain('SELECT');
  });

  it('renders completed fenced code with its language', () => {
    const html = renderDiagnosisMarkdown('```sql\nSELECT 1;\n```');
    expect(html).toContain('diagnosis-code-wrap');
    expect(html).toContain('language-sql');
    expect(html).toContain('SELECT');
    expect(html).toContain('hljs-number">1</span>;');
    expect(html).toContain('data-code-copy="SELECT%201%3B"');
  });

  it('uses identical rendering for the same live and persisted source', () => {
    const source = '## 结论\n\n**关键证据**\n\n```sql\nSELECT 1;\n```';
    expect(renderDiagnosisMarkdown(source)).toBe(renderDiagnosisMarkdown(source));
  });
});

import hljs from 'highlight.js/lib/common';
import { Marked, type RendererObject } from 'marked';

function escapeHTML(text: string) {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

/**
 * Make an incomplete provider response parseable without changing completed
 * Markdown semantics. The same function is used for live and persisted text;
 * `streaming` only controls temporary delimiter completion.
 */
export function normalizeDiagnosisMarkdown(value: string, streaming = false) {
  let source = String(value ?? '')
    .replace(/\r\n?/g, '\n')
    .replace(/\\r?\\n/g, '\n');

  if (streaming) {
    const fences = [...source.matchAll(/^\s*(`{3,}|~{3,})[^\n]*$/gm)].map((match) => match[1]);
    if (fences.length % 2 !== 0) {
      const fence = fences[fences.length - 1] ?? '```';
      const trimmed = source.replace(/[\s\r\n]*$/, '');
      // Do not flash a panel for an opening fence that has no content yet.
      if (new RegExp(`(?:^|\\n)\\s*${fence}[a-zA-Z0-9_+.-]*$`).test(trimmed)) {
        return trimmed.replace(new RegExp(`\\n?\\s*${fence}[a-zA-Z0-9_+.-]*$`), '');
      }
      source = `${trimmed}\n\n${fence.slice(0, 3)}`;
    }
  }

  // Protect complete code blocks from text-level repairs. This also means a
  // heading-looking line inside SQL, JSON or shell code remains code.
  const chunks = source.split(/(```[\s\S]*?```|~~~[\s\S]*?~~~)/g);
  return chunks.map((chunk, index) => {
    if (index % 2 === 1) return chunk;
    let normalized = chunk;
    if (streaming) {
      // Close delimiters temporarily so completed text renders immediately;
      // the next delta replaces this HTML with the canonical parse.
      if ((normalized.match(/\*\*/g) ?? []).length % 2 !== 0) normalized += '**';
      if ((normalized.match(/(?<!`)`(?!`)/g) ?? []).length % 2 !== 0) normalized += '`';
    }
    return normalized;
  }).join('');
}

function renderCode(source: string, language = '') {
  const normalized = language.trim().toLowerCase().replace(/[^a-z0-9_+-]/g, '');
  let rendered = escapeHTML(source);
  if (normalized) {
    try {
      rendered = hljs.highlight(source, { language: normalized, ignoreIllegals: true }).value;
    } catch {
      // Unknown language: keep a safe plain-text block.
    }
  }
  const encoded = encodeURIComponent(source);
  return `<div class="diagnosis-code-wrap"><button class="diagnosis-code-copy" type="button" data-code-copy="${encoded}" aria-label="复制代码" title="复制代码"><span aria-hidden="true">⧉</span></button><pre class="diagnosis-code-block"><code${normalized ? ` class="language-${normalized} hljs"` : ' class="hljs"'}>${rendered}</code></pre></div>`;
}

export function renderDiagnosisMarkdown(value: string, streaming = false) {
  const renderer: RendererObject = {
    code: ({ text, lang }) => {
      const source = String(text ?? '');
      return source.trim() ? renderCode(source, lang ?? '') : '';
    },
    html: ({ text }) => escapeHTML(text),
    link({ href, title, tokens }) {
      const safeHref = /^(?:https?:|mailto:)/i.test(href) ? href : '#';
      const label = this.parser.parseInline(tokens);
      const titleAttr = title ? ` title="${escapeHTML(title)}"` : '';
      return `<a href="${escapeHTML(safeHref)}"${titleAttr} target="_blank" rel="noreferrer">${label}</a>`;
    }
  };
  const parser = new Marked({ gfm: true, breaks: true, renderer });
  try {
    return parser.parse(normalizeDiagnosisMarkdown(value, streaming), { async: false }) as string;
  } catch {
    return `<p>${escapeHTML(String(value ?? ''))}</p>`;
  }
}

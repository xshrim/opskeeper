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
 * Keep the provider text byte-for-byte intact. Markdown is parsed only; the
 * client must not infer headings, insert line breaks, or close delimiters.
 * This same identity function is used for live and persisted messages.
 */
export function normalizeDiagnosisMarkdown(value: string) {
  return String(value ?? '');
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

export function renderDiagnosisMarkdown(value: string) {
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
    return parser.parse(normalizeDiagnosisMarkdown(value), { async: false }) as string;
  } catch {
    return `<p>${escapeHTML(String(value ?? ''))}</p>`;
  }
}

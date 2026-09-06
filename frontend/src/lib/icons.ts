import { icons as lucideIcons } from 'lucide-svelte';

export function formatIconName(name: string) {
  return name
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .replace(/([A-Z])([A-Z][a-z])/g, '$1 $2');
}

export function teamIconComponent(icon: string | undefined): any {
  const key = icon ?? 'UsersRound';
  return lucideIcons[key as keyof typeof lucideIcons] ?? lucideIcons.UsersRound;
}

export function iconGlyph(icon: string | undefined) {
  const glyphs: Record<string, string> = {
    platform: '▣',
    team: '♟',
    building: '▦',
    cloud: '☁',
    project: '▰',
    kubernetes: '☸',
    application: '⌘',
    endpoint: '↗',
    schedule: '◷',
    postgresql: '◉',
    redis: '◒',
    kafka: '◫',
    search: '⌕',
    middleware: '◇',
    llm: '✦',
    mcp: '⌁',
    skill: '✧',
    metrics: '▥',
    logs: '≋',
    traces: '⌁',
    observability: '◌',
    api: '⇄',
    notification: '♢',
    runbook: '☷',
    storage: '▤',
    credential: '⚿',
    resource: '◇'
  };
  return glyphs[icon ?? 'resource'] ?? icon?.slice(0, 1).toUpperCase() ?? '◇';
}

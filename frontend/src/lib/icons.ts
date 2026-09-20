export function formatIconName(name: string) {
  return name
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .replace(/([A-Z])([A-Z][a-z])/g, '$1 $2');
}

export function iconGlyph(icon: string | undefined) {
  const glyphs: Record<string, string> = {
    platform: 'lucide:Building2',
    team: 'lucide:UsersRound',
    building: 'lucide:Building2',
    cloud: 'lucide:Cloud',
    project: 'lucide:FolderKanban',
    kubernetes: 'iconify:simple-icons:kubernetes',
    application: 'lucide:AppWindow',
    endpoint: 'lucide:ArrowLeftRight',
    schedule: 'lucide:CalendarClock',
    postgresql: 'iconify:simple-icons:postgresql',
    redis: 'iconify:simple-icons:redis',
    kafka: 'iconify:simple-icons:apache-kafka',
    rabbitmq: 'iconify:simple-icons:rabbitmq',
    docker: 'iconify:simple-icons:docker',
    minio: 'iconify:simple-icons:minio',
    mongodb: 'iconify:simple-icons:mongodb',
    elasticsearch: 'iconify:simple-icons:elasticsearch',
    search: 'lucide:Search',
    middleware: 'lucide:Workflow',
    llm: 'lucide:BrainCircuit',
    mcp: 'lucide:Waypoints',
    skill: 'lucide:Sparkles',
    metrics: 'lucide:Activity',
    logs: 'lucide:Activity',
    traces: 'lucide:Activity',
    observability: 'lucide:Activity',
    api: 'lucide:ArrowLeftRight',
    notification: 'lucide:Bell',
    runbook: 'lucide:BookOpen',
    storage: 'lucide:Database',
    credential: 'lucide:KeyRound',
    resource: 'lucide:Circle'
  };
  return glyphs[icon ?? 'resource'] ?? (icon?.startsWith('lucide:') || icon?.startsWith('iconify:') ? icon : 'lucide:Circle');
}

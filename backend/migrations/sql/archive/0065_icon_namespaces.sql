-- Persist all selectable icon values using the explicit lucide:/iconify: scheme.
-- Existing legacy values are converted once; runtime code does not parse them.

UPDATE teams
SET icon = CASE
  WHEN icon = 'team' THEN 'lucide:UsersRound'
  WHEN icon LIKE 'lucide:%' OR icon LIKE 'iconify:%' OR icon LIKE 'data:image/%' THEN icon
  WHEN icon LIKE 'brand:%' THEN 'lucide:Circle'
  ELSE 'lucide:' || icon
END;

UPDATE projects
SET icon = CASE
  WHEN icon = 'project' THEN 'lucide:FolderKanban'
  WHEN icon LIKE 'lucide:%' OR icon LIKE 'iconify:%' OR icon LIKE 'data:image/%' THEN icon
  WHEN icon LIKE 'brand:%' THEN 'lucide:Circle'
  ELSE 'lucide:' || icon
END;

UPDATE platforms
SET icon = CASE
  WHEN icon = 'platform' THEN 'lucide:Building2'
  WHEN icon LIKE 'lucide:%' OR icon LIKE 'iconify:%' OR icon LIKE 'data:image/%' THEN icon
  WHEN icon LIKE 'brand:%' THEN 'lucide:Circle'
  ELSE 'lucide:' || icon
END;

UPDATE applications
SET icon = CASE
  WHEN icon LIKE 'lucide:%' OR icon LIKE 'iconify:%' OR icon LIKE 'data:image/%' THEN icon
  WHEN icon LIKE 'brand:%' THEN 'lucide:Circle'
  ELSE 'lucide:' || icon
END;

UPDATE engines
SET icon = CASE
  WHEN icon LIKE 'lucide:%' OR icon LIKE 'iconify:%' OR icon LIKE 'data:image/%' THEN icon
  WHEN icon LIKE 'brand:%' THEN 'lucide:Circle'
  ELSE 'lucide:' || icon
END;

UPDATE providers
SET config = jsonb_set(
  config,
  '{icon}',
  to_jsonb(CASE
    WHEN config->>'icon' LIKE 'lucide:%' OR config->>'icon' LIKE 'iconify:%' OR config->>'icon' LIKE 'data:image/%' THEN config->>'icon'
    WHEN config->>'icon' LIKE 'brand:%' THEN 'lucide:Bot'
    ELSE 'lucide:' || COALESCE(NULLIF(config->>'icon', ''), 'Bot')
  END),
  true
)
WHERE jsonb_typeof(config) = 'object';

-- Resource schema icons used the old semantic glyph keys. Resolve them once to
-- the same namespaced values used by every other persisted icon field.
UPDATE resource_schemas
SET icon = CASE
  WHEN icon LIKE 'lucide:%' OR icon LIKE 'iconify:%' OR icon LIKE 'data:image/%' THEN icon
  WHEN icon LIKE 'brand:%' THEN 'lucide:Circle'
  WHEN icon IN ('kubernetes', 'kubernetescluster') THEN 'iconify:simple-icons:kubernetes'
  WHEN icon IN ('postgresql', 'postgres') THEN 'iconify:simple-icons:postgresql'
  WHEN icon = 'redis' THEN 'iconify:simple-icons:redis'
  WHEN icon = 'kafka' THEN 'iconify:simple-icons:apache-kafka'
  WHEN icon = 'rabbitmq' THEN 'iconify:simple-icons:rabbitmq'
  WHEN icon = 'docker' THEN 'iconify:simple-icons:docker'
  WHEN icon = 'minio' THEN 'iconify:simple-icons:minio'
  WHEN icon = 'mongodb' THEN 'iconify:simple-icons:mongodb'
  WHEN icon IN ('elasticsearch', 'elastic', 'search') THEN 'iconify:simple-icons:elasticsearch'
  WHEN icon IN ('application', 'app') THEN 'lucide:AppWindow'
  WHEN icon IN ('artifact', 'package') THEN 'lucide:Package'
  WHEN icon IN ('repository', 'git') THEN 'lucide:FolderGit2'
  WHEN icon IN ('host', 'server') THEN 'lucide:Server'
  WHEN icon IN ('endpoint', 'api') THEN 'lucide:ArrowLeftRight'
  WHEN icon IN ('nacos', 'network') THEN 'lucide:Network'
  WHEN icon IN ('database', 'storage') THEN 'lucide:Database'
  WHEN icon IN ('llm', 'ai', 'engine') THEN 'lucide:BrainCircuit'
  WHEN icon IN ('mcp', 'waypoints') THEN 'lucide:Waypoints'
  WHEN icon IN ('skill', 'sparkles') THEN 'lucide:Sparkles'
  WHEN icon IN ('metrics', 'logs', 'traces', 'observability', 'monitor') THEN 'lucide:Activity'
  WHEN icon IN ('notification', 'alert') THEN 'lucide:Bell'
  WHEN icon IN ('runbook', 'book') THEN 'lucide:BookOpen'
  WHEN icon IN ('credential', 'key') THEN 'lucide:KeyRound'
  WHEN icon IN ('resource', 'generic') THEN 'lucide:Circle'
  WHEN icon = 'team' THEN 'lucide:UsersRound'
  WHEN icon = 'platform' THEN 'lucide:Building2'
  WHEN icon = 'project' THEN 'lucide:FolderKanban'
  WHEN icon = 'cloud' THEN 'lucide:Cloud'
  WHEN icon = 'schedule' THEN 'lucide:CalendarClock'
  WHEN icon = 'middleware' THEN 'lucide:Workflow'
  ELSE 'lucide:Circle'
END;

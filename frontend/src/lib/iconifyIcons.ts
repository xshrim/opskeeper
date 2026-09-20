import {
  siAnthropic,
  siApachekafka,
  siDeepseek,
  siDocker,
  siDatadog,
  siElastic,
  siElasticsearch,
  siGit,
  siGitea,
  siGithub,
  siGitlab,
  siGrafana,
  siHelm,
  siHarbor,
  siGooglegemini,
  siKimi,
  siKubernetes,
  siMinimax,
  siMysql,
  siMinio,
  siMongodb,
  siMoonshotai,
  siOllama,
  siOpenrouter,
  siPostgresql,
  siPrometheus,
  siQwen,
  siRabbitmq,
  siRedis,
  siJaeger
} from 'simple-icons';
import chatglm from '@iconify-icons/thesvg/chatglm';
import doubao from '@iconify-icons/thesvg/doubao';
import grokXai from '@iconify-icons/thesvg/grok-xai';
import longcat from '@iconify-icons/thesvg/longcat';
import mimo from '@iconify-icons/thesvg/xiaomi-mimo';
import openaiChatgpt from '@iconify-icons/thesvg/openai-chatgpt';
import siliconflow from '@iconify-icons/thesvg/siliconcloud-siliconflow';
import zhipu from '@iconify-icons/thesvg/zhipu';

type SimpleIconData = {
  title: string;
  path: string;
};

type IconifyIconData = {
  body: string;
  width?: number;
  height?: number;
  left?: number;
  top?: number;
};

export type IconifyIconDefinition =
  | {
      id: string;
      label: string;
      aliases: string[];
      source: 'simple-icons';
      icon: SimpleIconData;
    }
  | {
      id: string;
      label: string;
      aliases: string[];
      source: 'theSVG';
      icon: IconifyIconData;
    };

function simple(
  id: string,
  label: string,
  icon: SimpleIconData,
  aliases: string[] = []
): IconifyIconDefinition {
  return { id, label, aliases, source: 'simple-icons', icon };
}

function theSVG(
  id: string,
  label: string,
  icon: IconifyIconData,
  aliases: string[] = []
): IconifyIconDefinition {
  return { id, label, aliases, source: 'theSVG', icon };
}

export const iconifyIconEntries: IconifyIconDefinition[] = [
  simple('iconify:simple-icons:anthropic', 'Anthropic', siAnthropic),
  simple('iconify:simple-icons:apache-kafka', 'Apache Kafka', siApachekafka, ['Kafka']),
  simple('iconify:simple-icons:datadog', 'Datadog', siDatadog),
  simple('iconify:simple-icons:docker', 'Docker', siDocker),
  simple('iconify:simple-icons:elastic', 'Elastic', siElastic),
  simple('iconify:simple-icons:elasticsearch', 'Elasticsearch', siElasticsearch, ['Elastic Search']),
  simple('iconify:simple-icons:git', 'Git', siGit),
  simple('iconify:simple-icons:gitea', 'Gitea', siGitea),
  simple('iconify:simple-icons:github', 'GitHub', siGithub),
  simple('iconify:simple-icons:gitlab', 'GitLab', siGitlab),
  simple('iconify:simple-icons:grafana', 'Grafana', siGrafana),
  simple('iconify:simple-icons:gemini', 'Gemini', siGooglegemini, ['Google Gemini']),
  simple('iconify:simple-icons:harbor', 'Harbor', siHarbor),
  simple('iconify:simple-icons:helm', 'Helm', siHelm),
  simple('iconify:simple-icons:jaeger', 'Jaeger', siJaeger),
  simple('iconify:simple-icons:kimi', 'Kimi', siKimi),
  simple('iconify:simple-icons:kubernetes', 'Kubernetes', siKubernetes, ['K8s']),
  simple('iconify:simple-icons:minimax', 'MiniMax', siMinimax),
  simple('iconify:simple-icons:minio', 'MinIO', siMinio),
  simple('iconify:simple-icons:mongodb', 'MongoDB', siMongodb),
  simple('iconify:simple-icons:moonshot', 'Moonshot', siMoonshotai, ['Moonshot AI']),
  simple('iconify:simple-icons:mysql', 'MySQL', siMysql),
  simple('iconify:simple-icons:ollama', 'Ollama', siOllama),
  simple('iconify:simple-icons:openrouter', 'OpenRouter', siOpenrouter),
  simple('iconify:simple-icons:postgresql', 'PostgreSQL', siPostgresql, ['Postgres']),
  simple('iconify:simple-icons:prometheus', 'Prometheus', siPrometheus),
  simple('iconify:simple-icons:qwen', 'Qwen', siQwen, ['通义千问']),
  simple('iconify:simple-icons:rabbitmq', 'RabbitMQ', siRabbitmq),
  simple('iconify:simple-icons:redis', 'Redis', siRedis),
  simple('iconify:simple-icons:deepseek', 'DeepSeek', siDeepseek),
  theSVG('iconify:thesvg:chatglm', 'ChatGLM', chatglm, ['Chat GLM']),
  theSVG('iconify:thesvg:zhipu', 'GLM', zhipu, ['ChatGLM', '智谱']),
  theSVG('iconify:thesvg:doubao', 'Doubao', doubao, ['豆包']),
  theSVG('iconify:thesvg:grok-xai', 'Grok', grokXai, ['xAI']),
  theSVG('iconify:thesvg:longcat', 'LongCat', longcat),
  theSVG('iconify:thesvg:openai-chatgpt', 'OpenAI', openaiChatgpt, ['ChatGPT']),
  theSVG('iconify:thesvg:siliconcloud-siliconflow', 'SiliconFlow', siliconflow, ['硅基流动']),
  theSVG('iconify:thesvg:xiaomi-mimo', 'MiMo', mimo, ['Xiaomi MiMo'])
];

const entriesByID = new Map(iconifyIconEntries.map((entry) => [entry.id, entry]));

const namesToIDs: Record<string, string> = {
  Anthropic: 'iconify:simple-icons:anthropic',
  Kafka: 'iconify:simple-icons:apache-kafka',
  Docker: 'iconify:simple-icons:docker',
  Datadog: 'iconify:simple-icons:datadog',
  Elastic: 'iconify:simple-icons:elastic',
  ElasticSearch: 'iconify:simple-icons:elasticsearch',
  Git: 'iconify:simple-icons:git',
  Gitea: 'iconify:simple-icons:gitea',
  GitHub: 'iconify:simple-icons:github',
  GitLab: 'iconify:simple-icons:gitlab',
  Grafana: 'iconify:simple-icons:grafana',
  Gemini: 'iconify:simple-icons:gemini',
  Harbor: 'iconify:simple-icons:harbor',
  Helm: 'iconify:simple-icons:helm',
  Kimi: 'iconify:simple-icons:kimi',
  Kubernetes: 'iconify:simple-icons:kubernetes',
  MiniMax: 'iconify:simple-icons:minimax',
  MySQL: 'iconify:simple-icons:mysql',
  MinIO: 'iconify:simple-icons:minio',
  MongoDB: 'iconify:simple-icons:mongodb',
  Moonshot: 'iconify:simple-icons:moonshot',
  Ollama: 'iconify:simple-icons:ollama',
  OpenAI: 'iconify:thesvg:openai-chatgpt',
  OpenRouter: 'iconify:simple-icons:openrouter',
  PostgreSQL: 'iconify:simple-icons:postgresql',
  Prometheus: 'iconify:simple-icons:prometheus',
  Qwen: 'iconify:simple-icons:qwen',
  RabbitMQ: 'iconify:simple-icons:rabbitmq',
  Redis: 'iconify:simple-icons:redis',
  Jaeger: 'iconify:simple-icons:jaeger',
  DeepSeek: 'iconify:simple-icons:deepseek'
};

export const providerBrandIconValues: Record<string, string> = {
  openai_compatible: 'iconify:thesvg:openai-chatgpt',
  anthropic: namesToIDs.Anthropic,
  deepseek: namesToIDs.DeepSeek,
  gemini: namesToIDs.Gemini,
  kimi: namesToIDs.Kimi,
  minimax: namesToIDs.MiniMax,
  ollama: namesToIDs.Ollama,
  openrouter: namesToIDs.OpenRouter,
  qwen: namesToIDs.Qwen,
  openai: 'iconify:thesvg:openai-chatgpt',
  grok: 'iconify:thesvg:grok-xai',
  glm: 'iconify:thesvg:zhipu',
  mimo: 'iconify:thesvg:xiaomi-mimo',
  longcat: 'iconify:thesvg:longcat',
  doubao: 'iconify:thesvg:doubao',
  siliconflow: 'iconify:thesvg:siliconcloud-siliconflow'
};

export function iconifyIconForValue(value: string | undefined) {
  return value ? entriesByID.get(value) : undefined;
}

export function iconifyValueForName(name: string | undefined) {
  return name ? namesToIDs[name] ?? '' : '';
}

export function providerBrandIconValue(providerType: string | undefined) {
  return providerType ? providerBrandIconValues[providerType] ?? '' : '';
}

<script lang="ts">
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

  export let name = '';
  export let size = 16;

  const brands: Record<string, { title: string; path: string }> = {
    Anthropic: siAnthropic,
    Kafka: siApachekafka,
    Docker: siDocker,
    Datadog: siDatadog,
    Elastic: siElastic,
    ElasticSearch: siElasticsearch,
    Git: siGit,
    Gitea: siGitea,
    GitHub: siGithub,
    GitLab: siGitlab,
    Grafana: siGrafana,
    Helm: siHelm,
    Harbor: siHarbor,
    Gemini: siGooglegemini,
    Kimi: siKimi,
    Kubernetes: siKubernetes,
    MiniMax: siMinimax,
    MySQL: siMysql,
    MinIO: siMinio,
    MongoDB: siMongodb,
    Moonshot: siMoonshotai,
    Ollama: siOllama,
    OpenRouter: siOpenrouter,
    PostgreSQL: siPostgresql,
    Prometheus: siPrometheus,
    Qwen: siQwen,
    RabbitMQ: siRabbitmq,
    Redis: siRedis,
    Jaeger: siJaeger,
    DeepSeek: siDeepseek
  };

  $: brand = brands[name];
</script>

{#if brand}
  <svg
    class="brand-icon"
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="currentColor"
    role="img"
    aria-label={brand.title}
  >
    <path d={brand.path} />
  </svg>
{:else}
  <span class="brand-icon-fallback" aria-hidden="true">◇</span>
{/if}

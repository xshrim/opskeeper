<script lang="ts">
  import {
    siAnthropic,
    siApachekafka,
    siDeepseek,
    siDocker,
    siElasticsearch,
    siGit,
    siGithub,
    siGitlab,
    siGrafana,
    siHelm,
    siKubernetes,
    siMysql,
    siOllama,
    siPostgresql,
    siPrometheus,
    siQwen,
    siRabbitmq,
    siRedis
  } from 'simple-icons';

  export let name = '';
  export let size = 16;

  const brands: Record<string, { title: string; path: string }> = {
    Anthropic: siAnthropic,
    Kafka: siApachekafka,
    Docker: siDocker,
    ElasticSearch: siElasticsearch,
    Git: siGit,
    GitHub: siGithub,
    GitLab: siGitlab,
    Grafana: siGrafana,
    Helm: siHelm,
    Kubernetes: siKubernetes,
    MySQL: siMysql,
    Ollama: siOllama,
    PostgreSQL: siPostgresql,
    Prometheus: siPrometheus,
    Qwen: siQwen,
    RabbitMQ: siRabbitmq,
    Redis: siRedis,
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

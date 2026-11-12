<script lang="ts">
  import {
    AppWindow,
    Boxes,
    Check,
    ChevronRight,
    Cloud,
    Database,
    HardDrive,
    Network,
    Server,
    StickyNote
  } from 'lucide-svelte';

  let selected = 3;
  let dark = false;
  const plans = [
    {
      name: '清晰描边',
      note: '中性边框 + 关系类型着色',
      app: '#536fc4',
      resource: '#087f8c',
      entity: '#a46719',
      edge: 'solid',
      node: 'outline'
    },
    {
      name: '柔和底色',
      note: '低饱和填充 + 轻量曲线',
      app: '#536fc4',
      resource: '#087f8c',
      entity: '#a46719',
      edge: 'curve',
      node: 'soft'
    },
    {
      name: '分类色轨',
      note: '左侧色轨强化类别识别',
      app: '#536fc4',
      resource: '#087f8c',
      entity: '#a46719',
      edge: 'rail',
      node: 'rail'
    },
    {
      name: '分层标签',
      note: '紧凑图标块 + 标题/类型分层',
      app: '#536fc4',
      resource: '#087f8c',
      entity: '#a46719',
      edge: 'arrow',
      node: 'label'
    },
    {
      name: '纯线框',
      note: '无底色平面节点 + 直角连线',
      app: '#536fc4',
      resource: '#087f8c',
      entity: '#a46719',
      edge: 'orthogonal',
      node: 'flat'
    },
    {
      name: '状态圆点',
      note: '节点状态点与关系端点呼应',
      app: '#536fc4',
      resource: '#087f8c',
      entity: '#a46719',
      edge: 'dots',
      node: 'status'
    },
    {
      name: '虚实层级',
      note: '外部实体虚线，纳管节点实线',
      app: '#536fc4',
      resource: '#087f8c',
      entity: '#a46719',
      edge: 'dashed',
      node: 'dashed'
    },
    {
      name: '分组容器',
      note: '类型提示收进小型标签胶囊',
      app: '#536fc4',
      resource: '#087f8c',
      entity: '#a46719',
      edge: 'thin',
      node: 'grouped'
    },
    {
      name: '工程网格',
      note: '细网格画布 + 直线连接',
      app: '#536fc4',
      resource: '#087f8c',
      entity: '#a46719',
      edge: 'grid',
      node: 'engineering'
    },
    {
      name: '强调焦点',
      note: '更强轮廓 + 轻微选中光环',
      app: '#536fc4',
      resource: '#087f8c',
      entity: '#a46719',
      edge: 'focus',
      node: 'focus'
    }
  ];

  function select(index: number) {
    selected = index;
  }
  function toggleTheme() {
    dark = !dark;
    document.documentElement.dataset.theme = dark ? 'dark' : 'light';
  }
</script>

<svelte:head>
  <title>拓扑样式方案对比 · OpsKeeper</title>
</svelte:head>

<main class="options-page" class:dark>
  <header class="page-heading">
    <div>
      <p class="eyebrow">OPSKEEPER / TOPOLOGY STUDY</p>
      <h1>拓扑视觉方案</h1>
      <p class="subtitle">同一组组件与关系，10 种节点和连线语言</p>
    </div>
    <button
      class="theme-toggle"
      type="button"
      on:click={toggleTheme}
      aria-label={dark ? '切换浅色主题' : '切换深色主题'}
    >
      <span>{dark ? '深色' : '浅色'}</span>
      <span class="toggle-track"><i class:active={dark}></i></span>
    </button>
  </header>

  <section class="scheme-grid" aria-label="拓扑样式方案">
    {#each plans as plan, index}
      <button
        type="button"
        class:selected={selected === index}
        class={`scheme scheme-${plan.node}`}
        style={`--app:${plan.app};--resource:${plan.resource};--entity:${plan.entity}`}
        aria-pressed={selected === index}
        on:click={() => select(index)}
      >
        <span class="scheme-heading">
          <span class="scheme-index">{String(index + 1).padStart(2, '0')}</span>
          <span class="scheme-title"
            ><strong>{plan.name}</strong><small>{plan.note}</small></span
          >
          {#if selected === index}<span class="selected-mark"
              ><Check size={13} /></span
            >{/if}
        </span>
        <span class={`preview preview-${plan.edge}`}>
          <span class="mini-palette">
            <span class="palette-heading">组件</span>
            <span class="palette-row"
              ><AppWindow size={13} /><span>虚拟机应用</span><small>4</small
              ></span
            >
            <span class="palette-row"
              ><Boxes size={13} /><span>容器化应用</span><small>3</small></span
            >
            <span class="palette-row"
              ><Cloud size={13} /><span>云原生应用</span><small>2</small></span
            >
            <span class="palette-divider"></span>
            <span class="palette-row resource-row"
              ><Network size={13} /><span>Nacos</span><small>2</small></span
            >
            <span class="palette-row resource-row"
              ><Database size={13} /><span>MySQL</span><small>3</small></span
            >
            <span class="palette-row entity-row"
              ><StickyNote size={13} /><span>实体</span></span
            >
          </span>
          <span class="mini-canvas">
            <span class="canvas-tools"
              ><i></i><i></i><i></i><small>100%</small></span
            >
            <svg class="edges" viewBox="0 0 268 154" aria-hidden="true">
              <path class="edge-app" d="M95 53 C122 53 121 91 145 91" />
              <path class="edge-resource" d="M95 112 C119 112 123 91 145 91" />
              <path class="edge-entity" d="M145 91 C169 91 169 51 194 51" />
              <circle class="edge-point app-point" cx="95" cy="53" r="2.5" />
              <circle
                class="edge-point resource-point"
                cx="95"
                cy="112"
                r="2.5"
              />
              <circle
                class="edge-point entity-point"
                cx="194"
                cy="51"
                r="2.5"
              />
            </svg>
            <span class="mini-node node-app"
              ><AppWindow size={13} /><span
                ><b>支付 API</b><small>应用</small></span
              ></span
            >
            <span class="mini-node node-db"
              ><HardDrive size={13} /><span
                ><b>订单库</b><small>MySQL</small></span
              ></span
            >
            <span class="mini-node node-entity"
              ><StickyNote size={13} /><span
                ><b>支付网关</b><small>实体</small></span
              ></span
            >
            <span class="mini-node node-runtime"
              ><Server size={13} /><span
                ><b>Host-prod</b><small>运行时</small></span
              ></span
            >
          </span>
        </span>
        <span class="scheme-footer"
          ><span>组件列表</span><ChevronRight size={12} /><span>节点</span
          ><ChevronRight size={12} /><span>关系</span></span
        >
      </button>
    {/each}
  </section>
</main>

<style>
  :global(body) {
    min-width: 360px;
    background: var(--theme-bg-page);
  }
  .options-page {
    --app: var(--theme-application);
    min-height: 100vh;
    padding: 28px 32px 40px;
    color: var(--theme-fg);
    background: var(--theme-bg-page);
  }
  .page-heading {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 18px;
    max-width: 1440px;
    margin: 0 auto 22px;
    padding-bottom: 16px;
    border-bottom: 1px solid var(--theme-divider);
  }
  .eyebrow {
    margin: 0 0 7px;
    color: var(--theme-fg-subtle);
    font-size: 9px;
    font-weight: 700;
  }
  h1 {
    margin: 0;
    color: var(--theme-fg-strong);
    font-size: 23px;
    line-height: 1.25;
  }
  .subtitle {
    margin: 5px 0 0;
    color: var(--theme-fg-muted);
    font-size: 12px;
  }
  .theme-toggle {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0;
    color: var(--theme-fg-muted);
    background: none;
    border: 0;
    font-size: 11px;
  }
  .toggle-track {
    position: relative;
    display: block;
    width: 30px;
    height: 17px;
    padding: 2px;
    background: var(--theme-border-strong);
    border-radius: 10px;
  }
  .toggle-track i {
    display: block;
    width: 13px;
    height: 13px;
    background: var(--theme-bg-raised);
    border-radius: 50%;
    transition: transform 0.15s;
  }
  .toggle-track i.active {
    transform: translateX(13px);
  }
  .scheme-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 14px;
    max-width: 1440px;
    margin: auto;
  }
  .scheme {
    display: block;
    min-width: 0;
    padding: 0;
    overflow: hidden;
    color: var(--theme-fg);
    text-align: left;
    background: var(--theme-bg-raised);
    border: 1px solid var(--theme-border);
    border-radius: 6px;
    cursor: pointer;
    transition:
      border-color 0.15s,
      box-shadow 0.15s;
  }
  .scheme:hover {
    border-color: var(--theme-border-strong);
  }
  .scheme.selected {
    border-color: var(--theme-action);
    box-shadow: 0 0 0 2px
      color-mix(in srgb, var(--theme-action) 13%, transparent);
  }
  .scheme-heading {
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 54px;
    padding: 9px 12px;
  }
  .scheme-index {
    display: grid;
    flex: none;
    place-items: center;
    width: 25px;
    height: 25px;
    color: var(--theme-fg-muted);
    background: var(--theme-bg-subtle);
    border-radius: 4px;
    font-size: 10px;
    font-variant-numeric: tabular-nums;
  }
  .scheme-title {
    display: grid;
    min-width: 0;
    gap: 2px;
    flex: 1;
  }
  .scheme-title strong {
    color: var(--theme-fg-strong);
    font-size: 12px;
  }
  .scheme-title small {
    overflow: hidden;
    color: var(--theme-fg-muted);
    font-size: 10px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .selected-mark {
    display: grid;
    place-items: center;
    width: 22px;
    height: 22px;
    color: var(--theme-accent-contrast);
    background: var(--theme-action);
    border-radius: 50%;
  }
  .preview {
    display: grid;
    grid-template-columns: 118px minmax(0, 1fr);
    height: 184px;
    margin: 0 10px;
    overflow: hidden;
    background: var(--theme-bg-page);
    border: 1px solid var(--theme-border);
    border-radius: 4px;
  }
  .mini-palette {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 9px 7px;
    background: var(--theme-bg-subtle);
    border-right: 1px solid var(--theme-divider);
  }
  .palette-heading {
    padding: 0 4px 5px;
    color: var(--theme-fg-strong);
    font-size: 9px;
    font-weight: 700;
  }
  .palette-row {
    display: flex;
    align-items: center;
    gap: 5px;
    min-width: 0;
    min-height: 18px;
    padding: 2px 4px;
    color: var(--app);
    font-size: 8px;
  }
  .palette-row span {
    overflow: hidden;
    min-width: 0;
    flex: 1;
    color: var(--theme-fg-muted);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .palette-row small {
    color: var(--theme-fg-subtle);
    font-size: 7px;
  }
  .palette-divider {
    height: 1px;
    margin: 3px 2px;
    background: var(--theme-divider);
  }
  .palette-row.resource-row {
    color: var(--resource);
  }
  .palette-row.entity-row {
    color: var(--entity);
  }
  .mini-canvas {
    position: relative;
    overflow: hidden;
    background-color: var(--theme-bg-page);
  }
  .canvas-tools {
    position: absolute;
    z-index: 2;
    top: 6px;
    right: 7px;
    display: flex;
    align-items: center;
    gap: 3px;
    color: var(--theme-fg-subtle);
  }
  .canvas-tools i {
    width: 4px;
    height: 4px;
    background: var(--theme-border-strong);
    border-radius: 50%;
  }
  .canvas-tools small {
    margin-left: 4px;
    font-size: 7px;
  }
  .edges {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    overflow: visible;
  }
  .edges path {
    fill: none;
    stroke-width: 1.5;
  }
  .edge-app {
    stroke: var(--app);
  }
  .edge-resource {
    stroke: var(--resource);
  }
  .edge-entity {
    stroke: var(--entity);
  }
  .edge-point {
    fill: var(--theme-bg-raised);
    stroke-width: 1.6;
  }
  .app-point {
    stroke: var(--app);
  }
  .resource-point {
    stroke: var(--resource);
  }
  .entity-point {
    stroke: var(--entity);
  }
  .mini-node {
    position: absolute;
    z-index: 1;
    display: flex;
    align-items: center;
    gap: 5px;
    width: 91px;
    height: 37px;
    padding: 5px 6px;
    color: var(--theme-fg-muted);
    background: var(--theme-bg-raised);
    border: 1px solid var(--theme-border);
    border-radius: 4px;
    box-shadow: var(--theme-shadow-soft);
  }
  .mini-node > span {
    display: grid;
    min-width: 0;
    gap: 2px;
  }
  .mini-node b,
  .mini-node small {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .mini-node b {
    color: var(--theme-fg-strong);
    font-size: 8px;
    font-weight: 600;
  }
  .mini-node small {
    color: var(--theme-fg-subtle);
    font-size: 7px;
  }
  .node-app {
    top: 36px;
    left: 12px;
    color: var(--app);
  }
  .node-db {
    top: 95px;
    left: 12px;
    color: var(--resource);
  }
  .node-entity {
    top: 34px;
    right: 9px;
    color: var(--entity);
  }
  .node-runtime {
    top: 95px;
    right: 9px;
    color: var(--theme-success);
  }
  .scheme-footer {
    display: flex;
    align-items: center;
    gap: 5px;
    min-height: 27px;
    padding: 5px 12px;
    color: var(--theme-fg-subtle);
    font-size: 8px;
  }
  .scheme-outline .mini-node {
    border-radius: 2px;
    box-shadow: none;
  }
  .scheme-soft .mini-node {
    background: color-mix(in srgb, var(--app) 7%, var(--theme-bg-raised));
    border-color: color-mix(in srgb, var(--app) 26%, var(--theme-border));
    border-radius: 7px;
    box-shadow: 0 2px 7px color-mix(in srgb, var(--theme-fg) 8%, transparent);
  }
  .scheme-soft .node-db {
    background: color-mix(in srgb, var(--resource) 8%, var(--theme-bg-raised));
    border-color: color-mix(in srgb, var(--resource) 26%, var(--theme-border));
  }
  .scheme-soft .node-entity {
    background: color-mix(in srgb, var(--entity) 8%, var(--theme-bg-raised));
    border-color: color-mix(in srgb, var(--entity) 26%, var(--theme-border));
  }
  .scheme-rail .mini-node {
    border-left: 3px solid currentColor;
    border-radius: 3px;
  }
  .scheme-label .mini-node {
    width: 85px;
    height: 34px;
    border-radius: 6px;
  }
  .scheme-label .mini-node > :global(svg) {
    display: grid;
    place-items: center;
    width: 20px;
    height: 20px;
    padding: 3px;
    background: color-mix(in srgb, currentColor 12%, var(--theme-bg-raised));
    border-radius: 4px;
  }
  .scheme-flat .mini-node {
    background: transparent;
    border-color: transparent;
    box-shadow: none;
  }
  .scheme-flat .node-app,
  .scheme-flat .node-db {
    border-bottom: 1px solid currentColor;
  }
  .scheme-status .mini-node {
    border-radius: 7px;
  }
  .scheme-status .mini-node::after {
    position: absolute;
    top: 5px;
    right: 5px;
    width: 5px;
    height: 5px;
    background: var(--theme-success);
    border: 1px solid var(--theme-bg-raised);
    border-radius: 50%;
    content: '';
  }
  .scheme-dashed .node-entity {
    border-style: dashed;
  }
  .scheme-dashed .edge-entity {
    stroke-dasharray: 4 3;
  }
  .scheme-grouped .mini-node {
    width: 84px;
    border-radius: 3px;
    box-shadow: none;
  }
  .scheme-grouped .mini-node small {
    width: max-content;
    max-width: 100%;
    padding: 1px 3px;
    color: currentColor;
    background: color-mix(in srgb, currentColor 10%, var(--theme-bg-raised));
    border-radius: 3px;
  }
  .scheme-engineering .mini-canvas {
    background-image:
      linear-gradient(
        to right,
        color-mix(in srgb, var(--theme-border) 45%, transparent) 1px,
        transparent 1px
      ),
      linear-gradient(
        to bottom,
        color-mix(in srgb, var(--theme-border) 45%, transparent) 1px,
        transparent 1px
      );
    background-size: 14px 14px;
  }
  .scheme-engineering .mini-node {
    border-radius: 2px;
    box-shadow: none;
  }
  .scheme-focus .mini-node {
    border-width: 1.5px;
    border-radius: 6px;
    box-shadow: 0 0 0 2px color-mix(in srgb, currentColor 10%, transparent);
  }
  .preview-curve .edge-app,
  .preview-curve .edge-resource,
  .preview-curve .edge-entity {
    stroke-width: 2.2;
  }
  .preview-rail .edges path,
  .preview-dots .edges path {
    stroke-width: 2.2;
  }
  .preview-orthogonal .edge-app {
    d: path('M 95 53 H 119 V 91 H 145');
  }
  .preview-orthogonal .edge-resource {
    d: path('M 95 112 H 120 V 91 H 145');
  }
  .preview-orthogonal .edge-entity {
    d: path('M 145 91 H 169 V 51 H 194');
  }
  .preview-dashed .edge-app,
  .preview-dashed .edge-resource {
    stroke-dasharray: 4 3;
  }
  .preview-thin .edges path {
    stroke-width: 1;
  }
  .preview-focus .edges path {
    stroke-width: 2.6;
  }
  @media (min-width: 1500px) {
    .scheme-grid {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }
  }
  @media (max-width: 760px) {
    .options-page {
      padding: 20px 14px 28px;
    }
    .page-heading {
      align-items: flex-start;
    }
    .scheme-grid {
      grid-template-columns: 1fr;
    }
    .preview {
      grid-template-columns: 110px minmax(0, 1fr);
    }
    .mini-node {
      width: 80px;
    }
    .node-entity,
    .node-runtime {
      right: 4px;
    }
  }
</style>

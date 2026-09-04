<script lang="ts">
  export let resourceKind = '';
  export let resourceAddStep = 1;
  export let providerModels: unknown[] = [];
  export let resourceBasicConfigurationComplete: () => boolean;
  export let mcpConfigurationValid: () => boolean;
</script>

<aside class="resource-add-steps" aria-label="添加资源步骤">
  <button
    class:active={resourceAddStep === 1}
    class:done={resourceAddStep > 1}
    type="button"
    on:click={() => (resourceAddStep = 1)}
    ><b>1</b><span>基础配置</span></button
  >
  {#if resourceKind === 'AIProvider'}
    <button
      class:active={resourceAddStep === 2}
      class:done={resourceAddStep > 2}
      disabled={!resourceBasicConfigurationComplete()}
      type="button"
      on:click={() => {
        if (resourceBasicConfigurationComplete()) resourceAddStep = 2;
      }}><b>2</b><span>Provider 配置</span></button
    >
    <button
      class:active={resourceAddStep === 3}
      class:done={resourceAddStep > 3}
      type="button"
      on:click={() => (resourceAddStep = 3)}
      ><b>3</b><span>Model 配置</span></button
    >
    <button
      class:active={resourceAddStep === 4}
      disabled={providerModels.length === 0}
      type="button"
      on:click={() => {
        if (providerModels.length > 0) resourceAddStep = 4;
      }}><b>4</b><span>总结核验</span></button
    >
  {:else if resourceKind === 'MCPServer'}
    <button
      class:active={resourceAddStep === 2}
      class:done={resourceAddStep > 2}
      disabled={!resourceBasicConfigurationComplete()}
      type="button"
      on:click={() => { if (resourceBasicConfigurationComplete()) resourceAddStep = 2; }}><b>2</b><span>MCP 配置</span></button
    >
    <button
      class:active={resourceAddStep === 3}
      disabled={!mcpConfigurationValid()}
      type="button"
      on:click={() => { if (mcpConfigurationValid()) resourceAddStep = 3; }}><b>3</b><span>总结核验</span></button
    >
  {:else}
    <button
      class:active={resourceAddStep === 2}
      disabled={!resourceBasicConfigurationComplete()}
      type="button"
      on:click={() => {
        if (resourceBasicConfigurationComplete()) resourceAddStep = 2;
      }}
      ><b>2</b><span>配置资源</span></button
    >
  {/if}
</aside>

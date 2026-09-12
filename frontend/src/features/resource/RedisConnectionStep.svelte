<script lang="ts">
 import type { Resource } from '../../lib/api';
 export let accessMode:'direct'|'agent'='direct'; export let host=''; export let port=6379; export let database=0; export let username=''; export let password=''; export let timeoutSeconds=10; export let mcpServerResourceId=''; export let mcpServers:Resource[]=[]; export let configurationAttempted=false; export let onConfigurationChange:()=>void=()=>{};
</script>
{#if accessMode==='agent'}
<label class:invalid={configurationAttempted&&!mcpServerResourceId}><span><i>*</i>关联 MCPServer</span><select bind:value={mcpServerResourceId} required on:change={onConfigurationChange}><option value="">请选择活动的 MCPServer</option>{#each mcpServers as server}<option value={server.id}>{server.name}</option>{/each}</select></label><p class="docker-field-help">Redis Agent 通过关联 MCPServer 提供统一只读工具。</p>
{:else}
<div class="docker-form-grid"><label class:invalid={configurationAttempted&&!host.trim()}><span><i>*</i>Redis 主机</span><input bind:value={host} required on:input={onConfigurationChange} placeholder="例如 redis.example.com" /></label><label><span>端口</span><input type="number" min="1" max="65535" bind:value={port} on:input={onConfigurationChange}/></label><label><span>数据库编号</span><input type="number" min="0" bind:value={database} on:input={onConfigurationChange}/></label><label><span>用户名（可选）</span><input bind:value={username} on:input={onConfigurationChange}/></label><label><span>密码（可选）</span><input type="password" bind:value={password} on:input={onConfigurationChange}/></label><label><span>超时时间（秒）</span><input type="number" min="1" max="300" bind:value={timeoutSeconds} on:input={onConfigurationChange}/></label></div>
{/if}

<script lang="ts">
  import { Bot, Link2, Paperclip, Send, Sparkles, Square } from 'lucide-svelte';
  import type { AIProviderAvailability } from '../../lib/api';
  import DiagnosisModelPicker from './DiagnosisModelPicker.svelte';

  export let text = '';
  export let busy = false;
  export let generating = false;
  export let providers: AIProviderAvailability[] = [];
  export let selectedProviderId = '';
  export let modelName = '';
  export let modelMenuOpen = false;
  export let modelMenuProviderId = '';
  export let onTextChange: (text: string) => void;
  export let onKeydown: (event: KeyboardEvent) => void;
  export let onSubmit: () => void;
  export let onStop: () => void;
  export let onNotice: (message: string) => void;
  export let onModelToggle: () => void;
  export let onProvider: (providerID: string) => void;
  export let onModel: (modelName: string) => void;
</script>

<form class="diagnosis-composer-f" on:submit|preventDefault={onSubmit}>
  <div class="diagnosis-composer-shell">
    <textarea
      value={text}
      on:input={(event) => onTextChange((event.currentTarget as HTMLTextAreaElement).value)}
      on:keydown={onKeydown}
      placeholder="描述问题，或输入 / 调用 Skill…"
      aria-label="输入诊断问题"
      rows="3"
      maxlength="16000"
    ></textarea>
    <div class="diagnosis-composer-tools">
      <div>
        <button type="button" class="diagnosis-tool" title="添加附件" on:click={() => onNotice('附件入口已打开。')}><Paperclip size={15} />附件</button>
        <button type="button" class="diagnosis-tool" title="添加链接" on:click={() => onNotice('链接入口已打开。')}><Link2 size={15} />链接</button>
        <button type="button" class="diagnosis-tool" title="选择 Skills" on:click={() => onNotice('Skills：指标查询、日志查询、Kubernetes 只读查询。')}><Sparkles size={15} />Skills</button>
        <button type="button" class="diagnosis-tool" title="选择 Agent" on:click={() => onNotice('Agent：故障定位 Agent。')}><Bot size={15} />Agent</button>
      </div>
      <div>
        <DiagnosisModelPicker
          providers={providers}
          selectedProviderId={selectedProviderId}
          modelName={modelName}
          open={modelMenuOpen}
          menuProviderId={modelMenuProviderId}
          onToggle={onModelToggle}
          onProvider={onProvider}
          onModel={onModel}
        />
        <button class="primary diagnosis-send-button" type="button" disabled={busy || (!text.trim() && !generating)} on:click={() => (generating ? onStop() : onSubmit())}>{#if generating}<Square size={14} fill="currentColor" />停止{:else}<Send size={14} />发送{/if}</button>
      </div>
    </div>
  </div>
  <small class="diagnosis-composer-note"><span>Enter 发送 · Shift + Enter 换行</span><span>当前模型支持：文本、工具调用、流式输出</span></small>
</form>

<script lang="ts">
  import FormField from '../../components/FormField.svelte';
  import Switch from '../../components/Switch.svelte';
  import TextArea from '../../components/TextArea.svelte';
  import FilePicker from '../../components/FilePicker.svelte';

  export let ca = '';
  export let cert = '';
  export let key = '';
  export let skipVerify = false;
  export let configurationAttempted = false;
  export let disabled = false;
  export let showSkipVerify = true;
  export let legend = 'TLS 客户端配置（可选）';
  export let help = '';
  export let onChange: () => void = () => {};
  export let validate: (value: string) => boolean = () => true;
  export let fileLabelPrefix = '';

  type CredentialField = 'ca' | 'cert' | 'key';
  let selectedFileNames: Record<CredentialField, string> = {
    ca: '',
    cert: '',
    key: ''
  };
  let fileErrors: Record<CredentialField, string> = {
    ca: '',
    cert: '',
    key: ''
  };
  async function importFile(event: Event, field: CredentialField) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    fileErrors = { ...fileErrors, [field]: '' };
    try {
      const bytes = new Uint8Array(await file.arrayBuffer());
      const value = new TextDecoder('utf-8', { fatal: true })
        .decode(bytes)
        .trim();
      if (!value) throw new Error('文件为空');
      if (field === 'ca') ca = value;
      if (field === 'cert') cert = value;
      if (field === 'key') key = value;
      selectedFileNames = {
        ...selectedFileNames,
        [field]: file.webkitRelativePath || file.name
      };
      onChange();
    } catch (error) {
      fileErrors = {
        ...fileErrors,
        [field]: error instanceof Error ? error.message : '文件读取失败'
      };
      selectedFileNames = { ...selectedFileNames, [field]: '' };
    } finally {
      input.value = '';
    }
  }

  function invalid(value: string) {
    return configurationAttempted && Boolean(value.trim()) && !validate(value);
  }

  function fileAccept(field: CredentialField) {
    return field === 'key'
      ? '.pem,.key,.crt,text/plain,application/x-pem-file'
      : '.pem,.crt,.cer,text/plain,application/x-pem-file';
  }

  function fileLabel(field: CredentialField) {
    return `${fileLabelPrefix}${field === 'ca' ? 'CA 证书' : field === 'cert' ? '客户端证书' : '客户端私钥'}`;
  }
</script>

<fieldset class="docker-tls-fieldset" {disabled}>
  <legend>{legend}</legend>
  {#if help}<p class="docker-field-help">{help}</p>{/if}
  <div class="docker-form-grid docker-tls-grid">
    <div class="docker-tls-certificate-row">
      {#each [['ca', ca], ['cert', cert], ['key', key]] as entry}
        {@const field = entry[0] as CredentialField}
        {@const value = entry[1] as string}
        <FormField label={fileLabel(field)} invalid={invalid(value)}>
          <span class="docker-credential-label">
            <FilePicker
              className="docker-file-picker"
              inputClass="docker-file-input"
              buttonClass="docker-file-import"
              fileName={selectedFileNames[field]}
              accept={fileAccept(field)}
              label={`选择${fileLabel(field)}文件`}
              ariaLabel={`导入${fileLabel(field)}文件`}
              inputAriaLabel={`选择${fileLabel(field)}文件`}
              buttonLabel="导入"
              onFileChange={(event) => void importFile(event, field)}
            />
            {#if field === 'ca' && showSkipVerify}<span class="docker-tls-label"
                ><Switch
                  bind:checked={skipVerify}
                  ariaLabel="跳过 TLS 证书校验"
                  on:change={onChange}
                /><small>跳过校验</small></span
              >{/if}
          </span>
          <TextArea
            {value}
            rows={6}
            placeholder={`粘贴${fileLabel(field)} PEM 文本或 Base64 编码`}
            spellcheck={false}
            on:input={(event) => {
              const next = (event.currentTarget as HTMLTextAreaElement).value;
              if (field === 'ca') ca = next;
              if (field === 'cert') cert = next;
              if (field === 'key') key = next;
              onChange();
            }}
          />
          {#if fileErrors[field]}<small class="field-error"
              >{fileErrors[field]}</small
            >{/if}
        </FormField>
      {/each}
    </div>
  </div>
</fieldset>

<script lang="ts">
  import type { ConnectionCheck, Resource } from '../../lib/api';
  import { resourceEndpointFor } from './resourceCatalog';
  export let resource: Resource;
  export let selectedResourceId = '';
  export let connectionCheck: ConnectionCheck | null = null;
  export let formatDate: (value: string) => string;
  export let resourceCanManage: (resource: Resource, permission: string) => boolean;
</script>

<div class="resource-row-details">
  <div><span>资源地址</span><strong>{resourceEndpointFor(resource)}</strong></div>
  <div><span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong></div>
  <div><span>连接测试</span><strong>{selectedResourceId === resource.id && connectionCheck ? connectionCheck.status === 'succeeded' ? '连接正常' : '连接失败' : '展开后可测试'}</strong></div>
  <div><span>管理范围</span><strong>{resourceCanManage(resource, 'resource:update') ? '当前 Scope 可管理' : '继承资源，仅限查看'}</strong></div>
</div>

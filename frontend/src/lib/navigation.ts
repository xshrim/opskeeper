export type NavigationView =
  | 'overview'
  | 'project'
  | 'discovery'
  | 'resource'
  | 'skill'
  | 'agent'
  | 'operation'
  | 'diagnosis'
  | 'inspection'
  | 'access'
  | 'profile';

const titles: Record<NavigationView, string> = {
  overview: '平台总览',
  project: '项目管理',
  discovery: '集群项目与应用导入',
  resource: '资源目录',
  skill: 'Skill',
  agent: 'Agent 专家',
  operation: '受控操作与 MCP',
  diagnosis: 'AI 诊断工作台',
  inspection: '自动巡检与健康',
  access: '权限',
  profile: '个人中心'
};

const breadcrumbs: Record<NavigationView, string> = {
  overview: '总览',
  project: '项目',
  discovery: '集群导入',
  resource: '资源',
  skill: 'Skill',
  agent: 'Agent 专家',
  operation: '受控操作',
  diagnosis: 'AI 诊断',
  inspection: '自动巡检',
  access: '权限',
  profile: '个人中心'
};

export function viewTitle(view: NavigationView, accessTab?: 'teams' | 'users' | 'roles') {
  if (view === 'access') {
    return accessTab === 'teams' ? '团队管理' : accessTab === 'users' ? '用户管理' : '角色管理';
  }
  return titles[view];
}

export function viewBreadcrumb(view: NavigationView, accessTab?: 'teams' | 'users' | 'roles') {
  return view === 'access' ? `权限 / ${viewTitle(view, accessTab)}` : breadcrumbs[view];
}

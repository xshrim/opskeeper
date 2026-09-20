export type NavigationView =
  | 'overview'
  | 'project'
  | 'resource'
  | 'llm'
  | 'skill'
  | 'persona'
  | 'diagnosis'
  | 'inspection'
  | 'access'
  | 'profile';

const titles: Record<NavigationView, string> = {
  overview: '平台总览',
  project: '项目管理',
  resource: '资源目录',
  llm: '模型',
  skill: '技能',
  persona: '专家',
  diagnosis: '诊断工作台',
  inspection: '巡检与健康',
  access: '权限',
  profile: '个人中心'
};

const breadcrumbs: Record<NavigationView, string> = {
  overview: '总览',
  project: '项目',
  resource: '资源',
  llm: '模型',
  skill: '技能',
  persona: '专家',
  diagnosis: '诊断',
  inspection: '巡检',
  access: '权限',
  profile: '个人中心'
};

export function viewTitle(view: NavigationView, accessTab?: 'teams' | 'users' | 'roles') {
  if (view === 'access') return '权限';
  return titles[view];
}

export function viewBreadcrumb(view: NavigationView, accessTab?: 'teams' | 'users' | 'roles') {
  return view === 'access' ? `权限 / ${viewTitle(view, accessTab)}` : breadcrumbs[view];
}

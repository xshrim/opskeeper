export type NavigationView =
  | 'overview'
  | 'project'
  | 'resource'
  | 'llm'
  | 'skill'
  | 'persona'
  | 'diagnosis'
  | 'inspection'
  | 'notification'
  | 'access';

const titles: Record<NavigationView, string> = {
  overview: '平台总览',
  project: '项目管理',
  resource: '资源管理',
  llm: '模型引擎',
  skill: '技能编排',
  persona: '专家协同',
  diagnosis: '智能诊断',
  inspection: '智能巡检',
  notification: '通知中心',
  access: '权限审计'
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
  notification: '通知',
  access: '权限'
};

export function viewTitle(
  view: NavigationView,
  accessTab?: 'teams' | 'users' | 'roles'
) {
  if (view === 'access') return '权限审计';
  return titles[view];
}

export function viewBreadcrumb(
  view: NavigationView,
  accessTab?: 'teams' | 'users' | 'roles'
) {
  return view === 'access'
    ? `权限 / ${viewTitle(view, accessTab)}`
    : breadcrumbs[view];
}

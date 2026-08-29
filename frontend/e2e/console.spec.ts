import { expect, test, type Page } from '@playwright/test';

const ids = {
  platform: 'scope-platform',
  team: 'scope-team',
  project: 'scope-project',
  app: 'resource-app',
  provider: 'resource-provider',
  skill: 'resource-skill',
  session: 'diagnosis-session'
};

const user = {
  id: 'user-1',
  username: 'acceptance-admin',
  email: 'admin@example.test',
  display_name: '验收管理员',
  status: 'active',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z'
};

function pageData(page: Page, requireLogin = false, platformAdmin = false) {
  let authenticated = !requireLogin;
  let preferences = {
    theme: 'auto',
    sidebar_mode: 'fixed',
    sidebar_collapsed: false
  };
  return page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const json = (body: unknown, status = 200) =>
      route.fulfill({
        status,
        contentType: 'application/json',
        body: JSON.stringify(body)
      });
    if (path.endsWith('/auth/me/context'))
      return json({ platform_admin: platformAdmin, platform_role: platformAdmin });
    if (path.endsWith('/auth/me'))
      return authenticated
        ? json(user)
        : json({ code: 'invalid_session' }, 401);
    if (path.endsWith('/auth/login')) {
      authenticated = true;
      return json(user);
    }
    if (path.endsWith('/auth/logout')) return json({}, 204);
    if (path.endsWith('/auth/me/preferences')) {
      if (request.method() === 'PUT') {
        preferences = JSON.parse(request.postData() || '{}');
      }
      return json(preferences);
    }
    if (path.endsWith('/platform'))
      return json({
        id: 'platform-1',
        scope: { id: ids.platform, type: 'platform', status: 'active' },
        name: '验收平台',
        code: 'acceptance',
        status: 'active',
        created_at: user.created_at,
        updated_at: user.updated_at
      });
    if (path.endsWith('/teams/') && request.method() === 'POST') {
      const body = JSON.parse(request.postData() || '{}');
      return json(
        {
          id: 'team-created',
          platform_id: 'platform-1',
          scope: {
            id: 'scope-team-created',
            type: 'team',
            parent_id: ids.platform,
            status: 'active'
          },
          name: body.name,
          code: body.code,
          icon: body.icon,
          labels: body.labels ?? {},
          status: 'active',
          created_at: user.created_at,
          updated_at: user.updated_at
        },
        201
      );
    }
    if (path.endsWith('/teams/'))
      return json({
        items: [
          {
            id: 'team-1',
            platform_id: 'platform-1',
            scope: {
              id: ids.team,
              type: 'team',
              parent_id: ids.platform,
              status: 'active'
            },
            name: '平台工程',
            code: 'platform',
            icon: 'team',
            labels: {},
            status: 'active',
            created_at: user.created_at,
            updated_at: user.updated_at
          }
        ],
        page: 1,
        page_size: 100,
        total: 1
      });
    if (path.includes('/projects'))
      return json({
        items: [
          {
            id: 'project-1',
            platform_id: 'platform-1',
            team_id: 'team-1',
            scope: {
              id: ids.project,
              type: 'project',
              parent_id: ids.team,
              status: 'active'
            },
            name: '支付服务',
            code: 'payments',
            icon: 'project',
            labels: {},
            source: 'manual',
            source_config: {},
            status: 'active',
            created_at: user.created_at,
            updated_at: user.updated_at
          }
        ],
        page: 1,
        page_size: 100,
        total: 1
      });
    if (path.endsWith('/resources/schemas')) return json([]);
    if (path.endsWith('/resources'))
      return json({
        items: [
          {
            id: ids.app,
            scope_id: ids.project,
            kind: 'Application',
            schema_version: 1,
            name: 'payments-api',
            labels: { env: 'test' },
            config: {},
            status: 'active',
            created_at: user.created_at,
            updated_at: user.updated_at
          },
          {
            id: ids.provider,
            scope_id: ids.platform,
            kind: 'AIProvider',
            schema_version: 1,
            name: 'Test Provider',
            labels: {},
            config: {
              provider_type: 'openai_compatible',
              base_url: 'https://llm.test',
              models: [{ name: 'test-model', context_window: 8192 }]
            },
            status: 'active',
            created_at: user.created_at,
            updated_at: user.updated_at
          },
          {
            id: ids.skill,
            scope_id: ids.platform,
            kind: 'Skill',
            schema_version: 1,
            name: 'Default Diagnostic',
            labels: {},
            config: {},
            status: 'active',
            created_at: user.created_at,
            updated_at: user.updated_at
          }
        ],
        page: 1,
        page_size: 100,
        total: 3
      });
    if (path.includes('/skills/') && path.endsWith('/versions'))
      return json([
        {
          id: 'skill-version-1',
          skill_resource_id: ids.skill,
          version: 1,
          manifest: {
            name: 'Default Diagnostic',
            description: 'test',
            instruction: 'return JSON',
            target_kinds: ['Application']
          },
          input_schema: { type: 'object' },
          output_schema: { type: 'object' },
          tools: [],
          risk_level: 'read_only',
          status: 'published',
          created_at: user.created_at
        }
      ]);
    if (path.includes('/diagnosis-sessions/') && path.endsWith('/events'))
      return route.fulfill({
        status: 200,
        contentType: 'text/event-stream',
        body: ''
      });
    if (path.endsWith('/diagnosis-sessions'))
      return json([
        {
          id: ids.session,
          scope_id: ids.project,
          title: '支付服务诊断',
          status: 'succeeded',
          question: '检查健康状态',
          created_at: user.created_at,
          updated_at: user.updated_at
        }
      ]);
    if (path.includes('/diagnosis-sessions/'))
      return json({
        session: {
          id: ids.session,
          scope_id: ids.project,
          title: '支付服务诊断',
          status: 'succeeded',
          question: '检查健康状态',
          created_at: user.created_at,
          updated_at: user.updated_at
        },
        targets: [],
        messages: [],
        plan: [],
        events: [],
        evidence: [],
        hypotheses: [],
        report: null
      });
    if (path.endsWith('/users/') && request.method() === 'POST') {
      const body = JSON.parse(request.postData() || '{}');
      return json(
        {
          user: {
            ...user,
            id: 'user-created',
            username: body.username,
            display_name: body.display_name || body.username,
            email: body.email,
            phone: body.phone,
            can_manage: true
          },
          bindings: [],
          one_time_password: 'GeneratedPassword123!'
        },
        201
      );
    }
    if (path.endsWith('/users/'))
      return json([
        { ...user, can_manage: false },
        {
          ...user,
          id: 'user-2',
          username: 'payment-owner',
          email: 'owner@example.test',
          display_name: '支付负责人',
          can_manage: true
        }
      ]);
    if (path.endsWith('/groups/'))
      return json([
        {
          id: 'group-1',
          scope_id: ids.team,
          name: '平台工程成员',
          description: '平台工程团队',
          status: 'active',
          created_at: user.created_at,
          updated_at: user.updated_at
        }
      ]);
    if (path.endsWith('/groups/group-1/members'))
      return json([
        { group_id: 'group-1', user_id: 'user-2', created_at: user.created_at }
      ]);
    if (path.endsWith('/roles/'))
      return json([
        {
          id: 'role-team-owner',
          name: 'TeamOwner',
          scope_type: 'team',
          builtin: true,
          permissions: ['organization:read', 'member:grant', 'project:manage']
        },
        {
          id: 'role-platform-viewer',
          name: 'PlatformViewer',
          scope_type: 'platform',
          builtin: true,
          permissions: ['organization:read', 'resource:read']
        },
        {
          id: 'role-team-viewer',
          name: 'TeamViewer',
          scope_type: 'team',
          builtin: true,
          permissions: ['organization:read', 'resource:read']
        },
        {
          id: 'role-project-viewer',
          name: 'ProjectViewer',
          scope_type: 'project',
          builtin: true,
          permissions: ['organization:read', 'resource:read']
        }
      ]);
    if (path.endsWith('/role-bindings/'))
      return json([
        {
          id: 'binding-1',
          subject_type: 'user',
          subject_id: 'user-2',
          role_id: 'role-team-owner',
          role_name: 'TeamOwner',
          scope_id: ids.team,
          scope_type: 'team',
          created_at: user.created_at
        }
      ]);
    if (
      path.endsWith('/resource-roles/') ||
      path.endsWith('/resource-role-bindings/')
    )
      return json([]);
    if (path.endsWith('/inspection-policies') && request.method() === 'POST')
      return json(
        {
          id: 'policy-1',
          scope_id: ids.platform,
          name: 'Hourly health',
          cron: '0 * * * *',
          timezone: 'UTC',
          status: 'active',
          target_resource_ids: [],
          target_labels: { env: 'prod' },
          agent_profile_resource_id: '',
          timeout: 120000000000,
          retries: 1,
          max_concurrent: 2,
          max_tool_calls: 12,
          max_tokens: 20000
        },
        201
      );
    if (path.endsWith('/notification-channels') && request.method() === 'POST')
      return json(
        {
          id: 'channel-1',
          scope_id: ids.platform,
          name: 'Incident webhook',
          kind: 'webhook',
          webhook_url: 'https://hooks.example.test/incident',
          status: 'active',
          rate_limit_per_minute: 30
        },
        201
      );
    if (
      path.includes('/inspection-policies') ||
      path.includes('/inspection-runs') ||
      path.includes('/inspection-findings') ||
      path.includes('/notification-channels')
    )
      return json([]);
    if (path.includes('/operation-requests')) return json([]);
    return json({}, 200);
  });
}

test.describe('T07 console', () => {
  test('logs in and restores the workspace session', async ({ page }) => {
    let healthRequests = 0;
    await page.route('**/health/ready', (route) => {
      healthRequests += 1;
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          status: 'ready',
          service: 'opskeeper-api',
          version: 'test',
          commit: 'test',
          build_time: '2026-01-01T00:00:00Z',
          timestamp: '2026-01-01T00:00:00Z',
          checks: {}
        })
      });
    });
    await pageData(page, true);
    await page.goto('/');
    await expect(page.getByLabel('OpsKeeper 智能值守平台')).toBeVisible();
    await expect(page.getByRole('heading', { name: '欢迎回来' })).toBeVisible();
    expect(healthRequests).toBe(0);
    expect(
      await page
        .locator('.login-shell')
        .evaluate((node) => getComputedStyle(node).backgroundImage)
    ).toContain('login-background');
    await page.setViewportSize({ width: 390, height: 844 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth
      )
    ).toBeTruthy();
    await page.getByLabel('账号', { exact: true }).fill('admin@example.test');
    const passwordInput = page.getByLabel('密码', { exact: true });
    await passwordInput.fill('test-password');
    const showPassword = page.getByRole('button', { name: '显示密码' });
    await expect(showPassword.locator('svg')).toBeVisible();
    await showPassword.click();
    await expect(passwordInput).toHaveAttribute('type', 'text');
    const hidePassword = page.getByRole('button', { name: '隐藏密码' });
    await expect(hidePassword.locator('svg')).toBeVisible();
    await hidePassword.click();
    await page.getByRole('button', { name: '登录' }).click();
    await expect(page.getByRole('heading', { name: '平台总览' })).toBeVisible();
    await expect.poll(() => healthRequests).toBe(1);
  });

  test('switches scope and renders resources on desktop and mobile', async ({
    page
  }) => {
    await pageData(page);
    await page.goto('/');
    await page.getByRole('button', { name: '项目' }).click();
    await page.getByRole('button', { name: /平台工程/ }).click();
    await page.getByLabel('切换项目').selectOption('project-1');
    await page.getByRole('button', { name: '资源' }).click();
    await expect(
      page.getByRole('heading', { name: '资源目录' }).first()
    ).toBeVisible();
    await page.setViewportSize({ width: 390, height: 844 });
    await expect(
      page.getByRole('button', { name: '资源', exact: true })
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth
      )
    ).toBeTruthy();
  });

  test('shows team members, projects, user roles and permissions', async ({
    page
  }) => {
    await pageData(page, false, true);
    await page.goto('/');
    await page.getByRole('button', { name: '展开成员菜单' }).click();
    await page.getByRole('button', { name: '团队管理' }).click();
    await expect(
      page.getByRole('heading', { name: '团队管理', level: 1 })
    ).toBeVisible();
    await page.locator('.access-team-trigger').filter({ hasText: '平台工程' }).click();
    await expect(
      page.getByText('支付负责人', { exact: true }).first()
    ).toBeVisible();
    await expect(page.getByText('支付服务', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: '用户管理' }).click();
    await expect(
      page.getByText('TeamOwner', { exact: true }).first()
    ).toBeVisible();
    await expect(
      page.getByText('member:grant', { exact: true }).first()
    ).toBeVisible();
    await page.getByRole('button', { name: '添加用户' }).click();
    await expect(page.getByRole('dialog', { name: '新增用户' })).toBeVisible();
    await page.getByLabel('授权级别').selectOption('team');
    await page.getByLabel('授权对象').selectOption(ids.team);
    await page.getByLabel('角色').selectOption('role-team-owner');
    await expect(page.getByLabel('角色')).toHaveValue('role-team-owner');
    await page.getByRole('button', { name: '关闭' }).click();

    await page.setViewportSize({ width: 390, height: 844 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth
      )
    ).toBeTruthy();
  });

  test('hides team and project selectors for a platform administrator', async ({
    page
  }) => {
    await pageData(page, false, true);
    await page.goto('/');

    await expect(page.getByLabel('切换项目')).toHaveCount(0);
    await expect(page.getByLabel('打开用户菜单')).toBeVisible();
  });

  test('opens the personal center and saves user display preferences', async ({
    page
  }) => {
    await pageData(page);
    await page.goto('/');

    await page.getByLabel('打开用户菜单').click();
    await expect(page.getByRole('menu')).toBeVisible();
    await page.getByRole('heading', { name: '平台总览' }).click();
    await expect(page.getByRole('menu')).toHaveCount(0);
    await page.getByLabel('打开用户菜单').click();
    await page.getByRole('menuitem', { name: '个人中心' }).click();
    await expect(page.getByRole('heading', { name: '个人中心' })).toBeVisible();
    await page.getByLabel('显示名').fill('值守管理员');
    await page.getByRole('radio', { name: '深色' }).click();
    await page.getByRole('radio', { name: '窄栏悬浮展开' }).click();
    await page.getByRole('button', { name: '保存配置' }).click();
    await expect(page.getByText('个人中心配置已保存')).toBeVisible();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
    await expect
      .poll(() =>
        page.locator('html').evaluate((element) =>
          getComputedStyle(element).getPropertyValue('--color-primary').trim()
        )
      )
      .toBe('#18a27d');

    await page.getByLabel('展开导航栏').hover();
    await expect(page.getByRole('button', { name: '总览' })).toBeVisible();
  });

  test('uses the selected dark theme for navigation and management surfaces', async ({
    page
  }) => {
    await pageData(page);
    await page.goto('/');
    await page.getByLabel('打开用户菜单').click();
    await page.getByRole('menuitem', { name: '个人中心' }).click();
    await page.getByRole('radio', { name: '深色' }).click();
    await page.getByRole('button', { name: '保存配置' }).click();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');

    await expect(page.getByRole('radio', { name: '深色' })).toHaveCSS(
      'background-color',
      'rgb(23, 62, 53)'
    );
    await expect(page.locator('.profile-panel').first()).toHaveCSS(
      'background-color',
      'rgb(42, 38, 35)'
    );
  });

  test('creates a team with a searchable icon picker', async ({ page }) => {
    await pageData(page, false, true);
    await page.goto('/');
    await page.getByRole('button', { name: '展开成员菜单' }).click();
    await page.getByRole('button', { name: '团队管理' }).click();
    await page.getByRole('button', { name: '添加团队' }).click();

    const dialog = page.getByRole('dialog', { name: '新增团队' });
    await expect(dialog.getByLabel('名称')).toBeVisible();
    await expect(dialog.getByLabel('团队名称')).toHaveCount(0);
    await dialog.getByLabel('选择团队图标').click();
    const picker = page.getByRole('dialog').filter({ hasText: '选择图标' });
    await picker.getByLabel('搜索图标').fill('PostgreSQL');
    await picker.getByRole('button', { name: '选择图标 PostgreSQL' }).click();
    await dialog.getByLabel('选择团队图标').click();
    await picker.getByLabel('搜索图标').fill('Activity');
    await expect(picker.getByRole('button', { name: '选择图标 Activity' })).toBeVisible();
    await picker.getByRole('button', { name: '选择图标 Activity' }).click();
    await dialog.getByLabel('名称').fill('数据库平台');
    await dialog.getByLabel('团队编码').fill('database');
    await dialog.getByRole('button', { name: '创建团队' }).click();
    await expect(page.getByText('团队“数据库平台”已创建')).toBeVisible();
  });

  test('creates a user with an auto-filled display name and inline password', async ({
    page
  }) => {
    await pageData(page, false, true);
    await page.goto('/');
    await page.getByRole('button', { name: '展开成员菜单' }).click();
    await page.getByRole('button', { name: '用户管理' }).click();
    await page.getByRole('button', { name: '添加用户' }).click();

    const dialog = page.getByRole('dialog', { name: '新增用户' });
    await dialog.getByLabel('用户名').fill('database-operator');
    await expect(dialog.getByLabel('显示名')).toHaveValue('database-operator');
    await expect(dialog.locator('.new-user-grant-header')).toContainText('级别');
    await expect(dialog.locator('.new-user-grant-header')).toContainText('对象');
    await expect(dialog.locator('.new-user-grant-header')).toContainText('角色');
    await dialog.getByLabel('授权级别').selectOption('team');
    await dialog.getByLabel('授权对象').selectOption(ids.team);
    await dialog.getByLabel('角色').selectOption('role-team-owner');
    await dialog.getByRole('button', { name: '创建用户并授权' }).click();
    await expect(dialog.locator('.created-credentials-inline')).toContainText(
      'GeneratedPassword123!'
    );
    await expect(dialog.locator('.created-credentials-inline')).not.toContainText(
      'database-operator'
    );
    await expect(dialog.getByLabel('复制一次性密码')).toBeVisible();
  });

  test('offers same-scope resource grants for viewer roles', async ({ page }) => {
    await pageData(page, false, true);
    await page.goto('/');
    await page.getByRole('button', { name: '展开成员菜单' }).click();
    await page.getByRole('button', { name: '用户管理' }).click();
    await page.getByRole('button', { name: '添加用户' }).click();

    const dialog = page.getByRole('dialog', { name: '新增用户' });
    await dialog.getByLabel('用户名').fill('project-viewer');
    await dialog.getByLabel('授权级别').selectOption('project');
    await dialog.getByLabel('授权对象').selectOption(ids.project);
    await dialog.getByLabel('角色').selectOption('role-project-viewer');
    await expect(dialog.getByText('项目观察员默认可读取该范围资源')).toBeVisible();
    await expect(dialog.getByText('范围资源权限')).toBeVisible();
  });

  test('does not allow an administrator to edit a username', async ({ page }) => {
    await pageData(page, false, true);
    await page.goto('/');
    await page.getByRole('button', { name: '展开成员菜单' }).click();
    await page.getByRole('button', { name: '用户管理' }).click();
    await page.getByLabel('编辑用户 支付负责人').click();

    const dialog = page.getByRole('dialog', { name: '编辑用户与授权' });
    await expect(dialog.getByLabel('用户名不可修改')).toBeDisabled();
    await expect(dialog.getByLabel('显示名')).toBeEditable();
  });

  test('keeps the orange primary color in light mode', async ({ page }) => {
    await pageData(page);
    await page.goto('/');

    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
    await expect
      .poll(() =>
        page.locator('html').evaluate((element) =>
          getComputedStyle(element).getPropertyValue('--color-primary').trim()
        )
      )
      .toBe('#e7601b');
  });
});

test.describe('T10 and T11 workbenches', () => {
  test('shows provider and Skill versions', async ({ page }) => {
    await pageData(page);
    await page.goto('/');
    await expect(page.getByRole('button', { name: 'AI 引擎' })).toHaveCount(0);
    await page.getByRole('button', { name: 'Skill', exact: true }).click();
    await expect(
      page.getByRole('heading', { name: 'Skill', exact: true })
    ).toBeVisible();
    await expect(
      page.locator('option').filter({ hasText: 'Default Diagnostic' })
    ).toHaveCount(1);
  });

  test('opens the independent Agent profile management page', async ({ page }) => {
    await pageData(page);
    await page.goto('/');
    await page.getByRole('button', { name: 'Agent 专家' }).click();
    await expect(
      page.getByRole('heading', { name: 'Agent 专家配置', exact: true })
    ).toBeVisible();
    await expect(
      page.getByRole('heading', { name: '创建 AgentProfile', exact: true })
    ).toBeVisible();
  });

  test('restores a diagnosis session and its SSE-backed workbench', async ({
    page
  }) => {
    await pageData(page);
    await page.goto('/');
    await page.getByRole('button', { name: 'AI 诊断' }).click();
    await expect(
      page.getByRole('heading', { name: 'AI 诊断工作台' })
    ).toBeVisible();
    await expect(
      page.getByRole('heading', { name: '支付服务诊断' })
    ).toBeVisible();
    await expect(page.getByText('已完成').first()).toBeVisible();
  });
});

test.describe('T13 inspection console', () => {
  test('creates a label policy and Webhook channel', async ({ page }) => {
    await pageData(page);
    await page.goto('/');
    await page.getByRole('button', { name: '自动巡检' }).click();
    const policyForm = page.locator('form').filter({ hasText: '标签选择器' });
    await policyForm.getByLabel('名称').fill('Hourly health');
    await policyForm.getByLabel('时区').fill('UTC');
    await policyForm
      .getByLabel('标签选择器（JSON 对象）')
      .fill('{"env":"prod"}');
    await policyForm.getByLabel('Default Diagnostic').check();
    await policyForm.getByRole('button', { name: '创建策略' }).click();
    await expect(page.getByText('Hourly health')).toBeVisible();

    const channelForm = page
      .locator('form')
      .filter({ hasText: 'HTTPS Webhook' });
    await channelForm.getByLabel('名称').fill('Incident webhook');
    await channelForm
      .getByLabel('HTTPS Webhook')
      .fill('https://hooks.example.test/incident');
    await channelForm.getByRole('button', { name: '添加渠道' }).click();
    await expect(page.getByText('Incident webhook')).toBeVisible();
  });
});

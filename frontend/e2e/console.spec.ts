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
      return json({ platform_admin: platformAdmin });
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
            kind: 'LLMProvider',
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
    if (path.includes('/skill-executions')) return json([]);
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
          skill_resource_ids: [ids.skill],
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
    await expect(page.getByText('平台工程', { exact: true })).toBeVisible();
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
    await page.getByRole('button', { name: '展开团队与用户菜单' }).click();
    await page.getByRole('button', { name: '团队管理' }).click();
    await expect(
      page.getByRole('heading', { name: '团队管理', level: 1 })
    ).toBeVisible();
    await page.getByRole('button', { name: /平台工程/ }).click();
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
    await page.getByLabel('授权 Scope').selectOption(ids.team);
    await page.getByLabel('TeamOwner').check();
    await expect(
      page.getByRole('dialog').getByText('member:grant', { exact: true })
    ).toBeVisible();
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
    await page.getByRole('button', { name: '模型与 Skill' }).click();
    await expect(
      page.getByRole('heading', { name: '模型与 Skill' })
    ).toBeVisible();
    await expect(
      page.locator('option').filter({ hasText: 'Test Provider' })
    ).toHaveCount(1);
    await expect(
      page.locator('option').filter({ hasText: 'Default Diagnostic' })
    ).toHaveCount(1);
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
    await expect(page.getByText('succeeded').first()).toBeVisible();
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

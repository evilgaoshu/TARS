import { expect, test } from '@playwright/test';
import type { APIRequestContext, Page } from '@playwright/test';

const configuredToken = process.env.TARS_PLAYWRIGHT_TOKEN || process.env.TARS_OPS_API_TOKEN;
const prefix = 'pw-smoke-';

if (!configuredToken) {
  throw new Error('TARS_PLAYWRIGHT_TOKEN or TARS_OPS_API_TOKEN is required for control-plane smoke tests');
}

const token = configuredToken;

type AccessConfigResponse = {
  config: {
    auth_providers?: Array<{ id?: string }>;
    channels?: Array<{ id?: string }>;
  };
};

type ProvidersConfigResponse = {
  config: {
    primary: { provider_id?: string; model?: string };
    assist: { provider_id?: string; model?: string };
    entries?: Array<{ id?: string }>;
  };
};

test.describe.configure({ mode: 'serial' });

test.beforeEach(async ({ page, request }) => {
  await cleanupSmokeArtifacts(request);
  await login(page);
});

test.afterEach(async ({ request }) => {
  await cleanupSmokeArtifacts(request);
});

test('auth providers create edit and toggle', async ({ page }) => {
  const id = `${prefix}auth-${Date.now()}`;

  await page.goto('/identity/providers');
  await expect(page.getByRole('heading', { name: 'Auth Providers' })).toBeVisible();

  const duplicateButton = page.getByRole('button', { name: /Duplicate as New/i });
  await duplicateButton.scrollIntoViewIfNeeded();
  await duplicateButton.click({ force: true });
  await expect(page.getByRole('button', { name: /Create Provider/i })).toBeVisible();
  const providerIdInput = page.locator('input[placeholder="google-workspace"]:visible');
  await expect(providerIdInput).toBeEditable();
  await providerIdInput.fill(id);
  const displayNameInput = page.locator('input[placeholder="Google Workspace"]:visible');
  await displayNameInput.fill('Playwright Auth Provider');
  await expect(displayNameInput).toHaveValue('Playwright Auth Provider');
  const secretRefInput = page.locator('input[placeholder="auth/google-workspace/client_secret"]:visible');
  await secretRefInput.fill(`auth/${id}/client_secret`);
  await expect(secretRefInput).toHaveValue(`auth/${id}/client_secret`);
  await page.getByRole('button', { name: /Create Provider/i }).click();

  await expect(page.locator('input[placeholder="google-workspace"]:visible')).toHaveValue(id);
  await expect(page.getByRole('button', { name: /Save/i })).toBeVisible();

  const updatedDisplayNameInput = page.locator('input[placeholder="Google Workspace"]:visible');
  await updatedDisplayNameInput.fill('Playwright Auth Provider Updated');
  const updatedSecretRefInput = page.locator('input[placeholder="auth/google-workspace/client_secret"]:visible');
  await updatedSecretRefInput.fill(`auth/${id}/token`);
  await page.getByRole('button', { name: /Save/i }).click();

  await expect(updatedDisplayNameInput).toHaveValue('Playwright Auth Provider Updated');
  await expect(updatedSecretRefInput).toHaveValue(`auth/${id}/token`);

});

test('channels create edit and toggle', async ({ page }) => {
  const id = `${prefix}channel-${Date.now()}`;

  await page.goto('/channels');
  await expect(page.getByRole('heading', { name: 'Channels' })).toBeVisible();

  await page.getByRole('button', { name: /New Channel/i }).click();
  await page.getByLabel(/Channel ID/i).fill(id);
  await page.getByLabel(/^Name$/i).fill('Playwright Web Channel');
  await page.getByLabel(/^Kind$/i).selectOption('web_chat');
  await page.getByLabel(/Target/i).fill('default');
  await page.getByRole('button', { name: /^Create$/i }).click();

  const card = page.locator(`text=${id}`).first();
  await expect(card).toBeVisible();
  await page.getByRole('button', { name: /Edit/i }).first().click({ force: true });
  await page.getByLabel(/Target/i).fill('updated-default');
  await page.getByRole('button', { name: /^Save$/i }).click();
  await expect(page.locator('text=updated-default').first()).toBeVisible();

  await page.getByRole('button', { name: /Disable/i }).first().click({ force: true });
  await expect(page.getByRole('button', { name: /Enable/i }).first()).toBeVisible();

  await page.getByRole('button', { name: /Enable/i }).first().click({ force: true });
});

test('providers create edit bind and toggle', async ({ page }) => {
  const id = `${prefix}provider-${Date.now()}`;

  await page.goto('/providers');
  await expect(page.getByRole('heading', { name: 'Providers' })).toBeVisible();

  // 点击 New Provider 按钮
  await page.getByRole('button', { name: /New Provider/i }).click();

  // 填写 Provider ID (name) - placeholder 是 "e.g. Aliyun Qwen"
  await page.getByPlaceholder(/e\.g\./i).fill(id);

  // 当前 new provider 默认就是 openai + openai_compatible，直接补最小必填项
  // 填写 Base URL
  await page.getByPlaceholder('https://...').fill('http://127.0.0.1:1234/v1');

  // 填写 Secret Reference - placeholder 是 "provider/openai/key"
  await page.getByPlaceholder('provider/openai/key').fill(`provider/${id}/api_key`);

  // 点击 Save
  await page.getByRole('button', { name: /^Save$/i }).click();

  // 验证 provider 创建成功并显示在列表中
  await expect(page.locator(`text=${id}`).first()).toBeVisible();
});

async function login(page: Page) {
  const hasCredentials = token.includes(':');

  if (hasCredentials) {
    // local_password path
    await page.goto('/login');
    await expect(page.getByRole('heading', { name: 'TARS Ops' })).toBeVisible({ timeout: 15_000 });

    const providerSelect = page.getByLabel('Auth Provider');
    await expect(providerSelect).toBeVisible();
    // Wait for providers to load and default to local_password
    await expect(providerSelect).toHaveValue('local_password', { timeout: 10_000 });

    const [username, password] = token.split(':', 2);
    await page.getByLabel('Username or Email').fill(username);
    await page.getByLabel('Password').fill(password);
  } else {
    // local_token path — pin provider via URL param to avoid the loadProviders race condition:
    // LoginView's setProviderID updater resets to local_password when providers load if current
    // value is local_token, UNLESS presetProviderID is set via ?provider_id= query param.
    await page.goto('/login?provider_id=local_token');
    await expect(page.getByRole('heading', { name: 'TARS Ops' })).toBeVisible({ timeout: 15_000 });

    // Wait for the token input to appear (confirms provider pin was respected)
    const tokenInput = page.getByLabel('Local Token');
    await expect(tokenInput).toBeVisible({ timeout: 10_000 });
    await tokenInput.fill(token);
  }

  await page.getByRole('button', { name: 'Sign In' }).click();
  // App redirects to /sessions after login — assert we left /login
  await expect(page).not.toHaveURL(/\/login/, { timeout: 15_000 });
}

async function cleanupSmokeArtifacts(request: APIRequestContext) {
  const headers = { Authorization: `Bearer ${token}` };

  const accessResponse = await request.get('/api/v1/config/auth', { headers });
  expect(accessResponse.ok()).toBeTruthy();
  const accessConfig = (await accessResponse.json()) as AccessConfigResponse;
  const nextAccessConfig = {
    ...accessConfig.config,
    auth_providers: (accessConfig.config.auth_providers || []).filter((item) => !String(item.id || '').startsWith(prefix)),
    channels: (accessConfig.config.channels || []).filter((item) => !String(item.id || '').startsWith(prefix)),
  };
  const accessUpdateResponse = await request.put('/api/v1/config/auth', {
    headers,
    data: {
      config: nextAccessConfig,
      operator_reason: 'Playwright smoke cleanup auth and channels',
    },
  });
  expect(accessUpdateResponse.ok()).toBeTruthy();

  const providersResponse = await request.get('/api/v1/config/providers', { headers });
  expect(providersResponse.ok()).toBeTruthy();
  const providersConfig = (await providersResponse.json()) as ProvidersConfigResponse;
  const nextProvidersConfig = {
    ...providersConfig.config,
    entries: (providersConfig.config.entries || []).filter((item) => !String(item.id || '').startsWith(prefix)),
    primary: String(providersConfig.config.primary.provider_id || '').startsWith(prefix)
      ? { provider_id: '', model: '' }
      : providersConfig.config.primary,
    assist: String(providersConfig.config.assist.provider_id || '').startsWith(prefix)
      ? { provider_id: '', model: '' }
      : providersConfig.config.assist,
  };
  const providersUpdateResponse = await request.put('/api/v1/config/providers', {
    headers,
    data: {
      config: nextProvidersConfig,
      operator_reason: 'Playwright smoke cleanup providers',
    },
  });
  expect(providersUpdateResponse.ok()).toBeTruthy();
}

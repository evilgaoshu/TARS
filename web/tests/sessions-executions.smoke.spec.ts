/**
 * sessions-executions.smoke.spec.ts
 *
 * Read-only browser smoke for the on-call diagnosis main path:
 *   /sessions          — incident queue
 *   /sessions/:id      — session detail (first available session, read-only)
 *   /executions        — execution triage queue
 *
 * Does NOT create or modify any data.
 * Token is resolved via shared_ops_token fallback: TARS_PLAYWRIGHT_TOKEN →
 * TARS_OPS_API_TOKEN → SSH canonical env on shared-lab.
 */

import { expect, test } from '@playwright/test';
import type { Page } from '@playwright/test';

const configuredToken = process.env.TARS_PLAYWRIGHT_TOKEN || process.env.TARS_OPS_API_TOKEN;

if (!configuredToken) {
  throw new Error(
    'TARS_PLAYWRIGHT_TOKEN or TARS_OPS_API_TOKEN is required for sessions-executions smoke tests',
  );
}

const token = configuredToken;
const fatalErrorsByPage = new WeakMap<Page, string[]>();

test.describe.configure({ mode: 'serial' });

// These tests are read-only.  Log in only when needed: navigate to /sessions
// and, if the app bounces us back to /login, authenticate.  In serial mode
// Playwright reuses the same browser context, so the session cookie set by the
// first test is available to all subsequent tests.
test.beforeEach(async ({ page }) => {
  installFatalErrorCollector(page);
  // Always log in at the start of each test (same pattern as control-plane).
  // This avoids races with client-side auth-guard redirects and session refresh.
  await login(page);
});

// ── /sessions ─────────────────────────────────────────────────────────────────

test('/sessions shows incident queue headline', async ({ page }) => {
  await page.goto('/sessions');

  // Eyebrow: "Incident queue"  |  Hero title: "Sessions"
  await expect(page.locator('text=Incident queue').first()).toBeVisible({ timeout: 20_000 });
  await expect(page.getByRole('heading', { name: 'Sessions' }).first()).toBeVisible();

  // Stats chips: "Visible incidents" always rendered even if count is 0
  await expect(page.locator('text=Visible incidents').first()).toBeVisible();

  // No fatal JS errors on the page
  const errors = await collectFatalErrors(page);
  expect(errors, `fatal console errors on /sessions: ${errors.join('; ')}`).toHaveLength(0);
});

// ── /sessions/:id ──────────────────────────────────────────────────────────────

test('/sessions/:id shows session detail with current diagnosis section', async ({
  page,
  request,
}) => {
  // Resolve a real session ID from the API (read-only)
  const headers = { Authorization: `Bearer ${token}` };
  const resp = await request.get('/api/v1/sessions?limit=5&sort_by=updated_at&sort_order=desc', {
    headers,
  });

  if (!resp.ok()) {
    console.warn(`/sessions API returned ${resp.status()}, skipping detail smoke`);
    test.skip();
    return;
  }

  const data = (await resp.json()) as { items?: Array<{ session_id: string }> };
  const sessions = data.items ?? [];

  if (sessions.length === 0) {
    console.warn('No sessions found on shared-lab, skipping detail smoke');
    test.skip();
    return;
  }

  const sessionId = sessions[0].session_id;
  await page.goto(`/sessions/${sessionId}`);

  // "Current diagnosis" section header always visible on session detail
  await expect(page.locator('text=Current diagnosis').first()).toBeVisible({ timeout: 20_000 });

  // Eyebrow falls back to nav.sessions key: "Sessions"
  await expect(page.locator('text=Sessions').first()).toBeVisible();

  // No fatal JS errors
  const errors = await collectFatalErrors(page);
  expect(errors, `fatal console errors on /sessions/${sessionId}: ${errors.join('; ')}`).toHaveLength(0);
});

// ── /executions ───────────────────────────────────────────────────────────────

test('/executions shows execution triage queue', async ({ page }) => {
  await page.goto('/executions');

  // Hero title: "Executions"
  await expect(page.getByRole('heading', { name: 'Executions' }).first()).toBeVisible({
    timeout: 20_000,
  });

  // Stats chips: "Visible" count is always shown
  await expect(page.locator('text=Visible').first()).toBeVisible();

  // No fatal JS errors
  const errors = await collectFatalErrors(page);
  expect(errors, `fatal console errors on /executions: ${errors.join('; ')}`).toHaveLength(0);
});

// ── helpers ───────────────────────────────────────────────────────────────────

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

/**
 * Collect console errors that indicate a fatal/unhandled crash.
 * Filters out known benign warnings (peer-dep notices, HMR, etc.).
 */
async function collectFatalErrors(page: Page): Promise<string[]> {
  return fatalErrorsByPage.get(page) || [];
}

function installFatalErrorCollector(page: Page) {
  const errors: string[] = [];
  fatalErrorsByPage.set(page, errors);

  page.on('console', (message) => {
    if (message.type() !== 'error') {
      return;
    }
    const text = message.text();
    if (!isBenignBrowserError(text)) {
      errors.push(`console: ${text}`);
    }
  });

  page.on('pageerror', (error) => {
    const text = error.message || String(error);
    if (!isBenignBrowserError(text)) {
      errors.push(`pageerror: ${text}`);
    }
  });
}

function isBenignBrowserError(text: string): boolean {
  return [
    'ResizeObserver loop completed with undelivered notifications',
    'ResizeObserver loop limit exceeded',
  ].some((token) => text.includes(token));
}

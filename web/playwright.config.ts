import { defineConfig } from '@playwright/test';

const baseURL = process.env.TARS_PLAYWRIGHT_BASE_URL || 'http://192.168.3.106:8081';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  workers: 1,
  timeout: 90_000,
  expect: {
    timeout: 15_000,
  },
  reporter: 'list',
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
});

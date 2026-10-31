import { defineConfig, devices } from '@playwright/test'
import { mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

// Runs against TORQ_E2E_URL (e.g. the container in CI), or starts ../bin/torq
// with a fresh data directory.
const external = process.env.TORQ_E2E_URL
const port = 18090
const dataDir = mkdtempSync(join(tmpdir(), 'torq-e2e-'))

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [['list']],
  use: {
    baseURL: external ?? `http://127.0.0.1:${port}`,
    locale: 'en-US',
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  ...(external
    ? {}
    : {
        webServer: {
          command: '../bin/torq',
          url: `http://127.0.0.1:${port}/healthz`,
          reuseExistingServer: false,
          env: { TORQ_DATA_DIR: dataDir, TORQ_LISTEN_ADDR: `127.0.0.1:${port}`, TORQ_LOG_LEVEL: 'warn' },
        },
      }),
})

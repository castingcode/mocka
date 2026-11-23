// Browser tests for the mocka UI. The config builds mockasrv and serves a
// temporary copy of the Postman example responses, so the UI's writes never
// touch the repository. Run with: cd e2e && npm ci && npx playwright test
const { defineConfig, devices } = require('@playwright/test');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const port = Number(process.env.MOCKA_E2E_PORT || 4590);
// Playwright loads this config again in each worker process, which inherits
// the environment, so the folder is created once and then reused.
if (!process.env.MOCKA_E2E_FOLDER) {
  const workDir = fs.mkdtempSync(path.join(os.tmpdir(), 'mocka-e2e-'));
  process.env.MOCKA_E2E_FOLDER = path.join(workDir, 'responses');
  fs.cpSync(path.join(__dirname, '..', 'examples', 'postman', 'responses'), process.env.MOCKA_E2E_FOLDER, { recursive: true });
}
const folder = process.env.MOCKA_E2E_FOLDER;
const binary = path.join(path.dirname(folder), process.platform === 'win32' ? 'mockasrv.exe' : 'mockasrv');

module.exports = defineConfig({
  testDir: './tests',
  fullyParallel: false,
  workers: 1, // the tests share one server and its responses folder
  forbidOnly: !!process.env.CI,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: `http://localhost:${port}`,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: `go build -o "${binary}" ./cmd/mockasrv && "${binary}" -port ${port} -folder "${folder}"`,
    cwd: path.join(__dirname, '..'),
    url: `http://localhost:${port}/_mocka/ui/`,
    timeout: 120_000,
    reuseExistingServer: false,
  },
});

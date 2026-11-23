// Browser tests for the mocka UI. playwright.config.js starts mockasrv on a
// temporary copy of examples/postman/responses (MOCKA_E2E_FOLDER).
const { test, expect } = require('@playwright/test');
const fs = require('node:fs');
const path = require('node:path');

test.describe.configure({ mode: 'serial' }); // the tests share one server

const folder = process.env.MOCKA_E2E_FOLDER;
const uiYML = () => {
  const file = path.join(folder, 'responses.d', 'ui.yml');
  return fs.existsSync(file) ? fs.readFileSync(file, 'utf8') : '';
};

// moca sends a MOCA request to /service and returns the response XML.
async function moca(request, query, sessionKey = '') {
  const env = sessionKey ? `<environment><var name="SESSION_KEY" value="${sessionKey}"/></environment>` : '';
  const res = await request.post('/service', {
    headers: { 'Content-Type': 'application/moca-xml' },
    data: `<moca-request autocommit="True">${env}<query><![CDATA[${query}]]></query></moca-request>`,
  });
  expect(res.ok()).toBeTruthy();
  return res.text();
}

async function login(request) {
  const xml = await moca(request, "login user where usr_id = 'e2e' and usr_pswd = 'pw'");
  return xml.match(/<field>([0-9a-f-]{36})<\/field>/)[1];
}

test('test page shows the matching entry and its CSV-backed result set', async ({ page }) => {
  await page.goto('/_mocka/ui/test');
  await page.getByLabel('Query').fill('LIST locations where wh_id = "MHE"');
  await page.getByRole('button', { name: 'Test' }).click();

  const result = page.locator('#result');
  await expect(result).toContainText("exact: list locations where wh_id = 'mhe'");
  await expect(result).toContainText('responses.yml entry 2');
  await expect(result.locator('table.results tbody tr')).toHaveCount(3);
  await expect(result.locator('table.results th').first()).toHaveText('stoloc (S)');
  await expect(result.locator('.null')).toHaveCount(1);
});

test('only the fields of the selected match type are shown', async ({ page }) => {
  await page.goto('/_mocka/ui/responses');
  const field = (type) => page.locator(`fieldset[data-for="${type}"]`);
  await expect(field('exact')).toBeVisible();
  await expect(field('command')).toBeHidden();

  await page.selectOption('#type', 'command');
  await expect(field('command')).toBeVisible();
  await expect(field('exact')).toBeHidden();
  await expect(field('publish_data')).toBeHidden();
  await expect(field('prefix')).toBeHidden();
});

test('an unmatched request appears live and becomes a saved response in one click', async ({ page, request }) => {
  await page.goto('/_mocka/ui/requests?unmatched=1');
  await expect(page.locator('#request-rows')).toContainText('No requests without a response yet.');

  // Send the request after the page loaded: the row must arrive by polling.
  const key = await login(request);
  await moca(request, "list carriers where carcod = 'UPS'", key);
  const row = page.locator('#request-rows tr', { hasText: "list carriers where carcod = 'UPS'" });
  await expect(row).toBeVisible({ timeout: 5000 });
  await expect(row).toContainText('no response');

  await row.getByRole('link', { name: 'Create response' }).click();
  await expect(page).toHaveURL(/\/_mocka\/ui\/responses\?type=exact/);
  await expect(page.locator('textarea[name="query"]')).toHaveValue("list carriers where carcod = 'UPS'");

  await page.locator('textarea[name="results"]').fill(
    '<moca-results><metadata><column name="carcod" type="S" length="10" nullable="false"/></metadata><data><row><field>UPS</field></row></data></moca-results>');
  await page.getByRole('button', { name: 'Add response' }).click();

  await expect(page.locator('.notice')).toHaveText('Saved to responses.d/ui.yml.');
  const saved = page.locator('table.entries tbody tr', { hasText: 'list carriers' });
  await expect(saved).toContainText('responses.d/ui.yml entry 1');
  await expect(saved).toContainText('1 row');
  expect(uiYML()).toContain("query: list carriers where carcod = 'UPS'");

  // The client now gets the saved response.
  const xml = await moca(request, "list carriers where carcod = 'UPS'", key);
  expect(xml).toContain('<status>0</status>');
  expect(xml).toContain('<field>UPS</field>');
});

test('invalid input shows the error and keeps what was typed', async ({ page }) => {
  await page.goto('/_mocka/ui/responses');
  await page.locator('textarea[name="query"]').fill('list things');
  await page.locator('textarea[name="results"]').fill('<warehouses/>');
  await page.getByRole('button', { name: 'Add response' }).click();

  await expect(page.locator('.error')).toContainText('invalid results XML');
  await expect(page.locator('textarea[name="query"]')).toHaveValue('list things');
  expect(uiYML()).not.toContain('list things');
});

test('a command response with wildcard args is saved and matches', async ({ page }) => {
  await page.goto('/_mocka/ui/responses');
  await page.selectOption('#type', 'command');
  await page.locator('input[name="command"]').fill('list orders');
  await page.locator('textarea[name="args"]').fill('ordnum = ORD*');
  await page.locator('input[name="status"]').fill('510');
  await page.locator('input[name="message"]').fill('No Data Found');
  await page.getByRole('button', { name: 'Add response' }).click();
  await expect(page.locator('.notice')).toHaveText('Saved to responses.d/ui.yml.');

  await page.getByRole('link', { name: 'Test a query' }).click(); // boosted: htmx swaps the page
  await expect(page).toHaveURL(/\/_mocka\/ui\/test$/);
  await page.getByLabel('Query').fill("list orders where wh_id = 'MHE' and ordnum = 'ORD123'");
  await page.getByRole('button', { name: 'Test' }).click();
  await expect(page.locator('#result')).toContainText('command: list orders');
  await expect(page.locator('#result')).toContainText('510 · No Data Found');
});

test('deleting a saved response removes it from ui.yml', async ({ page }) => {
  await page.goto('/_mocka/ui/responses');
  const row = page.locator('table.entries tbody tr', { hasText: 'list carriers' });
  await row.getByRole('button', { name: 'Delete' }).click();

  await expect(page.locator('.notice')).toContainText('from responses.d/ui.yml');
  await expect(page.locator('table.entries tbody tr', { hasText: 'list carriers' })).toHaveCount(0);
  expect(uiYML()).not.toContain('list carriers');
  expect(uiYML()).toContain('list orders');
});

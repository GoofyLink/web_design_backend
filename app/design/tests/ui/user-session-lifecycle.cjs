// Disposable environment only; forces an invalid access token, keeps real refresh cookie.
const assert = require('node:assert/strict');
const { chromium, expect } = require('E:/go/vue-vben-admin-main/node_modules/playwright/test');
const base = 'http://localhost:18889';
(async () => {
  async function post(path, body, token) {
    const r = await fetch(base + path, { method: 'POST', headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}) }, body: JSON.stringify(body) });
    assert.equal(r.status, 200); return (await r.json()).data;
  }
  const auth = await post('/api/admin/v1/auth/login', { username: 'binary_admin', password: 'Temporary-admin-test-123' });
  const username = 'user_session_' + Date.now().toString().slice(-8);
  await post('/api/admin/v1/users', { username, password: 'Temporary-user-test-123' }, auth.accessToken);
  const b = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const p = await b.newPage();
    await p.goto(base + '/account');
    await p.locator('.login-panel input').nth(0).fill(username);
    await p.locator('.login-panel input').nth(1).fill('Temporary-user-test-123');
    await p.locator('.login-panel button[type=submit]').click();
    await expect(p.getByLabel('账号', { exact: true })).toHaveValue(username);
    const expired = p.waitForResponse(r => r.url().endsWith('/me') && r.status() === 401);
    const refreshed = p.waitForResponse(r => r.url().endsWith('/auth/refresh') && r.status() === 200);
    await p.evaluate(() => sessionStorage.setItem('design-user-token', 'invalid-token-for-session-test'));
    await p.reload();
    await Promise.all([expired, refreshed]);
    await expect(p.getByLabel('账号', { exact: true })).toHaveValue(username);
    const token = await p.evaluate(() => sessionStorage.getItem('design-user-token'));
    assert.ok(token && token !== 'invalid-token-for-session-test');
    await expect(p.getByText('内部任务额度', { exact: true })).toBeVisible();
    await p.goto(base + '/');
    const response = p.waitForResponse(r => r.url().endsWith('/auth/logout'));
    await p.getByRole('button', { name: '退出', exact: true }).click();
    assert.equal((await response).status(), 200);
    await expect.poll(() => p.evaluate(() => sessionStorage.getItem('design-user-token'))).toBeNull();
    assert.equal((await p.request.post(base + '/api/v1/auth/refresh', { headers: { Origin: base } })).status(), 401);
    assert.equal((await fetch(base + '/api/v1/me', { headers: { Authorization: 'Bearer ' + token } })).status, 401);
    await p.goto(base + '/account');
    await expect(p.locator('.login-panel')).toBeVisible();
    console.log('PASS real frontend 401 -> cookie refresh -> retried profile, logout clears token/revokes access and refresh, account returns to login');
  } finally { await b.close(); }
})().catch(e => { console.error(e); process.exit(1); });

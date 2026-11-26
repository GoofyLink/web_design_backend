// Disposable integration environment only. Never changes the bootstrap admin.
const assert = require('node:assert/strict');
const { chromium, expect } = require('E:/go/vue-vben-admin-main/node_modules/playwright/test');
const base = 'http://localhost:18890';
async function api(path, token, body, status = 200) {
  const r = await fetch(base + '/api/admin/v1' + path, { method: body ? 'POST' : 'GET', headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}) }, ...(body ? { body: JSON.stringify(body) } : {}) });
  const j = await r.json(); assert.equal(r.status, status, j.message); return j.data;
}
(async () => {
  const suffix = Date.now().toString().slice(-8), username = 'session_' + suffix;
  const password = 'Temporary-admin-test-123', nextPassword = 'Temporary-admin-changed-123';
  const root = (await api('/auth/login', null, { username: 'binary_admin', password })).accessToken;
  const role = await api('/roles', root, { name: '会话测试' + suffix, code: 'session_' + suffix, permissions: ['user:read'] });
  await api('/admins', root, { username, password, roleIds: [role.id] });
  const oldSession = (await api('/auth/login', null, { username, password })).accessToken;
  const b = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const p = await b.newPage();
    async function login(value, destination = '**/design-users') {
      await p.getByPlaceholder('请输入管理员账号').fill(username);
      await p.getByPlaceholder('请输入密码', { exact: true }).fill(value);
      await p.getByRole('button', { name: 'login', exact: true }).click();
      await p.waitForURL(destination);
    }
    await p.goto(base + '/auth/login'); await login(password);
    // A single synthetic expired-access response triggers the real refresh endpoint.
    // Refresh cookies, rotation and retried /me are handled by the real API/client.
    let injected = false;
    await p.route('**/api/admin/v1/me', async route => {
      if (!injected && route.request().method() === 'GET') {
        injected = true;
        await route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ code: 40100, message: 'test access token expiration', data: null }) });
      } else await route.continue();
    });
    const refreshed = p.waitForResponse(r => r.url().endsWith('/auth/refresh') && r.request().method() === 'POST');
    await p.goto(base + '/profile');
    assert.equal((await refreshed).status(), 200);
    await expect(p.locator('input[disabled]').first()).toHaveValue(username);
    await p.unroute('**/api/admin/v1/me');
    await p.getByLabel('管理员姓名').fill('会话验收管理员');
    await p.getByRole('button', { name: '保存资料', exact: true }).click();
    await p.getByText('个人资料已保存', { exact: true }).waitFor();
    assert.equal((await api('/me', oldSession)).nickname, '会话验收管理员');
    assert.equal((await api('/user/info', oldSession)).realName, '会话验收管理员');
    await p.reload();
    await expect(p.getByLabel('管理员姓名')).toHaveValue('会话验收管理员');
    const inputs = p.locator('input[type=password]');
    await inputs.nth(0).fill(password); await inputs.nth(1).fill(nextPassword); await inputs.nth(2).fill(nextPassword);
    await p.getByRole('button', { name: '修改密码', exact: true }).click();
    await p.waitForURL('**/auth/login**');
    await api('/me', oldSession, null, 401);
    await api('/auth/login', null, { username, password }, 401);
    await login(nextPassword, '**/profile');
    await p.locator('[aria-haspopup=menu]').filter({ has: p.locator('[data-slot=avatar]') }).click();
    await expect(p.getByText('内部管理员', { exact: true })).toBeVisible();
    await expect(p.getByText('ann.vben@gmail.com', { exact: true })).toHaveCount(0);
    await expect(p.getByText('收到了 14 份新周报', { exact: true })).toHaveCount(0);
    await p.getByRole('menuitem', { name: /退出登录|Logout|Log out/i }).click();
    const [loggedOut] = await Promise.all([
      p.waitForResponse(r => r.url().endsWith('/auth/logout')),
      p.getByRole('button', { name: /^确\s*认$/, exact: true }).click(),
    ]);
    assert.equal(loggedOut.status(), 200);
    await p.waitForURL('**/auth/login**');
    const r = await p.request.post(base + '/api/admin/v1/auth/refresh', { headers: { Origin: base } });
    assert.equal(r.status(), 401);
    console.log('PASS real admin profile save/reload, refresh after injected access expiry, password change revokes sessions, old password rejected, new login, genuine account menu and logout invalidates refresh');
  } finally { await b.close(); }
})().catch(e => { console.error(e); process.exit(1); });

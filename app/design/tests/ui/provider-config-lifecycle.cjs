// Real isolated configuration API only. Never submits jobs or calls an AI provider.
const assert = require('node:assert/strict');
const { chromium, expect } = require('E:/go/vue-vben-admin-main/node_modules/playwright/test');
const base = 'http://localhost:18890';
(async () => {
  const auth = await (await fetch(base + '/api/admin/v1/auth/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username: 'binary_admin', password: 'Temporary-admin-test-123' }) })).json();
  const token = auth.data.accessToken;
  async function providers() {
    const r = await fetch(base + '/api/admin/v1/providers', { headers: { Authorization: 'Bearer ' + token } });
    assert.equal(r.status, 200); return (await r.json()).data;
  }
  const b = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const p = await b.newPage();
    let jobRequests = 0, externalRequests = 0;
    p.on('request', r => { if (/\/jobs(?:\?|\/|$)/.test(new URL(r.url()).pathname)) jobRequests++; if (r.url().includes('config-test.invalid')) externalRequests++; });
    await p.goto(base + '/auth/login');
    await p.getByPlaceholder('请输入管理员账号').fill('binary_admin');
    await p.getByPlaceholder('请输入密码', { exact: true }).fill('Temporary-admin-test-123');
    await p.getByRole('button', { name: 'login', exact: true }).click();
    await p.waitForURL('**/design-users');
    await p.goto(base + '/design-providers');
    await p.getByRole('button', { name: '新增服务', exact: true }).click();
    const name = '仅配置验收' + Date.now();
    await p.getByPlaceholder('例如：公司图片服务').fill(name);
    await p.getByPlaceholder('https://your-service.example/v1/images/generations').fill('https://config-test.invalid/v1/images/generations');
    await p.getByPlaceholder('请按服务商文档填写').fill('configuration-only');
    await p.getByPlaceholder('输入新的 API Key').fill('fake-config-test-key');
    await p.getByRole('button', { name: '保存配置', exact: true }).click();
    const row = p.getByRole('row').filter({ hasText: name });
    await row.waitFor();
    let provider = (await providers()).find(v => v.name === name); assert.ok(provider);
    assert.equal(provider.status, 'disabled'); assert.equal(provider.hasKey, true);
    assert.equal(provider.apiKey, undefined); assert.ok(!JSON.stringify(provider).includes('fake-config-test-key'));
    await row.getByRole('button', { name: /编.*辑/, exact: true }).click();
    await expect(p.getByPlaceholder('输入新的 API Key')).toHaveValue('');
    await p.getByPlaceholder('请按服务商文档填写').fill('configuration-only-updated');
    await p.getByRole('button', { name: '保存配置', exact: true }).click();
    await expect(row).toContainText('configuration-only-updated');
    provider = (await providers()).find(v => v.name === name);
    assert.equal(provider.hasKey, true); assert.equal(provider.version, 2);
    await row.getByRole('button', { name: /编.*辑/, exact: true }).click();
    await p.getByRole('checkbox', { name: '清除已保存的密钥' }).check();
    await p.getByRole('button', { name: '保存配置', exact: true }).click();
    await expect(row).toContainText('未配置');
    provider = (await providers()).find(v => v.name === name);
    assert.equal(provider.hasKey, false); assert.equal(provider.status, 'disabled');
    assert.equal(jobRequests, 0); assert.equal(externalRequests, 0);
    console.log('PASS real provider create/edit/key retained when blank/explicit clear/masked response/disabled; no generation or external provider requests');
  } finally { await b.close(); }
})().catch(e => { console.error(e); process.exit(1); });

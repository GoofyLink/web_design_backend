// Disposable integration environment only.
const assert = require('node:assert/strict');
const { chromium, expect } = require('E:/go/vue-vben-admin-main/node_modules/playwright/test');
const base = 'http://localhost:18890';
(async () => {
  const b = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const p = await b.newPage();
    let firstCodes = true;
    await p.route('**/api/admin/v1/auth/codes', async route => {
      const response = await route.fetch();
      if (firstCodes) {
        firstCodes = false;
        const body = await response.json();
        body.data = body.data.filter(code => code !== 'canvas:write');
        await route.fulfill({ response, json: body });
      } else await route.fulfill({ response });
    });
    await p.goto(base + '/auth/login');
    await p.getByPlaceholder('请输入管理员账号').fill('binary_admin');
    await p.getByPlaceholder('请输入密码', { exact: true }).fill('Temporary-admin-test-123');
    await p.getByRole('button', { name: 'login', exact: true }).click();
    await p.waitForURL('**/design-users');
    await p.goto(base + '/design-canvas-presets');
    await p.getByRole('button', { name: '新增尺寸', exact: true }).click();
    const name = '尺寸验收' + Date.now();
    const last = p.locator('.ant-table-tbody tr').last();
    await last.getByLabel('尺寸名称', { exact: true }).fill(name);
    await last.getByLabel('尺寸分类', { exact: true }).fill('内部测试');
    await last.getByLabel('宽度', { exact: true }).fill('1200');
    await last.getByLabel('高度', { exact: true }).fill('800');
    async function save() {
      const response = p.waitForResponse(r => r.url().endsWith('/canvas-presets') && r.request().method() === 'PUT');
      await p.getByRole('button', { name: '保存配置', exact: true }).click();
      assert.equal((await response).status(), 200);
      await expect(p.getByRole('button', { name: '保存配置', exact: true })).toBeDisabled();
    }
    async function publicItems() { return (await (await fetch(base + '/api/v1/canvas-presets')).json()).data.items; }
    await save();
    assert.equal((await publicItems()).find(i => i.name === name).width, 1200);
    await p.reload();
    await expect(last.getByLabel('尺寸名称', { exact: true })).toHaveValue(name);
    await last.getByLabel('宽度', { exact: true }).fill('1600');
    await save();
    assert.equal((await publicItems()).find(i => i.name === name).width, 1600);
    await last.getByRole('switch').click(); await save();
    assert.ok(!(await publicItems()).some(i => i.name === name));
    await last.getByRole('switch').click(); await save();
    await last.getByRole('button', { name: /移.*除/, exact: true }).click();
    await p.locator('.ant-modal-confirm-btns button').last().click();
    await save();
    assert.ok(!(await publicItems()).some(i => i.name === name));
    await p.reload();
    assert.ok(!(await p.getByLabel('尺寸名称', { exact: true }).evaluateAll(nodes => nodes.map(n => n.value))).includes(name));
    console.log('PASS stale login permissions refreshed; preset add/edit/disable/enable/remove persist and public API updates');
  } finally { await b.close(); }
})().catch(e => { console.error(e); process.exit(1); });

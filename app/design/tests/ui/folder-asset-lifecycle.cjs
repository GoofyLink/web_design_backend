// Only the disposable integration API and UI proxies; never production.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const { chromium, expect } = require('E:/go/vue-vben-admin-main/node_modules/playwright/test');
const base = 'http://localhost:18889';
async function api(path, token, body, method = body ? 'POST' : 'GET', status = 200) {
  const r = await fetch(base + path, { method, headers: { 'Content-Type': 'application/json', 'Idempotency-Key': crypto.randomUUID(), ...(token ? { Authorization: 'Bearer ' + token } : {}) }, ...(body ? { body: JSON.stringify(body) } : {}) });
  const j = await r.json(); assert.equal(r.status, status, j.message); return j.data;
}
(async () => {
  const suffix = Date.now().toString().slice(-8), username = 'folder_' + suffix;
  const admin = (await api('/api/admin/v1/auth/login', null, { username: 'binary_admin', password: 'Temporary-admin-test-123' })).accessToken;
  await api('/api/admin/v1/users', admin, { username, password: 'Temporary-user-test-123' });
  const token = (await api('/api/v1/auth/login', null, { username, password: 'Temporary-user-test-123' })).accessToken;
  const design = await api('/api/v1/designs', token, { title: '文件夹验收作品', content: { width: 300, height: 200, engine: 'fabric', engineVersion: '6.9.1', schemaVersion: 1, document: { version: '6.9.1', objects: [] } } });
  const b = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const p = await b.newPage();
    await p.goto(base + '/account');
    await p.locator('.login-panel input').nth(0).fill(username);
    await p.locator('.login-panel input').nth(1).fill('Temporary-user-test-123');
    await p.locator('.login-panel button[type=submit]').click();
    await p.getByText('内部任务额度', { exact: true }).waitFor();
    await p.goto(base + '/my');
    await p.getByRole('button', { name: '新建文件夹', exact: true }).click();
    await p.getByLabel('文件夹名称', { exact: true }).fill('验收文件夹');
    await p.getByRole('button', { name: '保存', exact: true }).click();
    const folders = p.locator('.workspace-folders select');
    await expect(folders.locator('option')).toHaveCount(2);
    const folder = (await api('/api/v1/folders', token))[0];
    await folders.selectOption(folder.id);
    await p.getByRole('button', { name: '重命名文件夹', exact: true }).click();
    await p.getByLabel('文件夹名称', { exact: true }).fill('验收文件夹已更名');
    await p.getByRole('button', { name: '保存', exact: true }).click();
    await expect(folders.locator('option:checked')).toHaveText('验收文件夹已更名');
    assert.equal((await api('/api/v1/folders', token))[0].name, '验收文件夹已更名');
    // Prepare membership via API; folder move UI is covered by workspace-extra.
    await api('/api/v1/designs/' + design.id + '/folder', token, { folderId: folder.id, version: design.version }, 'PUT');
    await p.getByRole('button', { name: '刷新文件', exact: true }).click();
    await p.getByRole('heading', { name: '文件夹验收作品', exact: true }).waitFor();
    p.once('dialog', d => d.accept());
    await p.getByRole('button', { name: '删除文件夹', exact: true }).click();
    await expect(folders.locator('option')).toHaveCount(1);
    await p.getByRole('heading', { name: '文件夹验收作品', exact: true }).waitFor();
    assert.equal((await api('/api/v1/folders', token)).length, 0);
    assert.equal((await api('/api/v1/designs/' + design.id, token)).id, design.id);

    const a = await b.newPage();
    await a.goto('http://localhost:18890/auth/login');
    await a.getByPlaceholder('请输入管理员账号').fill('binary_admin');
    await a.getByPlaceholder('请输入密码', { exact: true }).fill('Temporary-admin-test-123');
    await a.getByRole('button', { name: 'login', exact: true }).click();
    await a.waitForURL('**/design-users');
    await a.goto('http://localhost:18890/design-assets');
    await a.getByRole('button', { name: '上传素材', exact: true }).click();
    const filename = 'asset-lifecycle-' + suffix + '.webp';
    await a.locator('.ant-modal input[type=file]').setInputFiles({ name: filename, mimeType: 'image/webp', buffer: await fs.readFile('app/design/internal/transport/testdata/solid.webp') });
    await a.getByPlaceholder('请注明公司原创或授权来源').fill('隔离测试原创素材');
    await a.locator('.ant-modal-footer button').last().click();
    await a.locator('.ant-modal').waitFor({ state: 'hidden' });
    await a.getByPlaceholder('搜索素材名称').fill(filename);
    await a.getByPlaceholder('搜索素材名称').press('Enter');
    const row = a.getByRole('row').filter({ hasText: filename });
    await row.waitFor();
    const assets = await api('/api/admin/v1/assets?scope=public&keyword=' + filename, admin);
    const asset = assets.items[0]; assert.ok(asset);
    await row.getByRole('button', { name: /详.*情/, exact: true }).click();
    await expect(a.locator('.ant-modal img')).toBeVisible();
    await expect.poll(() => a.locator('.ant-modal img').evaluate(img => img.naturalWidth)).toBeGreaterThan(0);
    await a.locator('.ant-modal:visible .ant-modal-close').click();
    await row.getByRole('button', { name: /引.*用/, exact: true }).click();
    await a.getByText('素材引用', { exact: true }).waitFor();
    assert.equal((await api('/api/admin/v1/assets/' + asset.id + '/references', admin)).length, 0);
    await a.locator('.ant-modal:visible .ant-modal-close').click();
    for (const [label, status] of [['停用', 'disabled'], ['启用', 'active']]) {
      await row.getByRole('button', { name: new RegExp(label.split('').join('.*')), exact: true }).click();
      await a.locator('.ant-modal-confirm-btns button').last().click();
      await expect.poll(async () => (await api('/api/admin/v1/assets/' + asset.id, admin)).status).toBe(status);
      await expect(row.getByRole('button', { name: status === 'disabled' ? /启.*用/ : /停.*用/, exact: true })).toBeEnabled();
    }
    const referenced = await api('/api/v1/designs', token, { title: '引用保护测试', content: { width: 300, height: 200, engine: 'fabric', engineVersion: '6.9.1', schemaVersion: 1, document: { version: '6.9.1', objects: [{ type: 'Image', assetId: asset.id, src: 'asset:' + asset.id, width: 100, height: 100 }] } } });
    await row.getByRole('button', { name: /引.*用/, exact: true }).click();
    await expect(a.locator('.ant-modal:visible')).toContainText(referenced.id);
    await a.locator('.ant-modal:visible .ant-modal-close').click();
    for (const label of [/停.*用/, /删.*除/]) {
      await row.getByRole('button', { name: label, exact: true }).click();
      await a.locator('.ant-modal-confirm-btns button').last().click();
      await expect(a.getByRole('alert').filter({ hasText: '素材仍被作品或模板引用' })).toBeVisible();
      assert.equal((await api('/api/admin/v1/assets/' + asset.id, admin)).status, 'active');
    }
    await api('/api/v1/designs/' + referenced.id, token, { version: referenced.version }, 'DELETE');
    const trash = await api('/api/v1/trash', token);
    const trashed = trash.items.find(d => d.id === referenced.id); assert.ok(trashed);
    await api('/api/v1/trash/' + referenced.id, token, { version: trashed.version }, 'DELETE');
    await row.getByRole('button', { name: /删.*除/, exact: true }).click();
    await a.locator('.ant-modal-confirm-btns button').last().click();
    await row.waitFor({ state: 'hidden' });
    await api('/api/admin/v1/assets/' + asset.id, admin, null, 'GET', 404);
    await a.goto('http://localhost:18890/design-categories');
    await a.getByRole('button', { name: '新增分类', exact: true }).click();
    const categoryName = '分类验收' + suffix;
    await a.locator('.ant-modal:visible input').first().fill(categoryName);
    await a.locator('.ant-modal:visible input').nth(1).fill('-100000');
    await a.locator('.ant-modal-footer:visible button').last().click();
    const cr = a.getByRole('row').filter({ hasText: categoryName });
    await cr.waitFor();
    const category = (await api('/api/admin/v1/categories', admin)).find(c => c.name === categoryName); assert.ok(category);
    assert.ok((await api('/api/v1/categories')).some(c => c.id === category.id));
    await cr.getByRole('button', { name: /编.*辑/, exact: true }).click();
    await a.locator('.ant-modal:visible input').first().fill(categoryName + '已更名');
    await a.locator('.ant-modal:visible .ant-select').click();
    await a.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({ hasText: '停用' }).click();
    await a.locator('.ant-modal-footer:visible button').last().click();
    await expect(cr).toContainText('已更名');
    await expect(cr).toContainText('停用');
    assert.ok(!(await api('/api/v1/categories')).some(c => c.id === category.id));
    await cr.getByRole('button', { name: /删.*除/, exact: true }).click();
    await a.locator('.ant-modal-confirm-btns button').last().click();
    await cr.waitFor({ state: 'hidden' });
    assert.ok(!(await api('/api/admin/v1/categories', admin)).some(c => c.id === category.id));
    console.log('PASS real UI/API: folder CRUD preserves designs; asset upload/search/detail/access/references/status/delete and reference protection; category create/update/disable/delete reflected in public API');
  } finally { await b.close(); }
})().catch(e => { console.error(e); process.exit(1); });

// Isolated database only; see README.md. No production account or data is modified.
const { chromium, expect } = require('E:/go/vue-vben-admin-main/node_modules/playwright/test');
const assert = require('node:assert/strict');
const base = 'http://localhost:18889';
async function api(path, body, token) {
  const r = await fetch(base + path, { method: body ? 'POST' : 'GET', headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}) }, body: body ? JSON.stringify(body) : undefined });
  const result = await r.json();
  assert(r.ok, path + ': ' + result.message);
  return result.data;
}
(async () => {
  const username = 'naive_' + Date.now().toString().slice(-9);
  const admin = await api('/api/admin/v1/auth/login', { username: 'binary_admin', password: 'Temporary-admin-test-123' });
  await api('/api/admin/v1/users', { username, password: 'Temporary-user-test-123', nickname: '设计体验测试' }, admin.accessToken);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
    const errors = [];
    page.on('pageerror', e => errors.push(e.message));
    await page.goto(base);
    await page.locator('.live-template').first().waitFor();
    await page.screenshot({ path: 'E:/go/yft-design-main/naive-home-preview.png' });
    await page.getByRole('button', { name: '发现', exact: true }).click();
    await expect(page).toHaveURL(/view=discover/);
    await expect(page.locator('.studio-hero')).toHaveCount(0);
    await page.locator('.live-filters .n-select').last().click();
    await page.getByText('竖版', { exact: true }).click();
    await expect(page.locator('.live-filters')).toContainText('竖版');
    await page.screenshot({ path: 'E:/go/yft-design-main/naive-discover-preview.png' });
    await page.goto(base + '/my');
    await page.getByRole('button', { name: '登录内部账号' }).click();
    await page.getByLabel('用户名', { exact: true }).fill(username);
    await page.getByLabel('密码', { exact: true }).fill('Temporary-user-test-123');
    await page.locator('.login-panel').screenshot({ path: 'E:/go/yft-design-main/naive-login-preview.png' });
    await page.locator('.login-panel button[type=submit]').click();
    await expect(page.getByRole('button', { name: '打开账户菜单' })).toBeVisible();
    await page.getByRole('button', { name: '打开账户菜单' }).click();
    await expect(page.locator('.studio-account-menu')).toContainText('设计体验测试');
    await page.waitForTimeout(350); // Capture the settled popover, after Naive UI's entrance transition.
    await page.screenshot({ path: 'E:/go/yft-design-main/naive-account-menu-preview.png' });
    await page.getByRole('button', { name: '账户设置与额度', exact: true }).click();
    await expect(page).toHaveURL(/account$/);
    await expect(page.getByRole('button', { name: '保存资料' })).toBeVisible();
    await page.screenshot({ path: 'E:/go/yft-design-main/naive-account-preview.png' });
    await page.goto(base + '/tools/edit');
    const open = page.getByRole('button', { name: '进入图片编辑器', exact: true });
    await expect(open).toBeDisabled();
    await page.screenshot({ path: 'E:/go/yft-design-main/naive-upload-preview.png' });
    const png = await page.evaluate(() => {
      const c = document.createElement('canvas'); c.width = 600; c.height = 800;
      const x = c.getContext('2d'); x.fillStyle = '#edf3ff'; x.fillRect(0, 0, 600, 800);
      x.fillStyle = '#3264ff'; x.fillRect(48, 48, 504, 260); x.fillStyle = 'white'; x.font = 'bold 52px sans-serif'; x.fillText('DESIGN', 95, 195);
      x.fillStyle = '#28344a'; x.font = '32px sans-serif'; x.fillText('A fresh start', 90, 440);
      return c.toDataURL().split(',')[1];
    });
    await page.locator('input[type=file]').setInputFiles({ name: '设计草稿.png', mimeType: 'image/png', buffer: Buffer.from(png, 'base64') });
    await expect(open).toBeEnabled();
    await open.click();
    await page.waitForURL('**/design/*');
    await expect(page.getByRole('button', { name: '保存作品', exact: true })).toBeEnabled();
    await page.getByRole('button', { name: '图层', exact: true }).click();
    await page.locator('.layer-name').first().click();
    await page.getByRole('button', { name: '编辑属性', exact: true }).click();
    const brightness = page.getByRole('slider', { name: '亮度', exact: true });
    await brightness.focus();
    await page.keyboard.press('ArrowRight');
    await expect(brightness).toHaveValue('1');
    await page.getByRole('button', { name: '保存作品', exact: true }).click();
    await page.getByText('已保存到我的作品', { exact: true }).waitFor();
    const imageSaved = await page.evaluate(async () => {
      const response = await fetch('/api/v1/designs/' + location.pathname.split('/').pop(), { headers: { Authorization: 'Bearer ' + sessionStorage.getItem('design-user-token') } });
      return (await response.json()).data;
    });
    assert(imageSaved.content.document.objects.some(object => object.filters?.some(filter => filter.type === 'Brightness' && filter.brightness === 0.01)));
    await page.reload();
    await expect(brightness).toHaveValue('1');
    await page.screenshot({ path: 'E:/go/yft-design-main/editor-color-adjustments-preview.png' });
    await page.getByRole('button', { name: '重置调色', exact: true }).click();
    await expect(brightness).toHaveValue('0');
    await page.getByRole('button', { name: '文字', exact: true }).click();
    await page.getByRole('button', { name: '全选元素', exact: true }).click();
    await page.getByRole('button', { name: '组合元素', exact: true }).click();
    await page.getByRole('button', { name: '保存作品', exact: true }).click();
    await page.getByText('已保存到我的作品', { exact: true }).waitFor();
    const grouped = await page.evaluate(async () => {
      const response = await fetch('/api/v1/designs/' + location.pathname.split('/').pop(), { headers: { Authorization: 'Bearer ' + sessionStorage.getItem('design-user-token') } });
      return (await response.json()).data.content.document;
    });
    assert.equal(grouped.objects.length, 1);
    assert.equal(grouped.objects[0].type.toLowerCase(), 'group');
    assert.equal(grouped.objects[0].objects.length, 2);
    await page.getByRole('button', { name: '取消组合', exact: true }).click();
    await expect(page.getByRole('button', { name: '组合元素', exact: true })).toBeVisible();
    await page.getByRole('button', { name: '撤销', exact: true }).click();
    await page.getByRole('button', { name: '图层', exact: true }).click();
    await expect(page.locator('.layer-name')).toHaveCount(1);
    await page.getByRole('button', { name: '重做', exact: true }).click();
    await expect(page.locator('.layer-name')).toHaveCount(2);
    await page.getByRole('button', { name: '素材', exact: true }).click();
    await expect(page.locator('.editor-resource-sidebar')).toBeVisible();
    await page.locator('.layer-name').first().click();
    await expect(page.getByRole('toolbar', { name: '选中元素快捷操作' })).toBeVisible();
    await page.screenshot({ path: 'E:/go/yft-design-main/editor-enhancements-preview.png' });
    await page.setViewportSize({ width: 390, height: 844 });
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'editor mobile overflow');
    await page.screenshot({ path: 'E:/go/yft-design-main/editor-enhancements-mobile.png' });
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.getByRole('button', { name: '保存作品', exact: true }).click();
    await page.getByText('已保存到我的作品', { exact: true }).waitFor();
    await page.goto(base + '/my');
    await page.locator('.my-file-card').first().waitFor();
    await page.getByRole('button', { name: '复制', exact: true }).click();
    await page.getByRole('button', { name: '确认', exact: true }).click();
    await expect(page.locator('.my-file-card')).toHaveCount(2);
    await page.getByRole('button', { name: '新建文件夹', exact: true }).click();
    await page.getByLabel('文件夹名称').fill('品牌物料');
    await page.getByRole('button', { name: '保存', exact: true }).click();
    await page.locator('.studio-dialog').waitFor({ state: 'hidden' });
    await page.screenshot({ path: 'E:/go/yft-design-main/naive-workspace-preview.png' });
    for (const route of ['/tools/all', '/tools/create', '/tools/crop', '/tools/cutout', '/tools/batch', '/jobs']) {
      await page.goto(base + route);
      await expect(page.locator('.studio-tools')).toBeVisible();
      await page.screenshot({ path: 'E:/go/yft-design-main/naive-' + route.replaceAll('/', '-') + '-preview.png' });
    }
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await page.setViewportSize({ width: 390, height: 844 });
    for (const route of ['/', '/?view=discover', '/my', '/tools/edit', '/account', '/jobs']) {
      await page.goto(base + route);
      await page.waitForTimeout(400);
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'page overflow ' + route);
      assert(await page.evaluate(() => [...document.querySelectorAll('.studio-main,.studio-tools')].every(el => el.scrollWidth <= el.clientWidth + 1)), 'content overflow ' + route);
      await page.screenshot({ path: 'E:/go/yft-design-main/naive-' + route.replace(/[^a-z]/g, '-') + '-mobile.png' });
    }
    await page.goto(base + '/my');
    await page.getByRole('button', { name: '打开账户菜单' }).click();
    await page.getByRole('button', { name: '退出登录', exact: true }).click();
    await expect(page.getByRole('button', { name: '登录内部账号' })).toBeVisible();
    assert.deepEqual(errors, []);
    console.log('PASS Naive controls, discover filters, login, account menu, image adjustment save/reload/reset, grouping/ungrouping/undo/redo, resource sidebar, upload/edit/save, copy/list animation, folders, responsive and reduced-motion views');
  } finally { await browser.close(); }
})().catch(e => { console.error(e); process.exit(1); });

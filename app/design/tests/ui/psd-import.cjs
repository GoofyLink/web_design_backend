// Isolated API/database only. Fixtures are generated here, without third-party artwork.
const { chromium, expect } = require('E:/go/vue-vben-admin-main/node_modules/playwright/test');
const { writePsdBuffer } = require('E:/go/yft-design-main/node_modules/ag-psd');
const assert = require('node:assert/strict');
const base = 'http://localhost:18889';
function pixels(width, height, color) {
  const data = new Uint8ClampedArray(width * height * 4);
  for (let i = 0; i < data.length; i += 4) data.set(color, i);
  return { width, height, data };
}
function fixture(complex = false) {
  return writePsdBuffer({ width: 64, height: 48, imageData: pixels(64, 48, [255, 128, 0, 255]), children: [
    { name: '隐藏分组', hidden: true, children: [{ name: '绿色', left: 24, top: 10, imageData: pixels(8, 8, [0, 255, 0, 255]) }] },
    { name: '半透明红色', left: 8, top: 6, opacity: 0.5, blendMode: complex ? 'multiply' : 'normal', imageData: pixels(16, 12, [255, 0, 0, 255]) },
    { name: '蓝色背景', left: 0, top: 0, imageData: pixels(64, 48, [0, 0, 255, 255]) },
  ] });
}
async function api(path, body, token) {
  const response = await fetch(base + path, { method: body ? 'POST' : 'GET', headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}) }, body: body ? JSON.stringify(body) : undefined });
  const result = await response.json();
  assert(response.ok, result.message);
  return result.data;
}
(async () => {
  const admin = await api('/api/admin/v1/auth/login', { username: 'binary_admin', password: 'Temporary-admin-test-123' });
  const username = 'psd_' + Date.now().toString().slice(-9);
  await api('/api/admin/v1/users', { username, password: 'Temporary-user-test-123' }, admin.accessToken);
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.goto(base + '/my');
    await page.getByRole('button', { name: '登录内部账号' }).click();
    await page.getByLabel('用户名', { exact: true }).fill(username);
    await page.getByLabel('密码', { exact: true }).fill('Temporary-user-test-123');
    await page.locator('.login-panel button[type=submit]').click();
    await expect(page.getByRole('button', { name: '打开账户菜单' })).toBeVisible();
    const token = await page.evaluate(() => sessionStorage.getItem('design-user-token'));
    async function select(buffer, name = '图层测试.psd') {
      await page.locator('.creation-upload input').setInputFiles({ name, mimeType: 'application/octet-stream', buffer });
    }
    await page.goto(base + '/tools/create?import=psd');
    await select(fixture());
    await page.getByRole('button', { name: '打开设计', exact: true }).click();
    await page.waitForURL('**/design/*').catch(async error => { throw Error(error.message + '\n' + await page.locator('.creation-body').innerText()); });
    const id = page.url().split('/').pop();
    const design = await api('/api/v1/designs/' + id, null, token);
    assert.equal(design.content.width, 64);
    assert.equal(design.content.height, 48);
    assert.equal(design.content.document.objects.length, 3);
    assert(design.content.coverAssetId, 'layered import should have a workspace thumbnail');
    const objects = design.content.document.objects;
    assert.deepEqual(objects.map(o => o.editorName), ['蓝色背景', '半透明红色', '隐藏分组 / 绿色']);
    assert.equal(objects[1].left, 8); assert.equal(objects[1].top, 6);
    assert(Math.abs(objects[1].opacity - 0.5) < 0.01);
    assert.equal(objects[2].visible, false);
    assert(objects.every(o => o.assetId && o.src === 'asset:' + o.assetId));
    await page.getByRole('button', { name: '图层', exact: true }).click();
    await expect(page.locator('.layer-name')).toHaveCount(3);
    await expect(page.locator('.layer-name').first()).toContainText('隐藏分组 / 绿色');
    await page.locator('.layer-name').filter({ hasText: '半透明红色' }).click();
    await page.getByRole('button', { name: '编辑属性', exact: true }).click();
    await page.getByLabel('位置 X', { exact: true }).fill('12');
    await page.getByLabel('位置 X', { exact: true }).blur();
    await page.getByRole('button', { name: '保存作品', exact: true }).click();
    await page.getByText('已保存到我的作品', { exact: true }).waitFor();
    await page.reload();
    await page.getByRole('button', { name: '图层', exact: true }).click();
    await page.locator('.layer-name').filter({ hasText: '半透明红色' }).click();
    await page.getByRole('button', { name: '编辑属性', exact: true }).click();
    await expect(page.getByLabel('位置 X', { exact: true })).toHaveValue('12');
    await page.screenshot({ path: 'E:/go/yft-design-main/psd-import-preview.png' });
    // Render and inspect actual exported pixels, not only document metadata.
    const downloadPromise = page.waitForEvent('download');
    await page.getByText('导出图片', { exact: true }).click();
    await page.getByRole('button', { name: '导出 PNG', exact: true }).click();
    const download = await downloadPromise;
    const bytes = require('node:fs').readFileSync(await download.path());
    const colors = await page.evaluate(async base64 => {
      const bitmap = await createImageBitmap(await (await fetch('data:image/png;base64,' + base64)).blob());
      const canvas = document.createElement('canvas'); canvas.width = bitmap.width; canvas.height = bitmap.height;
      const ctx = canvas.getContext('2d'); ctx.drawImage(bitmap, 0, 0); bitmap.close();
      return [[1, 1], [13, 7], [25, 11]].map(([x, y]) => [...ctx.getImageData(x, y, 1, 1).data]);
    }, bytes.toString('base64'));
    assert.deepEqual(colors[0], [0, 0, 255, 255]);
    assert(colors[1][0] > 120 && colors[1][0] < 135 && colors[1][2] > 120 && colors[1][2] < 135);
    assert.equal(colors[2][1], 0, 'hidden green layer must not render');
    await page.goto(base + '/tools/create?import=psd');
    await select(fixture(true), '复杂效果.psd');
    await page.getByRole('button', { name: '打开设计', exact: true }).click();
    await expect(page.locator('.creation-error')).toContainText('合成图片');
    await page.getByLabel('PSD 导入方式').selectOption('composite');
    await page.getByRole('button', { name: '打开设计', exact: true }).click();
    await page.waitForURL('**/design/*');
    const merged = await api('/api/v1/designs/' + page.url().split('/').pop(), null, token);
    assert.equal(merged.content.document.objects.length, 1);
    assert(merged.content.coverAssetId);
    await page.goto(base + '/tools/create?import=psd');
    await select(Buffer.alloc(40), '伪造.psd');
    await page.getByRole('button', { name: '打开设计', exact: true }).click();
    await expect(page.locator('.creation-error')).toContainText('不是 PSD');
    const oversized = Buffer.from(fixture()); oversized.writeUInt32BE(9000, 18);
    await select(oversized, '超大画布.psd');
    await page.getByRole('button', { name: '打开设计', exact: true }).click();
    await expect(page.locator('.creation-error')).toContainText('尺寸超限');
    assert.deepEqual(errors, []);
    console.log('PASS PSD real UI upload/import, layer order/names/positions/opacity/visibility, edit/save/reload, exported pixels, composite fallback, malformed and oversized file rejection');
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exit(1); });

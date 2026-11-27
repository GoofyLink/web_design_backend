// Uses only the isolated test services described in README.md.
const { chromium, expect } = require('E:/go/vue-vben-admin-main/node_modules/playwright/test');
const assert = require('node:assert/strict');
const base = 'http://localhost:18889';

(async () => {
  async function api(path, token, body, method = body ? 'POST' : 'GET') {
    const response = await fetch(base + path, {
      method, headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}) },
      body: body ? JSON.stringify(body) : undefined,
    });
    const result = await response.json();
    assert.equal(response.ok, true, path + ': ' + result.message);
    return result.data;
  }
  const admin = (await api('/api/admin/v1/auth/login', null, { username: 'binary_admin', password: 'Temporary-admin-test-123' })).accessToken;
  const templates = await api('/api/admin/v1/templates?pageSize=100', admin);
  const seed = templates.items.find(item => item.status === 'published');
  assert(seed, 'isolated database needs a published layered template');
  const suffix = Date.now().toString().slice(-9);
  const title = '直接编辑验证' + suffix;
  const seedDetail = await api(`/api/admin/v1/templates/${seed.id}/preview`, admin);
  const content = { ...seedDetail.published, title };
  const template = await api('/api/admin/v1/templates', admin, { content });
  await api(`/api/admin/v1/templates/${template.id}/publish`, admin, { version: template.version });
  const username = 'direct_' + suffix;
  await api('/api/admin/v1/users', admin, { username, password: 'Temporary-user-test-123' });
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    let uses = 0;
    page.on('request', request => { if (request.url().endsWith(`/templates/${template.id}/use`)) uses++; });
    const card = page.locator('.live-template').filter({ hasText: title });
    async function search() {
      await page.goto(base);
      await page.getByLabel('搜索模板').fill(title);
      await page.locator('.search-submit').click();
      await expect(card).toBeVisible();
    }
    await search();
    await page.mouse.move(0, 0);
    await expect(card.locator('.card-edit')).toHaveCSS('opacity', '0');
    await card.locator('.card-edit').focus();
    await expect(card.locator('.card-edit')).toHaveCSS('opacity', '1');
    await page.locator('.search-submit').focus();
    await card.hover();
    await expect(card.locator('.card-edit')).toHaveCSS('opacity', '1');
    await card.screenshot({ path: 'E:/go/yft-design-main/template-direct-edit-preview.png' });
    await card.locator('.card-edit').click();
    await expect(page.getByRole('dialog', { name: '登录后继续' })).toBeVisible();
    await page.getByRole('button', { name: '关闭弹窗' }).click();
    assert.equal(uses, 0);
    await card.locator('.card-preview').click();
    await expect(page.getByRole('button', { name: '使用此模板' })).toBeVisible();
    await page.getByRole('button', { name: '关闭弹窗' }).click();
    await card.locator('.card-edit').click();
    await page.locator('.login-panel input').nth(0).fill(username);
    await page.locator('.login-panel input').nth(1).fill('Temporary-user-test-123');
    await page.locator('.login-panel button[type=submit]').click();
    await page.waitForURL('**/design/*');
    await expect(page.getByRole('button', { name: '文字', exact: true })).toBeEnabled();
    assert.equal(uses, 1);
    const designId = page.url().split('/').pop();
    const token = await page.evaluate(() => sessionStorage.getItem('design-user-token'));
    const design = await api('/api/v1/designs/' + designId, token);
    assert.equal(design.content.width, content.width);
    assert.equal(design.content.height, content.height);
    assert.deepEqual(design.content.document, content.document);
    await page.getByRole('button', { name: '图层', exact: true }).click();
    await page.locator('.layer-name').filter({ hasText: content.document.objects.find(object => object.text)?.text }).first().click();
    await page.getByRole('button', { name: '编辑属性', exact: true }).click();
    await page.getByLabel('文字内容', { exact: true }).fill('原模板文字已编辑');
    await page.getByLabel('文字内容', { exact: true }).blur();
    await page.getByRole('button', { name: '斜体', exact: true }).click();
    await page.getByRole('button', { name: '下划线', exact: true }).click();
    await page.getByLabel('文字行距', { exact: true }).fill('1.8');
    await page.getByLabel('文字行距', { exact: true }).blur();
    await page.getByLabel('文字字距', { exact: true }).fill('120');
    await page.getByLabel('文字字距', { exact: true }).blur();
    await page.getByRole('button', { name: '文字', exact: true }).click();
    await page.getByLabel('文字内容', { exact: true }).fill('模板副本已修改');
    await page.getByLabel('文字内容', { exact: true }).blur();
    await page.getByRole('button', { name: '保存作品', exact: true }).click();
    await page.getByText('已保存到我的作品', { exact: true }).waitFor();
    const saved = await api('/api/v1/designs/' + designId, token);
    assert(saved.content.document.objects.some(object => object.text === '模板副本已修改'));
    assert(saved.content.document.objects.some(object => object.text === '原模板文字已编辑'));
    const formatted = saved.content.document.objects.find(object => object.text === '原模板文字已编辑');
    assert.equal(formatted.fontStyle, 'italic');
    assert.equal(formatted.underline, true);
    assert.equal(formatted.lineHeight, 1.8);
    assert.equal(formatted.charSpacing, 120);
    const source = await api(`/api/admin/v1/templates/${template.id}/preview`, admin);
    assert.deepEqual(source.published.document, content.document);
    await page.reload();
    await expect(page.getByRole('button', { name: '文字', exact: true })).toBeEnabled();
    await page.screenshot({ path: 'E:/go/yft-design-main/template-direct-editor-preview.png' });
    await search();
    // Delay the real request to exercise rapid repeated activation without fake success responses.
    await page.route('**/templates/*/use', async route => { await new Promise(resolve => setTimeout(resolve, 500)); await route.continue(); });
    await card.locator('.card-edit').evaluate(button => { button.click(); button.click(); button.click(); });
    await page.waitForURL('**/design/*');
    assert.equal(uses, 2);
    assert.notEqual(page.url().split('/').pop(), designId);
    await search();
    // Offline after listing: stale card must report the real backend error and stay on the gallery.
    let current = (await api('/api/admin/v1/templates?pageSize=100&keyword=' + encodeURIComponent(title), admin)).items.find(item => item.id === template.id);
    await api(`/api/admin/v1/templates/${template.id}/offline`, admin, { version: current.version });
    await card.locator('.card-edit').click();
    await expect(page.locator('.live-error').first()).not.toBeEmpty();
    assert.equal(new URL(page.url()).pathname, '/');
    await expect(card.locator('.card-edit')).toBeEnabled();
    current = (await api('/api/admin/v1/templates?pageSize=100&keyword=' + encodeURIComponent(title), admin)).items.find(item => item.id === template.id);
    await api(`/api/admin/v1/templates/${template.id}/publish`, admin, { version: current.version });
    await card.locator('.card-edit').click();
    await page.waitForURL('**/design/*');
    const mobile = await browser.newPage({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true });
    await mobile.goto(base);
    await mobile.getByLabel('搜索模板').fill(title);
    await mobile.locator('.search-submit').click();
    const mobileEdit = mobile.locator('.live-template').filter({ hasText: title }).locator('.card-edit');
    await expect(mobileEdit).toHaveCSS('opacity', '1');
    assert(await mobile.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
    await mobileEdit.click();
    await expect(mobile.getByRole('dialog', { name: '登录后继续' })).toBeVisible();
    assert.deepEqual(errors, []);
    console.log('PASS hover edit, preview, cancelled login, login continuation, layered clone, save/reload, unchanged source, repeated activation, offline error/retry, mobile entry');
  } finally {
    await browser.close();
    const current = (await api('/api/admin/v1/templates?pageSize=100&keyword=' + encodeURIComponent(title), admin)).items.find(item => item.id === template.id);
    if (current?.status === 'published') await api(`/api/admin/v1/templates/${template.id}/offline`, admin, { version: current.version });
  }
})().catch(error => { console.error(error); process.exit(1); });

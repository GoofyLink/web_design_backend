// Run only against the disposable integration API and UI proxies, never production.
const assert = require('node:assert/strict');
const http = require('node:http');
const { chromium } = require('E:/go/vue-vben-admin-main/node_modules/playwright');
const JSZip = require('E:/go/yft-design-main/node_modules/jszip');
const fs = require('node:fs/promises');
const base = 'http://localhost:18889';
async function api(path, token, body, method = body ? 'POST' : 'GET') {
  const response = await fetch(base + path, { method, headers: { 'Content-Type': 'application/json', 'Idempotency-Key': crypto.randomUUID(), ...(token ? { Authorization: 'Bearer ' + token } : {}) }, ...(body ? { body: JSON.stringify(body) } : {}) });
  const result = await response.json();
  assert.equal(response.status, 200, JSON.stringify(result)); return result.data;
}
(async () => {
  const browser = await chromium.launch({ channel: 'msedge', headless: true });
  let server, provider, adminToken, calls = 0, failNext = true;
  try {
    const renderPage = await browser.newPage();
    await renderPage.goto('http://localhost:5174/');
    await renderPage.evaluate(async () => { window.testFabric = await import('/node_modules/.vite/deps/fabric.js'); });
    server = http.createServer(async (req, res) => {
      try {
        calls++;
        assert.equal(req.headers.authorization, 'Bearer local-render-test-key');
        assert.match(req.headers['idempotency-key'], /^design-job-\d+-item-\d+$/);
        if (failNext) { failNext = false; res.writeHead(400); res.end('intentional first-attempt failure'); return; }
        const chunks = []; for await (const chunk of req) chunks.push(chunk);
        const payload = JSON.parse(Buffer.concat(chunks));
        assert.equal(payload.schemaVersion, 1);
        const imageBase64 = await renderPage.evaluate(async ({ content, resources, format }) => {
          const { StaticCanvas } = window.testFabric;
          async function hydrate(node) {
            if (!node || typeof node !== 'object') return;
            if (String(node.type).toLowerCase() === 'image') { const r = resources[node.assetId]; node.src = `data:${r.mime};base64,${r.base64}`; }
            if (node.fontAssetId) { const r = resources[node.fontAssetId]; const font = await new FontFace(r.fontFamily, `url(data:${r.mime};base64,${r.base64})`).load(); document.fonts.add(font); }
            await Promise.all(Object.values(node).filter(v => v && typeof v === 'object').map(hydrate));
          }
          await hydrate(content.document);
          const c = new StaticCanvas(document.createElement('canvas'), { width: content.width, height: content.height });
          try { await c.loadFromJSON(content.document); c.renderAll(); return c.toDataURL({ format, quality: 1, multiplier: 1 }).split(',')[1]; }
          finally { await c.dispose(); }
        }, payload);
        res.writeHead(200, { 'Content-Type': 'application/json' }); res.end(JSON.stringify({ imageBase64 }));
      } catch (e) { res.writeHead(500); res.end(String(e)); }
    });
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
    const suffix = Date.now().toString().slice(-8), username = 'render_' + suffix;
    adminToken = (await api('/api/admin/v1/auth/login', null, { username: 'binary_admin', password: 'Temporary-admin-test-123' })).accessToken;
    const user = await api('/api/admin/v1/users', adminToken, { username, password: 'Temporary-user-test-123' });
    await api('/api/admin/v1/users/' + user.id + '/task-quota', adminToken, { limitUnits: 10 }, 'PUT');
    const token = (await api('/api/v1/auth/login', null, { username, password: 'Temporary-user-test-123' })).accessToken;
    const config = { name: '本地真实渲染' + suffix, kind: 'renderer', endpoint: 'http://127.0.0.1:' + server.address().port + '/render', model: '', apiKey: 'local-render-test-key', status: 'active' };
    provider = { ...config, ...await api('/api/admin/v1/providers', adminToken, config) };
    const fonts = await api('/api/admin/v1/assets?scope=public&kind=font&pageSize=1', adminToken);
    const images = await api('/api/admin/v1/assets?scope=public&kind=image&pageSize=1', adminToken);
    const font = fonts.items[0], image = images.items[0]; assert(font && image, 'run public asset fixture tests first');
    await api('/api/v1/designs', token, { title: '真实渲染作品', content: { width: 320, height: 200, engine: 'fabric', engineVersion: '6.9.1', schemaVersion: 1, document: { version: '6.9.1', background: '#123456', objects: [{ type: 'Image', assetId: image.id, src: 'asset:' + image.id, left: 0, top: 0, width: 100, height: 100 }, { type: 'Textbox', text: '渲染验证', fontAssetId: font.id, fontFamily: 'asset-font-' + font.id, left: 110, top: 20, width: 200, fontSize: 24, fill: '#ffffff' }] } } });
    const page = await browser.newPage();
    await page.goto(base + '/jobs'); await page.locator('.login-panel input').nth(0).fill(username); await page.locator('.login-panel input').nth(1).fill('Temporary-user-test-123'); await page.locator('.login-panel button[type=submit]').click();
    await page.getByText('新建服务端批量导出', { exact: true }).click();
    await page.getByLabel('渲染服务', { exact: true }).selectOption({ label: config.name });
    page.on('dialog', d => d.accept());
    const adminPage = await browser.newPage();
    await adminPage.goto('http://localhost:18890/auth/login');
    await adminPage.getByPlaceholder('请输入管理员账号').fill('binary_admin');
    await adminPage.getByPlaceholder('请输入密码', { exact: true }).fill('Temporary-admin-test-123');
    await adminPage.getByRole('button', { name: 'login', exact: true }).click();
    await adminPage.waitForURL('**/design-users');
    async function waitStatus(id, status) {
      for (let i = 0; i < 25; i++) { const job = await api('/api/v1/jobs/' + id, token); if (job.status === status) return job; await new Promise(r => setTimeout(r, 500)); }
      throw Error('job never reached ' + status);
    }
    for (const format of ['png', 'jpeg']) {
      if (format === 'jpeg') failNext = true;
      await page.getByRole('checkbox', { name: /真实渲染作品/ }).check();
      await page.getByLabel('格式', { exact: true }).selectOption(format);
      await page.getByRole('button', { name: '提交批量导出', exact: true }).click();
      await page.getByText('批量导出已提交，可在下方刷新查看进度').waitFor();
      const list = await api('/api/v1/jobs', token), id = list.items[0].id;
      if (format === 'png') { await waitStatus(id, 'failed'); await page.getByRole('button', { name: '刷新任务', exact: true }).click(); await page.locator('.job-card').filter({ hasText: '任务 ' + id }).getByRole('button', { name: '重试导出' }).click(); }
      else {
        await waitStatus(id, 'failed');
        await adminPage.goto('http://localhost:18890/design-jobs');
        const row = adminPage.getByRole('row').filter({ has: adminPage.getByRole('cell', { name: id, exact: true }) });
        await row.getByRole('button', { name: /重.*试/, exact: true }).click();
        await adminPage.locator('.ant-modal-confirm-btns button').last().click();
      }
      const done = await waitStatus(id, 'succeeded'); assert.equal(done.completedItems, 1); assert.equal(done.results.length, 1);
      await page.getByRole('button', { name: '刷新任务', exact: true }).click();
      const event = page.waitForEvent('download'); await page.locator('.job-card').filter({ hasText: '任务 ' + id }).getByRole('button', { name: '下载 ZIP' }).click();
      const download = await event; const zip = await JSZip.loadAsync(await fs.readFile(await download.path()));
      const files = Object.values(zip.files).filter(f => !f.dir); assert.equal(files.length, 1); assert(files[0].name.endsWith('.' + format));
      const bytes = await files[0].async('base64');
      const decoded = await renderPage.evaluate(async ({ bytes, format }) => { const blob = await (await fetch(`data:image/${format};base64,${bytes}`)).blob(); const bitmap = await createImageBitmap(blob); const c = document.createElement('canvas'); c.width = bitmap.width; c.height = bitmap.height; const x = c.getContext('2d'); x.drawImage(bitmap, 0, 0); const result = { width: bitmap.width, height: bitmap.height, pixel: [...x.getImageData(319, 199, 1, 1).data] }; bitmap.close(); return result; }, { bytes, format });
      assert.equal(decoded.width, 320); assert.equal(decoded.height, 200); [18, 52, 86].forEach((v, i) => assert(Math.abs(decoded.pixel[i] - v) <= 3));
      const attempts = await api('/api/admin/v1/jobs/' + id + '/attempts', adminToken); assert.equal(attempts.length, 2);
    }
    const quota = await api('/api/v1/task-quota', token); assert.equal(quota.reservedUnits, 0); assert.equal(quota.consumedUnits, 2); assert.equal(calls, 4);
    console.log('PASS real browser+API+worker+local Fabric renderer: failed/retry, PNG/JPEG, image/font resources, output assets, ZIP download/decode/dimensions/pixels, attempts, quota consumption. No AI calls.');
  } finally {
    if (provider && adminToken) { const { id, ...config } = provider; await api('/api/admin/v1/providers/' + id, adminToken, { ...config, apiKey: '', status: 'disabled' }, 'PUT').catch(e => console.error('cleanup:', e)); }
    if (server) await new Promise(resolve => server.close(resolve));
    await browser.close();
  }
})().catch(e => { console.error(e); process.exit(1); });

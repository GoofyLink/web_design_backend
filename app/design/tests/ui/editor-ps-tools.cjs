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


    await page.getByRole('button', { name: '发现', exact: true }).click();
    await expect(page).toHaveURL(/view=discover/);
    await expect(page.locator('.studio-hero')).toHaveCount(0);
    await page.locator('.live-filters .n-select').last().click();
    await page.getByText('竖版', { exact: true }).click();
    await expect(page.locator('.live-filters')).toContainText('竖版');

    await page.goto(base + '/my');
    await page.getByRole('button', { name: '登录内部账号' }).click();
    await page.getByLabel('用户名', { exact: true }).fill(username);
    await page.getByLabel('密码', { exact: true }).fill('Temporary-user-test-123');

    await page.locator('.login-panel button[type=submit]').click();
    await expect(page.getByRole('button', { name: '打开账户菜单' })).toBeVisible();
    await page.getByRole('button', { name: '打开账户菜单' }).click();
    await expect(page.locator('.studio-account-menu')).toContainText('设计体验测试');
    await page.waitForTimeout(350); // Capture the settled popover, after Naive UI's entrance transition.

    await page.getByRole('button', { name: '账户设置与额度', exact: true }).click();
    await expect(page).toHaveURL(/account$/);
    await expect(page.getByRole('button', { name: '保存资料' })).toBeVisible();

    await page.goto(base + '/tools/edit');
    const open = page.getByRole('button', { name: '进入图片编辑器', exact: true });
    await expect(open).toBeDisabled();

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

    const save=async()=>{await page.getByRole('button',{name:'保存作品',exact:true}).click();await page.getByText('已保存到我的作品',{exact:true}).waitFor();return await page.evaluate(async()=>{const r=await fetch('/api/v1/designs/'+location.pathname.split('/').pop(),{headers:{Authorization:'Bearer '+sessionStorage.getItem('design-user-token')}});const j=await r.json();if(!r.ok)throw Error(j.message);return j.data.content.document;});};
    await page.getByRole('button',{name:'黑白',exact:true}).click();
    await expect(page.getByLabel('饱和度',{exact:true})).toHaveValue('-100');
    await page.getByLabel('描边宽度',{exact:true}).fill('6');await page.getByLabel('描边宽度',{exact:true}).press('Tab');
    await page.getByRole('switch',{name:'投影',exact:true}).click();
    await page.getByLabel('阴影模糊',{exact:true}).fill('20');await page.getByLabel('阴影模糊',{exact:true}).press('Tab');
    await page.getByRole('button',{name:'左对齐',exact:true}).click();
    let doc=await save();assert.equal(doc.objects[0].strokeWidth,6);assert.equal(doc.objects[0].shadow.blur,20);assert(doc.objects[0].filters.some(f=>f.type==='Saturation'&&f.saturation===-1));
    await page.reload();await expect(page.getByLabel('描边宽度',{exact:true})).toHaveValue('6');await expect(page.getByLabel('阴影模糊',{exact:true})).toHaveValue('20');
    await page.locator('.editor-file-actions').getByRole('button',{name:'复制',exact:true}).click();await expect(page.getByRole('button',{name:'粘贴',exact:true})).toBeEnabled();await page.getByRole('button',{name:'粘贴',exact:true}).click();
    doc=await save();assert.equal(doc.objects.length,2);assert.notEqual(doc.objects[0].editorId,doc.objects[1].editorId);
    await page.getByRole('button',{name:'图层',exact:true}).click();
    page.once('dialog',d=>d.accept('海报人物'));await page.getByRole('button',{name:/重命名/}).first().click();
    await page.locator('.layer-name').first().click();
    await page.getByRole('button',{name:'编辑属性',exact:true}).click();
    const beforeX=Number(await page.getByLabel('位置 X',{exact:true}).inputValue());
    await page.locator('.editor-document strong').click();await page.keyboard.press('Shift+ArrowRight');
    await expect(page.getByLabel('位置 X',{exact:true})).toHaveValue(String(beforeX+10));
    await page.getByRole('button',{name:'撤销',exact:true}).click();await expect(page.getByLabel('位置 X',{exact:true})).toHaveValue(String(beforeX));
    doc=await save();assert(doc.objects.some(o=>o.editorName==='海报人物'));
    await page.getByRole('button',{name:'全选元素',exact:true}).click();await page.getByRole('button',{name:'组合元素',exact:true}).click();
    await page.locator('.editor-file-actions').getByRole('button',{name:'复制',exact:true}).click();await expect(page.getByRole('button',{name:'粘贴',exact:true})).toBeEnabled();await page.getByRole('button',{name:'粘贴',exact:true}).click();
    doc=await save();const ids=[];const visit=o=>{ids.push(o.editorId);(o.objects||[]).forEach(visit)};doc.objects.forEach(visit);assert.equal(ids.length,new Set(ids).size,'group and child IDs unique');
    await page.reload();await expect(page.getByRole('button',{name:'保存作品',exact:true})).toBeEnabled();
    await page.screenshot({path:'E:/go/yft-design-main/editor-ps-tools-preview.png'});
    await page.setViewportSize({width:390,height:844});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
    assert.deepEqual(errors,[]);console.log('PASS presets, stroke/shadow save/reload, canvas align, clipboard, rename, nudge/undo, group IDs, mobile');
  }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});

const assert = require('node:assert/strict');
const {chromium} = require('E:/go/vue-vben-admin-main/node_modules/playwright');
const base = 'http://localhost:18890';
(async()=>{
  const suffix=Date.now().toString().slice(-7),roleName='life_'+suffix,adminName='lifeadmin_'+suffix,topicName='生命周期专题'+suffix;
  async function api(path,token,body,method=body?'POST':'GET',expected=200){const r=await fetch(base+'/api/admin/v1'+path,{method,headers:{'Content-Type':'application/json',...(token?{Authorization:'Bearer '+token}:{})},...(body?{body:JSON.stringify(body)}:{})});const j=await r.json();assert.equal(r.status,expected,j.message);return j.data;}
  const token=(await api('/auth/login',null,{username:'binary_admin',password:'Temporary-admin-test-123'})).accessToken;
  const role=await api('/roles',token,{code:roleName,name:roleName,permissions:['category:read']});
  const fallback=await api('/roles',token,{code:'backup_'+suffix,name:'backup_'+suffix,permissions:['category:read']});
  const account=await api('/admins',token,{username:adminName,password:'Temporary-admin-test-123',roleIds:[role.id]});
  const session=(await api('/auth/login',null,{username:adminName,password:'Temporary-admin-test-123'})).accessToken;
  const templates=await api('/templates?status=published&pageSize=20',token);assert(templates.items.length);
  const topic=await api('/topics',token,{title:topicName,description:'测试专题',slot:'home',sort:0,status:'published',startsAt:new Date(Date.now()-60000).toISOString(),endsAt:new Date(Date.now()+86400000).toISOString(),templateIds:[templates.items[0].id],version:0});
  const b=await chromium.launch({channel:'msedge',headless:true});try{
    const p=await b.newPage();await p.goto(base+'/auth/login');await p.getByPlaceholder('请输入管理员账号').fill('binary_admin');await p.getByPlaceholder('请输入密码',{exact:true}).fill('Temporary-admin-test-123');await p.getByRole('button',{name:'login',exact:true}).click();await p.waitForURL('**/design-users');
    await p.goto(base+'/design-roles');await p.getByPlaceholder('搜索角色名称或代码').fill(roleName);let rr=p.getByRole('row').filter({hasText:roleName});await rr.getByRole('button',{name:/编.*辑/,exact:true}).click();await p.getByLabel('角色名称',{exact:true}).fill(roleName+'_updated');await p.getByRole('checkbox',{name:/template:read/}).check();await p.locator('.ant-modal-footer button').last().click();await rr.getByRole('cell',{name:roleName+'_updated',exact:true}).waitFor();assert((await api('/auth/codes',session)).includes('template:read'));
    await rr.getByRole('button',{name:/删.*除/,exact:true}).click();await p.locator('.ant-modal-confirm-btns button').last().click();await p.locator('.ant-alert-error').first().waitFor();assert((await api('/roles',token)).some(r=>r.id===role.id));
    await p.goto(base+'/design-admins');await p.getByPlaceholder('搜索管理员').fill(adminName);await p.getByPlaceholder('搜索管理员').press('Enter');const ar=p.getByRole('row').filter({hasText:adminName});await ar.getByRole('button',{name:'分配角色'}).click();await p.locator('.ant-modal .ant-select-selection-item-remove').click();await p.locator('.ant-modal-title').click();await p.locator('.ant-modal .ant-select-selector').click();await p.locator('.ant-modal .ant-select-selection-search-input').fill('backup_'+suffix);await p.locator('.ant-select-item-option-content').filter({hasText:'backup_'+suffix}).click();await p.locator('.ant-modal-title').click();await p.locator('.ant-modal-footer button').last().click();await p.locator('.ant-modal').waitFor({state:'hidden'});assert.deepEqual((await api('/admins/'+account.id,token)).roleIds,[fallback.id]);assert(!(await api('/auth/codes',session)).includes('template:read'));
    await ar.getByRole('button',{name:/停.*用/,exact:true}).click();await p.locator('.ant-modal-confirm-btns button').last().click();await ar.getByRole('button',{name:/启.*用/,exact:true}).waitFor();await api('/auth/login',null,{username:adminName,password:'Temporary-admin-test-123'},'POST',401);await ar.getByRole('button',{name:/启.*用/,exact:true}).click();await p.locator('.ant-modal-confirm-btns button').last().click();await ar.getByRole('button',{name:/停.*用/,exact:true}).waitFor();await api('/auth/login',null,{username:adminName,password:'Temporary-admin-test-123'});
    await p.goto(base+'/design-roles');await p.getByPlaceholder('搜索角色名称或代码').fill(roleName);rr=p.getByRole('row').filter({hasText:roleName});await rr.getByRole('button',{name:/删.*除/,exact:true}).click();await p.locator('.ant-modal-confirm-btns button').last().click();await rr.waitFor({state:'hidden'});
    await p.goto(base+'/design-topics');const tr=p.getByRole('row').filter({hasText:topicName});await tr.getByRole('button',{name:/编.*辑/,exact:true}).click();await p.locator('.ant-modal .ant-select-selector').first().click();await p.locator('.ant-select-item-option-content').filter({hasText:/^下线$/}).click();await p.locator('.ant-modal-footer button').last().click();await tr.getByText('已下线',{exact:true}).waitFor();const publicTopic=await fetch('http://localhost:18889/api/v1/topics/'+topic.id);assert.equal(publicTopic.status,404);await tr.getByRole('button',{name:/删.*除/,exact:true}).click();await p.locator('.ant-modal-confirm-btns button').last().click();await tr.waitFor({state:'hidden'});await api('/topics/'+topic.id,token,null,'GET',404);
    console.log('PASS real UI/API: role permission update effective, assigned-role delete protected, role reassignment/removal, admin disable/enable, unused-role delete, topic offline hidden from public and delete');
  }finally{await b.close();}
})().catch(e=>{console.error(e);process.exit(1)});









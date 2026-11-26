# 页面集成验证

这些脚本只针对本机隔离测试库，不能改为正式服务运行。默认用户为隔离库的 `binary_admin`，密码为测试专用值；实际用户数据库不参与。

环境要求：

- MySQL 测试实例 13307，API 18888，前台代理18889、后台代理18890。
- 依赖当前机器两个前端项目中的 Playwright、JSZip，以及 Edge。
- 已有公共图片、字体和已发布模板测试素材。
- `render-lifecycle.cjs` 还要求隔离API开启 `AllowPrivateProviders`；脚本启动临时本地HTTP渲染器并以浏览器Fabric真实渲染，不调用AI。渲染器结束后关闭，其配置停用。

执行：

```powershell
node app/design/tests/ui/render-lifecycle.cjs
node app/design/tests/ui/security-topic-lifecycle.cjs
node app/design/tests/ui/user-workspace-lifecycle.cjs
node app/design/tests/ui/folder-asset-lifecycle.cjs
node app/design/tests/ui/admin-session-lifecycle.cjs
node app/design/tests/ui/presets-lifecycle.cjs
node app/design/tests/ui/template-lifecycle.cjs
node app/design/tests/ui/provider-config-lifecycle.cjs
node app/design/tests/ui/account-operations.cjs
node app/design/tests/ui/workspace-editor-lifecycle.cjs
node app/design/tests/ui/user-session-lifecycle.cjs
node app/design/tests/ui/security-topic-create.cjs
node app/design/tests/ui/user-create-quota.cjs
```

第一个脚本验证前台提交、前后台失败重试、Go执行器、图片/字体传输、PNG/JPEG输出、ZIP解压与像素、额度和执行记录。第二个验证角色修改/删除保护/重新分配、管理员启停和专题下线删除。脚本会在隔离库创建带时间后缀的数据，重复运行不会覆盖原有用户。

其他模块的既有测试路径记录在 `app/design/api/FRONTEND-INTEGRATION.md`，部分仍位于本机临时目录，最终交付前需要统一归档和完成逐接口审计。

`user-workspace-lifecycle.cjs` 验证后台用户详情、账号和姓名编辑、重置密码后旧会话及旧密码失效、启停用户与新密码登录；前台作品重命名、移入回收站、恢复、永久删除，并通过 API 确认最终404。账号与初始作品由测试API创建，不能将其视为“页面创建用户/作品”验收证据。

`folder-asset-lifecycle.cjs` 验证文件夹新建/改名/按文件夹查看/删除保留作品，后台公共图片上传/搜索/详情与真实图片加载/引用列表/启停/删除、已引用素材的停用及删除保护，以及分类创建/改名/停用/删除和公共分类API同步隐藏。仅使用本仓库的WebP测试图片；账号、作品及文件夹成员关系通过API准备。

`admin-session-lifecycle.cjs` 新建临时管理员，验证个人资料持久化、修改密码撤销旧会话/旧密码、新密码登录、退出后刷新失败，以及真实账户菜单。通过模拟一次GET /me的401触发真实刷新接口，不模拟刷新响应，不等待自然过期；用于防止HTTP响应包装层解析错误导致自动续期失效。不会更改初始管理员密码。

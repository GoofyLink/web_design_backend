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
```

第一个脚本验证前台提交、前后台失败重试、Go执行器、图片/字体传输、PNG/JPEG输出、ZIP解压与像素、额度和执行记录。第二个验证角色修改/删除保护/重新分配、管理员启停和专题下线删除。脚本会在隔离库创建带时间后缀的数据，重复运行不会覆盖原有用户。

其他模块的既有测试路径记录在 `app/design/api/FRONTEND-INTEGRATION.md`，部分仍位于本机临时目录，最终交付前需要统一归档和完成逐接口审计。

# 公司内部账号与“我的”空间

## 管理员添加用户

新增 `POST /api/admin/v1/users`，管理员 Bearer 登录，需要 `user:create` 权限。
请求：`{username,password,nickname?}`；响应：`{id}`，使用统一响应结构。

- 账号为 3–32 位字母、数字或下划线，自动转小写；重复账号返回 409。
- 密码为 8–72 UTF-8 字节，bcrypt 保存，不回显、不写入审计。
- 姓名最多 64 字，省略时使用账号名。
- 创建账号和 `user.create` 审计在同一事务提交，角色固定为普通用户。
- 现有 GET `/users` 和 PATCH `/users/:id/status` 提供查询及启停，停用撤销会话。

数据库迁移升级至 schema 5，包含 `user:create`、`user:update` 和 `user:password` 权限。超级管理员自动拥有；普通管理员需分配此权限。
`AllowPublicRegistration` 默认 false，POST `/api/v1/auth/register` 默认返回 403。内部部署保持 false。

## 前端接入

- 用户站 `http://localhost:5174/my`：登录后读取自己的作品、上传、收藏和回收站。
- 支持图片上传/预览/删除，作品重命名/删除/恢复/永久删除，取消收藏；文件分页 24 项。
- 搜索作品、上传使用服务端查询，收藏搜索和名称排序作用于当前页，界面有明确标识。
- 用户刷新凭据存储在 HttpOnly Cookie，访问令牌保存在当前标签页 sessionStorage，刷新页面和过期时自动刷新；退出撤销会话。
- 首页模板仍是示意数据，画布创建、套用模板和编辑器完整联动尚未接入。
- Vben web-antd 改接 Go 管理员登录，移除演示账号和第三方登录入口，默认进入“前台用户”菜单。
- 开发代理连接 `127.0.0.1:8888`，部署时由同源代理提供 `/api/`。

初次初始化：在后端根目录执行 `go run ./app/design/cmd/server admin create`，交互输入管理员账号密码。
然后在 Vben 登录 → 前台用户 → 添加用户；添加的普通用户用于前台登录，不用于后台登录。

验证：独立 MySQL 测试覆盖创建、重复/非法参数、审计、用户与只读管理员拒绝、停用后会话失效。
浏览器连接隔离测试库验证 Vben 添加用户、前台登录/刷新/退出、真实图片上传/预览/删除、作品改名/删除/恢复，以及窄屏布局。

## 编辑用户与重置密码

- `PATCH /api/admin/v1/users/:id`，权限 `user:update`，请求 `{username,nickname}`。可修改登录账号和姓名；账号自动去空格转小写，重复返回 409。修改账号撤销会话，单独修改姓名不退出登录。用户 ID、作品、素材和状态保持不变。
- `PUT /api/admin/v1/users/:id/password`，权限 `user:password`，请求 `{newPassword}`。密码须为 8–72 UTF-8 字节，bcrypt 保存；成功后撤销全部访问及刷新会话。支持停用用户重置密码，不自动启用账号。
- 两个接口仅能操作普通用户，管理员或不存在的 ID 返回 404。修改与审计在同一事务完成，审计动作 `user.update` / `user.password`，不记录密码。
- Vben 用户列表增加“编辑”“重置密码”弹窗，重置要求确认新密码。操作按钮按管理员权限展示。
- 升级：停止旧后端，执行 `go run ./app/design/cmd/server migrate`，再执行 `go run ./app/design/cmd/server serve`；后台退出后重新登录以加载新增权限。

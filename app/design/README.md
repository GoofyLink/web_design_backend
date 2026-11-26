# Design API：公司内部使用版

当前账号模式：管理员统一添加前台用户，公开注册默认关闭。新增 `POST /api/admin/v1/users`，schema 升级至 4；Vben 前台用户管理和用户站“我的”空间已接入。见 [内部账号说明](api/INTERNAL-USERS.md)。

已实现单体 Go 服务，125 个端点从运行时注册表导出（不含 OPTIONS）。只运行此服务即可使用设计平台 B1—B6 / B8 API，无需启动旧 usercenter/travel/order/payment RPC。

B1—B6 与 B8 后端接口已实现：身份、模板、素材、作品、导出检查、收藏/文件夹/历史/专题、任务队列与服务商配置。会员支付按公司内部使用要求移出范围。AI 厂商未连接，批量渲染需配置受信渲染服务；前端页面和浏览器渲染验收仍未完成。

## 1. 运行要求

- Go 构建环境：仓库 go.mod 的依赖；本次实际使用 Go 1.26.1。部署只需要生成的二进制，不需要安装 Go。
- MySQL：推荐 8.0；本次集成测试在隔离的 MySQL 5.7.26 实例通过，MySQL 8 尚未实际验证。
- 预先创建独立数据库 `looklook_design`。数据库账号需要该库的建表、索引、查询与写入权限。迁移不创建数据库，也不读写民宿表。
- 一个可写文件目录，存储用户图片和管理员上传的公共图片/字体。数据库与该目录必须一起备份。

初始化数据库示例（使用你自己的数据库管理账号执行）：

```sql
CREATE DATABASE looklook_design CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

## 2. Windows 构建与启动

在仓库根目录 PowerShell 中执行：

```powershell
cd E:\go\go-zero-looklook-main
$env:CGO_ENABLED='0'
go build -o dist/design-server.exe ./app/design/cmd/server
if (!(Test-Path app/design/etc/design.yaml)) {
  Copy-Item app/design/etc/design.example.yaml app/design/etc/design.yaml
}

# 修改为自己的数据库账号与密码；不要使用下面的占位值。
$env:DESIGN_MYSQL_DSN='design_user:你的数据库密码@tcp(127.0.0.1:3306)/looklook_design'
# 生成并安全保存密钥；以后重启应使用同一个值，不要每次重新生成。
$env:DESIGN_JWT_SECRET=[Guid]::NewGuid().ToString('N')+[Guid]::NewGuid().ToString('N')

.\dist\design-server.exe migrate -f .\app\design\etc\design.yaml
.\dist\design-server.exe admin create -f .\app\design\etc\design.yaml
.\dist\design-server.exe serve -f .\app\design\etc\design.yaml
```

`admin create` 交互输入用户名、密码、确认密码；密码不回显。它只允许初始化第一位管理员，后续通过管理员接口创建。不提供默认账号密码。DSN 中有特殊字符时按 go-sql-driver/mysql DSN 格式填写。

默认地址 `http://127.0.0.1:8888`。`/healthz` 检查进程存活，`/readyz` 检查 MySQL、迁移校验和及目录写入。迁移需显式执行，不在 serve 中自动改表；重复运行迁移安全，已执行脚本被修改会拒绝运行。

配置中的 `AllowedOrigins` 要写前端实际 origin（协议、主机、端口），例如 `http://localhost:5666`。`localhost` 与 `127.0.0.1` 是不同 origin。前端/后端开发尽量统一使用 localhost，避免 SameSite Cookie 被跨站限制。生产开启 HTTPS 和 `CookieSecure: true`，不要开放任意来源。

exe 可以复制到部署目录，配合 YAML、数据库和文件目录运行；从项目根目录以外启动时，注意配置和存储目录的相对路径。不同系统需分别编译，Windows exe 不能在 Linux 直接运行。

## 3. 接口资料

- [OpenAPI 3.0 JSON](api/openapi.json)：可导入 Apifox、Postman 等。
- [接口说明与字段](api/API.md)。
- [B5/B6/B8 完整接口、配置和任务契约](api/B5-B6-B8.md)。
- [HTTP 请求示例](api/examples.http)。
- [自制模板创建请求](api/template-create.example.json)：将 categoryId 替换为真实分类 ID。

OpenAPI 从实际路由定义生成，不依赖手工维护的重复路由。新增接口后运行：

```powershell
.\dist\design-server.exe openapi | Set-Content -Encoding utf8 app/design/api/openapi.json
```

当前采用手写 Go handler 与路由注册，无需安装 goctl；OpenAPI 是对外接口契约，没有放置会与实际代码脱节的占位 `.api` 文件。

## 4. 验证

```powershell
go test ./app/design/...
go vet ./app/design/...

# 可选的真实数据库集成测试。只能指向测试实例。
# 该账号需 CREATE/DROP DATABASE 权限；测试创建自己的 design_it_* 随机库并清理。
$env:DESIGN_TEST_MYSQL_DSN='test_user:测试密码@tcp(127.0.0.1:13307)/mysql'
go test -count=1 -v ./app/design/...
```

不设置测试 DSN 时，数据库集成测试会明确 SKIP，其余校验继续运行，不应把这种结果描述为数据库验证通过。测试无需现有设计库数据，不操作既有业务表。

本次验证：真实 MySQL 上的注册登录、两端隔离、刷新重放、退出/封禁/修改密码撤销会话、RBAC、最后超级管理员并发保护、分类与模板 CRUD、发布校验、草稿隔离、并发版本冲突、搜索过滤、重新创建连接后的持久化、迁移重复执行、就绪失败；独立 exe 的 migrate/serve 和 HTTP 注册登录刷新退出冒烟通过。`CGO_ENABLED=0` Windows 构建及 go vet 通过。

原仓库 `data/usercenter-comment-review/before` 含不合法的跨 internal 引用，因此不要用旧仓库 `go test ./...` 的备份目录错误判断新模块失败；本模块测试范围为 `./app/design/...`。

## 5. 当前实现边界

- 用户和管理员采用同一账户表的 `kind` 区分，用户名按 kind 唯一。会话、JWT audience、Cookie、HTTP 路由均校验 kind，用户身份不能升级为管理员。
- 角色与权限使用关系表；没有删除账号接口，以停用保留审计。最后一个有效超级管理员不能停用或移除角色。
- 模板保留独立草稿、发布快照和不可变发布修订；版本号保护并发更新。用户作品保留最近 20 份旧文档，引用与额度随历史维护。
- 公开模板详情只包含目录元信息；登录用户套用时创建独立作品，模板下架/删除不会删除已创建作品。
- 文档以资源 ID 引用图片与字体，支持基本形状、文字、分组、路径、渐变和资源图片。拒绝外链、临时 URL、Pattern 和未支持的类型；见 [编辑器协议](api/EDITOR-CONTRACT.md)。Fabric 6.7.1 序列化样例已验证，尚未对完整 yft 编辑器做浏览器验收。
- 热门排序按真实模板套用次数排序，24 小时幂等键防止网络重试重复计数。
- 单进程 IP 限流每种认证端点每分钟 20 次，重启清空；不信任 X-Forwarded-For。部署反向代理后应在网关设置限流，并另行设计受信代理链。
- 全局身份写锁简化了管理员保护、登录与会话撤销的一致性，适用于初期规模；高并发时需要再细化锁粒度。
- 角色与账号管理、模板变更写入持久化审计，审计只存操作元信息，不存密码、Token 和作品正文。
- 作品回收站默认保留 30 天；每小时执行一批清理，或手动运行 maintenance。过期会话清理仍待后续；SiteDir/AdminDir 可选择托管已构建的前端。
- 内容写入使用数据库全局锁，确保多进程下额度与引用一致；适合早期规模，高吞吐需再拆分锁粒度。
- 单文件上限 20 MiB、图片单边 8192/总像素 4000 万、文档 5 MiB、账号额度默认 200 MiB；生产按内存与磁盘容量调整额度。

## 6. 从 B1/B2 升级

先停止旧服务并备份数据库和 StorageDir，然后用新 exe 执行 migrate，再 serve。001 未修改，002 增加素材、引用、额度、作品、修订和幂等表，并补录已有发布模板修订。无需重新创建管理员。旧版程序不能运行 schema 2。

```powershell
.\dist\design-server.exe migrate -f .\app\design\etc\design.yaml
.\dist\design-server.exe serve -f .\app\design\etc\design.yaml
# 手动执行一批过期回收站/孤儿文件清理（常规由服务每小时执行）
.\dist\design-server.exe maintenance -f .\app\design\etc\design.yaml
```

新增配置 StorageQuotaBytes=209715200、TrashDays=30，有默认值。配置加载会将 HTTP 请求上限至少设为 21 MiB、Timeout 至少设为 60 秒；JSON 单独限 6 MiB。文件每次限 20 MiB。清理每批最多 100 个作品和 100 个文件；较大积压可多次运行 maintenance。

B3/B4 验证：真实 MySQL 5.7.26 集成测试覆盖 PNG/JPEG/WebP 上传、字体内容校验、拒绝伪造/损坏/超限文件、越权、文件 ticket 篡改/过期/会话撤销、并发额度与保存、事务失败引用回滚、幂等创建、模板发布与独立套用、修订读取、回收站恢复/到期清理、上传失败补偿、孤儿文件清理。Fabric 6.7.1 实际序列化的分组/渐变 JSON 通过验证和往返保存测试。旧 schema 1 库通过新 exe 升级至 2，原有两个账号保留；独立 exe 上传/创建/保存/读回/文件访问冒烟通过。MySQL 8、完整浏览器编辑器与双前端尚未验收。

## 7. 内部版 B5/B6/B8 升级（schema 3）

备份并停止旧服务，执行新版 migrate，再 serve。新增 003_operations_jobs.sql，001/002 校验和保持不变，不重新创建管理员。新权限 operation:read、operation:write、job:read、job:write、provider:write 由超级管理员自动拥有，其他角色按需分配。

服务商默认无配置。后台 POST/PUT /api/admin/v1/providers 管理服务地址、模型、密钥与启停；密钥只写、AES-GCM 加密保存。不要在前端代码中保存服务商密钥。现有 JWTSecret 同时用于派生配置加密密钥，应保持稳定并安全备份；直接更换后须重新配置服务商密钥。

用户任务额度初始 0，管理员用 PUT /api/admin/v1/users/:id/task-quota 分配累计内部单位。没有会员、订单或支付接口。图片服务适配 OpenAI 兼容 b64_json 响应；renderer 适配受信的外部 Fabric 渲染服务。二者都没有真实接入，也不会因启动 exe 自动启用。普通单张导出由前端浏览器完成；未配置 renderer 时不能使用服务端批量渲染。

可选配置 SiteDir / AdminDir 分别指向 Vue 构建产物，默认不托管；管理员站需构建 base=/admin/。AllowPrivateProviders 默认 false，只有部署公司受信的私网渲染服务才考虑开启。所有接口、错误码、额度和外部调用重试边界见 api/B5-B6-B8.md。


本轮验证（2026-11-26）：全部模块测试（设置真实 MySQL 测试 DSN）与 go vet 通过，CGO_ENABLED=0 Windows exe 构建通过。旧 schema 2 测试库经 exe 升级至 3，账号和原有作品保留；独立进程验证导出事件去重、历史、文件夹及禁用 AI 配置。进一步用 mysqldump 和文件目录副本恢复到独立临时库/目录，另一个 exe 成功登录并读取作品、历史和私有图片。真实 AI / Chromium 服务、MySQL 8 和 Vue 双前端没有验收。
# 浏览器接口文档与 Vben 后台

启动后打开 `http://127.0.0.1:8888/docs` 查看全部接口，`/openapi.json` 返回 OpenAPI 定义。
未配置 `SiteDir` 时，首页 `/` 自动跳转到文档；配置用户网站后仍展示用户网站。
文档内嵌在 Go 程序中，无需 CDN，支持搜索、参数和权限查阅，暂不提供在线请求执行。

本地 Vben `apps/web-antd` 已添加“接口文档”菜单（`/design-api-docs`），复用 `IFrameView`。
开发默认连接 `http://127.0.0.1:8888/docs`；生产默认 `/docs`，需由 Go 或同源反向代理提供。
如需独立部署，在 Vben 的环境文件中设置 `VITE_DESIGN_DOCS_URL=https://你的后端域名/docs` 后重新构建。
HTTPS 后台应搭配 HTTPS 文档地址；反向代理须允许该后台嵌入文档。
Vben web-antd 已接入设计平台管理员登录，并提供“前台用户”管理菜单。

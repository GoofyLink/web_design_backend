# 内部版接口约定

完整路由及请求 JSON schema 见 [openapi.json](./openapi.json)。全部端点均从运行时注册表导出；本文说明响应和业务行为。

## 通用协议

- 用户端 `/api/v1`；管理员 `/api/admin/v1`。管理员凭据不能作为用户凭据使用，反之亦然。
- 请求 JSON 须 `Content-Type: application/json`；拒绝未知字段、null、多个 JSON 值。JSON 请求体最多 6 MiB，文档最多 5 MiB；上传使用 multipart/form-data，单文件最多 20 MiB。
- Bearer token 放 `Authorization: Bearer <token>`。未登录/过期/已撤销返回 401，无权限返回 403。
- 响应 `{code,message,data,requestId}`，成功 code=0。错误 code 为 HTTP 状态 ×100，例如 40900；data=null。不能只检查 HTTP 200 以外的 body。
- 全部数据库 ID 使用字符串。日期使用 UTC RFC3339。状态值和权限码大小写敏感。
- 列表 `{items,total,page,pageSize}`；默认 page=1、pageSize=20，最大 pageSize=100；keyword 最长 200 UTF-8 字节。
- 文件内容成功直接返回二进制；其余业务接口使用 JSON envelope。未实现的接口（例如支付）返回 404。

## 身份、会话

POST `/auth/register`（仅用户）输入 username/password → `{id}`；用户名 3—32 位 ASCII 字母、数字、下划线，存储前转小写；密码 8—72 UTF-8 字节。重复用户名 409。

POST `/auth/login` → `{accessToken,expiresAt}` 并设置 HttpOnly refresh Cookie。Cookie 名分别为 design_user_refresh、design_admin_refresh，path 分别为 `/api/v1/auth`、`/api/admin/v1/auth`。

POST `/auth/refresh` → 相同 token 响应结构，Cookie 轮换；旧 Cookie 再用返回 401。刷新有效期固定为初始登录后 7 天，不无限滑动。

POST `/auth/logout` → data=null，撤销当前会话并清除 Cookie；可以使用 Cookie 或有效 Bearer。刷新和退出必须带配置允许的 Origin。浏览器请求设置 credentials/include（Axios withCredentials=true）。开发示例不带 Secure，生产须开启。

GET `/me` → `{id,kind,username,nickname,avatar,status}`。PATCH `/me` 输入 nickname/avatar，成功 data=null。当前头像只允许空值或 `/api/v1/assets/` 开头的平台路径；头像不是独立上传模块；私有文件地址在 img 中加载需用资源访问链接。

PUT `/me/password` 输入 oldPassword/newPassword；修改成功撤销该账号全部会话，前端回登录页。

管理员附加 GET `/user/info` → `{userId,username,realName,avatar,roles,homePath}`，用于 Vben；GET `/auth/codes` → 权限码数组。后台默认使用前端固定路由，没有 `/menu/all` 动态菜单接口，需将 Vben 切换到固定路由权限模式。

Vben 原 refresh 实现读取字符串 `resp.data`，联调时要修改为读取 `resp.data.accessToken`，并更新 baseURL。本次没有修改 Vben 代码。

## 账号与权限管理

| 方法/路径（管理员前缀） | 权限 | 行为 |
| --- | --- | --- |
| GET /users、/users/:id | user:read | 普通用户分页/详情，支持 keyword、status |
| PATCH /users/:id/status | user:status | `{status:active或disabled}`；停用即撤销会话 |
| GET /admins、/admins/:id | admin:read | 管理员分页/详情，详情额外返回 roleIds |
| POST /admins | admin:write | username/password/roleIds → `{id}` |
| PATCH /admins/:id/status | admin:write | 启停管理员，保护最后有效超级管理员 |
| PUT /admins/:id/roles | admin:write | `{roleIds:["1"]}`，覆盖角色分配，保护最后超级管理员 |
| GET /roles | role:read | `[{id,code,name,permissions}]`；super_admin 隐式拥有所有权限 |
| GET /permissions | role:read | `[{code,name}]` |
| POST /roles、PUT /roles/:id | role:write | code/name/permissions → `{id}`；未知权限码拒绝 |
| DELETE /roles/:id | role:write | 已分配角色不能删除，内置 super_admin 不能编辑或删除 |
| GET /audit-logs | audit:read | 分页返回 id/actorId/action/target/requestId/createdAt |

账号不提供物理删除，使用 disabled。角色代码遵守 3—32 位小写字母、数字、下划线；普通角色可拥有空权限集；管理员必须关联至少一个角色。角色/权限管理权本身是高权限，不分配给普通内容运营。

## 分类、首页和搜索

GET `/api/v1/categories` → 已启用分类数组 `[{id,name,sort,status}]`，按 sort、id 升序。

GET `/api/v1/home` → `{categories,templates}`，templates 为最新 12 条的分页对象。运营推荐位将在 B6 增加。

GET `/api/v1/templates` 支持 keyword、categoryId、sort=latest/popular、ratio=square/portrait/landscape，以及分页。搜索已发布快照的标题、标签和分类名。只返回启用分类内已发布模板。

列表 item 和 GET `/api/v1/templates/:id` 均为 `{id,title,categoryId,tags,width,height,cover}`；不公开文档和草稿。模板下架或分类停用后详情返回 404。

管理端 GET `/categories` 需要 category:read，返回全部状态；POST `/categories`、PUT `/categories/:id` 输入 name/sort/status，返回 `{id}`，需要 category:write。DELETE 同权限，仍有草稿或已发布版本引用时返回 409。

## 模板草稿、发布与下架

管理员 GET `/templates`、`/templates/:id` 需要 template:read。列表支持用户搜索参数及 status=draft/published/offline。响应 item 为 `{id,status,version,draft,published,updatedAt}`；列表不返回两个快照的 document，详情才返回文档。

POST `/templates` 输入 `{content:{...}}`，需要 template:write，返回 `{id,version:1}`。PUT `/templates/:id` 输入 `{version,content:{...}}`，完整替换草稿，返回新的版本号。

content 字段：

| 字段 | 类型与限制 |
| --- | --- |
| title | 1—100 字 |
| categoryId | 存在的分类 ID 字符串 |
| tags | 最多 20 个，每项 1—32 字 |
| width/height | 正整数，单边≤8192，总像素≤4000 万 |
| cover | 内置 `/api/v1/seed-assets/poster.svg`，或匹配 coverAssetId 的 `/api/v1/assets/ID/content` |
| engine/engineVersion | 发布时 engine=fabric，必须提供引擎版本标识 |
| schemaVersion | 发布时必须为 1 |
| document | 原始 JSON 对象，必须有 objects 数组，协议见 EDITOR-CONTRACT.md |

草稿可暂缺封面、引擎和 document，但标题、分类、尺寸仍需有效。一旦提供 document 就校验完整文档。发布必须有完整内容；上传封面同时传 coverAssetId。草稿、发布与修订均只允许公共且启用的图片/字体引用。

POST `/templates/:id/publish` 与 `/offline` 需要 template:publish，body `{version}`。发布将整个草稿复制为已发布快照并记住分类；之后改草稿不会修改用户站所见内容。再次发布切换新快照。下架仅允许 published 状态。

DELETE `/templates/:id` 需要 template:write，body `{version}`，已发布状态不能删除，须先下架。删除会清理模板草稿、发布及修订引用；用户作品为独立快照，保留自身的资源引用和 sourceTemplateId 来源记录。

所有修改、发布、下架、删除都验证版本号，冲突返回 409；操作成功版本递增，发布/下架返回 data=null，可重新 GET 取得最新 version。不允许失败后盲目自动覆盖。

## 可预期错误

400 参数/状态值/未知字段/无效文档；401 登录失效；403 权限或 Origin 不允许；404 不存在或未发布；405 方法错误；409 重复、引用冲突或版本冲突；413 体积超限；429 认证请求限流；500 内部错误；503 readiness 依赖异常。

healthz/readyz 返回同一 JSON envelope。自制示例封面返回 image/svg+xml；资源 content 路由返回真实文件 MIME，支持 Range 请求。

## B3 图片、字体、文件访问

用户前缀 `/api/v1`，管理员前缀 `/api/admin/v1`。管理员公共素材读取需要 asset:read，修改需要 asset:write；超级管理员自动拥有新权限，其他角色须显式分配。

| 方法/路径 | 输入与行为 |
| --- | --- |
| POST /assets | multipart/form-data：file 单文件、kind=image/font、license；用户仅 image/private；管理员上传 public，license 必填 |
| GET /assets | page/pageSize/keyword；用户 scope=mine/public，管理员只列公共素材并支持 status；kind=image/font |
| GET /assets/:id | 返回素材详情；用户只能看自己的私有或已启用公共素材；管理员也不能查看用户私有素材 |
| DELETE /assets/:id | 逻辑删除；存在引用返回 409；释放额度，物理文件由 maintenance 清理 |
| POST /assets/:id/access-url | 返回 `{url,expiresAt}`，url 是相对路径，60 秒有效 |
| GET /api/v1/assets/:id/content | 公共启用文件匿名访问；私有文件必须 Bearer 或 ticket，禁止把 accessToken 放查询字符串 |
| GET /api/v1/fonts | 无需登录，公共启用字体分页；返回 fontFamily=asset-font-ID |
| GET /api/v1/storage | 当前用户 `{usedBytes,limitBytes}`，默认 200 MiB |
| GET 管理端 /assets/:id/references | `[{kind,id}]`；kind 为 design/template_draft/template_published/template_revision |
| PATCH 管理端 /assets/:id/status | `{status:active或disabled}`；被引用时不允许停用 |

字体上传/分页/停用/引用均复用 `/assets`，以 kind=font 区分。支持 PNG/JPEG/WebP 和单字体 TTF/OTF，按实际内容解码校验；不信任扩展名或客户端 MIME。SVG/GIF/WOFF/字体集合暂不接受。单文件最多 20 MiB；单边 ≤8192，总像素 ≤4000 万。文件以随机存储键保存，用户文件名仅作显示。每进程最多同时解码两份上传。

素材详情为 `{id,ownerId,visibility,kind,name,mime,size,sha256,width,height,license,status,createdAt,contentUrl,fontFamily?}`。contentUrl 是稳定路径，私有素材不能直接匿名放进 img；应使用 access-url。成功 content 响应直接返回二进制，Range 支持 206。授权响应不缓存；访问链接过期返回 401。被引用的素材拒绝删除/停用，模板历史发布修订也计入引用。

额度包括该账号上传文件字节及作品 Content 序列化字节，不代表 MySQL 索引/日志的物理体积。管理员上传的公共素材计入上传者额度。回收站作品仍占额度并保留引用，永久清除才释放。未引用但未删除的素材仍是用户素材库，清理任务不会擅自删除。

## B4 作品与回收站

以下都是用户身份接口，不允许管理员冒充用户操作。作品 content 见 [EDITOR-CONTRACT.md](EDITOR-CONTRACT.md)。

| 方法/路径（/api/v1） | 输入 → 输出 |
| --- | --- |
| POST /designs | `{title?,content}` → `{id,version:1}` |
| POST /templates/:id/use | `{title?}` → 独立作品 `{id,version:1}`；只接受已发布、分类启用的模板 |
| POST /designs/:id/copy | `{title?}` → 独立副本 `{id,version:1}`；仅自己的非回收站作品 |
| GET /designs | 我的作品分页，支持 keyword；列表 document=null，不返回正文 |
| GET /designs/:id | `{id,title,version,content,sourceTemplateId?,createdAt,updatedAt}` |
| PUT /designs/:id/document | `{version,content}` → `{id,version}`；完整替换内容、校验引用与额度 |
| PATCH /designs/:id | `{version,title}` → `{id,version}` |
| DELETE /designs/:id | `{version}` → null，放入回收站，版本 +1 |
| GET /trash | 回收站分页，含 deletedAt、最新 version |
| POST /designs/:id/restore | `{version}` → null，恢复且版本 +1；超过保留期返回 409 |
| DELETE /trash/:id | `{version}` → null，永久清除自己的回收站作品 |

前三种创建请求必须携带 `Idempotency-Key`，8—100 位字母、数字、点、下划线、冒号或连字符，大小写敏感。键按用户/创建模式隔离，有效 24 小时。相同键和相同参数返回初次结果；同键不同参数返回 409；成功后删除作品再重试原键仍返回原结果，新的创建操作应生成新键。套用次数只在真正创建时增加一次。

保存、改名、回收站操作都用 version 乐观锁；外部用户得到 404。修改内容、资源引用、额度和审计在一个事务提交；写失败不会遗留一半更新。作品独立持有文档和引用，源模板下架/删除不影响已保存作品。文档保存保留最近 20 份历史，详见 B5-B6-B8.md。

回收站默认 30 天；到期不能恢复，等待下一轮 maintenance 永久清除。每小时自动执行一批（最多 100 个作品），支持 CLI 手动执行。文件清理删除逻辑删除的文件及超过 24 小时的无记录孤儿/临时文件（每批最多 100 个）；数据库查询失败不会推断文件是孤儿。硬盘写失败不会创建有效资源行；数据库写失败会删除本次落盘文件；进程崩溃遗留文件由后续清理回收。

## B4 模板修订与预览

管理员读取需要 template:read：

- GET `/api/admin/v1/templates/:id/preview` → 与详情一致的 `{id,status,version,draft,published,updatedAt}`，前端自行渲染。
- GET `/api/admin/v1/templates/:id/revisions` → 分页 `{items:[{id,version,createdAt}],total,page,pageSize}`。
- GET `/api/admin/v1/templates/:id/revisions/:revisionId` → `{id,version,content,createdAt}`，revisionId 必须属于指定模板。

每次 publish 生成一份不可变发布修订并保存其资源引用。修改草稿不改变已发布内容；发布、下架、删除需携带当前版本。已有 schema 1 发布模板在迁移时补录一份当前快照，不虚构历史记录。当前不提供修订恢复接口；管理员可读旧修订再按当前版本保存草稿，重新检查资源后发布。

## B5 / B6 / B8 新增能力

见 [B5-B6-B8.md](B5-B6-B8.md)：导出检查与回报、封面替换、收藏、文件夹、历史恢复、专题推荐、统计、服务商配置、任务与内部额度。会员支付移出范围；AI 服务未连接，配置后才启用。

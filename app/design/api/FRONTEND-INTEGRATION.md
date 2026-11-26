# 前后台接口对接验收清单

目标：全部公司内部图片设计业务接口有可用页面/流程，完成后按接口逐项验证。AI真实生成、公共注册、会员支付与视频不开放（沿用用户范围）。

## 模块进度

- [ ] 真实首页、分类、模板搜索、专题、详情、收藏、套用
- [ ] 后台分类、模板草稿/发布/下架/修订预览、公共素材与字体
- [ ] 前台作品复制、文件夹、历史版本恢复、封面、导出校验与事件
- [ ] 前台账户资料/密码、存储与任务额度、公共素材和字体使用
- [ ] 后台管理员、角色权限、审计日志
- [ ] 后台专题推荐、统计与导出记录
- [ ] 前后台任务查询/详情/取消/重试/下载、内部额度（AI不提交真实生成）
- [ ] 全模块真实隔离数据库集成验证与浏览器验证

已有但需纳入最终回归：登录会话、前台用户管理、画布推荐尺寸、服务商配置、作品基本增删改与编辑。

### 第三批补充（2026-11-26）

- 后台新增管理员、角色权限、专题运营及真实账户资料/密码页面；前台专题支持按展示位置浏览和翻页。
- 隔离数据库浏览器验证已通过：创建角色和受限管理员、只读分类权限、禁止角色页访问、专题上线及前台详情展示。角色修改删除、管理员启停、专题完整生命周期仍需进一步回归。
- 后台 `/design-jobs` 接入任务分页、状态过滤、详情、执行记录、取消及批量导出重试；AI 重试入口与处理逻辑均禁用。模拟接口浏览器验证详情、执行记录、取消后刷新和 AI 无重试入口通过；真实任务生命周期尚未验证。
- AI 入口回归通过：参考图片与文字经过登录进入编辑页、标签页内暂存与刷新恢复、读取服务配置、移动端不溢出；测试确认未提交生成任务。描述仅暂存在当前标签页，不是服务端持久化。
- 待完成：前台任务中心、内部额度管理、渲染服务配置、编辑器公共素材与字体、历史视觉预览、逐接口验收和完整回归。以上记录不表示全部接口已验收。

## 逐接口验收

### 第十五批：任务详情、执行记录与输出保留

- 扩展并重新执行 `render-lifecycle.cjs` 通过：前台任务详情列出真实输出文件名，后台详情展示两条执行记录（首次失败及重试成功）；下载PNG/JPEG ZIP后删除任务记录，任务列表消失但输出素材仍可读取。
- 原有真实Fabric画布渲染、前后台失败重试、ZIP解包与图片尺寸/像素校验、额度扣减与预留释放继续通过。渲染器仅为本地隔离测试服务，不调用AI。
- 更新11个任务相关接口行。任务取消的两端页面仍待单独验证，不将删除和重试视作取消证据。

### 第十四批：管理员角色、专题与内部账号额度

- `security-topic-create.cjs` 归档并适配角色搜索后复测通过：页面新建角色/分配权限、新建管理员、受限管理员分类只读及角色页拒绝访问、创建上线专题与前台详情展示。
- `security-topic-lifecycle.cjs` 当前环境复测通过：角色修改即时生效、已分配角色删除保护、重分配角色、管理员启停、删除闲置角色、专题下线与删除。
- `user-create-quota.cjs` 新增纯真实API流程：后台页面添加内部用户、设置20单位额度、重新打开读取已保存额度；该用户登录前台，账户页显示20可用/20总额，存储及额度API一致。未使用任务模拟响应。
- 本批更新22个接口证据行。旧临时脚本的部分记录仍只代表历史阶段，最终以逐接口表与归档测试为准。

### 第十三批：个人资料、会话、作品编辑及运营页面

- 将临时脚本整理为 `account-operations.cjs` 和 `workspace-editor-lifecycle.cjs` 并在当前源码上复测通过：前台资料保存/刷新、改密后重新登录；后台统计/导出记录/审计页等待真实请求200且业务code=0后检查页面，无错误提示。
- 作品移动文件夹、复制、文档保存、历史详情/恢复、封面更新、导出校验与结果上报、浏览器下载事件通过。此脚本只验证导出触发与事件，不额外宣称校验了导出文件像素。
- `user-session-lifecycle.cjs` 通过：将标签页访问令牌换成无效值后，真实API返回401，前台使用真实Cookie刷新并成功重试个人资料；退出清除本地令牌、旧访问令牌和刷新Cookie均失效，再打开账户页显示登录。没有修改正式库。
- 本轮更新17个接口证据行，剩余接口仍按实际证据推进。

### 第十二批：模板完整生命周期和真实AI配置保存

- `template-lifecycle.cjs` 已通过（原进程94987结果已确认）：浏览器新建分类/模板与上传封面、发布、预览、搜索、登录套用、收藏/取消收藏；修改草稿不影响已发布内容，再发布形成第二条修订，旧修订内容不可变且可预览；下架后公共详情404、前台搜索空态，后台删除后列表消失。
- `provider-config-lifecycle.cjs` 使用真实隔离API通过创建停用的图片服务配置、修改模型、密钥空值保持、显式清除、只返回hasKey且不回显密钥。使用无效域名与测试假密钥，未提交任务、未请求外部AI。没有将模拟接口验证作为真实保存证据。
- 补齐上述模板15项、尺寸管理2项和服务商3项接口行的证据。其余待核验项保持未完成。

### 第十一批：画布尺寸增删改与权限刷新

- 修复后台尺寸页使用旧登录权限缓存导致所有编辑入口隐藏的问题：加载和重新加载时从 `/auth/codes` 更新权限，再读取尺寸配置。不绕过后端权限，真正只读账号显示授权提示。
- 页面保留新增、行内修改名称/分类/宽高、启停、排序、确认移除、按版本统一保存，并增加未保存提醒。
- `presets-lifecycle.cjs` 已通过：模拟登录时缺少canvas:write的旧缓存，进入页面后通过真实权限接口恢复编辑入口；新增、改宽度、停用、启用、移除、保存及重新加载持久化、公共接口同步生效。测试仅操作隔离库新增项，不修改正式库配置。后台类型检查通过。

### 第十批：管理员会话与个人中心

- 修复后台自动刷新：基础客户端返回原始HTTP响应，令牌位于 `response.data.data.accessToken`；原实现误读为 `response.data.accessToken`，导致刷新200后重试失败并退出。刷新方法现在校验业务code和token并返回明确的登录结果。
- `admin-session-lifecycle.cjs` 通过：临时管理员浏览器登录、模拟一次访问令牌过期401后调用真实刷新接口并成功重试、个人资料保存/刷新、修改密码使旧会话与旧密码失效、新密码登录返回原页面、账户菜单退出及刷新cookie失效。
- 只模拟触发刷新所需的首次401，刷新接口、Cookie轮换、重试以及其余业务操作全部使用隔离真实API。没有等待15分钟自然过期，不将本脚本描述为自然过期测试。
- 顶部移除示例通知、固定邮箱、Pro标签及无关框架外链；账户菜单显示真实账号和“内部管理员”，保留个人中心、平台接口文档及退出登录。后台类型检查通过。

### 第九批：文件夹、素材与分类生命周期

- `folder-asset-lifecycle.cjs` 真实隔离DB和浏览器通过：前台文件夹新建/改名/分类列表/删除，删除文件夹后作品仍保留。文件夹成员关系由API准备，不将其计作本脚本覆盖了移动作品按钮。
- 后台公共图片上传、搜索、详情及签名图片加载、引用查询、停用/启用、删除通过；被作品引用时停用和删除均显示服务端拒绝原因，素材保持启用；永久删除测试作品后可删除素材。
- 后台分类新建、改名并停用、删除通过；公共分类API同步隐藏停用分类。此脚本没有验证前台分类按钮的视觉展示，也未覆盖字体上传，相关证据需单独核验。
- 逐接口表只标记本轮实际通过的UI动作；仍需完成其余接口的证据审计。

### 第八批：用户管理与回收站

- `user-workspace-lifecycle.cjs` 在隔离API与真实浏览器通过：后台详情、编辑账号/姓名、重置密码撤销旧会话及旧密码、停用/启用；前台新密码登录、作品重命名、删除、恢复、永久删除后API返回404。
- 后台用户详情弹窗使用 `GET /users/{id}`；编辑与重置前重新取详情。后台类型检查通过。
- 修正测试的标签页定位为 `role=tab`。复测发现前台5174服务已停止，确认端口无监听后重启，再完整执行通过。
- 本批仅更新有直接证据的接口行；测试以API准备账号和作品，不代表对应创建页面已在此脚本验收。其余接口仍需逐项核验，不表示总体完成。

### 第七批：真实渲染链路验收

- `app/design/tests/ui/render-lifecycle.cjs` 已通过：前台提交→Go worker→本地浏览器 Fabric 渲染服务→输出素材入库→前台 ZIP 下载。测试渲染真实画布（含图片/字体），PNG 与 JPEG 均检查ZIP文件数量、后缀、解码尺寸320×200和背景像素值，非固定占位图。
- 故意使上游首次失败，再分别通过前台重试和后台重试完成；执行记录均为2次、总调用4次、额度预留归零/消耗2单位验证通过。不调用 AI。
- 隔离测试配置临时 AllowPrivateProviders=true，仅允许本地测试渲染。正式配置未改。测试最终将本轮 renderer 停用并关闭本地HTTP服务。测试API已重启为当前源码构建，PID见运行状态文件。
- `DESIGN_TEST_MYSQL_DSN=root@tcp(127.0.0.1:13307)/mysql?parseTime=true&charset=utf8mb4` 下 `go test ./app/design/... -count=1` 全部通过，包括transport集成测试。
- 更正：serve 自动启动 Tasks.Run。第四批声称“未启动worker”不准确，其证据仅覆盖排队/取消，已更正；本轮提供实际执行与下载内容证据。
- 后台行内错误保留服务端原因；管理员角色表单新增1–20个角色校验，并更正文案：停用撤销会话，角色变更立即影响接口权限但不撤销会话。
- `app/design/tests/ui/security-topic-lifecycle.cjs` 通过：角色权限编辑即时生效、已分配角色删除保护、管理员重新分配角色、停用后登录失败/启用恢复、删除未使用角色、专题下线使公共详情404、删除后管理详情404。新增角色名称/代码搜索与分配角色名称搜索。

### 第六批：后台模板可视化编辑与页面收尾

- `TemplateCanvas.vue`：独立编辑弹窗，拖动/缩放/旋转、文字/矩形新增、文字内容/字号/填充/位置、删除、图层选择与前后顺序；公共图片/字体按名称分页选择、图片插入、字体应用和恢复系统字体。应用到父表单草稿后再保存。
- `C:/Users/A/AppData/Local/Temp/design-template-canvas.cjs`：真实隔离DB模板编辑文字与X位置、插入公共图片、自定义字体、图层调整、应用/保存后查询持久化字段、后台自定义字体预览通过。尚需更多异常场景与发布后前台套用回归。
- 后台动态路由仅加载设计业务与账户路由；旧 dashboard 路径重定向到真实运营统计，演示/框架项目菜单不再加载，保留真实管理员个人中心。
- 前台账户头像通过素材签名地址读取并预览，切换与卸载释放对象URL；上传/保存/刷新/清除已作真实接口验证。
- 前后台类型检查、后台生产构建通过。本轮不是全目标验收：服务端worker到ZIP内容链路、角色和专题完整生命周期、逐接口证据审计仍待完成。

### 第五批：编辑器素材、字体与历史预览

- 编辑器新增“素材库”面板：公共图片分页/名称搜索/插入；公共字体分页/搜索/应用到选中文字；可恢复系统字体。
- 作品加载、撤销恢复与历史预览支持 fontAssetId / asset-font-ID，使用受控素材链接加载 FontFace，字体读取失败会明确报错。后台模板预览也已移除自定义字体拒绝逻辑并加载字体（后台字体预览尚待浏览器专项验证）。
- 历史详情新增 Fabric 画布预览，并修复直接访问编辑器时弹窗缺少样式的问题；编辑器四个面板标签改为两列，避免文字挤压。
- `C:/Users/A/AppData/Local/Temp/design-editor-resources.cjs`：隔离真实DB上传公共图片/字体、通过素材面板插入/应用、保存验证资源ID、重开验证字体加载、含图片与字体的历史画布预览通过。已查看截图。尚未验证字体PNG导出和管理员模板字体全流程。
- 前后台类型检查和后台构建通过；修复 ProviderModal 多行条件表达式触发 vue-tsc 模板解析错误。

### 第四批：任务中心与内部额度

- 前台 `/jobs`：登录、分页/状态筛选、详情与输出名称、取消、批量重试、已结束记录删除、带会话刷新能力的 ZIP 下载；首页已增加入口。AI 不创建或重试。
- `BatchExportForm.vue`：分页搜索已保存作品，跨页选择最多10项，选择启用的 renderer、PNG/JPEG，读取额度，按作品版本提交；失败保留相同请求的幂等键。
- 后台用户列表新增 `UserQuotaModal.vue`：读取/设置累计内部额度、消耗/预留/可用展示、最小额度校验和独立 job:read/job:write 权限。
- 后台 `/design-renderers` 复用配置表单，只管理 renderer；允许模型和密钥留空。AI 配置入口继续只管理 image，无连接测试。
- `C:/Users/A/AppData/Local/Temp/design-quota-jobs.cjs`：真实隔离DB额度保存/重开读取、用户任务空态；模拟任务详情/结果/ZIP下载/删除/AI无重试/390px布局通过。
- `C:/Users/A/AppData/Local/Temp/design-batch-jobs-real.cjs`：真实隔离DB新建启用 renderer（无模型/密钥）、选择作品提交、queued列表、额度预留、取消及额度释放、删除通过。此测试只验证排队与取消；后续核对发现 serve 自动启动 worker，因此不能以此声称未启动执行器，也不能作为渲染结果与ZIP内容验收证据。
- AI配置模拟回归继续通过；前后台类型检查通过。下一步仍需字体/公共素材在编辑器使用、历史视觉预览、服务端真实渲染生命周期与最终125接口审计。

| 方法 | 接口 | 功能 | 当前证据 |
|---|---|---|---|
| GET | `/api/admin/v1/admins` | 账户分页列表 | 真实浏览器与隔离API通过：security-topic-create.cjs |
| POST | `/api/admin/v1/admins` | 创建管理员 | 真实浏览器与隔离API通过：security-topic-create.cjs |
| GET | `/api/admin/v1/admins/{id}` | 账户详情（管理员包含 roleIds） | 真实浏览器与隔离API通过：security-topic-lifecycle.cjs |
| PUT | `/api/admin/v1/admins/{id}/roles` | 分配管理员角色 | 真实浏览器与隔离API通过：security-topic-lifecycle.cjs |
| PATCH | `/api/admin/v1/admins/{id}/status` | 启停账户并撤销会话 | 真实浏览器与隔离API通过：security-topic-lifecycle.cjs |
| GET | `/api/admin/v1/assets` | 素材分页；scope=mine/public，kind=image/font | 真实浏览器与隔离DB通过：folder-asset-lifecycle.cjs（2026-11-26） |
| POST | `/api/admin/v1/assets` | 上传图片或管理员公共字体 | 真实浏览器与隔离DB通过：folder-asset-lifecycle.cjs（2026-11-26） |
| DELETE | `/api/admin/v1/assets/{id}` | 删除未引用素材 | 真实浏览器与隔离DB通过：folder-asset-lifecycle.cjs（2026-11-26） |
| GET | `/api/admin/v1/assets/{id}` | 素材元数据 | 真实浏览器与隔离DB通过：folder-asset-lifecycle.cjs（2026-11-26） |
| POST | `/api/admin/v1/assets/{id}/access-url` | 获取 60 秒文件访问链接 | 真实浏览器与隔离DB通过：folder-asset-lifecycle.cjs（2026-11-26） |
| GET | `/api/admin/v1/assets/{id}/references` | 查询公共素材引用 | 真实浏览器与隔离DB通过：folder-asset-lifecycle.cjs（2026-11-26） |
| PATCH | `/api/admin/v1/assets/{id}/status` | 启停未引用公共素材 | 真实浏览器与隔离DB通过：folder-asset-lifecycle.cjs（2026-11-26） |
| GET | `/api/admin/v1/audit-logs` | 审计日志分页 | 真实浏览器与隔离API通过：account-operations.cjs |
| GET | `/api/admin/v1/auth/codes` | 当前管理员权限码 | 真实浏览器与隔离API通过：admin-session-lifecycle.cjs（刷新过期触发为模拟401） |
| POST | `/api/admin/v1/auth/login` | admin 登录 | 真实浏览器与隔离API通过：admin-session-lifecycle.cjs（刷新过期触发为模拟401） |
| POST | `/api/admin/v1/auth/logout` | admin 退出会话 | 真实浏览器与隔离API通过：admin-session-lifecycle.cjs（刷新过期触发为模拟401） |
| POST | `/api/admin/v1/auth/refresh` | admin 刷新会话 | 真实浏览器与隔离API通过：admin-session-lifecycle.cjs（刷新过期触发为模拟401） |
| GET | `/api/admin/v1/canvas-presets` | 画布推荐尺寸配置 | 真实浏览器与隔离API通过：presets-lifecycle.cjs |
| PUT | `/api/admin/v1/canvas-presets` | 按版本保存画布尺寸配置（数组顺序即展示顺序） | 真实浏览器与隔离API通过：presets-lifecycle.cjs |
| GET | `/api/admin/v1/categories` | 分类列表 | 真实浏览器与隔离DB通过：folder-asset-lifecycle.cjs（2026-11-26） |
| POST | `/api/admin/v1/categories` | 创建或修改分类 | 真实浏览器与隔离DB通过：folder-asset-lifecycle.cjs（2026-11-26） |
| DELETE | `/api/admin/v1/categories/{id}` | 删除未被引用分类 | 真实浏览器与隔离DB通过：folder-asset-lifecycle.cjs（2026-11-26） |
| PUT | `/api/admin/v1/categories/{id}` | 创建或修改分类 | 真实浏览器与隔离DB通过：folder-asset-lifecycle.cjs（2026-11-26） |
| GET | `/api/admin/v1/export-events` | 导出结果及异常分页 | 真实浏览器与隔离API通过：account-operations.cjs |
| GET | `/api/admin/v1/jobs` | 任务分页；管理员仅元信息 | 真实浏览器/API/渲染执行器通过：render-lifecycle.cjs（仅批量导出，不调用AI） |
| GET | `/api/admin/v1/jobs/{id}` | 任务详情；用户含输出素材，管理员不含私有内容 | 真实浏览器/API/渲染执行器通过：render-lifecycle.cjs（仅批量导出，不调用AI） |
| GET | `/api/admin/v1/jobs/{id}/attempts` | 任务执行与重试记录 | 真实浏览器/API/渲染执行器通过：render-lifecycle.cjs（仅批量导出，不调用AI） |
| POST | `/api/admin/v1/jobs/{id}/cancel` | 任务操作：cancel | 待核验/补齐 |
| POST | `/api/admin/v1/jobs/{id}/retry` | 任务操作：retry | 真实浏览器/API/渲染执行器通过：render-lifecycle.cjs（仅批量导出，不调用AI） |
| GET | `/api/admin/v1/me` | 当前账户 | 真实浏览器与隔离API通过：admin-session-lifecycle.cjs（刷新过期触发为模拟401） |
| PATCH | `/api/admin/v1/me` | 修改个人资料 | 真实浏览器与隔离API通过：admin-session-lifecycle.cjs（刷新过期触发为模拟401） |
| PUT | `/api/admin/v1/me/password` | 修改密码并撤销全部会话 | 真实浏览器与隔离API通过：admin-session-lifecycle.cjs（刷新过期触发为模拟401） |
| GET | `/api/admin/v1/permissions` | 全部可分配权限 | 真实浏览器与隔离API通过：security-topic-create.cjs |
| GET | `/api/admin/v1/providers` | 服务商配置；密钥仅返回 hasKey | 真实浏览器与隔离API通过：provider-config-lifecycle.cjs |
| POST | `/api/admin/v1/providers` | 创建服务商配置；apiKey 只写并加密存储 | 真实浏览器与隔离API通过：provider-config-lifecycle.cjs |
| PUT | `/api/admin/v1/providers/{id}` | 按版本更新服务商及启停；空密钥保持原值 | 真实浏览器与隔离API通过：provider-config-lifecycle.cjs |
| GET | `/api/admin/v1/roles` | 角色列表及权限 | 真实浏览器与隔离API通过：security-topic-create.cjs |
| POST | `/api/admin/v1/roles` | 保存角色与权限 | 真实浏览器与隔离API通过：security-topic-create.cjs |
| DELETE | `/api/admin/v1/roles/{id}` | 删除未使用角色 | 真实浏览器与隔离API通过：security-topic-lifecycle.cjs |
| PUT | `/api/admin/v1/roles/{id}` | 保存角色与权限 | 真实浏览器与隔离API通过：security-topic-lifecycle.cjs |
| GET | `/api/admin/v1/statistics` | 全局数量与区间导出统计 | 真实浏览器与隔离API通过：account-operations.cjs |
| GET | `/api/admin/v1/templates` | 模板搜索与分页 | 真实浏览器与隔离API通过：template-lifecycle.cjs |
| POST | `/api/admin/v1/templates` | 创建或更新模板草稿 | 真实浏览器与隔离API通过：template-lifecycle.cjs |
| DELETE | `/api/admin/v1/templates/{id}` | 模板状态操作 delete | 真实浏览器与隔离API通过：template-lifecycle.cjs |
| GET | `/api/admin/v1/templates/{id}` | 模板详情 | 真实浏览器与隔离API通过：template-lifecycle.cjs |
| PUT | `/api/admin/v1/templates/{id}` | 创建或更新模板草稿 | 真实浏览器与隔离API通过：template-lifecycle.cjs |
| POST | `/api/admin/v1/templates/{id}/offline` | 模板状态操作 offline | 真实浏览器与隔离API通过：template-lifecycle.cjs |
| GET | `/api/admin/v1/templates/{id}/preview` | 读取草稿和已发布文档供编辑器预览 | 真实浏览器与隔离API通过：template-lifecycle.cjs |
| POST | `/api/admin/v1/templates/{id}/publish` | 模板状态操作 publish | 真实浏览器与隔离API通过：template-lifecycle.cjs |
| GET | `/api/admin/v1/templates/{id}/revisions` | 模板发布修订列表 | 真实浏览器与隔离API通过：template-lifecycle.cjs |
| GET | `/api/admin/v1/templates/{id}/revisions/{revisionId}` | 不可变模板修订详情 | 真实浏览器与隔离API通过：template-lifecycle.cjs |
| GET | `/api/admin/v1/topics` | 专题列表；slot=home/featured | 真实浏览器与隔离API通过：security-topic-create.cjs |
| POST | `/api/admin/v1/topics` | 创建专题推荐 | 真实浏览器与隔离API通过：security-topic-create.cjs |
| DELETE | `/api/admin/v1/topics/{id}` | 删除已下线专题 | 真实浏览器与隔离API通过：security-topic-lifecycle.cjs |
| GET | `/api/admin/v1/topics/{id}` | 专题详情与排序模板 | 真实浏览器与隔离API通过：security-topic-lifecycle.cjs |
| PUT | `/api/admin/v1/topics/{id}` | 按版本修改专题与上下线 | 真实浏览器与隔离API通过：security-topic-lifecycle.cjs |
| GET | `/api/admin/v1/user/info` | Vben 当前管理员资料 | 真实浏览器与隔离API通过：admin-session-lifecycle.cjs（刷新过期触发为模拟401） |
| GET | `/api/admin/v1/users` | 账户分页列表 | 真实浏览器与隔离DB通过：user-workspace-lifecycle.cjs（2026-11-26） |
| POST | `/api/admin/v1/users` | 管理员创建内部用户 | 真实浏览器与隔离API通过：user-create-quota.cjs |
| GET | `/api/admin/v1/users/{id}` | 账户详情（管理员包含 roleIds） | 真实浏览器与隔离DB通过：user-workspace-lifecycle.cjs（2026-11-26） |
| PATCH | `/api/admin/v1/users/{id}` | 编辑内部用户账号和姓名 | 真实浏览器与隔离DB通过：user-workspace-lifecycle.cjs（2026-11-26） |
| PUT | `/api/admin/v1/users/{id}/password` | 重置内部用户密码并撤销全部会话 | 真实浏览器与隔离DB通过：user-workspace-lifecycle.cjs（2026-11-26） |
| PATCH | `/api/admin/v1/users/{id}/status` | 启停账户并撤销会话 | 真实浏览器与隔离DB通过：user-workspace-lifecycle.cjs（2026-11-26） |
| GET | `/api/admin/v1/users/{id}/task-quota` | 用户任务额度 | 真实浏览器与隔离API通过：user-create-quota.cjs |
| PUT | `/api/admin/v1/users/{id}/task-quota` | 分配累计任务额度；不含会员支付 | 真实浏览器与隔离API通过：user-create-quota.cjs |
| GET | `/api/v1/assets` | 素材分页；scope=mine/public，kind=image/font | 待核验/补齐 |
| POST | `/api/v1/assets` | 上传图片或管理员公共字体 | 待核验/补齐 |
| DELETE | `/api/v1/assets/{id}` | 删除未引用素材 | 待核验/补齐 |
| GET | `/api/v1/assets/{id}` | 素材元数据 | 待核验/补齐 |
| POST | `/api/v1/assets/{id}/access-url` | 获取 60 秒文件访问链接 | 待核验/补齐 |
| GET | `/api/v1/assets/{id}/content` | 文件内容；公共资源匿名，私有资源凭 Bearer 或 ticket | 待核验/补齐 |
| POST | `/api/v1/auth/login` | user 登录 | 真实浏览器与隔离DB通过：user-workspace-lifecycle.cjs（2026-11-26） |
| POST | `/api/v1/auth/logout` | user 退出会话 | 真实浏览器与隔离API通过：user-session-lifecycle.cjs |
| POST | `/api/v1/auth/refresh` | user 刷新会话 | 真实浏览器与隔离API通过：user-session-lifecycle.cjs |
| POST | `/api/v1/auth/register` | 用户注册 | 按内部使用要求不提供入口 |
| GET | `/api/v1/canvas-presets` | 启用的推荐画布尺寸 | 待核验/补齐 |
| GET | `/api/v1/categories` | 分类列表 | 待核验/补齐 |
| GET | `/api/v1/designs` | 作品或回收站分页 | 真实浏览器与隔离DB通过：user-workspace-lifecycle.cjs（2026-11-26） |
| POST | `/api/v1/designs` | 创建空白作品；必填 Idempotency-Key | 待核验/补齐 |
| DELETE | `/api/v1/designs/{id}` | 作品回收站操作：delete | 真实浏览器与隔离DB通过：user-workspace-lifecycle.cjs（2026-11-26） |
| GET | `/api/v1/designs/{id}` | 作品文档和版本 | 待核验/补齐 |
| PATCH | `/api/v1/designs/{id}` | 按版本重命名 | 真实浏览器与隔离DB通过：user-workspace-lifecycle.cjs（2026-11-26） |
| POST | `/api/v1/designs/{id}/copy` | 复制或套用为独立作品；必填 Idempotency-Key | 真实浏览器与隔离API通过：workspace-editor-lifecycle.cjs |
| POST | `/api/v1/designs/{id}/cover` | 按版本替换上传成功的封面 | 真实浏览器与隔离API通过：workspace-editor-lifecycle.cjs |
| PUT | `/api/v1/designs/{id}/document` | 按版本保存完整文档；冲突 409 | 真实浏览器与隔离API通过：workspace-editor-lifecycle.cjs |
| POST | `/api/v1/designs/{id}/export-check` | 校验导出版本、资源和像素 | 真实浏览器与隔离API通过：workspace-editor-lifecycle.cjs |
| PUT | `/api/v1/designs/{id}/folder` | 移动作品；folderId 空字符串表示移出 | 真实浏览器与隔离API通过：workspace-editor-lifecycle.cjs |
| GET | `/api/v1/designs/{id}/history` | 最近 20 次文档历史 | 真实浏览器与隔离API通过：workspace-editor-lifecycle.cjs |
| GET | `/api/v1/designs/{id}/history/{historyId}` | 历史文档详情 | 真实浏览器与隔离API通过：workspace-editor-lifecycle.cjs |
| POST | `/api/v1/designs/{id}/history/{historyId}/restore` | 历史内容恢复为新版本 | 真实浏览器与隔离API通过：workspace-editor-lifecycle.cjs |
| POST | `/api/v1/designs/{id}/restore` | 作品回收站操作：restore | 真实浏览器与隔离DB通过：user-workspace-lifecycle.cjs（2026-11-26） |
| POST | `/api/v1/export-events` | 幂等记录浏览器导出结果 | 真实浏览器与隔离API通过：workspace-editor-lifecycle.cjs |
| GET | `/api/v1/favorites` | 当前仍可见的收藏模板 | 待核验/补齐 |
| DELETE | `/api/v1/favorites/{id}` | 收藏或取消收藏模板 | 真实浏览器与隔离API通过：template-lifecycle.cjs |
| PUT | `/api/v1/favorites/{id}` | 收藏或取消收藏模板 | 真实浏览器与隔离API通过：template-lifecycle.cjs |
| GET | `/api/v1/folders` | 我的文件夹（最多 200 个） | 真实浏览器与隔离DB通过：folder-asset-lifecycle.cjs（2026-11-26） |
| POST | `/api/v1/folders` | 创建文件夹 | 真实浏览器与隔离DB通过：folder-asset-lifecycle.cjs（2026-11-26） |
| DELETE | `/api/v1/folders/{id}` | 删除文件夹，保留作品 | 真实浏览器与隔离DB通过：folder-asset-lifecycle.cjs（2026-11-26） |
| PUT | `/api/v1/folders/{id}` | 按版本修改文件夹 | 真实浏览器与隔离DB通过：folder-asset-lifecycle.cjs（2026-11-26） |
| GET | `/api/v1/folders/{id}/designs` | 文件夹作品分页 | 真实浏览器与隔离DB通过：folder-asset-lifecycle.cjs（2026-11-26） |
| GET | `/api/v1/fonts` | 可用公共字体 | EditorResources.vue；design-editor-resources.cjs 真实列表/应用/保存重开通过 |
| GET | `/api/v1/home` | 首页分类与最新模板 | 待核验/补齐 |
| GET | `/api/v1/job-capabilities` | 已启用服务商能力（无密钥与地址） | 真实浏览器/API/渲染执行器通过：render-lifecycle.cjs（仅批量导出，不调用AI） |
| GET | `/api/v1/jobs` | 任务分页；管理员仅元信息 | 真实浏览器/API/渲染执行器通过：render-lifecycle.cjs（仅批量导出，不调用AI） |
| POST | `/api/v1/jobs` | 创建 AI 图片或批量导出任务；需要幂等键 | 真实浏览器/API/渲染执行器通过：render-lifecycle.cjs（仅批量导出，不调用AI） |
| DELETE | `/api/v1/jobs/{id}` | 删除已结束任务并释放快照；输出保留在素材库 | 真实浏览器/API/渲染执行器通过：render-lifecycle.cjs（仅批量导出，不调用AI） |
| GET | `/api/v1/jobs/{id}` | 任务详情；用户含输出素材，管理员不含私有内容 | 真实浏览器/API/渲染执行器通过：render-lifecycle.cjs（仅批量导出，不调用AI） |
| POST | `/api/v1/jobs/{id}/cancel` | 任务操作：cancel | 待核验/补齐 |
| GET | `/api/v1/jobs/{id}/download` | 下载成功批量任务的 ZIP 文件 | 真实浏览器/API/渲染执行器通过：render-lifecycle.cjs（仅批量导出，不调用AI） |
| POST | `/api/v1/jobs/{id}/retry` | 任务操作：retry | 真实浏览器/API/渲染执行器通过：render-lifecycle.cjs（仅批量导出，不调用AI） |
| GET | `/api/v1/me` | 当前账户 | 真实浏览器与隔离API通过：account-operations.cjs |
| PATCH | `/api/v1/me` | 修改个人资料 | 真实浏览器与隔离API通过：account-operations.cjs |
| PUT | `/api/v1/me/password` | 修改密码并撤销全部会话 | 真实浏览器与隔离API通过：account-operations.cjs |
| GET | `/api/v1/seed-assets/poster.svg` | 自制示例模板封面 | 待核验/补齐 |
| GET | `/api/v1/storage` | 我的存储额度 | 真实浏览器与隔离API通过：user-create-quota.cjs |
| GET | `/api/v1/task-quota` | 我的内部任务额度 | 真实浏览器与隔离API通过：user-create-quota.cjs |
| GET | `/api/v1/templates` | 模板搜索与分页 | 真实浏览器与隔离API通过：template-lifecycle.cjs |
| GET | `/api/v1/templates/{id}` | 模板详情 | 真实浏览器与隔离API通过：template-lifecycle.cjs |
| POST | `/api/v1/templates/{id}/use` | 复制或套用为独立作品；必填 Idempotency-Key | 真实浏览器与隔离API通过：template-lifecycle.cjs |
| GET | `/api/v1/topics` | 专题列表；slot=home/featured | 真实浏览器与隔离API通过：security-topic-create.cjs |
| GET | `/api/v1/topics/{id}` | 专题详情与排序模板 | 真实浏览器与隔离API通过：security-topic-create.cjs |
| GET | `/api/v1/trash` | 作品或回收站分页 | 真实浏览器与隔离DB通过：user-workspace-lifecycle.cjs（2026-11-26） |
| DELETE | `/api/v1/trash/{id}` | 作品回收站操作：purge | 真实浏览器与隔离DB通过：user-workspace-lifecycle.cjs（2026-11-26） |
| GET | `/healthz` | 进程存活 | 基础设施接口，纳入运行检查 |
| GET | `/readyz` | 数据库和存储就绪 | 基础设施接口，纳入运行检查 |

## 当前已实施与证据（第一批，仍未完成总体目标）

- 前台 `useTemplateCatalog.ts` + `LiveCatalog.vue` 替换首页静态模板；调用 home/categories/templates/topics detail、template detail/use、favorite PUT/DELETE。保留真实空态与错误重试，不再展示模拟可套用模板。
- 后台 `/design-categories` 分类增改删、启停与排序。
- 后台 `/design-templates` 草稿编辑、封面上传、简单文字文档建立/导入、发布下架删除、Fabric 预览、修订列表与修订预览。
- 后台 `/design-assets` 公共图片/字体上传、列表详情、签名链接预览、引用列表、启停和删除。
- 独立 MySQL 13307 + API 18888 + 前台代理18889/后台代理18890，未改真实 design 数据库。
- 浏览器测试 `C:/Users/A/AppData/Local/Temp/design-catalog-ui.cjs` 实测新建分类、公共封面上传（已修复 multipart 头）、草稿保存发布、预览修订入口、前台搜索登录套用、收藏/取消收藏通过。
- 尚需补全：模板任意公共图片/字体的可视化编辑与预览；专题后台；公共素材行为完整回归。第一批测试不是全目标完成证据。
- 下一批：作品复制/文件夹/历史/封面/导出事件，账户资料/密码/存储；管理员角色权限与审计统计；任务管理/额度。

测试运行状态文件：`data/design-runtime/catalog-ui-test.json`。测试数据库与服务仅供本目标后续集成验证，结束时清理。代理工具会话26159（每次继续需先确认进程和端口是否仍活跃）。

## 第二批：个人作品与运营记录

- `WorkspaceFolders.vue` 文件夹列表/新建/更名/删除，作品按文件夹分页；`MyWorkspace.vue` 复制、移动、从上传图片或收藏模板创建作品。
- `DesignHistory.vue` 历史列表/文档详情/下载历史JSON/恢复为新版本；编辑器恢复后同步当前版本与撤销栈。
- `useDesignEditor.ts` 更新封面（渲染缩略图、上传、版本校验）；导出自动先保存未保存编辑，再 export-check，浏览器渲染后上报 export-events；失败明确提示，不伪报成功。
- `/account` 修改姓名、头像上传/清除、修改密码、存储/任务额度；改密成功清除本地会话并要求重新登录。
- 后台 `/design-statistics` 全量指标及区间/每日导出；`/design-exports` 结果分页/筛选；`/design-audit` 审计分页。
- 实测脚本 `C:/Users/A/AppData/Local/Temp/design-workspace-extra.cjs` 已通过：文件夹创建、作品移动与复制、历史详情与恢复、封面上传、导出校验与事件（真实隔离数据库）。
- `C:/Users/A/AppData/Local/Temp/design-account-operations.cjs` 已通过：资料更新/刷新、改密撤销/重新登录、额度读取、后台统计/导出/审计真实接口页面。
- 下一批仍需：管理员角色权限、专题运营、任务管理/用户额度编辑；公共素材与自定义字体实际编辑；已有页面完整动作回归、清理旧演示入口。
- 未进行总体完成验收，目标保持进行中。历史文档目前提供图层信息和JSON下载，后续补视觉预览；账户头像待完善显示预览。



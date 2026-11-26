// @Title doc.go
// @Description 用户中心 Model 、数据库事务和缓存的阅读说明。
// @Author Codex ，教学注释作者。
// @Update Codex 2026-10-16 ，新增数据访问说明。
//
// model 包连接 RPC 业务逻辑与 MySQL 、 Redis 。先读 userModel.go 与 userAuthModel.go ，
// 再按以下方法名查阅同名 _gen.go 文件。生成文件保留原样，避免注释被重新生成覆盖。
//
// User 对应 user 表： Id 为主键， Mobile 为手机号， Password 为当前实现的密码摘要。
// Nickname 、 Sex 、 Avatar 、 Info 为展示字段； CreateTime 、 UpdateTime 为审计时间。
// DeleteTime 与 DelState 配合软删除， Version 供版本控制方法使用。
// UserAuth 对应 user_auth 表： UserId 关联本地用户， AuthType 区分登录平台。
// AuthKey 在 system 类型下是手机号，在 wxMini 类型下是 OpenID 。
//
// TODO NewUserModel 与 NewUserAuthModel 包装生成的默认实现，内部使用 sqlc.CachedConn 。
// TODO RegisterLogic 先调用 FindOneByMobile ，检查用户是否存在。
// TODO FindOneByMobile 使用手机号索引缓存，再通过主键定位用户；未命中时查库。
// TODO GetUserInfoLogic 使用 FindOne ，按主键读取缓存或查询尚未软删除的数据库记录。
// TODO 授权查询使用 AuthType/AuthKey 或 UserId/AuthType 组合索引定位记录。
// TODO 查询不到时返回 ErrNotFound ，调用方必须与连接失败等其他错误区分。
//
// TODO Trans 把回调交给数据库事务实现，回调返回 nil 提交，返回错误回滚。
// TODO RegisterLogic 将同一个 sqlx.Session 传给两张表的 Insert ，保证本地事务边界一致。
// TODO Insert 的 session 非 nil 时走事务连接，为 nil 时使用普通连接。
// TODO Insert 返回 sql.Result ， LastInsertId 取得本次数据库生成的用户主键。
//
// 写操作通过 ExecCtx 处理相关缓存键；自定义 SQL 不能假设会自动维护所有缓存。
// 事务保证数据库中的两张表原子提交，不代表 Redis 缓存也参与同一个数据库事务。
// 更新、软删除、物理删除、分页与版本检查的具体行为，应逐个阅读生成方法。
//
// 自定义业务查询应扩展 UserModel 或 UserAuthModel ，并实现在 custom...Model 上。
// 不要把数据库原始 User 直接作为 HTTP 响应，否则可能把 Password 等字段一起暴露。
package model

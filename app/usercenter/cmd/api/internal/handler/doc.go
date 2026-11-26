// @Title doc.go
// @Description 解释生成路由的分组、鉴权和调用顺序。
// @Author Codex ，教学注释作者。
// @Update Codex 2026-10-16 ，新增路由说明。
//
// handler 包把 go-zero HTTP 服务与具体请求处理器连接起来。
// routes.go 由 desc/usercenter.api 生成；路由变更应先修改 .api ，再生成代码。
//
// TODO RegisterHandlers 在进程启动时执行，并调用 server.AddRoutes 绑定两组路由。
// TODO 第一组包含 POST /user/register 和 POST /user/login ，不要求 JWT 。
// TODO 第二组包含 POST /user/detail 和 POST /user/wxMiniAuth ，使用 rest.WithJwt 。
// TODO 两组均通过 rest.WithPrefix 增加 /usercenter/v1 路径前缀。
// TODO 真实请求匹配路由后，框架先执行中间件，再执行对应 handler 返回的闭包。
//
// JWT 校验失败时，请求不会进入 DetailLogic ，响应也不一定经过 pkg/result 。
// 用户标识由框架将令牌声明写入请求上下文，再由 pkg/ctxdata.GetUidFromCtx 读取。
// 当前没有额外的 identity 服务，也没有注册 SetUidToCtxMiddleware 字段。
package handler

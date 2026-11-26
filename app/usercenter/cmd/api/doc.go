// @Title doc.go
// @Description 用户中心 HTTP 调用链与源码阅读入口。
// @Author Codex ，教学注释作者。
// @Update Codex 2026-10-16 ，新增调用链说明。
//
// 用户中心分为两个进程：本包启动 HTTP API ， cmd/rpc 启动业务 RPC 。
// 初学者可以按以下顺序设置断点，观察一次注册请求如何到达数据库：
//
// TODO 1. usercenter.go 加载 YAML ，创建 svc.ServiceContext 并注册路由。
// TODO 2. internal/handler/routes.go 将请求分发给 user.RegisterHandler 。
// TODO 3. handler 解析 JSON ，调用 API 的 RegisterLogic.Register 。
// TODO 4. API logic 调用 svcCtx.UsercenterRpc.Register ，通过 gRPC 进入另一进程。
// TODO 5. RPC 的 internal/server 将请求交给 RPC 的 RegisterLogic.Register 。
// TODO 6. RPC logic 查询手机号，在本地事务中写入 user 与 user_auth 。
// TODO 7. RPC logic 直接复用 GenerateTokenLogic 签发令牌，再逐层返回。
// TODO 8. API handler 调用 pkg/result.HttpResult ，输出 code 、 msg 、 data 。
//
// 登录沿用相同入口层次，在 RPC 校验密码后签发令牌，不再创建用户。
// 详情接口先由 JWT 中间件验证令牌，再由 API logic 读取 jwtUserId 并调用 GetUserInfo 。
// 微信授权先调用微信接口，再按授权关系选择注册或签发令牌；当前路由也要求 JWT 。
//
// 注释格式参考 https://learnku.com/articles/41344 。
// 本次使用 TODO 作为该规范要求的流程讲解前缀，不表示每段代码都尚未实现。
// @Author 与 @auth 只说明本次注释的作者，不替原项目声明代码著作权。
// 生成文件的讲解放在同包 doc.go 与 .api 、.proto 源定义中，避免重新生成时丢失。
package main

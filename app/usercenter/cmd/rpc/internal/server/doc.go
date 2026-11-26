// @Title doc.go
// @Description 解释生成的 RPC 服务适配器如何分发业务请求。
// @Author Codex ，教学注释作者。
// @Update Codex 2026-10-16 ，新增 RPC 服务端说明。
//
// server 包实现 protobuf 声明的服务接口，生成代码位于 usercenterServer.go 。
// UsercenterServer 的 svcCtx 保存 RPC 进程共享的数据库与缓存依赖。
// 嵌入的 pb.UnimplementedUsercenterServer 提供未实现方法的兼容行为。
// NewUsercenterServer 在启动时接收依赖，真正的业务逻辑对象在每次 RPC 调用时创建。
//
// TODO gRPC 根据方法名把网络请求分发给 Login 、 Register 或其他服务方法。
// TODO 每个方法接收 ctx 与 protobuf 请求 in ，并创建对应的 New...Logic 。
// TODO 将同一个 ctx 传给 logic ，继续传播请求取消、超时和链路信息。
// TODO 调用 logic 方法，把 protobuf 响应及 error 原样交回 gRPC 框架。
// TODO 已注册的 LoggerInterceptor 处理返回错误，再通过网络传回 API 客户端。
//
// 例如 Register 对应 NewRegisterLogic 与 Register ； GetUserInfo 对应用户信息查询。
// 此层只做转发，不负责 HTTP 参数解析、 JSON 包装或数据库事务。
package server

// @Title doc.go
// @Description 解释其他服务调用用户中心时使用的 RPC 客户端封装。
// @Author Codex ，教学注释作者。
// @Update Codex 2026-10-16 ，新增 RPC 客户端说明。
//
// usercenter 包是 API 与其他业务服务导入的客户端入口，不是 RPC 服务端实现。
// usercenter.go 由 goctl 生成，消息类型通过类型别名指向 pb 包。
// Usercenter 接口定义六个 RPC 方法； defaultUsercenter 持有 zrpc.Client 连接封装。
//
// TODO API 的 svc.NewServiceContext 先按 Endpoints 创建 zrpc.Client 。
// TODO NewUsercenter 将连接包装为 Usercenter 接口，不会在此时执行注册或登录。
// TODO 每次调用 Register 等方法时，从已有连接创建 pb.NewUsercenterClient 代理。
// TODO 代理序列化请求并发起网络调用，服务端执行后返回消息或 gRPC 错误。
//
// ctx 控制本次调用的取消和截止时间； opts 是可选的 gRPC 调用参数。
// 本地方法调用看起来像普通 Go 函数，但可能因为连接、超时或远端业务错误而失败。
// 使用本包的客户端，避免跨服务导入 cmd/rpc/internal/logic 绕过服务边界。
package usercenter

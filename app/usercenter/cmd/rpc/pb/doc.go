// @Title doc.go
// @Description 解释 protobuf 协议和生成代码的职责。
// @Author Codex ，教学注释作者。
// @Update Codex 2026-10-16 ，新增协议阅读说明。
//
// pb 包定义 API 与 RPC 之间传输的用户消息和服务协议。
// usercenter.proto 是维护入口；字段含义和服务说明写在该源文件中。
// usercenter.pb.go 负责消息类型、字段读取和序列化描述。
// usercenter_grpc.pb.go 负责客户端调用、服务端接口和 gRPC 方法注册。
//
// TODO 从 usercenter.proto 找到请求、响应和方法名称，再进入 internal/logic 阅读业务。
// TODO 新增协议字段时使用新的字段编号，不随意改动已有编号，保持通信兼容。
// TODO 修改协议后重新生成代码，不手工修改 .pb.go 中的描述字节和框架实现。
//
// GenerateTokenReq 只携带 UserId ，验证登录凭据由上游业务负责。
// GetUserAuthyUserIdResp 的拼写是现有协议名称，学习注释不改变协议标识。
package pb

// @Title doc.go
// @Description 解释 HTTP 类型的来源以及与 RPC 消息的区别。
// @Author Codex ，教学注释作者。
// @Update Codex 2026-10-16 ，新增类型说明。
//
// types 包由 desc/user/user.api 生成，字段注释维护在该源定义中。
// RegisterReq 与 LoginReq 使用 mobile 和 password ；业务校验不等于 JSON 解析。
// UserInfoReq 为空，当前用户编号由 JWT 提供，不接受客户端指定查询对象。
// User 只暴露公开信息，没有 Password 字段。
//
// TODO handler 通过 httpx.Parse 构造请求类型，并将值传给 API logic 。
// TODO API logic 显式构造 protobuf 请求，调用 RPC 后再转换为这里的响应类型。
// TODO handler 最后用 pkg/result 增加 code 、 msg 、 data 包装。
//
// AccessToken 是签名后的 JWT ； AccessExpire 、 RefreshAfter 都是 Unix 秒时间戳。
// 配置中的 JwtAuth.AccessExpire 则是有效时长，二者含义不同。
// pb.User 与 types.User 是不同类型，字段相同也不能直接当作同一个结构体使用。
package types

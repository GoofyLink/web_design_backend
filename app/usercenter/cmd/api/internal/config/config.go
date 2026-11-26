// @Title config.go
// @Description 声明 API 配置结构，字段与启动时读取的 YAML 对应。
// @Author 原代码作者未标注；本次教学注释由 Codex 编写。
// @Update Codex 2026-10-16 ，仅补充注释。
package config

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

// Config 映射服务启动配置，嵌入字段来自 go-zero 的基础配置。
type Config struct {
	rest.RestConf          // 嵌入 HTTP 服务配置，包含监听地址、日志和监控等。
	JwtAuth       struct { // JWT 配置；签发端与校验端的签名密钥必须一致。
		AccessSecret string // JWT 签名密钥，不是用户密码。
	}
	WxMiniConf        WxMiniConf         // 微信小程序配置，仅在调用微信授权业务时使用。
	UsercenterRpcConf zrpc.RpcClientConf // 用户中心 RPC 连接配置，由 API 创建客户端时读取。
}

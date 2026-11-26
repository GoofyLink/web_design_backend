// @Title config.go
// @Description 声明 RPC 配置结构，字段与启动时读取的 YAML 对应。
// @Author 原代码作者未标注；本次教学注释由 Codex 编写。
// @Update Codex 2026-10-16 ，仅补充注释。
package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

// Config 映射服务启动配置，嵌入字段来自 go-zero 的基础配置。
type Config struct {
	zrpc.RpcServerConf          // 嵌入 RPC 服务配置，也提供当前代码使用的 Redis 配置。
	JwtAuth            struct { // JWT 配置；签发端与校验端的签名密钥必须一致。
		AccessSecret string // JWT 签名密钥，不是用户密码。
		AccessExpire int64  // 令牌有效时长，单位为秒，不是过期时间戳。
	}
	DB struct { // 用户中心数据库连接配置。
		DataSource string // MySQL 连接串，包含地址、账号和库名。
	}
	Cache cache.CacheConf // Model 使用的 Redis 缓存配置，与显式 Redis 客户端分别初始化。
}

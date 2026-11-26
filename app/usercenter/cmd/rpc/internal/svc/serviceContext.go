// @Title serviceContext.go
// @Description 组装 RPC 进程共享依赖，供请求处理逻辑复用。
// @Author 原代码作者未标注；本次教学注释由 Codex 编写。
// @Update Codex 2026-10-16 ，仅补充注释。
package svc

import (
	"looklook/app/usercenter/cmd/rpc/internal/config"
	"looklook/app/usercenter/model"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceContext 保存进程级依赖；多个请求共享客户端，但各自持有请求上下文。
type ServiceContext struct {
	Config      config.Config // 启动时读取的配置副本，供业务逻辑访问。
	RedisClient *redis.Redis  // 显式 Redis 客户端； JWT 签发逻辑当前没有用它保存令牌。

	UserModel     model.UserModel     // 访问 user 表的接口，包含查询、写入和本地事务方法。
	UserAuthModel model.UserAuthModel // 访问 user_auth 表的接口，维护用户与登录平台的关联。
}

// NewServiceContext 在 RPC 启动时创建共享依赖，返回值传给所有请求的逻辑对象。
// @title NewServiceContext
// @description 在 RPC 启动时创建共享依赖，返回值传给所有请求的逻辑对象。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param c config.Config "已加载的配置。"
// @return result *ServiceContext "初始化后的依赖对象或逻辑对象。"
func NewServiceContext(c config.Config) *ServiceContext {

	// TODO DB.DataSource 决定连接哪个 MySQL 库；两个 Model 复用同一个连接对象。
	sqlConn := sqlx.NewMysql(c.DB.DataSource)

	// TODO 分别初始化显式 Redis 客户端和 Model 缓存；两者读取不同配置字段。
	// TODO 缓存连接通过 c.Cache 交给 Model ，不是复用下方的 RedisClient 字段。
	return &ServiceContext{
		Config: c,
		RedisClient: redis.New(c.Redis.Host, func(r *redis.Redis) {
			r.Type = c.Redis.Type
			r.Pass = c.Redis.Pass
		}),

		UserAuthModel: model.NewUserAuthModel(sqlConn, c.Cache),
		UserModel:     model.NewUserModel(sqlConn, c.Cache),
	}
}

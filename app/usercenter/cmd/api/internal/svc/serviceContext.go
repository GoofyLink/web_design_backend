// @Title serviceContext.go
// @Description 组装 API 进程共享依赖，供请求处理逻辑复用。
// @Author 原代码作者未标注；本次教学注释由 Codex 编写。
// @Update Codex 2026-10-16 ，仅补充注释。
package svc

import (
	"looklook/app/usercenter/cmd/api/internal/config"
	"looklook/app/usercenter/cmd/rpc/usercenter"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

// ServiceContext 保存进程级依赖；多个请求共享客户端，但各自持有请求上下文。
type ServiceContext struct {
	Config        config.Config         // 启动时读取的配置副本，供业务逻辑访问。
	UsercenterRpc usercenter.Usercenter // 可复用的用户中心 RPC 客户端封装。

	SetUidToCtxMiddleware rest.Middleware // 预留中间件字段；当前未初始化，也未注册到路由。
}

// NewServiceContext 在 API 启动时创建共享依赖，返回值传给所有请求的逻辑对象。
// @title NewServiceContext
// @description 在 API 启动时创建共享依赖，返回值传给所有请求的逻辑对象。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param c config.Config "已加载的配置。"
// @return result *ServiceContext "初始化后的依赖对象或逻辑对象。"
func NewServiceContext(c config.Config) *ServiceContext {

	// TODO 按 UsercenterRpcConf 创建连接封装； NewUsercenter 暴露 Register 、 Login 等方法。
	// TODO NonBlock 为 true 时，创建成功并不证明 RPC 此刻可达。
	return &ServiceContext{
		Config:        c,
		UsercenterRpc: usercenter.NewUsercenter(zrpc.MustNewClient(c.UsercenterRpcConf)),
	}
}

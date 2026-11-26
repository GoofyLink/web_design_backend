// @Title loginLogic.go
// @Description 用户中心 API 业务逻辑，说明输入来源、下游调用和返回过程。
// @Author 原代码作者未标注；本次教学注释由 Codex 编写。
// @Update Codex 2026-10-16 ，仅补充注释。
package user

import (
	"context"

	"looklook/app/usercenter/cmd/api/internal/svc"
	"looklook/app/usercenter/cmd/api/internal/types"
	"looklook/app/usercenter/cmd/rpc/usercenter"
	"looklook/app/usercenter/model"

	"github.com/jinzhu/copier"
	"github.com/zeromicro/go-zero/core/logx"
)

// LoginLogic 保存当前请求上下文、共享依赖和日志能力。
type LoginLogic struct {
	logx.Logger                     // 关联请求上下文的日志记录器。
	ctx         context.Context     // 当前请求的上下文，传递取消、超时和链路信息。
	svcCtx      *svc.ServiceContext // 进程共享的依赖，不是当前登录用户的数据。
}

// NewLoginLogic 创建当前请求的逻辑对象，复用服务依赖，并让日志携带请求上下文。
// @title NewLoginLogic
// @description 创建当前请求的逻辑对象，复用服务依赖，并让日志携带请求上下文。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param ctx context.Context "调用方传入的请求上下文。"
// @param svcCtx *svc.ServiceContext "启动时组装的共享依赖。"
// @return result LoginLogic "初始化后的依赖对象或逻辑对象。"
func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) LoginLogic {
	return LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Login 验证登录凭据并返回令牌。
// @title Login
// @description 验证登录凭据并返回令牌。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param req types.LoginReq "handler 解析的 HTTP 请求参数。"
// @return resp *types.LoginResp "成功结果；业务失败时通常为 nil 。"
// @return err error "失败原因；为 nil 表示本方法成功。"
func (l *LoginLogic) Login(req types.LoginReq) (*types.LoginResp, error) {
	// TODO 将 HTTP 参数转换成 protobuf 请求； system 类型以手机号作为 AuthKey 。
	// TODO 客户端通过 gRPC 调用 RPC server ，再进入同名 RPC logic ；这里不直接访问数据库。
	loginResp, err := l.svcCtx.UsercenterRpc.Login(l.ctx, &usercenter.LoginReq{
		AuthType: model.UserAuthTypeSystem,
		AuthKey:  req.Mobile,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}

	// TODO 按同名字段把 RPC 消息复制成 API 响应； HTTP 响应包装由 handler 完成。
	// TODO 当前代码忽略 copier.Copy 的错误，扩展字段类型时需要重新核对映射。
	var resp types.LoginResp
	_ = copier.Copy(&resp, loginResp)

	return &resp, nil
}

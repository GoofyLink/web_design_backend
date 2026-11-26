// @Title detailLogic.go
// @Description 用户中心 API 业务逻辑，说明输入来源、下游调用和返回过程。
// @Author 原代码作者未标注；本次教学注释由 Codex 编写。
// @Update Codex 2026-10-16 ，仅补充注释。
package user

import (
	"context"

	"looklook/app/usercenter/cmd/api/internal/svc"
	"looklook/app/usercenter/cmd/api/internal/types"
	"looklook/app/usercenter/cmd/rpc/usercenter"
	"looklook/pkg/ctxdata"

	"github.com/jinzhu/copier"
	"github.com/zeromicro/go-zero/core/logx"
)

// DetailLogic 保存当前请求上下文、共享依赖和日志能力。
type DetailLogic struct {
	logx.Logger                     // 关联请求上下文的日志记录器。
	ctx         context.Context     // 当前请求的上下文，传递取消、超时和链路信息。
	svcCtx      *svc.ServiceContext // 进程共享的依赖，不是当前登录用户的数据。
}

// NewDetailLogic 创建当前请求的逻辑对象，复用服务依赖，并让日志携带请求上下文。
// @title NewDetailLogic
// @description 创建当前请求的逻辑对象，复用服务依赖，并让日志携带请求上下文。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param ctx context.Context "调用方传入的请求上下文。"
// @param svcCtx *svc.ServiceContext "启动时组装的共享依赖。"
// @return result DetailLogic "初始化后的依赖对象或逻辑对象。"
func NewDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) DetailLogic {
	return DetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Detail 读取当前已登录用户的信息。
// @title Detail
// @description 读取当前已登录用户的信息。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param req types.UserInfoReq "handler 解析的 HTTP 请求参数。"
// @return resp *types.UserInfoResp "成功结果；业务失败时通常为 nil 。"
// @return err error "失败原因；为 nil 表示本方法成功。"
func (l *DetailLogic) Detail(req types.UserInfoReq) (*types.UserInfoResp, error) {

	// TODO 详情路由先由 rest.WithJwt 验证令牌，再从上下文读取 jwtUserId 。
	// TODO 这里没有读取客户端传入的用户编号； UserInfoReq 是空请求类型。
	userId := ctxdata.GetUidFromCtx(l.ctx)

	// TODO 把身份转换为显式 RPC 参数； HTTP 上下文中的用户值不会自动成为 RPC 请求字段。
	userInfoResp, err := l.svcCtx.UsercenterRpc.GetUserInfo(l.ctx, &usercenter.GetUserInfoReq{
		Id: userId,
	})
	if err != nil {
		return nil, err
	}

	// TODO 从 RPC 的公开用户信息复制到 API 类型，不向前端返回数据库密码字段。
	var userInfo types.User
	_ = copier.Copy(&userInfo, userInfoResp.User)

	return &types.UserInfoResp{
		UserInfo: userInfo,
	}, nil
}

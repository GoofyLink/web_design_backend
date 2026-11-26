// @Title getUserAuthByUserIdLogic.go
// @Description 用户中心 RPC 业务逻辑，说明输入来源、下游调用和返回过程。
// @Author 原代码作者未标注；本次教学注释由 Codex 编写。
// @Update Codex 2026-10-16 ，仅补充注释。
package logic

import (
	"context"

	"looklook/app/usercenter/cmd/rpc/internal/svc"
	"looklook/app/usercenter/cmd/rpc/pb"
	"looklook/app/usercenter/cmd/rpc/usercenter"
	"looklook/app/usercenter/model"
	"looklook/pkg/xerr"

	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
)

// GetUserAuthByUserIdLogic 保存当前请求上下文、共享依赖和日志能力。
type GetUserAuthByUserIdLogic struct {
	ctx         context.Context     // 当前请求的上下文，传递取消、超时和链路信息。
	svcCtx      *svc.ServiceContext // 进程共享的依赖，不是当前登录用户的数据。
	logx.Logger                     // 关联请求上下文的日志记录器。
}

// NewGetUserAuthByUserIdLogic 创建当前请求的逻辑对象，复用服务依赖，并让日志携带请求上下文。
// @title NewGetUserAuthByUserIdLogic
// @description 创建当前请求的逻辑对象，复用服务依赖，并让日志携带请求上下文。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param ctx context.Context "调用方传入的请求上下文。"
// @param svcCtx *svc.ServiceContext "启动时组装的共享依赖。"
// @return result *GetUserAuthByUserIdLogic "初始化后的依赖对象或逻辑对象。"
func NewGetUserAuthByUserIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserAuthByUserIdLogic {
	return &GetUserAuthByUserIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetUserAuthByUserId 根据本地用户与平台类型查询授权关系。
// @title GetUserAuthByUserId
// @description 根据本地用户与平台类型查询授权关系。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param in *pb.GetUserAuthByUserIdReq "RPC 请求消息，字段含义参见 pb/usercenter.proto 。"
// @return resp *pb.GetUserAuthyUserIdResp "成功结果；业务失败时通常为 nil 。"
// @return err error "失败原因；为 nil 表示本方法成功。"
func (l *GetUserAuthByUserIdLogic) GetUserAuthByUserId(in *pb.GetUserAuthByUserIdReq) (*pb.GetUserAuthyUserIdResp, error) {

	// TODO 通过 Model 查询单条记录；缓存未命中时由生成的实现读取 MySQL 。
	userAuth, err := l.svcCtx.UserAuthModel.FindOneByUserIdAuthType(l.ctx, in.UserId, in.AuthType)
	if err != nil && err != model.ErrNotFound {
		return nil, errors.Wrapf(xerr.NewErrMsg("get user auth  fail"), "err : %v , in : %+v", err, in)
	}

	// TODO 没有授权记录时仍返回零值 UserAuth ，调用方通过 Id 为零判断未关联。
	// TODO 当前代码忽略复制错误，不会把 ErrNotFound 作为 RPC 错误返回。
	var respUserAuth usercenter.UserAuth
	_ = copier.Copy(&respUserAuth, userAuth)

	return &pb.GetUserAuthyUserIdResp{
		UserAuth: &respUserAuth,
	}, nil
}

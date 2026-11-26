// @Title getUserInfoLogic.go
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

var ErrUserNoExistsError = xerr.NewErrMsg("用户不存在")

// GetUserInfoLogic 保存当前请求上下文、共享依赖和日志能力。
type GetUserInfoLogic struct {
	ctx         context.Context     // 当前请求的上下文，传递取消、超时和链路信息。
	svcCtx      *svc.ServiceContext // 进程共享的依赖，不是当前登录用户的数据。
	logx.Logger                     // 关联请求上下文的日志记录器。
}

// NewGetUserInfoLogic 创建当前请求的逻辑对象，复用服务依赖，并让日志携带请求上下文。
// @title NewGetUserInfoLogic
// @description 创建当前请求的逻辑对象，复用服务依赖，并让日志携带请求上下文。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param ctx context.Context "调用方传入的请求上下文。"
// @param svcCtx *svc.ServiceContext "启动时组装的共享依赖。"
// @return result *GetUserInfoLogic "初始化后的依赖对象或逻辑对象。"
func NewGetUserInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserInfoLogic {
	return &GetUserInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetUserInfo 根据用户主键查询基本信息。
// @title GetUserInfo
// @description 根据用户主键查询基本信息。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param in *pb.GetUserInfoReq "RPC 请求消息，字段含义参见 pb/usercenter.proto 。"
// @return resp *pb.GetUserInfoResp "成功结果；业务失败时通常为 nil 。"
// @return err error "失败原因；为 nil 表示本方法成功。"
func (l *GetUserInfoLogic) GetUserInfo(in *pb.GetUserInfoReq) (*pb.GetUserInfoResp, error) {

	// TODO 通过 Model 查询单条记录；缓存未命中时由生成的实现读取 MySQL 。
	user, err := l.svcCtx.UserModel.FindOne(l.ctx, in.Id)
	if err != nil && err != model.ErrNotFound {
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.DB_ERROR), "GetUserInfo find user db err , id:%d , err:%v", in.Id, err)
	}
	// TODO 用户详情不存在属于业务失败，不返回空用户信息。
	if user == nil {
		return nil, errors.Wrapf(ErrUserNoExistsError, "id:%d", in.Id)
	}
	// TODO 转换为 protobuf 的公开用户字段； User 消息没有 Password 字段。
	var respUser usercenter.User
	_ = copier.Copy(&respUser, user)

	return &usercenter.GetUserInfoResp{
		User: &respUser,
	}, nil

}

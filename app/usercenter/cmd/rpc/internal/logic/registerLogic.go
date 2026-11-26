// @Title registerLogic.go
// @Description 用户中心 RPC 业务逻辑，说明输入来源、下游调用和返回过程。
// @Author 原代码作者未标注；本次教学注释由 Codex 编写。
// @Update Codex 2026-10-16 ，仅补充注释。
package logic

import (
	"context"
	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"looklook/app/usercenter/cmd/rpc/internal/svc"
	"looklook/app/usercenter/cmd/rpc/usercenter"
	"looklook/app/usercenter/model"
	"looklook/pkg/tool"
	"looklook/pkg/xerr"
)

var ErrUserAlreadyRegisterError = xerr.NewErrMsg("user has been registered")

// RegisterLogic 保存当前请求上下文、共享依赖和日志能力。
type RegisterLogic struct {
	ctx         context.Context     // 当前请求的上下文，传递取消、超时和链路信息。
	svcCtx      *svc.ServiceContext // 进程共享的依赖，不是当前登录用户的数据。
	logx.Logger                     // 关联请求上下文的日志记录器。
}

// NewRegisterLogic 创建当前请求的逻辑对象，复用服务依赖，并让日志携带请求上下文。
// @title NewRegisterLogic
// @description 创建当前请求的逻辑对象，复用服务依赖，并让日志携带请求上下文。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param ctx context.Context "调用方传入的请求上下文。"
// @param svcCtx *svc.ServiceContext "启动时组装的共享依赖。"
// @return result *RegisterLogic "初始化后的依赖对象或逻辑对象。"
func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Register 注册用户并返回登录令牌。
// @title Register
// @description 注册用户并返回登录令牌。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param in *usercenter.RegisterReq "RPC 请求消息，字段含义参见 pb/usercenter.proto 。"
// @return resp *usercenter.RegisterResp "成功结果；业务失败时通常为 nil 。"
// @return err error "失败原因；为 nil 表示本方法成功。"
func (l *RegisterLogic) Register(in *usercenter.RegisterReq) (*usercenter.RegisterResp, error) {

	// TODO 先查手机号； ErrNotFound 表示可以继续注册，连接等其他错误不能当作用户不存在。
	user, err := l.svcCtx.UserModel.FindOneByMobile(l.ctx, in.Mobile)
	if err != nil && err != model.ErrNotFound {
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.DB_ERROR), "mobile:%s,err:%v", in.Mobile, err)
	}
	// TODO 提前拒绝已存在的手机号；并发注册最终仍需依靠数据库唯一索引防止重复。
	if user != nil {
		return nil, errors.Wrapf(ErrUserAlreadyRegisterError, "Register user exists mobile:%s,err:%v", in.Mobile, err)
	}

	var userId int64
	// TODO 在同一个 MySQL 本地事务中写两张表；回调返回 nil 才提交，返回错误则回滚。
	// TODO 两次 Insert 都传同一个 session ，不能传 nil ，否则会脱离这个事务。
	if err := l.svcCtx.UserModel.Trans(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		user := new(model.User)
		user.Mobile = in.Mobile
		// TODO 只在没有昵称时生成随机值；当前代码没有处理传入非空昵称时的赋值。
		if len(in.Nickname) == 0 {
			user.Nickname = tool.Krand(8, tool.KC_RAND_KIND_ALL)
		}
		// TODO 微信注册可以不传密码；手机号入口的密码规则需要另行校验。
		// TODO 当前使用 MD5 摘要，这是现有实现，不是推荐的生产密码存储算法。
		if len(in.Password) > 0 {
			user.Password = tool.Md5ByString(in.Password)
		}
		// TODO 先写用户表；此处 session 来自事务回调，因此写入受同一事务控制。
		insertResult, err := l.svcCtx.UserModel.Insert(ctx, session, user)
		if err != nil {
			return errors.Wrapf(xerr.NewErrCode(xerr.DB_ERROR), "Register db user Insert err:%v,user:%+v", err, user)
		}
		// TODO 取得数据库自增主键，用于建立 user_auth.user_id 与 user.id 的关联。
		lastId, err := insertResult.LastInsertId()
		if err != nil {
			return errors.Wrapf(xerr.NewErrCode(xerr.DB_ERROR), "Register db user insertResult.LastInsertId err:%v,user:%+v", err, user)
		}
		userId = lastId

		// TODO 登录类型区分 system 与 wxMini ；登录标识分别为手机号或微信 OpenID 。
		userAuth := new(model.UserAuth)
		userAuth.UserId = lastId
		userAuth.AuthKey = in.AuthKey
		userAuth.AuthType = in.AuthType
		if _, err := l.svcCtx.UserAuthModel.Insert(ctx, session, userAuth); err != nil {
			return errors.Wrapf(xerr.NewErrCode(xerr.DB_ERROR), "Register db user_auth Insert err:%v,userAuth:%v", err, userAuth)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	// TODO 事务已经提交，再直接调用本进程的 GenerateTokenLogic ，不绕网络调用自身。
	// TODO 签发失败不会回滚已经提交的用户记录，重试注册时可能提示用户已存在。
	generateTokenLogic := NewGenerateTokenLogic(l.ctx, l.svcCtx)
	tokenResp, err := generateTokenLogic.GenerateToken(&usercenter.GenerateTokenReq{
		UserId: userId,
	})
	if err != nil {
		return nil, errors.Wrapf(ErrGenerateTokenError, "GenerateToken userId : %d", userId)
	}

	// TODO 把令牌、过期时间和建议刷新时点返回 API ，由 API 转成 HTTP 响应。
	return &usercenter.RegisterResp{
		AccessToken:  tokenResp.AccessToken,
		AccessExpire: tokenResp.AccessExpire,
		RefreshAfter: tokenResp.RefreshAfter,
	}, nil
}

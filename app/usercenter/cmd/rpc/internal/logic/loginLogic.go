// @Title loginLogic.go
// @Description 用户中心 RPC 业务逻辑，说明输入来源、下游调用和返回过程。
// @Author 原代码作者未标注；本次教学注释由 Codex 编写。
// @Update Codex 2026-10-16 ，仅补充注释。
package logic

import (
	"context"

	"looklook/app/usercenter/cmd/rpc/internal/svc"
	"looklook/app/usercenter/cmd/rpc/usercenter"
	"looklook/app/usercenter/model"
	"looklook/pkg/tool"
	"looklook/pkg/xerr"

	"github.com/pkg/errors"
	"github.com/zeromicro/go-zero/core/logx"
)

// LoginLogic 保存当前请求上下文、共享依赖和日志能力。
type LoginLogic struct {
	ctx         context.Context     // 当前请求的上下文，传递取消、超时和链路信息。
	svcCtx      *svc.ServiceContext // 进程共享的依赖，不是当前登录用户的数据。
	logx.Logger                     // 关联请求上下文的日志记录器。
}

var ErrGenerateTokenError = xerr.NewErrMsg("生成token失败")
var ErrUsernamePwdError = xerr.NewErrMsg("账号或密码不正确")

// NewLoginLogic 创建当前请求的逻辑对象，复用服务依赖，并让日志携带请求上下文。
// @title NewLoginLogic
// @description 创建当前请求的逻辑对象，复用服务依赖，并让日志携带请求上下文。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param ctx context.Context "调用方传入的请求上下文。"
// @param svcCtx *svc.ServiceContext "启动时组装的共享依赖。"
// @return result *LoginLogic "初始化后的依赖对象或逻辑对象。"
func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Login 验证登录凭据并返回令牌。
// @title Login
// @description 验证登录凭据并返回令牌。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param in *usercenter.LoginReq "RPC 请求消息，字段含义参见 pb/usercenter.proto 。"
// @return resp *usercenter.LoginResp "成功结果；业务失败时通常为 nil 。"
// @return err error "失败原因；为 nil 表示本方法成功。"
func (l *LoginLogic) Login(in *usercenter.LoginReq) (*usercenter.LoginResp, error) {

	var userId int64
	var err error
	// TODO 当前 Login RPC 只实现 system 类型；微信授权通过 API 的另一条业务流程完成。
	// 前面 API 传入的是 "system"，所以进入 loginByMobile。本次入参对应手机号 13800138000 和请求密码。
	switch in.AuthType {
	case model.UserAuthTypeSystem:
		userId, err = l.loginByMobile(in.AuthKey, in.Password)
	default:
		return nil, xerr.NewErrCode(xerr.SERVER_COMMON_ERROR)
	}
	if err != nil {
		return nil, err
	}
	// TODO 密码验证成功后，在当前进程直接复用签发逻辑，不额外发起一次 RPC 。
	// 验证成功后 签发token
	generateTokenLogic := NewGenerateTokenLogic(l.ctx, l.svcCtx)
	tokenResp, err := generateTokenLogic.GenerateToken(&usercenter.GenerateTokenReq{
		UserId: userId,
	})
	if err != nil {
		return nil, errors.Wrapf(ErrGenerateTokenError, "GenerateToken userId : %d", userId)
	}

	return &usercenter.LoginResp{
		AccessToken:  tokenResp.AccessToken,
		AccessExpire: tokenResp.AccessExpire,
		RefreshAfter: tokenResp.RefreshAfter,
	}, nil
}

// loginByMobile 查询手机号并比较密码摘要，成功时返回用户主键。
// @title loginByMobile
// @description 查询手机号并比较密码摘要，成功时返回用户主键。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param mobile string "平台账号使用的手机号。"
// @param password string "本次请求提供的原始密码。"
// @return userId int64 "校验成功的用户主键；失败时为零。"
// @return err error "失败原因；为 nil 表示本方法成功。"
func (l *LoginLogic) loginByMobile(mobile, password string) (int64, error) {

	// TODO 按手机号查询用户；查不到与数据库异常分别返回不同错误。
	user, err := l.svcCtx.UserModel.FindOneByMobile(l.ctx, mobile)
	if err != nil && err != model.ErrNotFound {
		return 0, errors.Wrapf(xerr.NewErrCode(xerr.DB_ERROR), "根据手机号查询用户信息失败，mobile:%s,err:%v", mobile, err)
	}
	if user == nil {
		return 0, errors.Wrapf(ErrUserNoExistsError, "mobile:%s", mobile)
	}

	// TODO 将本次密码转换为与注册时相同的摘要进行比较，数据库中不是原始密码。
	// TODO 这是原有 MD5 方案；生产密码存储应采用 bcrypt 或 Argon2 等专用算法。
	if !(tool.Md5ByString(password) == user.Password) {
		return 0, errors.Wrap(ErrUsernamePwdError, "密码匹配出错")
	}

	// TODO 仅把已校验用户的主键交给调用方，随后由 Login 签发令牌。
	return user.Id, nil
}

// loginBySmallWx 保留的微信登录占位方法，当前没有业务实现，也没有被登录分支调用。
// @title loginBySmallWx
// @description 保留的微信登录占位方法，当前没有业务实现，也没有被登录分支调用。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @return err error "失败原因；为 nil 表示本方法成功。"
func (l *LoginLogic) loginBySmallWx() error {
	// TODO 当前只是占位，不会验证微信身份；返回 nil 不能解释成已完成微信登录。
	return nil
}

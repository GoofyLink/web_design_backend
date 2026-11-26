// @Title wxMiniAuthLogic.go
// @Description 用户中心 API 业务逻辑，说明输入来源、下游调用和返回过程。
// @Author 原代码作者未标注；本次教学注释由 Codex 编写。
// @Update Codex 2026-10-16 ，仅补充注释。
package user

import (
	"context"
	"fmt"

	"looklook/app/usercenter/cmd/api/internal/svc"
	"looklook/app/usercenter/cmd/api/internal/types"
	"looklook/app/usercenter/cmd/rpc/usercenter"
	usercenterModel "looklook/app/usercenter/model"
	"looklook/pkg/xerr"

	"github.com/pkg/errors"
	wechat "github.com/silenceper/wechat/v2"
	"github.com/silenceper/wechat/v2/cache"
	miniConfig "github.com/silenceper/wechat/v2/miniprogram/config"
	"github.com/zeromicro/go-zero/core/logx"
)

var ErrWxMiniAuthFailError = xerr.NewErrMsg("wechat mini auth fail")

// WxMiniAuthLogic 保存当前请求上下文、共享依赖和日志能力。
type WxMiniAuthLogic struct {
	logx.Logger                     // 关联请求上下文的日志记录器。
	ctx         context.Context     // 当前请求的上下文，传递取消、超时和链路信息。
	svcCtx      *svc.ServiceContext // 进程共享的依赖，不是当前登录用户的数据。
}

// NewWxMiniAuthLogic 创建当前请求的逻辑对象，复用服务依赖，并让日志携带请求上下文。
// @title NewWxMiniAuthLogic
// @description 创建当前请求的逻辑对象，复用服务依赖，并让日志携带请求上下文。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param ctx context.Context "调用方传入的请求上下文。"
// @param svcCtx *svc.ServiceContext "启动时组装的共享依赖。"
// @return result WxMiniAuthLogic "初始化后的依赖对象或逻辑对象。"
func NewWxMiniAuthLogic(ctx context.Context, svcCtx *svc.ServiceContext) WxMiniAuthLogic {
	return WxMiniAuthLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// WxMiniAuth 处理微信小程序授权并关联本地用户。
// @title WxMiniAuth
// @description 处理微信小程序授权并关联本地用户。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param req types.WXMiniAuthReq "handler 解析的 HTTP 请求参数。"
// @return resp *types.WXMiniAuthResp "成功结果；业务失败时通常为 nil 。"
// @return err error "失败原因；为 nil 表示本方法成功。"
func (l *WxMiniAuthLogic) WxMiniAuth(req types.WXMiniAuthReq) (*types.WXMiniAuthResp, error) {
	// TODO 使用配置创建微信客户端；当前路由仍受 JWT 保护，不是匿名登录入口。
	miniprogram := wechat.NewWechat().GetMiniProgram(&miniConfig.Config{
		AppID:     l.svcCtx.Config.WxMiniConf.AppId,
		AppSecret: l.svcCtx.Config.WxMiniConf.Secret,
		Cache:     cache.NewMemory(),
	})
	// TODO 用前端提供的临时 code 向微信换取 OpenID 和会话密钥，这是外部网络调用。
	authResult, err := miniprogram.GetAuth().Code2Session(req.Code)
	if err != nil || authResult.ErrCode != 0 || authResult.OpenID == "" {
		return nil, errors.Wrapf(ErrWxMiniAuthFailError, "发起授权请求失败 err : %v , code : %s  , authResult : %+v", err, req.Code, authResult)
	}
	// TODO 使用会话密钥、密文和 IV 解密微信数据；失败时不继续关联本地用户。
	userData, err := miniprogram.GetEncryptor().Decrypt(authResult.SessionKey, req.EncryptedData, req.IV)
	if err != nil {
		return nil, errors.Wrapf(ErrWxMiniAuthFailError, "解析数据失败 req : %+v , err: %v , authResult:%+v ", req, err, authResult)
	}
	var userId int64
	// TODO 通过 wxMini 与 OpenID 查询授权表，确认微信身份是否已经关联本地用户。
	rpcRsp, err := l.svcCtx.UsercenterRpc.GetUserAuthByAuthKey(l.ctx, &usercenter.GetUserAuthByAuthKeyReq{
		AuthType: usercenterModel.UserAuthTypeSmallWX,
		AuthKey:  authResult.OpenID,
	})
	if err != nil {
		return nil, errors.Wrapf(ErrWxMiniAuthFailError, "rpc call userAuthByAuthKey err : %v , authResult : %+v", err, authResult)
	}
	// TODO 未找到授权关系时， RPC 可能返回 Id 为零的空对象，因此同时检查指针和主键。
	if rpcRsp.UserAuth == nil || rpcRsp.UserAuth.Id == 0 {
		// TODO 尝试为未关联的微信身份注册本地用户；该分支不是给已有手机号自动绑定微信。
		mobile := userData.PhoneNumber
		// TODO 当前实现假定手机号至少有七个字节；截取前没有长度校验，短字符串可能导致异常。
		nickName := fmt.Sprintf("LookLook%s", mobile[7:])
		// TODO 调用注册 RPC ，同时写 user 与 user_auth ；该调用不传密码。
		// TODO 当前 RPC 仅在昵称为空时赋随机昵称，并没有把传入的非空昵称保存下来。
		registerRsp, err := l.svcCtx.UsercenterRpc.Register(l.ctx, &usercenter.RegisterReq{
			AuthKey:  authResult.OpenID,
			AuthType: usercenterModel.UserAuthTypeSmallWX,
			Mobile:   mobile,
			Nickname: nickName,
		})
		if err != nil {
			return nil, errors.Wrapf(ErrWxMiniAuthFailError, "UsercenterRpc.Register err :%v, authResult : %+v", err, authResult)
		}

		return &types.WXMiniAuthResp{
			AccessToken:  registerRsp.AccessToken,
			AccessExpire: registerRsp.AccessExpire,
			RefreshAfter: registerRsp.RefreshAfter,
		}, nil

	} else {
		// TODO 已关联用户直接签发令牌，无需再次注册； GenerateToken 是独立 RPC 调用。
		userId = rpcRsp.UserAuth.UserId
		tokenResp, err := l.svcCtx.UsercenterRpc.GenerateToken(l.ctx, &usercenter.GenerateTokenReq{
			UserId: userId,
		})
		if err != nil {
			return nil, errors.Wrapf(ErrWxMiniAuthFailError, "usercenterRpc.GenerateToken err :%v, userId : %d", err, userId)
		}
		return &types.WXMiniAuthResp{
			AccessToken:  tokenResp.AccessToken,
			AccessExpire: tokenResp.AccessExpire,
			RefreshAfter: tokenResp.RefreshAfter,
		}, nil
	}
}

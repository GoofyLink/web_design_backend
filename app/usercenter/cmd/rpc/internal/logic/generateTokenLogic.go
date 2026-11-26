// @Title generateTokenLogic.go
// @Description 用户中心 RPC 业务逻辑，说明输入来源、下游调用和返回过程。
// @Author 原代码作者未标注；本次教学注释由 Codex 编写。
// @Update Codex 2026-10-16 ，仅补充注释。
package logic

import (
	"context"
	"github.com/pkg/errors"
	"looklook/pkg/ctxdata"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"looklook/app/usercenter/cmd/rpc/internal/svc"
	"looklook/app/usercenter/cmd/rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

// GenerateTokenLogic 保存当前请求上下文、共享依赖和日志能力。
type GenerateTokenLogic struct {
	ctx         context.Context     // 当前请求的上下文，传递取消、超时和链路信息。
	svcCtx      *svc.ServiceContext // 进程共享的依赖，不是当前登录用户的数据。
	logx.Logger                     // 关联请求上下文的日志记录器。
}

// NewGenerateTokenLogic 创建当前请求的逻辑对象，复用服务依赖，并让日志携带请求上下文。
// @title NewGenerateTokenLogic
// @description 创建当前请求的逻辑对象，复用服务依赖，并让日志携带请求上下文。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param ctx context.Context "调用方传入的请求上下文。"
// @param svcCtx *svc.ServiceContext "启动时组装的共享依赖。"
// @return result *GenerateTokenLogic "初始化后的依赖对象或逻辑对象。"
func NewGenerateTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GenerateTokenLogic {
	return &GenerateTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GenerateToken 为指定用户签发 JWT 令牌。
// @title GenerateToken
// @description 为指定用户签发 JWT 令牌。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param in *pb.GenerateTokenReq "RPC 请求消息，字段含义参见 pb/usercenter.proto 。"
// @return resp *pb.GenerateTokenResp "成功结果；业务失败时通常为 nil 。"
// @return err error "失败原因；为 nil 表示本方法成功。"
func (l *GenerateTokenLogic) GenerateToken(in *pb.GenerateTokenReq) (*pb.GenerateTokenResp, error) {
	// TODO 取得 Unix 秒时间戳，并从配置读取有效时长；二者相加才是绝对过期时间。
	now := time.Now().Unix()
	accessExpire := l.svcCtx.Config.JwtAuth.AccessExpire
	// TODO 仅根据传入的用户主键签名；本方法不重新查库，也不负责验证账号密码。
	accessToken, err := l.getJwtToken(l.svcCtx.Config.JwtAuth.AccessSecret, now, accessExpire, in.UserId)
	if err != nil {
		return nil, errors.Wrapf(ErrGenerateTokenError, "getJwtToken err userId:%d , err:%v", in.UserId, err)
	}

	// TODO RefreshAfter 是建议刷新时点，当前没有自动刷新或专门的刷新 HTTP 接口。
	return &pb.GenerateTokenResp{
		AccessToken:  accessToken,
		AccessExpire: now + accessExpire,
		RefreshAfter: now + accessExpire/2,
	}, nil
}

// getJwtToken 构造身份声明并使用 HS256 签名，不查询数据库，也不验证用户是否存在。
// @title getJwtToken
// @description 构造身份声明并使用 HS256 签名，不查询数据库，也不验证用户是否存在。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param secretKey string "服务端 JWT 签名密钥。"
// @param iat int64 "签发时的 Unix 秒时间戳。"
// @param seconds int64 "有效时长，单位为秒。"
// @param userId int64 "本地用户主键，写入 jwtUserId 声明。"
// @return token string "签名后的 JWT 字符串。"
// @return err error "失败原因；为 nil 表示本方法成功。"
func (l *GenerateTokenLogic) getJwtToken(secretKey string, iat, seconds, userId int64) (string, error) {

	// TODO JWT 声明会编码到令牌中，并非加密；不要把密码等秘密放入声明。
	claims := make(jwt.MapClaims)
	claims["exp"] = iat + seconds
	claims["iat"] = iat
	// TODO 使用与 API 读取逻辑一致的 jwtUserId 键，让鉴权后能找到当前用户。
	claims[ctxdata.CtxKeyJwtUserId] = userId
	// TODO 使用 HS256 和服务端密钥签名； API 必须使用同一密钥校验，令牌没有写入 Redis 。
	token := jwt.New(jwt.SigningMethodHS256)
	token.Claims = claims
	return token.SignedString([]byte(secretKey))
}

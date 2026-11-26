// @Title registerHandler.go
// @Description HTTP 请求入口，将协议解析、业务调用和响应包装串联起来。
// @Author 原代码作者未标注；本次教学注释由 Codex 编写。
// @Update Codex 2026-10-16 ，仅补充注释。
package user

import (
	"net/http"

	"looklook/app/usercenter/cmd/api/internal/logic/user"
	"looklook/app/usercenter/cmd/api/internal/svc"
	"looklook/app/usercenter/cmd/api/internal/types"
	"looklook/pkg/result"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterHandler 启动时创建 HTTP 处理函数；函数内的闭包在每次匹配到该路由时执行。
// @title RegisterHandler
// @description 启动时创建 HTTP 处理函数；函数内的闭包在每次匹配到该路由时执行。
// @auth Codex 2026-10-16 ，仅编写教学注释。
// @param ctx *svc.ServiceContext "请求上下文； handler 构造函数中则是共享服务依赖。"
// @return handler http.HandlerFunc "交给路由执行的 HTTP 请求处理函数。"
func RegisterHandler(ctx *svc.ServiceContext) http.HandlerFunc {
	// TODO 这个闭包在每次 HTTP 请求到达时执行，外层构造函数只在注册路由时执行。
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO 把 HTTP 输入解析为 API 类型；解析失败返回参数错误，不继续调用 RPC 。
		var req types.RegisterReq
		if err := httpx.Parse(r, &req); err != nil {
			result.ParamErrorResult(r, w, err)
			return
		}

		// TODO 传递 r.Context()，保留超时、取消信号、链路信息以及已验证的 JWT 声明。
		l := user.NewRegisterLogic(r.Context(), ctx)
		resp, err := l.Register(req)
		// TODO 统一输出 code 、 msg 和成功时的 data ； logic 的错误也在这里转换为 HTTP 响应。
		result.HttpResult(r, w, resp, err)
	}
}

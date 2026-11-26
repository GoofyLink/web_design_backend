// @Title usercenter.go
// @Description 用户中心 API 进程入口，说明配置加载、依赖组装和服务启动顺序。
// @Author 原代码作者未标注；本次教学注释由 Codex 编写。
// @Update Codex 2026-10-16 ，仅补充注释。
package main

import (
	"flag"
	"fmt"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"looklook/app/usercenter/cmd/api/internal/config"
	"looklook/app/usercenter/cmd/api/internal/handler"
	"looklook/app/usercenter/cmd/api/internal/svc"
)

// TODO 读取 -f 指定的配置文件；相对路径以启动命令所在目录为准。
var configFile = flag.String("f", "etc/usercenter.yaml", "the config file")

// main 按配置、依赖、服务注册的顺序启动 API 进程，随后阻塞等待请求。
// @title main
// @description 按配置、依赖、服务注册的顺序启动 API 进程，随后阻塞等待请求。
// @auth Codex 2026-10-16 ，仅编写教学注释。
func main() {
	// TODO 先解析启动参数，再读取配置；配置错误会由 MustLoad 终止启动。
	flag.Parse()
	var c config.Config
	// TODO UseEnv 允许替换配置中的环境变量引用，不是任意环境变量自动覆盖字段。
	conf.MustLoad(*configFile, &c, conf.UseEnv())

	// TODO 创建共享依赖；后续请求复用这些对象，而不是每次重新创建连接。
	ctx := svc.NewServiceContext(c)
	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	// TODO 注册路由和 JWT 中间件；此时只绑定 handler ，不执行注册或登录业务。
	// TODO 调用链：请求 → routes.go → handler/user → logic/user → RPC 客户端。
	handler.RegisterHandlers(server, ctx)

	// TODO 输出启动地址并开始监听；打印该消息本身不代表所有下游服务已就绪。
	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}

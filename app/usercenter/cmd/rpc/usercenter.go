// @Title usercenter.go
// @Description 用户中心 RPC 进程入口，说明配置加载、依赖组装和服务启动顺序。
// @Author 原代码作者未标注；本次教学注释由 Codex 编写。
// @Update Codex 2026-10-16 ，仅补充注释。
package main

import (
	"flag"
	"fmt"
	"github.com/zeromicro/go-zero/core/service"
	"google.golang.org/grpc/reflection"

	"looklook/app/usercenter/cmd/rpc/internal/config"
	"looklook/app/usercenter/cmd/rpc/internal/server"
	"looklook/app/usercenter/cmd/rpc/internal/svc"
	"looklook/app/usercenter/cmd/rpc/pb"
	"looklook/pkg/interceptor/rpcserver"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

// TODO 读取 -f 指定的配置文件；相对路径以启动命令所在目录为准。
var configFile = flag.String("f", "etc/usercenter.yaml", "the config file")

// main 按配置、依赖、服务注册的顺序启动 RPC 进程，随后阻塞等待请求。
// @title main
// @description 按配置、依赖、服务注册的顺序启动 RPC 进程，随后阻塞等待请求。
// @auth Codex 2026-10-16 ，仅编写教学注释。
func main() {
	// TODO 先解析启动参数，再读取配置；配置错误会由 MustLoad 终止启动。
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	// TODO 创建共享依赖；后续请求复用这些对象，而不是每次重新创建连接。
	ctx := svc.NewServiceContext(c)
	// TODO 创建生成的服务适配器；它把每个 gRPC 方法转交给对应 RPC logic 。
	srv := server.NewUsercenterServer(ctx)

	// TODO 将服务实现绑定到 protobuf 描述，客户端才能通过网络调用这些方法。
	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterUsercenterServer(grpcServer, srv)
		// TODO 仅在开发和测试模式开启反射，便于调试工具查询 RPC 方法与消息类型。
		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	// TODO 注册统一错误拦截器；业务错误在返回 API 前转换为 gRPC 状态并记录日志。
	s.AddUnaryInterceptors(rpcserver.LoggerInterceptor)

	defer s.Stop()

	// TODO 输出启动地址并开始监听；打印该消息本身不代表所有下游服务已就绪。
	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}

/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-15 19:14:53
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-17 16:41:01
 * @FilePath: /MLC_GO/TestNotes/gRPC_practice/server.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
//title: 服务端启动
//package grpc_practice
package main

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"MLC_GO/TestNotes/gRPC_practice/gRPC_practice_v2/pkg/gtls"
	pb "MLC_GO/TestNotes/gRPC_practice/gRPC_practice_v2/proto"
	"context"
	"net"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/grpc-ecosystem/go-grpc-middleware" //拦截器中间件
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SearchService struct{
	// 新版 protoc-gen-go-grpc 生成的 gRPC 代码中，所有 service 定义的接口都会包含一个默认的未实现结构体:
	// 		type UnimplementedSearchServiceServer struct{}
	// 添加一个空实现
	pb.UnimplementedSearchServiceServer
}

func (s *SearchService) Search(ctx context.Context, r *pb.SearchRequest) (*pb.SearchResponse, error) {
	return &pb.SearchResponse{Response: r.GetRequest() + " Server"}, nil
}

const (
	PORT = "9001"
)

func main() {
	// gRPCServerPractice_v1()
	// gRPCServerPractice_v2()
	// gRPCServerPractice_v3()
	// gRPCServerPractice_v4()
	gRPCServerPractice_v5()
}

//gRPC提供http接口的服务端(和simple_client/client.go文件的 gRPCSimpleClient_test_v5 方法对应)
// 使用的时候在PostMan调用接口: https://127.0.0.1:9001/eddycjy
func gRPCServerPractice_v5() {
	certFile := "../../conf/server/server.pem"
	keyFile := "../../conf/server/server.key"
	tlsServer := gtls.Server{
		CertFile: certFile,
		KeyFile:  keyFile,
	}

	c, err := tlsServer.GetTLSCredentials()
	if err != nil {
		logging.ErrInfo("gRPC提供http接口--tlsServer.GetTLSCredentials err: ", err)
	}

	// 获取 HTTP 处理器。
	mux := GetHTTPServeMux()

	// 创建 gRPC 服务器
	server := grpc.NewServer(grpc.Creds(c))
	// 注册 SearchService gRPC 服务。
	pb.RegisterSearchServiceServer(server, &SearchService{})

	// 可简单的理解为提供监听 HTTPS 服务的方法，重点的协议判断转发，也在这里面
	// 其实，你理解后就会觉得很简单，核心步骤：判断 -> 转发 -> 响应。我们改变了前两步的默认逻辑，仅此而已
	// 启动 HTTPS 服务器
	http.ListenAndServeTLS(":"+PORT,
		certFile,
		keyFile,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 如果 ProtoMajor == 2（即 HTTP/2）且 Content-Type 包含 "application/grpc"，则 走 gRPC 处理
			if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
				server.ServeHTTP(w, r)
			} else {// 否则 走普通 HTTP 处理（mux.ServeHTTP）。
				mux.ServeHTTP(w, r)
			}

			return
		}),
	)
}
// 普通 HTTP 处理器
func GetHTTPServeMux() *http.ServeMux {
	// 创建一个新的 ServeMux，ServeMux 本质上是一个路由表。
	// 		它默认实现了 ServeHTTP，因此返回 Handler 后可直接通过 HandleFunc 注册 pattern 和处理逻辑的方法
	mux := http.NewServeMux()
	// / 访问返回 "eddycjy: gRPC提供http接口>>>>go-grpc-example"
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("eddycjy: gRPC提供http接口>>>>go-grpc-example"))
	})

	return mux
}

// <<<<<<<分隔符1======================================

// 拦截器服务端(和simple_client/client.go文件的 gRPCSimpleClient_test_v3 方法对应)
func gRPCServerPractice_v4() {
	tlsServer := gtls.Server{
		CaFile:   "../../conf/ca.pem",
		CertFile: "../../conf/server/server.pem",
		KeyFile:  "../../conf/server/server.key",
	}
	c, err := tlsServer.GetCredentialsByCA()
	if err != nil {
		logging.ErrInfo("拦截器服务端-GetTLSCredentialsByCA err: ", err)
	}

	opts := []grpc.ServerOption{
		// grpc.Creds(c) 设置 TLS 证书，强制客户端必须使用 TLS 连接。
		grpc.Creds(c),
		// grpc_middleware.WithUnaryServerChain(...) 添加 拦截器
		grpc_middleware.WithUnaryServerChain(
			RecoveryInterceptor,// 用于 捕获 panic 并返回错误，防止程序崩溃。
			LoggingInterceptor,// 用于 记录 gRPC 方法调用日志
		),
	}

	server := grpc.NewServer(opts...)
	// 创建 grpc.NewServer，注册 SearchService 作为 gRPC 服务。
	pb.RegisterSearchServiceServer(server, &SearchService{})

	// 监听 PORT 端口，等待 gRPC 请求。
	lis, err := net.Listen("tcp", ":"+PORT)
	if err != nil {
		logging.ErrInfo("拦截器服务端-net.Listen err: ", err)
	}

	server.Serve(lis)
}
// 日志拦截器,记录 gRPC 请求日志
func LoggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// info.FullMethod 记录 gRPC 方法名
	// req 记录 gRPC 请求参数
	// resp 记录 gRPC 响应参数
	logging.DebugInfo("gRPC method: %s", info.FullMethod, "%v", req)
	resp, err := handler(ctx, req)
	logging.DebugInfo("gRPC method: %s", info.FullMethod, "%v", resp)
	return resp, err
}
// 错误恢复拦截器,防止 gRPC 服务器因 panic 崩溃
func RecoveryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	defer func() {
		// 通过 recover() 捕获 panic
		if e := recover(); e != nil {
			debug.PrintStack()// 打印错误堆栈信息。
			err = status.Errorf(codes.Internal, "Panic err: %v", e) // 返回 codes.Internal gRPC 错误码
		}
	}()

	return handler(ctx, req)
}

// <<<<<<<分隔符0======================================

// 基于CA的TLS证书认证的服务端(和simple_client/client.go文件的 gRPCSimpleClient_test_v3 方法对应)
func gRPCServerPractice_v3() {
	certFile := "../../conf/server/server.pem"
	keyFile := "../../conf/server/server.key"
	tlsServer := gtls.Server{
		CaFile:   "../../conf/ca.pem",
		CertFile: certFile,
		KeyFile:  keyFile,
	}

	c, err := tlsServer.GetCredentialsByCA()
	if err != nil {
		logging.ErrInfo("基于CA的TLS证书认证的服务端V3--credentials.NewServerTLSFromFile err: ", err)
	}

	server := grpc.NewServer(grpc.Creds(c))
	pb.RegisterSearchServiceServer(server, &SearchService{})

	lis, err := net.Listen("tcp", ":"+PORT)
	if err != nil {
		logging.ErrInfo("基于CA的TLS证书认证的服务端V3net.Listen err: ", err)
	}

	server.Serve(lis)
}

// 加入TLS证书认证的服务端(和simple_client/client.go文件的 gRPCSimpleClient_test_v2 方法对应)
func gRPCServerPractice_v2() {
	certFile := "../../conf/server/server.pem"
	keyFile := "../../conf/server/server.key"
	tlsServer := gtls.Server{
		CertFile: certFile,
		KeyFile:  keyFile,
	}

	c, err := tlsServer.GetTLSCredentials()
	if err != nil {
		logging.ErrInfo("加入TLS的服务端V2--credentials.NewServerTLSFromFile err: ", err)
	}

	// 创建 gRPC 服务器实例
	// 	调用 grpc.NewServer 创建一个新的 gRPC 服务器实例。
	// 	grpc.Creds(c) 表示在创建服务器时传入 TLS 凭证（c 为之前构造好的 credentials.TransportCredentials 对象），使服务器启用 TLS 加密通信。
	// 		这样，客户端连接时必须使用符合此 TLS 配置的证书，否则连接会被拒绝。
	server := grpc.NewServer(grpc.Creds(c))
	pb.RegisterSearchServiceServer(server, &SearchService{})

	lis, err := net.Listen("tcp", ":"+PORT)
	if err != nil {
		logging.ErrInfo("加入TLS的服务端V2--net.Listen err: ", err)
	}

	server.Serve(lis)
}
	
// gRPC简单服务端
func gRPCServerPractice_v1() {
	// 创建 gRPC Server 对象，你可以理解为它是 Server 端的抽象对象
	server := grpc.NewServer()

	// 注册服务:
	// 	使用 pb.RegisterSearchServiceServer 将你实现的 SearchService 服务注册到 gRPC 服务器上。
	// 	&SearchService{} 是你实现的服务端对象（应满足 SearchServiceServer 接口要求）。这意味着当客户端调用 Search 方法时，服务器将执行你在 SearchService 中定义的逻辑。
	// 将 SearchService（其包含需要被调用的服务端接口）注册到 gRPC Server 的内部注册中心。这样可以在接受到请求时，通过内部的服务发现，发现该服务端接口并转接进行逻辑处理
	pb.RegisterSearchServiceServer(server, &SearchService{})

	// 调用 net.Listen("tcp", ":"+PORT) 在指定端口（例如 9002）上启动 TCP 监听器，用于接收客户端的连接请求。
	// 创建 Listen，监听 TCP 端口
	// gRPC Server 开始 lis.Accept，直到 Stop 或 GracefulStop
	lis, err := net.Listen("tcp", ":"+PORT)
	if err != nil {
		logging.ErrInfo("grpc 错误 net.Listen err: ", err)
	}

	// 通过 server.Serve(lis) 启动 gRPC 服务，将监听器 lis 传入，让服务器开始接受并处理客户端连接。
	server.Serve(lis)
}
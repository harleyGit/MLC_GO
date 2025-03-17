/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-15 19:14:53
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-17 11:10:33
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

	"google.golang.org/grpc"
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
	gRPCServerPractice_v2()
}



// 加入TLS的服务端
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
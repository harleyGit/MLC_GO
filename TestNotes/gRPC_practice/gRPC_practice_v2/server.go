/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-15 19:14:53
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-16 14:00:11
 * @FilePath: /MLC_GO/TestNotes/gRPC_practice/server.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
//title: 服务端启动
//package grpc_practice
package main

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
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
	gRPCServerPractice_v1()
}
	
// gRPC简单服务端
func gRPCServerPractice_v1() {
	// 创建 gRPC Server 对象，你可以理解为它是 Server 端的抽象对象
	server := grpc.NewServer()
	// 将 SearchService（其包含需要被调用的服务端接口）注册到 gRPC Server 的内部注册中心。这样可以在接受到请求时，通过内部的服务发现，发现该服务端接口并转接进行逻辑处理
	pb.RegisterSearchServiceServer(server, &SearchService{})

	// 创建 Listen，监听 TCP 端口
	// gRPC Server 开始 lis.Accept，直到 Stop 或 GracefulStop
	lis, err := net.Listen("tcp", ":"+PORT)
	if err != nil {
		logging.ErrInfo("grpc 错误 net.Listen err: ", err)
	}

	server.Serve(lis)
}
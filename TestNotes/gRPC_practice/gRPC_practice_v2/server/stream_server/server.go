/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-16 13:44:39
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-16 16:19:25
 * @FilePath: /MLC_GO/TestNotes/gRPC_practice/gRPC_practice_v2/server/stream_server/server.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	pb "MLC_GO/TestNotes/gRPC_practice/gRPC_practice_v2/proto"
	"io"
	"net"

	"google.golang.org/grpc"
)


type StreamService struct{
	// 新版 protoc-gen-go-grpc 生成的 gRPC 代码中，所有 service 定义的接口都会包含一个默认的未实现结构体:
	// 		type UnimplementedStreamServiceServer struct{}
	// 添加一个空实现
	pb.UnimplementedStreamServiceServer
}

const (
	PORT = "9002"
)

func main() {
	gRPCStreamServerPractice_v1()
}

// 流式gRPC的服务端
func gRPCStreamServerPractice_v1() {
	server := grpc.NewServer()
	pb.RegisterStreamServiceServer(server, &StreamService{})

	lis, err := net.Listen("tcp", ":"+PORT)
	if err != nil {
		logging.ErrInfo("net.Listen err: %v", err)
	}

	server.Serve(lis)
}
func (s *StreamService) List(r *pb.StreamRequest, stream pb.StreamService_ListServer) error {
	for n := 0; n <= 6; n++ {
		err := stream.Send(&pb.StreamResponse{
			Pt: &pb.StreamPoint{
				Name:  r.Pt.Name,
				Value: r.Pt.Value + int32(n),
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *StreamService) Record(stream pb.StreamService_RecordServer) error {
	for {
		r, err := stream.Recv()
		if err == io.EOF {
			// 我们对每一个 Recv 都进行了处理，当发现 io.EOF (流关闭) 后，需要将最终的响应结果发送给客户端，同时关闭正在另外一侧等待的 Recv
			return stream.SendAndClose(&pb.StreamResponse{Pt: &pb.StreamPoint{Name: "gRPC Stream Server: Record", Value: 1}})
		}
		if err != nil {
			return err
		}

		logging.ErrInfo("gRPC 流式服务端-Record stream.Recv pt.name: ", r.Pt.Name,  "pt.value: ", r.Pt.Value)
	}

	return nil
}

// 双向流式 RPC- 路由
func (s *StreamService) Route(stream pb.StreamService_RouteServer) error {
	n := 0
	for {
		err := stream.Send(&pb.StreamResponse{
			Pt: &pb.StreamPoint{
				Name:  "gPRC Stream Client: Route",
				Value: int32(n),
			},
		})
		if err != nil {
			return err
		}

		r, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		n++

		logging.DebugInfo("gRPC 流式服务端 路由-Route stream.Recv pt.name: ", r.Pt.Name, "pt.value: ",  r.Pt.Value)
	}

	return nil
}
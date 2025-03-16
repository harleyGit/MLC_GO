/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-16 13:44:39
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-16 17:12:17
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
	// 嵌入默认的未实现结构体，确保实现接口要求
	// 这里 StreamService 结构体嵌入了 pb.UnimplementedStreamServiceServer，这样可以避免因为未实现所有接口方法而出错。
	// 这也是新版 protoc-gen-go-grpc 的要求。
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
	// 创建 gRPC 服务器，并注册 StreamService 服务。
	server := grpc.NewServer()
	pb.RegisterStreamServiceServer(server, &StreamService{})
	
	// 在指定端口（9002）监听，然后调用 server.Serve(lis) 启动服务。
	lis, err := net.Listen("tcp", ":"+PORT)
	if err != nil {
		logging.ErrInfo("net.Listen err: ", err)
	}

	server.Serve(lis)
}

// List 方法（服务器端流式 RPC）
// 客户端调用 List 后，服务器端会在一个循环中调用 stream.Send()，连续发送多个响应消息。
// 每个响应消息中，Value 字段的值逐渐增加。
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

// Record 方法（服务端流式 RPC）
// 在 Record 方法中，服务器端不断调用 stream.Recv() 来接收客户端发送的数据。
// 当客户端发送完数据并关闭流后（返回 io.EOF），服务器调用 SendAndClose 发送最终响应，然后结束方法。
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

		logging.DebugInfo("gRPC 流式服务端-Record stream.Recv pt.name: ", r.Pt.Name,  "🥕 pt.value: ", r.Pt.Value)
	}

	return nil
}

// Route 方法（双向流式 RPC
// 双向流中，客户端和服务器可以互相发送和接收消息。
// 服务器端在一个循环中：先调用 stream.Send() 发送一条消息，然后调用 stream.Recv() 等待客户端的消息。
// 当接收到 io.EOF 时，表示客户端已经关闭发送通道，服务器返回结束。
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

		logging.DebugInfo("gRPC 流式服务端 路由-Route stream.Recv pt.name: ", r.Pt.Name, "🍏 pt.value: ",  r.Pt.Value)
	}

	return nil
}
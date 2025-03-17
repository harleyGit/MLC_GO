/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-16 13:45:06
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-16 19:19:49
 * @FilePath: /MLC_GO/TestNotes/gRPC_practice/gRPC_practice_v2/client/stream_client/client.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	pb "MLC_GO/TestNotes/gRPC_practice/gRPC_practice_v2/proto"
	"context"
	"io"

	"google.golang.org/grpc"
)


const (
	PORT = "9002"
)

func main() {
	gRPCStreamClient_test_v1()
}

// gRPC流式客户端
// 建立连接
func gRPCStreamClient_test_v1() {
	// 客户端使用 grpc.Dial 连接到服务器（9002 端口），并创建了 StreamServiceClient 对象
	conn, err := grpc.Dial(":"+PORT, grpc.WithInsecure())
	if err != nil {
		logging.ErrInfo("流式客户端 grpc.Dial err: ", err)
	}

	defer conn.Close()

	client := pb.NewStreamServiceClient(conn)

	err = printLists(client, &pb.StreamRequest{Pt: &pb.StreamPoint{Name: "gRPC Stream Client: List", Value: 2018}})
	if err != nil {
		logging.ErrInfo("流式客户端 printLists.err: ", err)
	}

	err = printRecord(client, &pb.StreamRequest{Pt: &pb.StreamPoint{Name: "gRPC Stream Client: Record", Value: 2019}})
	if err != nil {
		logging.ErrInfo("流式客户端 printRecord.err: ", err)
	}

	err = printRoute(client, &pb.StreamRequest{Pt: &pb.StreamPoint{Name: "gRPC Stream Client: Route", Value: 2020}})
	if err != nil {
		logging.ErrInfo("流式客户端 printRoute.err: ", err)
	}
}

// 调用 List 方法（服务器端流)
func printLists(client pb.StreamServiceClient, r *pb.StreamRequest) error {
	// 客户端调用 List 方法，获取一个流对象。
	stream, err := client.List(context.Background(), r)
	if err != nil {
		return err
	}

	for {
		// 在循环中不断调用 stream.Recv() 获取服务器端发送的响应，直到遇到 io.EOF 表示数据全部接收完毕。
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		logging.DebugInfo("gRPC 流式客户端方法printLists resp- pj.name: ", resp.Pt.Name, "🍔 pt.value: ", resp.Pt.Value)
	}

	return nil
	
}

// 调用 Record 方法（客户端流）
func printRecord(client pb.StreamServiceClient, r *pb.StreamRequest) error {
	// 客户端调用 Record 方法后，获取一个流对象。
	stream, err := client.Record(context.Background())
	if err != nil {
		return err
	}

	for n := 0; n < 6; n++ {
		// 客户端连续调用 stream.Send() 发送多个请求消息。
		err := stream.Send(r)
		if err != nil {
			return err
		}
	}

	// 当数据发送完毕后，调用 stream.CloseAndRecv() 关闭发送通道并等待服务器的响应。
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return err
	}

	logging.DebugInfo("gRPC 流式客户端方法printRecord resp- pj.name: ", resp.Pt.Name, "🍭 pt.value: ", resp.Pt.Value)

	return nil
}

// 双向流式-路由
// 调用 Route 方法（双向流）
// 对于双向流，客户端既发送消息，也接收服务器的消息。
// 在每个循环中，客户端发送一条消息，然后接收一条响应。
// 最后调用 stream.CloseSend() 关闭发送通道。
func printRoute(client pb.StreamServiceClient, r *pb.StreamRequest) error {
	stream, err := client.Route(context.Background())
	if err != nil {
		return err
	}

	for n := 0; n <= 6; n++ {
		err = stream.Send(r)
		if err != nil {
			return err
		}

		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		logging.DebugInfo("gRPC 流式客户端-printRoute resp- pj.name: ", resp.Pt.Name, "🥑 pt.value: ", resp.Pt.Value)
	}

	stream.CloseSend()

	return nil
}
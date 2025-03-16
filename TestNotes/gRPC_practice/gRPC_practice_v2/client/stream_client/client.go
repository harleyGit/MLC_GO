/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-16 13:45:06
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-16 16:22:58
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
func gRPCStreamClient_test_v1() {
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

	err = printRecord(client, &pb.StreamRequest{Pt: &pb.StreamPoint{Name: "gRPC Stream Client: Record", Value: 2018}})
	if err != nil {
		logging.ErrInfo("流式客户端 printRecord.err: ", err)
	}

	err = printRoute(client, &pb.StreamRequest{Pt: &pb.StreamPoint{Name: "gRPC Stream Client: Route", Value: 2018}})
	if err != nil {
		logging.ErrInfo("流式客户端 printRoute.err: ", err)
	}
}

func printLists(client pb.StreamServiceClient, r *pb.StreamRequest) error {
	stream, err := client.List(context.Background(), r)
	if err != nil {
		return err
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		logging.DebugInfo("gRPC 流式客户端-printLists resp: pj.name: ", resp.Pt.Name, "pt.value: ", resp.Pt.Value)
	}

	return nil
	
}

func printRecord(client pb.StreamServiceClient, r *pb.StreamRequest) error {
	stream, err := client.Record(context.Background())
	if err != nil {
		return err
	}

	for n := 0; n < 6; n++ {
		err := stream.Send(r)
		if err != nil {
			return err
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return err
	}

	logging.DebugInfo("gRPC 流式客户端-printRecord resp: pj.name: ", resp.Pt.Name, "pt.value: ", resp.Pt.Value)

	return nil
}

// 双向流式-路由
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

		logging.DebugInfo("gRPC 流式客户端-printRoute resp: pj.name: ", resp.Pt.Name, "pt.value: ", resp.Pt.Value)
	}

	stream.CloseSend()

	return nil
}
/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-16 13:44:58
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-17 11:25:06
 * @FilePath: /MLC_GO/TestNotes/gRPC_practice/gRPC_practice_v2/client/simple_client/client.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"MLC_GO/TestNotes/gRPC_practice/gRPC_practice_v2/pkg/gtls"
	pb "MLC_GO/TestNotes/gRPC_practice/gRPC_practice_v2/proto"
	"context"

	"google.golang.org/grpc"
)

const PORT = "9001"

func main() {
	gRPCSimpleClient_test_v2()
}

// tls加密传输-客户端
func gRPCSimpleClient_test_v2() {
	tlsClient := gtls.Client{
		ServerName: "HuangGang.dev.use",//"gRPC_practice-gRPC_practice_v2",
		CaFile:     "../../conf/ca.pem",
		CertFile:   "../../conf/server/server.pem",
		// KeyFile:    "../../conf/client/client.key",
	}

	c, err := tlsClient.GetTLSCredentials()
	if err != nil {
		logging.ErrInfo("客户端tls- GetTLSCredentialsByCA err: ", err)
	}

	// 建立 gRPC 连接:
	// 	使用 grpc.Dial 建立到服务器的 gRPC 连接。
	// 	"."+PORT 指定连接的地址和端口（例如 :9002）。
	// 	grpc.WithTransportCredentials(c) 表示客户端使用 TLS 凭证 c 来建立安全连接。这里的 c 必须与服务端 TLS 配置相匹配，否则连接会失败。
	// 	如果连接过程中发生错误，会记录错误信息；连接成功后，通过 defer conn.Close() 确保在函数退出前关闭连接。
	conn, err := grpc.Dial(":"+PORT, grpc.WithTransportCredentials(c))
	if err != nil {
		logging.ErrInfo("客户端tls- grpc.Dial err: ", err)
	}
	defer conn.Close()

	// 创建服务端 Stub
	// 	通过 pb.NewSearchServiceClient(conn) 使用连接 conn 创建 SearchService 的客户端对象（Stub）。
	// 	这个 client 对象提供了与服务器交互的方法，例如调用 Search。
	client := pb.NewSearchServiceClient(conn)
	// 调用 RPC 方法:
	// 	调用 client.Search 方法发送一个 RPC 请求。
	// 	context.Background() 用于传递上下文，可以用于设置超时、取消等。
	// 	请求消息为 &pb.SearchRequest{ Request: "gRPC" }，这里构造了一个 SearchRequest 消息，其 Request 字段被赋值为 "gRPC"。
	// 	如果调用过程中出现错误，则记录错误信息；否则得到返回的响应 resp，包含服务器端处理后的数据。
	resp, err := client.Search(context.Background(), &pb.SearchRequest{
		Request: "gRPC",
	})
	if err != nil {
		logging.ErrInfo("客户端tls- client.Search err: ", err)
	}

	logging.DebugInfo("客户端tls- resp: ", resp.GetResponse())
}

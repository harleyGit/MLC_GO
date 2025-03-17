/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-16 13:44:58
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-17 17:54:34
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
	// gRPCSimpleClient_test_v2()
	// gRPCSimpleClient_test_v3()
	// gRPCSimpleClient_test_v5()
	gRPCSimpleClient_test_v6()
}

// gRPC自定义认证客户端(和simple_server/server.go文件的 gRPCServerPractice_v6 方法对应))
func gRPCSimpleClient_test_v6() {
	tlsClient := gtls.Client{
		ServerName: "HuangGang.dev.use", // 需要与 服务器证书的 CN（Common Name）匹配。
		CertFile:   "../../conf/server/server.pem", // 使用服务器的 CA 证书 进行认证。
	}
	c, err := tlsClient.GetTLSCredentials()
	if err != nil {
		logging.ErrInfo("gRPC自定义认证客户端>>>tlsClient.GetTLSCredentials err: ", err)
	}

	auth := Auth{
		AppKey:    "eddycjy",
		AppSecret: "20181005",
	}
	// grpc.WithTransportCredentials(c) 开启 TLS 加密 连接
	// grpc.WithPerRPCCredentials(&auth) 每次 gRPC 请求都携带 app_key 和 app_secret。
	conn, err := grpc.Dial(":"+PORT, grpc.WithTransportCredentials(c), grpc.WithPerRPCCredentials(&auth))
	if err != nil {
		logging.ErrInfo("gRPC自定义认证客户端>>>grpc.Dial err: ", err)
	}
	defer conn.Close()

	client := pb.NewSearchServiceClient(conn)
	// 调用 Search 方法，发送 gRPC 请求
	resp, err := client.Search(context.Background(), &pb.SearchRequest{
		Request: "gRPC",
	})
	if err != nil {
		logging.ErrInfo("gRPC自定义认证客户端>>>>client.Search err: ", err)
	}

	logging.DebugInfo("gRPC自定义认证客户端>>>>resp: %s", resp.GetResponse())
}
func (a *Auth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	// 返回 app_key 和 app_secret，让 gRPC 自动添加到 metadata 请求头中
	return map[string]string{"app_key": a.AppKey, "app_secret": a.AppSecret}, nil
}
// 强制使用 TLS
func (a *Auth) RequireTransportSecurity() bool {
	return true
}
type Auth struct {
	AppKey    string
	AppSecret string
}

// <<<<<<<分隔符1======================================

// gRPC提供http接口的客户端(和simple_server/server.go文件的 gRPCServerPractice_v5 方法对应)
func gRPCSimpleClient_test_v5() {
	tlsClient := gtls.Client{
		ServerName: "HuangGang.dev.use",
		CertFile:   "../../conf/server/server.pem",
	}
	c, err := tlsClient.GetTLSCredentials()
	if err != nil {
		logging.ErrInfo("gRPC提供http接口的客户端>>>tlsClient.GetTLSCredentials err: ", err)
	}

	// 建立 TLS 连接
	conn, err := grpc.Dial(":"+PORT, grpc.WithTransportCredentials(c))
	if err != nil {
		logging.ErrInfo("gRPC提供http接口的客户端>>>grpc.Dial err: ", err)
	}
	defer conn.Close()

	client := pb.NewSearchServiceClient(conn)
	resp, err := client.Search(context.Background(), &pb.SearchRequest{
		Request: "gRPC提供http接口的客户端🍊🍊🍊gRPC",
	})
	if err != nil {
		logging.ErrInfo("gRPC提供http接口的客户端>>>client.Search err: ", err)
	}

	logging.DebugInfo("gRPC提供http接口的客户端>>>resp: ", resp.GetResponse())
}

// 基于CA的TLS证书认证的客户端(和simple_server/server.go文件的 gRPCServerPractice_v3 方法对应)
func gRPCSimpleClient_test_v3() {
	tlsClient := gtls.Client{
		CaFile:     "../../conf/ca.pem",
		CertFile:   "../../conf/client/client.pem",
		KeyFile:    "../../conf/client/client.key",
	}

	c, err := tlsClient.GetCredentialsByCA()
	if err != nil {
		logging.ErrInfo("基于CA的TLS证书认证的客户端tls- GetTLSCredentialsByCA err: ", err)
	}

	conn, err := grpc.Dial(":"+PORT, grpc.WithTransportCredentials(c))
	if err != nil {
		logging.ErrInfo("基于CA的TLS证书认证的客户端grpc.Dial err: ", err)
	}
	defer conn.Close()

	client := pb.NewSearchServiceClient(conn)
	resp, err := client.Search(context.Background(), &pb.SearchRequest{
		Request: "gRPC🍎🍎",
	})
	if err != nil {
		logging.ErrInfo("基于CA的TLS证书认证的客户端client.Search err: ", err)
	}

	logging.DebugInfo("基于CA的TLS证书认证的客户端resp: ", resp.GetResponse())
}

// 加入TLS证书认证的客户端(和simple_server/server.go文件的 gRPCServerPractice_v2 方法对应)
func gRPCSimpleClient_test_v2() {
	tlsClient := gtls.Client{
		ServerName: "HuangGang.dev.use",
		CaFile:     "../../conf/ca.pem",
		CertFile:   "../../conf/server/server.pem",
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



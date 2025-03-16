/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-03 20:24:58
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-16 11:45:06
 * @FilePath: /MLC_GO/TestNotes/PracticeGRPCExample/client/client.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
// 编写测试客户端
package main

import (
	pb "MLC_GO/TestNotes/gRPC_practice/gRPC_practice_v1/proto/github.com/your-username/your-repo/grpc-hello-world/proto"
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"os"

	"golang.org/x/net/context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// 测试：
//
//	cd ./TestNotes/PracticeGRPCExample/client
//	go run client.go
func main() {
	// 获取当前工作目录
	dir, err := os.Getwd()
	if err != nil {
		logging.ErrInfo("Error getting working directory:", err)
		return
	}
	logging.DebugInfo("2--------在 Go 中使用 os.ReadFile(\"example.txt\") 读取文件时，" + 
	"相对路径是相对于程序的 当前工作目录，当前工作目录路径:", dir)


	// 1. 加载 TLS 证书
	// 作者博客有问题：可以是localhost 或者 dev
	creds, err := credentials.NewClientTLSFromFile("../certs/client_server.pem", "localhost")
	if err != nil {
		logging.ErrInfo("Failed to create TLS credentials %v", err)
	}
	// 2. 建立连接, ":50052", // 明确指定主机
	conn, err := grpc.Dial(":50052", grpc.WithTransportCredentials(creds))
	defer conn.Close()

	if err != nil {
		logging.ErrInfo("1---:", err)
	}
	// 3. 创建客户端
	c := pb.NewHelloWorldClient(conn)
	context := context.Background()
	body := &pb.HelloWorldRequest{
		Referer: "Grpc",
	}

	_, err = c.SayHelloWorld(context, body)
	if err != nil {
		logging.ErrInfo("2---:", err)
	}

	// logging.Debug(r.Message)
}

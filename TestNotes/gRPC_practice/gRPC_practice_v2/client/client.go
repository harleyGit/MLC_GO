/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-15 20:45:16
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-16 11:39:56
 * @FilePath: /MLC_GO/TestNotes/gRPC_practice/client/client.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
/// title: 客户端启动
package main

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	pb "MLC_GO/TestNotes/gRPC_practice/gRPC_practice_v2/proto"
	"context"

	"google.golang.org/grpc"
)

const PORT = "9001"

func main() {
	// 创建与给定目标（服务端）的连接交互
	conn, err := grpc.Dial(":"+PORT, grpc.WithInsecure())
	if err != nil {
		logging.ErrInfo("grpc.Dial err: ", err)
	}
	defer conn.Close()

	// 创建 SearchService 的客户端对象
	client := pb.NewSearchServiceClient(conn)
	resp, err := client.Search(context.Background(), &pb.SearchRequest{
		Request: "gRPC", //发送 RPC 请求，等待同步响应，得到回调后返回响应结果
	})
	if err != nil {
		logging.ErrInfo("grpc 客户端 client.Search err: ", err)
	}

	// 输出响应结果
	logging.DebugInfo("响应 resp: ", resp.GetResponse())
}

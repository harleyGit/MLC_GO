/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-03 17:02:10
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-03 23:15:41
 * @FilePath: /MLC_GO/TestNotes/PracticeGRPCExample/server/hello.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package server

import (
	pb "MLC_GO/TestNotes/gRPC_practice/gRPC_practice_v1/proto/github.com/your-username/your-repo/grpc-hello-world/proto"

	"golang.org/x/net/context"
)

type helloService struct {
	pb.UnimplementedHelloWorldServer // ✅ 嵌入生成的 Unimplemented 结构体
}

func NewHelloService() *helloService {
	return &helloService{}
}

/*
ctx context.Context用于接受上下文参数、r *pb.HelloWorldRequest用于接受protobuf的Request参数（对应.proto的message HelloWorldRequest）
*/
func (h helloService) SayHelloWorld(ctx context.Context, r *pb.HelloWorldRequest) (*pb.HelloWorldResponse, error) {
	return &pb.HelloWorldResponse{
		Message: "restful_api(‼️🍎 作者这个地方有问题，注意了)",
	}, nil
}

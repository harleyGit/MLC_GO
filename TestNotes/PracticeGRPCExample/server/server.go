/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-03 16:21:27
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-03 21:54:42
 * @FilePath: /MLC_GO/TestNotes/PracticeGRPCExample/server/server.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package server

import (
	"MLC_GO/TestNotes/PracticeGRPCExample/pkg/util"
	pb "MLC_GO/TestNotes/PracticeGRPCExample/proto/github.com/your-username/your-repo/grpc-hello-world/proto"
	"MLC_GO/TestNotes/PracticeGenExample/pkg/logging"
	"crypto/tls"
	"net"
	"net/http"

	"golang.org/x/net/context"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	"google.golang.org/grpc"
	// credentials的第三方包，它实现了grpc库支持的各种凭证，该凭证封装了客户机需要的所有状态，以便与服务器进行身份验证并进行各种断言，例如关于客户机的身份，角色或是否授权进行特定的呼叫
	"google.golang.org/grpc/credentials"
)

var (
	ServerPort string
	CertName string
	CertPemPath string
	CertKeyPath string
	EndPoint string
)

func Serve() (err error) {

	EndPoint = ":" + ServerPort
	// 用于监听本地的网络地址通知，它的函数原型func Listen(network, address string) (Listener, error)
	/* 
	最后net.Listen会返回一个监听器的结构体，返回给接下来的动作，让其执行下一步的操作，它可以执行三类操作
		Accept：接受等待并将下一个连接返回给Listener
		Close：关闭Listener
		Addr：返回Listener的网络地址
	*/
	conn, err := net.Listen("tcp", EndPoint)
	if err != nil {
		logging.DebugInfo("TCP Listen err: %v", err)
	}
	// util.GetTLSConfig解析得到tls.Config，传达给http.Server服务的TLSConfig配置项使用
	tlsConfig := util.GetTLSConfig(CertPemPath, CertKeyPath)
	srv := createInternalServer(conn, tlsConfig)

	logging.DebugInfo("gRPC and https listen on: ", ServerPort, "\nCertPemPath:", CertPemPath, "CertKeyPath:", CertKeyPath)

	if  err = srv.Serve(tls.NewListener(conn, tlsConfig)); err != nil {
		logging.DebugInfo("ListenAndServe: %v\n", err)
	}

	return err

	/* logging.Info(ServerPort)

	logging.Info(CertName)

	logging.Info(CertPemPath)

	logging.Info(CertKeyPath)
 */
	return nil
}

func createInternalServer(conn net.Listener, tlsCOnfig *tls.Config) (*http.Server) {
	var opts []grpc.ServerOption

	// grpc server
	creds, err := credentials.NewServerTLSFromFile(CertPemPath, CertKeyPath)
	if err != nil {
		logging.DebugInfo("Failed to create server TLS credentials %v", err)
	}

	// Creds 该函数返回ServerOption，它为服务器连接设置凭据
	opts = append(opts, grpc.Creds(creds))
	// 创建了一个没有注册服务的grpc服务端，还没有开始接受请求
	grpcServer := grpc.NewServer(opts...)

	// register grpc pb
	// 注册grpc服务
	pb.RegisterHelloWorldServer(grpcServer, NewHelloService())

	// gw server
	// 创建grpc-gateway关联组件
	ctx := context.Background() //返回一个非空的空上下文。它没有被注销，没有值，没有过期时间。它通常由主函数、初始化和测试使用，并作为传入请求的顶级上下文
	// 从输入证书文件和服务器的密钥文件构造TLS证书凭证
	dcreds, err := credentials.NewClientTLSFromFile(CertPemPath, CertName) //从客户机的输入证书文件构造TLS凭证
	if err != nil {
		logging.DebugInfo("Failed to create client TLS credentials %v", err)
	}
	logging.DebugInfo("server/server.go CertPemPath:", CertPemPath, "CertName: ", CertName)
	
	// grpc.WithTransportCredentials：配置一个连接级别的安全凭据(例：TLS、SSL)，返回值为type DialOption
	// grpc.DialOption：DialOption选项配置我们如何设置连接（其内部具体由多个的DialOption组成，决定其设置连接的内容）
	dopts := []grpc.DialOption{grpc.WithTransportCredentials(dcreds)}
	// 6、创建HTTP NewServeMux及注册grpc-gateway逻辑
	// runtime.NewServeMux：返回一个新的ServeMux，它的内部映射是空的；ServeMux是grpc-gateway的一个请求多路复用器。它将http请求与模式匹配，并调用相应的处理程序
	gwmux := runtime.NewServeMux()
	
	// register grpc-gateway pb
	// RegisterHelloWorldHandlerFromEndpoint：如函数名，注册HelloWorld服务的HTTP Handle到grpc端点
	if err := pb.RegisterHelloWorldHandlerFromEndpoint(ctx, gwmux, EndPoint, dopts); err != nil {
		logging.ErrInfo("Failed to register gw server: %v\n", err)
	}

	// http服务
	// http.NewServeMux：分配并返回一个新的ServeMux
	mux := http.NewServeMux()
	// 为给定模式注册处理程序
	mux.Handle("/", gwmux)

	return &http.Server{
		Addr: EndPoint,
		Handler: util.GrpcHandleFunc(grpcServer, mux),
		TLSConfig: tlsCOnfig,
	}
}
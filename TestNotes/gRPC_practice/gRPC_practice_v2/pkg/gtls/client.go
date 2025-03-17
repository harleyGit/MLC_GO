/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-17 10:26:28
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-17 10:46:56
 * @FilePath: /MLC_GO/TestNotes/gRPC_practice/gRPC_practice_v2/pkg/gtls/client.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */

//title: 客户端代码定义了一个 Client 结构体以及两个获取 TLS 凭证的方法。
package gtls


import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"

	"google.golang.org/grpc/credentials"
)

type Client struct {
	// 指定要连接的服务端域名或 IP。在 TLS 握手中，会比对服务端证书中的 CN 或 SAN 是否与之匹配。
	ServerName string	// 服务器的主机名，用于验证服务端证书中的主机名
	// CA 根证书文件，客户端利用它来验证服务端发送的证书是否合法（是否由该 CA 签发）。
	CaFile     string	// CA 根证书文件路径，用于验证服务端证书是否由受信任的 CA 签发
	
	// 客户端证书和私钥，用于实现双向认证（可选，如果服务端要求客户端提供证书）。
	CertFile   string	// 客户端证书文件路径，用于双向 TLS 时客户端身份认证
	KeyFile    string	// 客户端私钥文件路径，与客户端证书对应
}

func (t *Client) GetCredentialsByCA() (credentials.TransportCredentials, error) {
	
	// 加载客户端证书:
	// 		调用 tls.LoadX509KeyPair(t.CertFile, t.KeyFile) 从指定路径加载客户端证书和对应私钥，返回一个 tls.Certificate 对象。
	// 		这对于双向 TLS（mutual TLS）很重要。
	// 加载客户端证书和私钥
	cert, err := tls.LoadX509KeyPair(t.CertFile, t.KeyFile)
	if err != nil {
		return nil, err
	}

	// 建立证书池
	// 		使用 x509.NewCertPool() 创建一个新的证书池（CertPool），用于存放受信任的 CA 证书。
	// 		通过 ioutil.ReadFile(t.CaFile) 读取 CA 根证书文件内容（PEM 格式），然后用 AppendCertsFromPEM 将证书添加到证书池中。
	// 创建一个新的证书池，用于存放受信任的 CA 根证书
	certPool := x509.NewCertPool()
	// 读取 CA 根证书文件
	ca, err := ioutil.ReadFile(t.CaFile)
	if err != nil {
		return nil, err
	}

	// 将 CA 根证书追加到证书池中
	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		return nil, err
	}

	// 构造 TLS 配置
	// 		使用 credentials.NewTLS(&tls.Config{...}) 构造一个 TLS 配置：
	// 		Certificates：设置了客户端的证书列表（通常只有一对证书）。
	// 		ServerName：设置服务器名称，用于主机名验证，确保连接的服务器与证书匹配。
	// 		RootCAs：设置 CA 证书池，用于验证服务端证书是否由该 CA 签发。
	// 构造一个 TLS 配置，包含客户端证书、服务器名称验证和受信任的 CA 列表
	c := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		ServerName:   t.ServerName,
		RootCAs:      certPool,
	})

	return c, err
}

func (t *Client) GetTLSCredentials() (credentials.TransportCredentials, error) {
	
	// 这里使用 credentials.NewClientTLSFromFile 方法直接从指定的证书文件中加载 TLS 凭证，适用于只需要验证服务端身份，而不需要双向验证的场景。
	// 参数中：
	// 		第一个参数为证书文件路径（通常是服务端证书，也可配置为 CA 证书，视实际需求而定）。
	// 		第二个参数为服务器名称，用于验证服务端证书中的主机名。
	// 从文件中直接加载 TLS 客户端凭证，自动使用指定 ServerName 验证服务端证书
	c, err := credentials.NewClientTLSFromFile(t.CertFile, t.ServerName)
	if err != nil {
		return nil, err
	}

	return c, err
}

/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-17 10:26:35
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-17 15:09:21
 * @FilePath: /MLC_GO/TestNotes/gRPC_practice/gRPC_practice_v2/pkg/gtls/server.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */

// title: 服务端代码定义了一个 Server 结构体和两个获取 TLS 凭证的方法，目的是对客户端进行身份验证（如果开启双向 TLS）以及为服务端提供安全通信。
package gtls

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"

	"google.golang.org/grpc/credentials"
)

type Server struct {
	// CaFile：服务端用于验证客户端证书的 CA 文件。
	// 		如果开启双向 TLS，客户端也需要提供证书，服务端会检查其是否由受信任的 CA 签发。
	CaFile string // CA 根证书文件路径，用于验证客户端证书

	// CertFile、KeyFile：服务端证书和私钥，用于在 TLS 握手中向客户端证明自身身份。
	CertFile string // 服务端证书文件路径
	KeyFile  string // 服务端私钥文件路径
}

func (t *Server) GetCredentialsByCA() (credentials.TransportCredentials, error) {

	// 加载服务端证书
	// 		使用 tls.LoadX509KeyPair 从 t.CertFile 和 t.KeyFile 中加载服务端的证书和私钥。
	// 加载服务端证书和私钥
	cert, err := tls.LoadX509KeyPair(t.CertFile, t.KeyFile)
	if err != nil {
		return nil, err
	}

	// 创建一个证书池，用于存放客户端受信任的 CA 证书
	certPool := x509.NewCertPool()
	// 建立客户端 CA 证书池
	// 		创建新的证书池，然后读取 t.CaFile 中的 CA 根证书，将其加入池中，用于验证客户端提交的证书。
	// 读取 CA 文件内容
	ca, err := ioutil.ReadFile(t.CaFile)
	if err != nil {
		return nil, err
	}

	// 将 CA 证书追加到证书池中
	// 尝试解析所传入的 PEM 编码的证书。如果解析成功会将其加到 CertPool 中，便于后面的使用
	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		return nil, errors.New("certPool.AppendCertsFromPEM err")
	}

	// 	构造 TLS 配置
	// 		Certificates: 服务端用于自身身份认证的证书列表(设置证书链，允许包含一个或多个)。
	// 		ClientAuth: tls.RequireAndVerifyClientCert: 强制要求客户端提供证书(要求必须校验客户端的证书)，并且验证客户端证书是否有效（双向 TLS）。
	// 		ClientCAs: 指定信任的客户端 CA 证书池(设置根证书的集合，校验方式使用 ClientAuth 中设定的模式)。
	// credentials.NewTLS：构建基于 TLS 的 TransportCredentials 选项
	// tls.Config：Config 结构用于配置 TLS 客户端或服务器
	// 构造 TLS 配置，启用双向认证
	c := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		// 启用双向 TLS，要求客户端也提供证书，并进行验证
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  certPool,
	})

	return c, err
}

// 与客户端对应，此方法调用 credentials.NewServerTLSFromFile 直接从服务端证书文件和私钥文件中加载 TLS 凭证。
// 这种方式通常只适用于单向 TLS（服务端提供证书，但不要求客户端验证证书）。
// 返回的 TLS 凭证可用于启动 gRPC 服务器时的安全配置。
func (t *Server) GetTLSCredentials() (credentials.TransportCredentials, error) {
	// 从文件中直接加载服务端 TLS 凭证
	c, err := credentials.NewServerTLSFromFile(t.CertFile, t.KeyFile)
	if err != nil {
		return nil, err
	}

	return c, err
}

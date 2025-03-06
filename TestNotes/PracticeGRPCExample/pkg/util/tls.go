/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-03 17:28:45
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-06 18:00:38
 * @FilePath: /MLC_GO/TestNotes/PracticeGRPCExample/pkg/util/tls.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package util

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"crypto/tls"
	"os"

	"golang.org/x/net/http2"
)

// 用于获取TLS配置，在内部，我们读取了server.key和server.pem这类证书凭证文件
// 用于处理从证书凭证文件（PEM），最终获取tls.Config作为HTTP2的使用参数
func GetTLSConfig(certPemPath, certKeyPath string) *tls.Config {
	var certKeyPair *tls.Certificate
	cert,_ := os.ReadFile(certPemPath)
	key, _ := os.ReadFile(certKeyPath)

	// tls.X509KeyPair：从一对PEM编码的数据中解析公钥/私钥对。成功则返回公钥/私钥对
	pair, err := tls. X509KeyPair(cert, key)
	if err != nil {
		logging.DebugInfo("🍎 TLS KeyPair err: ", err)
	}
	certKeyPair = &pair
	return &tls.Config{
		// tls.Certificate：返回一个或多个证书，实质我们解析PEM调用的X509KeyPair的函数声明就是func X509KeyPair(certPEMBlock, keyPEMBlock []byte) (Certificate, error)，返回值就是Certificate
		Certificates: []tls.Certificate{*certKeyPair},
		// http2.NextProtoTLS: NextProtoTLS是谈判期间的NPN/ALPN协议，用于HTTP/2的TLS设置
		NextProtos: []string{http2.NextProtoTLS},
	}
}
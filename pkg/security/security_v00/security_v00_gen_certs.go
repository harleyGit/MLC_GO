/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-08-24 08:10:07
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-08-24 10:09:50
 * @FilePath: /MLC_GO/.vscode/security/security_v00/security_gen_certs.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 *
 * 用途： 用 Go 生成 CA、server/client 证书（RSA）
 * security_v00_gen_certs.go
 * 生成 CA、server、client 的 RSA 证书与私钥（PEM 文件）
 * 生成的文件： ca.pem, ca.key, server.pem, server.key, client.pem, client.key
 */

/* 这个文件做了什么事？
* 我们生成了一个自签 CA（`ca.pem` / `ca.key`），然后用 CA 签发了 `server.pem` 和 `client.pem`。
* 证书使用 RSA 2048（可以用作加密/解密），便于演示 RSA-OAEP 对称密钥封装。
* 私钥文件权限写成 `0600`（安全）。
* 如果你想让证书有效期更长/短，改 `NotAfter` 即可。
*
* 优化地方： 将证书和文件写在固定文件夹内，不要放在根目录： ./security/security_v00/xxx
 */

package securityv00

import (
	"MLC_GO/pkg/logHG"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"time"
)

func mustWriteFile(path string, data []byte, perm os.FileMode) {
	if err := os.WriteFile(path, data, perm); err != nil {
		panic(err)
	}
}

// 返回 PEM 编码的证书与私钥文件内容
func generateRSAKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

func createCertificate(template, parent *x509.Certificate, pub interface{}, parentPriv interface{}) ([]byte, error) {
	return x509.CreateCertificate(rand.Reader, template, parent, pub, parentPriv)
}

func pemEncodeCert(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func pemEncodePrivateKeyRSA(priv *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
}

func Security_v00_Gen_Certs_Main() {
	logHG.DebugInfo("Generating CA, server and client certificates...")

	// 1) 生成 CA key & cert (自签)
	caPriv, _ := generateRSAKey()
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Example CA Org"},
			CommonName:   "Example-Root-CA",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	caDer, err := createCertificate(caTemplate, caTemplate, &caPriv.PublicKey, caPriv)
	if err != nil {
		panic(err)
	}
	caPEM := pemEncodeCert(caDer)
	caKeyPEM := pemEncodePrivateKeyRSA(caPriv)
	mustWriteFile("ca.pem", caPEM, 0644)
	mustWriteFile("ca.key", caKeyPEM, 0600)
	logHG.DebugInfo(" -> ca.pem, ca.key generated")

	// 2) 生成 server key & cert, 用 CA 签名
	serverPriv, _ := generateRSAKey()
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"Example Server Org"},
			CommonName:   "localhost",
		},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{"localhost"},
	}
	serverDer, err := createCertificate(serverTemplate, caTemplate, &serverPriv.PublicKey, caPriv)
	if err != nil {
		panic(err)
	}
	serverPEM := pemEncodeCert(serverDer)
	serverKeyPEM := pemEncodePrivateKeyRSA(serverPriv)
	mustWriteFile("server.pem", serverPEM, 0644)
	mustWriteFile("server.key", serverKeyPEM, 0600)
	logHG.DebugInfo(" -> server.pem, server.key generated")

	// 3) 生成 client key & cert, 用 CA 签名（允许 clientAuth）
	clientPriv, _ := generateRSAKey()
	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			Organization: []string{"Example Client Org"},
			CommonName:   "example-client",
		},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientDer, err := createCertificate(clientTemplate, caTemplate, &clientPriv.PublicKey, caPriv)
	if err != nil {
		panic(err)
	}
	clientPEM := pemEncodeCert(clientDer)
	clientKeyPEM := pemEncodePrivateKeyRSA(clientPriv)
	mustWriteFile("client.pem", clientPEM, 0644)
	mustWriteFile("client.key", clientKeyPEM, 0600)
	logHG.DebugInfo(" -> client.pem, client.key generated")

	logHG.DebugInfo("All certs generated: ca.pem, server.pem, server.key, client.pem, client.key")
}

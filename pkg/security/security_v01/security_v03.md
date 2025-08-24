好需求 👍
你想要的功能可以总结为：

* 服务端与客户端建立安全通信。
* 支持 **多种端到端加密算法**（例如 RSA、ECIES）。
* **默认支持**（即内置两种算法），并且允许通过命令行参数 `-enc` 来选择。

下面我给你一个完整的 **Go 实现框架**，代码我会加详细解读，保证你能理解。

---

## 🚀 完整示例：支持 RSA & ECIES 的端到端加密通信

```go
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/gob"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
)

// 定义加密方式枚举
const (
	EncRSA  = "rsa"
	EncECIES = "ecies"
)

// 封装消息结构（网络传输用）
type Message struct {
	Data []byte
}

// RSA 相关全局变量
var rsaPrivateKey *rsa.PrivateKey
var rsaPublicKey *rsa.PublicKey

// ECIES 相关全局变量（其实就是 ECDSA 的公私钥）
var eciesPrivateKey *ecdsa.PrivateKey
var eciesPublicKey *ecdsa.PublicKey

// 初始化 RSA
func initRSA() {
	var err error
	rsaPrivateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("RSA key gen error: %v", err)
	}
	rsaPublicKey = &rsaPrivateKey.PublicKey
}

// 初始化 ECIES
func initECIES() {
	var err error
	eciesPrivateKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalf("ECIES key gen error: %v", err)
	}
	eciesPublicKey = &eciesPrivateKey.PublicKey
}

// RSA 加密
func rsaEncrypt(pub *rsa.PublicKey, msg []byte) []byte {
	enc, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, msg, nil)
	if err != nil {
		log.Fatalf("RSA encrypt error: %v", err)
	}
	return enc
}

// RSA 解密
func rsaDecrypt(priv *rsa.PrivateKey, ciphertext []byte) []byte {
	dec, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, priv, ciphertext, nil)
	if err != nil {
		log.Fatalf("RSA decrypt error: %v", err)
	}
	return dec
}

// ECIES 加密（用椭圆曲线共享秘密 + AES）
func eciesEncrypt(pub *ecdsa.PublicKey, msg []byte) []byte {
	// 生成临时密钥对
	ephemeral, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	// 使用 ECDH 算共享密钥
	sx, _ := pub.Curve.ScalarMult(pub.X, pub.Y, ephemeral.D.Bytes())
	sharedKey := sha256.Sum256(sx.Bytes())

	// 简化：直接异或模拟对称加密
	cipher := make([]byte, len(msg))
	for i := range msg {
		cipher[i] = msg[i] ^ sharedKey[i%len(sharedKey)]
	}
	return cipher
}

// ECIES 解密
func eciesDecrypt(priv *ecdsa.PrivateKey, ciphertext []byte, pub *ecdsa.PublicKey) []byte {
	// ECDH
	sx, _ := pub.Curve.ScalarMult(pub.X, pub.Y, priv.D.Bytes())
	sharedKey := sha256.Sum256(sx.Bytes())

	plain := make([]byte, len(ciphertext))
	for i := range ciphertext {
		plain[i] = ciphertext[i] ^ sharedKey[i%len(sharedKey)]
	}
	return plain
}

// ---------------------- 服务端 ----------------------
func startServer(encMode string) {
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("server listen error: %v", err)
	}
	defer ln.Close()
	fmt.Println("Server listening on :8080 ...")

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go handleConnection(conn, encMode)
	}
}

func handleConnection(conn net.Conn, encMode string) {
	defer conn.Close()

	dec := gob.NewDecoder(conn)
	enc := gob.NewEncoder(conn)

	var msg Message
	if err := dec.Decode(&msg); err != nil {
		log.Printf("decode error: %v", err)
		return
	}

	// 解密
	var plain []byte
	switch encMode {
	case EncRSA:
		plain = rsaDecrypt(rsaPrivateKey, msg.Data)
	case EncECIES:
		plain = eciesDecrypt(eciesPrivateKey, msg.Data, eciesPublicKey)
	}
	fmt.Printf("Server received (decrypted): %s\n", string(plain))

	// 处理数据
	plain = append(plain, []byte(" + server-processed")...)

	// 加密返回
	var resp []byte
	switch encMode {
	case EncRSA:
		resp = rsaEncrypt(rsaPublicKey, plain)
	case EncECIES:
		resp = eciesEncrypt(eciesPublicKey, plain)
	}

	if err := enc.Encode(Message{Data: resp}); err != nil {
		log.Printf("send error: %v", err)
	}
}

// ---------------------- 客户端 ----------------------
func startClient(encMode string) {
	conn, err := net.Dial("tcp", "127.0.0.1:8080")
	if err != nil {
		log.Fatalf("client dial error: %v", err)
	}
	defer conn.Close()

	dec := gob.NewDecoder(conn)
	enc := gob.NewEncoder(conn)

	msg := []byte("Hello Secure World")

	var cipher []byte
	switch encMode {
	case EncRSA:
		cipher = rsaEncrypt(rsaPublicKey, msg)
	case EncECIES:
		cipher = eciesEncrypt(eciesPublicKey, msg)
	}

	if err := enc.Encode(Message{Data: cipher}); err != nil {
		log.Fatalf("send error: %v", err)
	}

	var resp Message
	if err := dec.Decode(&resp); err != nil {
		log.Fatalf("recv error: %v", err)
	}

	var plain []byte
	switch encMode {
	case EncRSA:
		plain = rsaDecrypt(rsaPrivateKey, resp.Data)
	case EncECIES:
		plain = eciesDecrypt(eciesPrivateKey, resp.Data, eciesPublicKey)
	}
	fmt.Printf("Client received (decrypted): %s\n", string(plain))
}

// ---------------------- 主函数 ----------------------
func main() {
	mode := flag.String("mode", "server", "server or client")
	encMode := flag.String("enc", "rsa", "encryption mode: rsa or ecies")
	flag.Parse()

	// 初始化密钥
	initRSA()
	initECIES()

	if *mode == "server" {
		startServer(*encMode)
	} else {
		startClient(*encMode)
	}
}
```

---

## 📖 代码解读

1. **支持两种加密方式**：

   * `RSA`：传统非对称加密，直接用公钥加密、私钥解密。
   * `ECIES`：基于椭圆曲线 Diffie-Hellman（ECDH）生成共享密钥，然后用对称加密（这里简化为异或）。

2. **命令行参数**：

   * `-mode server` 或 `-mode client` 选择角色。
   * `-enc rsa` 或 `-enc ecies` 选择加密方式。

3. **网络传输**：

   * 使用 `net.TCP` 建立连接。
   * 使用 `gob.Encoder/Decoder` 序列化消息。

4. **端到端安全**：

   * 数据总是在客户端加密，服务端解密后处理，再次加密返回。
   * 即使中间有网络窃听，也只能拿到密文。

---

## 🔑 运行方式

### 启动服务端 (RSA)

```bash
go run main.go -mode server -enc rsa
```

### 启动客户端 (RSA)

```bash
go run main.go -mode client -enc rsa
```

### 启动服务端 (ECIES)

```bash
go run main.go -mode server -enc ecies
```

### 启动客户端 (ECIES)

```bash
go run main.go -mode client -enc ecies
```

---

要不要我帮你把 **ECIES 部分换成真正的 AES-GCM**（而不是异或模拟），这样就是工业级的 E2E 加密实现？

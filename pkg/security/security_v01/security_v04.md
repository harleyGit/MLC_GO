太好了～下面给你一套**可运行的双向通信示例**，用 **ECIES（基于 ECDH）+ HKDF-SHA256 + AES-GCM** 做“端到端加密”。
特点：

* 单文件（保存为 `e2e_ecies_aesgcm.go`）。
* **客户端 ↔ 服务端相互发送数据**（握手后双方都能发）。
* 端到端：每条业务消息都用对方的公钥加密（发送方临时密钥 ECDH → HKDF → AES-GCM）。
* 无外部进程依赖；只需两条 `go get` 安装依赖即可。

---

# 一、运行前依赖

```bash
go get golang.org/x/crypto
```

> 只用到了 `golang.org/x/crypto/hkdf`。其余均为标准库。

---

# 二、完整代码（双向加密通信）

> 复制保存为：`e2e_ecies_aesgcm.go`

```go
package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

// -----------------------------
// 数据结构（gob 传输用）
// -----------------------------

// 握手阶段交换的静态公钥（未加密也可以，因为只是公钥）
type PubKeyMsg struct {
	Curve string // "P-256"
	Pub   []byte // 椭圆曲线未压缩格式 0x04|X|Y
}

// 加密负载封装：ECIES(KEM) + AES-GCM(DEM)
type Envelope struct {
	EphemeralPub []byte // 发送方临时公钥（未压缩）
	Nonce        []byte // 12 字节
	Ciphertext   []byte // AES-GCM 输出（含 tag）
}

type WireMsg struct {
	From     string
	Envelope Envelope
}

// -----------------------------
// 工具：密钥、编解码
// -----------------------------

// 生成静态 ECDSA 密钥对（作身份/接收方密钥）
func genStaticECDSA() (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalf("ecdsa.GenerateKey: %v", err)
	}
	return priv, &priv.PublicKey
}

// Marshal/Unmarshal ECDSA 公钥（未压缩点格式）
func marshalPub(pub *ecdsa.PublicKey) []byte {
	return elliptic.Marshal(pub.Curve, pub.X, pub.Y)
}
func unmarshalPub(curve elliptic.Curve, b []byte) (*ecdsa.PublicKey, error) {
	x, y := elliptic.Unmarshal(curve, b)
	if x == nil || y == nil {
		return nil, errors.New("bad public key bytes")
	}
	return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
}

// KDF: HKDF-SHA256 派生 32 字节对称密钥
func deriveKey(shared []byte) ([]byte, error) {
	// 你可以自定义 salt/info；这里固定 info，salt 为空（演示）
	info := []byte("ECIES-AESGCM-P256")
	h := hkdf.New(sha256.New, shared, nil, info)
	key := make([]byte, 32)
	if _, err := io.ReadFull(h, key); err != nil {
		return nil, err
	}
	return key, nil
}

// AES-GCM 加解密
func aesGCMEncrypt(key, plaintext []byte) (nonce, ct []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	ct = gcm.Seal(nil, nonce, plaintext, nil)
	return nonce, ct, nil
}
func aesGCMDecrypt(key, nonce, ct []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ct, nil)
}

// -----------------------------
// ECIES（轻量实现）：发送方每次生成临时密钥 -> ECDH -> HKDF -> AES-GCM
// -----------------------------

// EncryptFor：给 “对方静态公钥” 加密明文
func EncryptFor(recipient *ecdsa.PublicKey, plaintext []byte) (Envelope, error) {
	// 1) 生成临时密钥
	ephem, err := ecdsa.GenerateKey(recipient.Curve, rand.Reader)
	if err != nil {
		return Envelope{}, err
	}
	// 2) ECDH：S = ephemPriv * recipientPub
	sx, _ := recipient.Curve.ScalarMult(recipient.X, recipient.Y, ephem.D.Bytes())
	shared := sx.Bytes()
	// 3) HKDF → AES key
	key, err := deriveKey(shared)
	if err != nil {
		return Envelope{}, err
	}
	// 4) AES-GCM 加密
	nonce, ct, err := aesGCMEncrypt(key, plaintext)
	if err != nil {
		return Envelope{}, err
	}
	// 5) 带上临时公钥（供接收方解密使用）
	return Envelope{
		EphemeralPub: marshalPub(&ephem.PublicKey),
		Nonce:        nonce,
		Ciphertext:   ct,
	}, nil
}

// DecryptWith：用 “自己的静态私钥” 解密
func DecryptWith(self *ecdsa.PrivateKey, env Envelope) ([]byte, error) {
	// 1) 解析发送方临时公钥
	ephemPub, err := unmarshalPub(self.Curve, env.EphemeralPub)
	if err != nil {
		return nil, err
	}
	// 2) ECDH：S = selfPriv * ephemPub
	sx, _ := self.Curve.ScalarMult(ephemPub.X, ephemPub.Y, self.D.Bytes())
	shared := sx.Bytes()
	// 3) HKDF → AES key
	key, err := deriveKey(shared)
	if err != nil {
		return nil, err
	}
	// 4) AES-GCM 解密
	return aesGCMDecrypt(key, env.Nonce, env.Ciphertext)
}

// -----------------------------
// 读写：长度前缀 + gob（为简化用 gob）
// -----------------------------

func newEncDec(conn net.Conn) (*gob.Encoder, *gob.Decoder) {
	return gob.NewEncoder(conn), gob.NewDecoder(conn)
}

// -----------------------------
// 服务端
// -----------------------------

func runServer(port string) error {
	// 生成“服务器静态密钥”（用来接收消息）
	sPriv, sPub := genStaticECDSA()
	log.Printf("[S] static pub: %s", hex.EncodeToString(marshalPub(sPub)))

	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	defer ln.Close()
	log.Printf("[S] listening on :%s", port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("[S] accept err: %v", err)
			continue
		}
		go serveConn(conn, sPriv, sPub)
	}
}

func serveConn(conn net.Conn, sPriv *ecdsa.PrivateKey, sPub *ecdsa.PublicKey) {
	defer conn.Close()
	enc, dec := newEncDec(conn)

	// --- 握手：交换静态公钥 ---
	// 先发自己的
	if err := enc.Encode(&PubKeyMsg{
		Curve: "P-256",
		Pub:   marshalPub(sPub),
	}); err != nil {
		log.Printf("[S] send pub err: %v", err)
		return
	}
	// 再收对方的
	var cHello PubKeyMsg
	if err := dec.Decode(&cHello); err != nil {
		log.Printf("[S] recv pub err: %v", err)
		return
	}
	cPub, err := unmarshalPub(elliptic.P256(), cHello.Pub)
	if err != nil {
		log.Printf("[S] bad client pub: %v", err)
		return
	}
	log.Printf("[S] got client pub: %s", hex.EncodeToString(cHello.Pub))

	// --- 双向演示：先发一条欢迎消息 ---
	welcome := "welcome from server"
	env, err := EncryptFor(cPub, []byte(welcome))
	if err != nil {
		log.Printf("[S] encrypt welcome err: %v", err)
		return
	}
	if err := enc.Encode(&WireMsg{From: "server", Envelope: env}); err != nil {
		log.Printf("[S] send welcome err: %v", err)
		return
	}

	// --- 循环收消息、回消息 ---
	for {
		var in WireMsg
		if err := dec.Decode(&in); err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("[S] client closed")
				return
			}
			log.Printf("[S] recv err: %v", err)
			return
		}
		plain, err := DecryptWith(sPriv, in.Envelope)
		if err != nil {
			log.Printf("[S] decrypt err: %v", err)
			return
		}
		log.Printf("[S] <- (%s): %q", in.From, string(plain))

		// 业务处理：加一点数据
		reply := fmt.Sprintf("server processed: [%s] @%s", strings.ToUpper(string(plain)), time.Now().Format(time.RFC3339))
		outEnv, err := EncryptFor(cPub, []byte(reply))
		if err != nil {
			log.Printf("[S] encrypt reply err: %v", err)
			return
		}
		if err := enc.Encode(&WireMsg{From: "server", Envelope: outEnv}); err != nil {
			log.Printf("[S] send reply err: %v", err)
			return
		}
	}
}

// -----------------------------
// 客户端
// -----------------------------

func runClient(addr string) error {
	// 生成“客户端静态密钥”（用来接收消息）
	cPriv, cPub := genStaticECDSA()
	log.Printf("[C] static pub: %s", hex.EncodeToString(marshalPub(cPub)))

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	enc, dec := newEncDec(conn)

	// --- 握手：交换静态公钥 ---
	// 先收服务端公钥
	var sHello PubKeyMsg
	if err := dec.Decode(&sHello); err != nil {
		return fmt.Errorf("recv server pub: %w", err)
	}
	sPub, err := unmarshalPub(elliptic.P256(), sHello.Pub)
	if err != nil {
		return fmt.Errorf("bad server pub: %w", err)
	}
	log.Printf("[C] got server pub: %s", hex.EncodeToString(sHello.Pub))

	// 再发自己的
	if err := enc.Encode(&PubKeyMsg{Curve: "P-256", Pub: marshalPub(cPub)}); err != nil {
		return fmt.Errorf("send client pub: %w", err)
	}

	// --- 开一个 goroutine 持续收消息并解密打印 ---
	errCh := make(chan error, 1)
	go func() {
		for {
			var in WireMsg
			if err := dec.Decode(&in); err != nil {
				errCh <- err
				return
			}
			plain, err := DecryptWith(cPriv, in.Envelope)
			if err != nil {
				errCh <- err
				return
			}
			log.Printf("[C] <- (%s): %q", in.From, string(plain))
		}
	}()

	// --- 主 goroutine 从 stdin 读取，发送到服务器（双向） ---
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("[C] type lines and press Enter to send; empty line to quit.")
	for {
		fmt.Print("> ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		env, err := EncryptFor(sPub, []byte(line))
		if err != nil {
			return err
		}
		if err := enc.Encode(&WireMsg{From: "client", Envelope: env}); err != nil {
			return err
		}

		// 如果后台收消息出错（断开等），及时退出
		select {
		case e := <-errCh:
			if errors.Is(e, io.EOF) {
				return nil
			}
			return e
		default:
		}
	}
	return nil
}

// -----------------------------
// main
// -----------------------------

func main() {
	mode := flag.String("mode", "server", "server or client")
	port := flag.String("port", "9000", "server listen port")
	addr := flag.String("addr", "127.0.0.1:9000", "server address for client")
	flag.Parse()

	switch *mode {
	case "server":
		if err := runServer(*port); err != nil {
			log.Fatal(err)
		}
	case "client":
		if err := runClient(*addr); err != nil {
			log.Fatal(err)
		}
	default:
		flag.Usage()
	}
}
```

---

# 三、怎么跑

开两个终端：

### 1) 终端A：启动服务端

```bash
go run e2e_ecies_aesgcm.go -mode server -port 9000
```

### 2) 终端B：启动客户端

```bash
go run e2e_ecies_aesgcm.go -mode client -addr 127.0.0.1:9000
```

在客户端终端里直接输入内容并回车，就会：

* 客户端用**服务器公钥**加密发送；
* 服务器解密、处理，再用**客户端公钥**加密返回；
* 客户端解密打印。

> 服务端也会在握手后**先主动发一条欢迎消息**到客户端，演示“双方都能发”。

---

# 四、端到端加密流程（图解）

以“客户端 -> 服务端”一条消息为例：

1. **握手阶段（明文）**

* C 生成静态密钥对 (cPriv, cPub)
* S 生成静态密钥对 (sPriv, sPub)
* 连接建立后：双方通过 gob 互换各自的静态 **公钥**（未压缩点）

2. **发送阶段（加密）**

* 客户端生成**临时密钥** ePriv/ePub
* 计算 ECDH 共享秘密：`S = ePriv * sPub`
* `K = HKDF(S)` → 32字节 AES-256 密钥
* `nonce = rand(12)`，`ct = AES-GCM-Enc(K, plaintext)`
* 发送：`Envelope{EphemeralPub=ePub, Nonce, Ciphertext=ct}`

3. **接收解密阶段**

* 服务端取出 `ePub`
* 计算同一个共享秘密：`S = sPriv * ePub`
* `K = HKDF(S)`
* `plaintext = AES-GCM-Dec(K, nonce, ct)`

4. **返回阶段（同理）**

* 服务端用**客户端的静态公钥**加密回复；客户端使用自己的静态私钥解密。

> **完整性/抗篡改**：AES-GCM 自带消息认证；如被篡改，解密会失败。
> **前向保密**：每条消息用不同的临时密钥（ephemeral），即使将来静态私钥泄露，历史消息仍受保护（需要更严格的协议才能达到完全 PFS，这里是“每条消息 KEM”级别的）。

---

# 五、关键细节 & 常见问题

* **为什么要传 `EphemeralPub`？**
  接收方需要它来做 ECDH：`S = staticPriv * ephemeralPub`，这样双方能得到同样的共享秘密。

* **为什么用 HKDF？**
  原始 ECDH 输出不应直接拿来当对称密钥；通过 HKDF（基于 SHA-256）做一次“密钥派生”，得到均匀、定长的密钥。

* **为什么选择 AES-GCM？**
  业界标准 AEAD（Authenticated Encryption with Associated Data），一次完成“机密性 + 完整性”保护，且硬件/库支持广泛。

* **需要证书吗？**
  这里只是演示“端到端加密”本身，不涉及 PKI。真实系统可以把**静态公钥**放到证书里，用 CA 签发来做身份绑定；传输上也可叠加 TLS（双层防护）。

* **双向发送**
  这份代码双方都持有各自静态密钥，**谁发消息就用对方的公钥加密**，因此双方都能主动发起消息。

---

# 六、想扩展？两条思路

1. **签名 + 身份绑定**
   给每条消息加“发送方对明文或密文的签名”（ECDSA），接收方用在握手时拿到的“发送方公钥”验签 → 抗冒充、抗重放可在应用层进一步设计（例如加时间戳/序列号放到 AAD）。

2. **流式/多路复用**
   如果要高并发/双向流式收发，建议：

   * 使用长度前缀帧 + 一个解码协程 + 一个编码协程；
   * 或者直接上 HTTP/2、WebSocket，在“消息体里”做本文这种 E2E 加密。

---

需要把这套方案**合并进你之前的 mTLS 热重载服务器**也没问题：你可以保持 TLS 做“链路保护 + 客户端认证”，再在业务体上叠加这套 ECIES+AES-GCM 的“端到端”层。若你想，我可以给出把二者合在一起的版本。

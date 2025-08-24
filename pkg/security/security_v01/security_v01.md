好——我把你要的**全部功能**整合到一个可运行的单文件 Go 程序里，并在代码后逐段详细注解（为什么这么写、关键点、替代方案、以及如何用 ECIES 做基于 ECC 的“应用层加密”）。

> 设计目标（你之前要求的）：
>
> 1. 用 Go 生成证书（RSA 和可选的 ECDSA），替代 openssl。
> 2. 单一可执行带子命令：`--gen`（生成证书）、`--server`（启动 HTTPS/mTLS 服务并热重载证书）、`--client`（客户端向服务端发起 mTLS 请求并演示应用层加密/解密）。
> 3. TLS 层仍用证书（可选择 RSA 或 ECDSA），应用层示例用混合加密（AES-GCM + RSA-OAEP）。
> 4. 额外说明：如何用 ECIES（基于 ECC 的加密）替代 RSA-OAEP（我会给出示例代码片段和 `go get` 命令）。
> 5. 每个重要部分后面都有详细解读，帮助你读懂复杂代码。

下面是单文件程序：把整个文件保存为 `security_mtls_tool.go`。

> 运行前准备：需要安装依赖 `fsnotify`（用于文件变更监听）。
>
> ```bash
> go get github.com/fsnotify/fsnotify
> ```

---

### 完整代码在：`security_mtls_tool.go`

***

## 逐段详解（按你之前说“太复杂，有的代码不懂”来解释每部分）

下面按模块分解并解释关键实现细节、设计抉择与注意点。

### 1) 单一入口与子命令（`flag` 部分）

* 通过 `-gen`、`-server`、`-client` 三个布尔 flag 来切换功能，减少你管理多个文件的麻烦。
* `-ecdsa` 只在 `-gen` 时有效，用来生成 ECDSA 签名证书（供 TLS 使用）。**重要**：ECDSA 证书仅改变 TLS 签名算法；应用层仍示范使用 RSA-OAEP（因为 RSA 私钥可以直接做加密/解密，便于演示密钥封装）。如果你想把应用层也改为 ECC+ECIES，我在后面给出 ECIES 的替代实现说明与示例。

### 2) 证书生成 `genCerts(useECDSA bool)`

* 先生成一个自签 CA（`ca.pem`、`ca.key`），然后用该 CA 签发 `server.pem`（server cert）和 `client.pem`（client cert）。
* 如果 `useECDSA==true`，所有私钥采用 ECDSA P-256（PEM 标记为 `EC PRIVATE KEY`），证书用于 TLS（签名与验证）。如果 `false`，使用 RSA 2048。
* 为什么把 CA 椎心放在本地：便于做内部测试、mTLS 环境、热重载演示。生产请用受信任 CA 或企业 PKI。
* 输出私钥权限 `0600`，证书 `0644`。

### 3) TLS config / 热重载（`loadTLSConfig` 与 `watchCertFiles` / `tlsCfgAtomic`）

* `loadTLSConfig()` 从 `server.pem/server.key/ca.pem` 读取并返回一个 `*tls.Config`，并配置 `ClientAuth: tls.RequireAndVerifyClientCert`（强制客户端提供证书并由 `ClientCAs` 验证）。
* 热重载实现关键：`tlsCfgAtomic.Store(cfg)` 存放最新的 `tls.Config`。服务端在启动时使用一个 `wrapper`（`GetConfigForClient` 回调）来在每次握手时取当前的配置。这样**新连接**会用新证书/新 CA，而已存活连接不受影响。
* `fsnotify` 监听当前目录（可改为某个 cert 目录），检测 `Create|Write` 事件并尝试 reload。若 reload 失败，会保留旧配置（这是安全谨慎的行为）。

**注意与坑**：

* `GetConfigForClient` 在每次新 TLS 握手时被调用；返回的 `*tls.Config` 对象可包含证书、ClientCAs 等。
* 已建立连接继续使用旧加密上下文：热重载不会影响已有连接，需要关闭连接或等待自然断开后生效。

### 4) 应用层混合加密（AES-GCM + RSA-OAEP）

* 为什么要在 TLS 之上再加一层？示例为了演示如何利用证书里的公钥做**端到端业务内容加密**（比如某些网络拓扑下中间代理会终止 TLS，但应用内容要对最终目标保密）。
* 流程（客户端 -> 服务端 -> 客户端）：

  1. 客户端产生随机 AES-256 密钥 `K`，用 AES-GCM 用 `K` 加密明文（高效且带认证）。
  2. 客户端用服务端的 RSA 公钥对 `K` 做 RSA-OAEP(SHA-256) 加密，得到 `encK`。
  3. 客户端把 `{encK, nonce, ciphertext}` 包成 JSON 发给服务端（HTTP POST）。
  4. 服务端用自己的 RSA 私钥解出 `K`，用 `K` 做 AES-GCM 解密，得到明文 → 处理（拼接` + server-data`）。
  5. 服务端为客户端生成新 AES key `K2`，用客户端公钥（从 TLS 客户端证书里取得）对 `K2` 做 RSA-OAEP 加密，然后用 `K2` 对响应进行 AES-GCM 加密并返回 JSON。
  6. 客户端用自己的 RSA 私钥解开 `K2`，用 `K2` 解密响应得到处理过的明文。

**为什么用 RSA?** RSA 私钥能做解密，且在示例中 server/key/client/key 都是 RSA 2048（可直接做 OAEP）。如果你用 ECDSA key 生成证书（`-ecdsa`），这些 keys **不能** 做 RSA-OAEP：ECDSA 是签名算法，不直接支持加密。要在 ECC 上做加密，需要 ECIES（我在后面说明）。

### 5) serverHandler 的实现要点

* 首先检查 `r.TLS.PeerCertificates`，以获取客户端在 TLS 握手中提供的证书（server 已强制 `RequireAndVerifyClientCert`，所以这里应该存在）。
* 读取 JSON、base64 解码字段、用 `rsa.DecryptOAEP`（server 私钥）解 AES key、用 `aesGCMDecrypt` 解明文。
* 处理后取客户端证书公钥（`r.TLS.PeerCertificates[0].PublicKey`），若为 RSA 则用 `rsa.EncryptOAEP` 加密响应 AES key（如果客户端证书为 ECDSA，则该分支会失败——说明：若你生成 ECDSA certs就不能用 RSA-OAEP；要么生成 RSA certs 用于加密，要么改为 ECIES）。
* 返回 JSON。

### 6) client 的实现要点

* 创建 `http.Client` 时把 `tls.Config` 设置为包含 client cert（`client.pem/client.key`）和 `RootCAs`（`ca.pem`）。
* 构造混合加密 payload，POST 到 `/api/process`，解析 JSON 响应并进行对应解密步骤。
* 注意：如果服务端证书改成了 ECDSA（`-ecdsa`），客户端依然能做 TLS（ECDSA 签名用于握手），但要做应用层 RSA-OAEP，你必须确保有**RSA**公钥可用（server cert must be RSA). 如果 server cert is ECDSA, `loadRSAPubFromCert` 会失败。

### 7) 关于 ECDSA / ECIES：如何用 ECC 做应用层加密（可选）

* **问题**：ECDSA 是签名算法，不能直接用来加密。要在椭圆曲线上实现加密，需要 **ECIES**（Elliptic Curve Integrated Encryption Scheme）。Go 标准库不包含 ECIES 实现，但有第三方包（例如 `github.com/ecies/go` 或 `github.com/ethereum/go-ethereum/crypto/ecies`）。
* **如果你想把应用层改为 ECIES**，示例工作流程会是：

  * 服务器和客户端证书使用 ECDSA keys (或单独生成 EC keypair 用于加密)。
  * 使用第三方 ECIES 库将随机对称 key 用对方的公钥加密/解密（相当于 RSA-OAEP 的替代）。
* 简单示例（使用 `github.com/ecies/go`，你需 `go get github.com/ecies/go`）：

```go
import "github.com/ecies/go/v2"

// 假设 clientPubEC is *ecies.PublicKey (constructed from ecdsa.PublicKey)
var plainKey = make([]byte, 32) // symmetric
ct, err := ecies.Encrypt(rand.Reader, clientPubEC, plainKey, nil, nil)
// 服务端：解密
decKey, err := ecies.Decrypt(serverPrivEC, ct, nil, nil)
```

* 要把 `x509` 里的 `*ecdsa.PublicKey` 转成 `ecies.PublicKey`，库通常提供转换方法或需要自行转换（把 ecdsa 公钥坐标复制到 ecies 公钥结构里）。我可以为你写出完整 ECIES 的示例（包括如何把证书里的 ECDSA 公钥包装成 ECIES 公钥），但那需要引入并安装第三方包；如果需要我现在就把 ECIES 的完整示例加进单文件程序，我会把 `go.mod` / `go get` 指令一并给出。

### 8) 如何运行（复述）

1. 安装 fsnotify：

```bash
go get github.com/fsnotify/fsnotify
```

2. 编译或直接运行生成证书（RSA 默认）：

```bash
go run security_mtls_tool -gen
# 若要 ECDSA（TLS签名只）：
go run security_mtls_tool -gen -ecdsa
```

3. 启动服务端（单独终端）：

```bash
go run security_mtls_tool -server
```

4. 在另一个终端运行客户端：

```bash
go run security_mtls_tool -client
```

5. 演示热重载：再次执行 `go run mtls_tool.go -gen`（会覆盖 cert 文件），服务端会检测到文件更改并 `reload`（新连接将使用新证书）。若你用 `-ecdsa` 生成 ECDSA certs，则 TLS 工作正常，但应用层 RSA-OAEP 将失败（因为 cert keys are ECDSA）；这时你应同时用 ECIES 或生成 RSA certs for app-layer encryption.

---

## ECIES：如果你想切换到纯 ECC 的端到端加密（完整替代 RSA）

我把关键步骤写清楚：

1. 安装 ECIES 实现（示例用 `github.com/ecies/go/v2`）：

```bash
go get github.com/ecies/go/v2
```

2. 把 x509 里 `*ecdsa.PublicKey` 转成 `ecies.PublicKey`：

```go
import ecies "github.com/ecies/go/v2"

func ecdsaPubToECIES(pub *ecdsa.PublicKey) *ecies.PublicKey {
    return ecies.ImportECDSAPublic(pub)
}
```

（不同库函数名可能不同；上面示例是常见包装方式，具体以库的 API 为准。）

3. 用 `ecies.Encrypt` / `ecies.Decrypt` 替换 RSA-OAEP 操作：

* 客户端：`encKeyBytes, _ := ecies.Encrypt(rand.Reader, clientPubECIES, aesKey, nil, nil)`
* 服务端：`aesKey, _ := ecies.Decrypt(serverPrivECIES, encKeyBytes, nil, nil)`

我可以把上面 ECIES 替换的完整代码加入 `mtls_tool.go`，并在 `-gen` 时同时生成 ECDSA keypair 用于 ECIES（或生成单独的 EC keypair file），但这需要第三方库。你想让我把 ECIES 完整整合进单文件并提供 `go get` 指令吗？

---

## 最后：我可以为你做的后续工作（选一或多项）

* A) 把 ECIES（基于 ECC 的应用层加密）**完整整合**到当前单文件程序，并加入 `-ecies` flag（需要引入 `github.com/ecies/go/v2`）。我会把所有转换/错误处理写好，并给 `go get` 说明。
* B) 把程序改成三种模式并打包成一个小 `Makefile` / `go.mod`，便于 `go build` 打包分发。
* C) 增强热重载：在 reload 失败时发告警（stdout/HTTP endpoint），或实现证书回滚策略。
* D) 写一个简短的测试脚本（bash）来自动演示生成 -> server -> client -> gen(again) -> client(new) 的完整流程并展示热重载生效。

你先选 A/B/C/D 的一个或多个，我就接着把相应代码/脚本和详细说明补上（如果选择 A，我会整合 ECIES 并让示例可选使用 RSA-OAEP 或 ECIES）。

<!--
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-08-24 08:11:03
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-08-24 09:42:04
 * @FilePath: /MLC_GO/.vscode/security/security_v00/security_v00.md
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
-->

- [3个文件用途：](#3个文件用途：) 
- [实现细节与要点（快速概览）](#实现细节与要点（快速概览）)  
- [运行顺序](#运行顺序)  
- [security_v00_server.go说明](#security_v00_server.go说明)  
- [security_v00_ client.go说明](#security_v00_client.go说明) 
- [运行示例](#运行示例)

***
<br/><br/><br/>
> <h2 id="3个文件用途：">3个文件用途：</h2>

- **3个文件用途：**
	* `gen_certs.go` —— 用 Go 生成自签名 CA、并用该 CA 签发 server/client 证书（**RSA**，可用于加解密）。
	* `server.go` —— HTTPS（HTTP over TLS）服务端：**mTLS（双向证书验证）** + **证书 & CA 热重载** + 接收客户端的“加密包”，解密、处理、再用客户端公钥加密返回。
	* `client.go` —— HTTPS 客户端：用 client 证书做 mTLS；把要发送的明文用 AES-GCM 对称加密，再用 server 公钥（RSA-OAEP）加密 AES key，发给服务端；收到响应后用自己的私钥解 AES key、再解密响应数据。

***
<br/><br/><br/>
> <h2 id="实现细节与要点（快速概览）">实现细节与要点（快速概览）</h2>


**实现细节与要点（快速概览）**

* 我使用 RSA 2048 来生成证书 & 私钥（RSA 可以直接做加密/解密，便于示例中的 RSA-OAEP）。
* 应用层数据加密：使用 **AES-GCM**（对称），AES key 用 **RSA-OAEP(SHA-256)** 加密传输。
* TLS 层依旧负责端到端传输安全（加密 + 证书校验），应用层再做一层“端到端内容加密”以示范如何使用证书中的公私钥做业务级加密/解密。
* 服务端支持热重载：当 `server.crt` / `server.key` / `ca.pem` 发生变动时，会重新构造 `*tls.Config` 并在新握手中生效（使用 `GetConfigForClient`，握手时读取当前配置）。
* 所有文件均有必要错误处理，代码注释较多，方便理解和修改。

***
<br/><br/><br/>
> <h2 id="">运行顺序</h2>
> 1. `go run gen_certs.go` （生成 `ca.pem`, `server.pem`, `server.key`, `client.pem`, `client.key`）
> 2. `go run server.go` （启动 HTTPS 服务）
> 3. `go run client.go` （客户端发一次请求并打印响应）
> 4. 若你想演示热重载，可再次运行 `go run gen_certs.go`（覆盖证书），server 会检测到文件更新并在随后的新连接握手中使用新证书/CA。


***
<br/><br/><br/>
> <h2 id="security_v00_server.go说明">security_v00_server.go说明</h2>


 `security_v00_server.go` — HTTPS 服务端（mTLS + 证书 & CA 热重载 + 应用层加解密）


> **说明 / 重点（server.go）**
>
> * `loadServerTLSConfig()` 构造一个 `*tls.Config`，包含 `Certificates`（服务器证书/私钥），`ClientCAs`（CA pool，用于校验客户端证书）以及 `ClientAuth: tls.RequireAndVerifyClientCert`（强制客户端提供证书并验证）。
> * `watchCerts()` 使用 `fsnotify` 监听当前目录里的 `server.pem` / `server.key` / `ca.pem`，发生改动则重新加载 `tls.Config` 并 `tlsConfigAtomic.Store(cfg)`。
> * 所有新的 TLS 握手都会触发 `wrapper.GetConfigForClient`，它会返回 `tlsConfigAtomic.Load()` 的当前配置，从而实现热重载（只有新连接受影响，已建立连接不受影响）。
> * `processHandler` 实现应用层加解密流程：
>
>   * 从请求 JSON 中解出 `enc_key`（RSA-OAEP 加密的 AES key）、`nonce`、`ciphertext`（AES-GCM）。
>   * 用 server 私钥解出 AES key，再用 AES-GCM 解出明文。
>   * 处理（示例把字符串拼接 ` + server-data`）。
>   * 用客户端证书中的公钥（`r.TLS.PeerCertificates[0]`）对响应的随机 AES key 做 RSA-OAEP 加密，然后 AES-GCM 加密响应体，最终返回与请求同样的 `EncryptedMessage` JSON。
> * 你可以把 `processHandler` 改为更复杂的业务逻辑（解析 JSON、数据库、流操作等）。

***
<br/><br/><br/>
> <h2 id="security_v00_client.go说明">security_v00_ client.go说明</h2>


**`security_v00_client.go` — 客户端（mTLS + 应用层加密/解密）**


> **说明 / 重点（client.go）**
>
> * 客户端使用 `tls.LoadX509KeyPair("client.pem","client.key")` 做 mTLS。
> * 在应用层，客户端先生成一个随机 AES-256 密钥、使用 AES-GCM 加密明文，再使用 server 的 RSA 公钥（从 `server.pem` 读取）对 AES key 做 RSA-OAEP 加密，按 JSON 发给服务端。
> * 收到服务端返回的 JSON 后，用客户端私钥解密响应的 AES key，再解密响应体。最后打印明文响应。
> * 注意：TLS 层已经为你提供了运输级别的机密性和认证；这里再加一层“应用级加密”是为了展示如何用证书中的公钥做端到端消息封装（比如在中间代理能看到 TLS 层，但不能解开应用的 AES payload）。

<br/>

**说明：部分细节 & 为什么这样实现**

1. **为什么既有 TLS 又有应用层加密？**

	* TLS（mTLS）保证传输通道被加密、双方证书被验证（防中间人），但在某些架构里，你可能还要对业务数据进行端到端加密（例如：中间代理需要对 TLS 做终端，但你仍要求数据对最终接收方可读）。示例展示如何在证书体系下使用 RSA 公钥封装对称密钥（常见「混合加密」模式）。

2. **为什么使用 RSA-OAEP + AES-GCM？**

	* RSA（OAEP）安全地用于封装对称密钥（短小），对大数据性能差；AES-GCM 用于实际数据加解密（高效并认证完整性）。这是现实中常见的混合加密做法。

3. **证书热重载实现原理**

	* `fsnotify` 监听证书/私钥/CA 文件变更，重新构造新的 `*tls.Config`，并 `atomic.Store`。在 TLS 握手时我们使用 `GetConfigForClient` 从 `atomic` 里拿当前配置，从而让新握手使用新的证书/CA（不需要重启服务）。已经建立的连接保持原有配置信任链。

4. **注意点 / 可扩展项**

   * 示例里证书都在当前目录。生产环境中请放在安全目录并妥善管理权限。
   * 真实生产环境通常使用专门的证书管理 / ACME / Vault / KMS 来签发与下发证书。热重载可以配合它们做零宕机证书更新。
   * 异常情况（加载证书失败）会在日志中输出，示例选择忽略失败的 reload（保留旧配置）。你可调整为更严格的行为（比如回滚或报警）。
   * `rsa.DecryptOAEP`/`rsa.EncryptOAEP` 我使用 `sha256.New()` 作为哈希函数（强推荐使用 SHA-256 而非 SHA-1）。

***
<br/><br/><br/>
> <h2 id="运行示例">运行示例</h2>

**1.生成证书：**

```bash
go run security_v00_client.go
# 输出:
# Generating CA, server and client certificates...
#  -> ca.pem, ca.key generated
#  -> server.pem, server.key generated
#  -> client.pem, client.key generated
# All certs generated: ca.pem, server.pem, server.key, client.pem, client.key
```

<br/>

**2.启动服务端（在一个终端）：**

```bash
go run security_v00_server.go
# 输出:
# HTTPS server (mTLS) listening on https://localhost:8443
```


<br/>

**3.运行客户端（另一个终端）：**

```bash
go run client.go
# 输出示例:
# Client received plain response: hello-from-client + server-data
```

<br/>

**4.演示热重载（可选）：**

* 在服务端运行时，**再次运行** `go run gen_certs.go`（会覆盖 `server.pem`/`server.key`/`ca.pem` 等）或手动替换证书文件。
* 服务端日志会显示检测到变更并重新加载 TLS 配置。
* 新建立的连接将使用新的证书/CA；旧连接继续不受影响。

<br/><br/>

**最后的小建议（生产级改进方向）**

* 使用更完善的证书管理（ACME / Let's Encrypt / Vault / Cloud CA），并把热重载与密钥轮换策略结合。
* 使用硬件密钥隔离（HSM / KMS）来保护私钥（避免写到磁盘）。
* 添加详细的错误日志与指标（证书加载失败告警），并在 reload 失败时采取安全策略。
* 如果需要在中间代理/负载均衡器处终止 TLS，但仍要端到端加密，应用层的 AES+RSA 模式（本示例）就是常用方案。

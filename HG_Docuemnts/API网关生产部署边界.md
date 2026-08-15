- [API Gateway](#API-Gateway)
	- [为什么需要 API Gateway](#为什么需要-API-Gateway)
	- [核心功能](#核心功能)
	- [大型系统架构](#大型系统架构)
	- [Go 项目中的实现方案](#Go-项目中的实现方案)
	- [API Gateway 与普通 HTTP Server 的区别](#API-Gateway-与普通-HTTP-Server-的区别)
- [API 网关生产部署边界](#API-网关生产部署边界)



<br/>

***
<br/><br/><br/>
># <h1 id="API-Gateway">API Gateway</h1>

```text
App (iOS / Android / Web)
          |
          |
     API Gateway
          |
  -----------------
  |       |       |
User   Video   Comment
Service Service Service
```

**API Gateway（API 网关）是客户端访问后端服务的统一入口。** App 不直接访问多个后端服务，而是先将请求发送到 API Gateway，再由网关转发给对应服务，并统一处理认证、限流、安全、日志等公共能力。

以视频平台为例：

```text
客户端
   |
   |
API Gateway
   |
   |------ 用户服务 user-service
   |
   |------ 视频服务 video-service
   |
   |------ 评论服务 comment-service
   |
   |------ 点赞服务 like-service
   |
   |------ 推荐服务 recommend-service
```

***
<br/>

> <h3 id="为什么需要-API-Gateway">为什么需要 API Gateway</h3>

没有网关时，客户端需要直接调用每个微服务：

```text
App
 |
 |---> user-service
 |
 |---> video-service
 |
 |---> comment-service
 |
 |---> like-service
 |
 |---> message-service
```

客户端还必须知道所有服务的地址：

```text
user-service:
http://10.0.1.20:8080

video-service:
http://10.0.1.30:8081

comment-service:
http://10.0.1.40:8082
```

例如 App 将用户服务地址写死：

```swift
let url =
"http://10.0.1.20:8080/user/info"
```

服务地址迁移后：

```text
10.0.1.20
变成
10.0.5.100
```

所有客户端都需要更新，这不适合大型系统。

有网关后，App 永远只访问统一域名：

```text
https://api.bilibili.com
```

例如获取用户信息：

```http
GET https://api.bilibili.com/user/profile?id=100
```

网关根据路由将请求转发给用户服务：

```text
/user/*
        |
        |
        v

user-service
```

**客户端不需要感知后端服务的数量、地址和部署变化。**

***
<br/>

> <h3 id="核心功能">核心功能</h3>

## 路由转发

路由转发是 API Gateway 的核心功能。例如：

```http
GET /api/video/detail/123
```

匹配网关规则后转发到视频服务：

```text
/api/video/*
        |
        v

video-service
```

评论请求同理：

```http
GET /api/comment/list/123
```

```text
/api/comment/*
        |
        v

comment-service
```

API Gateway 的基础转发能力类似 Nginx，但通常还集成认证、服务发现、限流、协议转换等功能。

---
<br/>

## 身份认证

用户登录：

```http
POST /login
```

服务端返回 Token：

```json
{
  "token": "eyxxxxxxx"
}
```

后续请求携带认证信息：

```http
GET /video/list
Authorization: Bearer eyxxxx
```

请求流程：

```text
App

 |
 |
API Gateway

 |
 | 检查token
 |
 |----有效
 |
 v

video-service
```

Token 失效时，网关直接返回：

```json
{
  "code": 401,
  "message": "token expired"
}
```

这样后端业务服务不需要重复实现登录校验。

---
<br/>

## 限流

热门视频可能在一分钟内出现大量用户同时点赞：

```text
100万人
同时点赞
```

请求直接进入点赞服务时：

```text
App
 |
 |
like-service

100万请求
```

可能导致：

```text
MySQL挂掉
Redis压力爆炸
服务崩溃
```

网关可以按用户、IP 或接口设置限流规则。例如用户：

```text
user_id=10001
```

点赞接口限制为：

```text
点赞接口:

10次/秒
```

超过限制后直接拒绝请求：

```text
HTTP 429 Too Many Requests
```

---
<br/>

## 安全防护

恶意请求可能持续遍历接口：

```text
/api/video?id=1
/api/video?id=2
/api/video?id=3
...
100万次
```

网关检测到异常访问：

```text
IP:
8.8.8.8

一分钟请求:
50000
```

可以直接封禁：

```text
deny
```

攻击流量在进入业务服务前被拦截，从而保护后端。

---
<br/>

## 统一日志

请求链路：

```text
App
 |
 |
Gateway
 |
 |
video-service
```

网关统一记录请求信息：

```text
request_id:
abc123

user:
10001

path:
/video/detail

time:
20ms
```

这些日志可用于**链路追踪、问题排查和性能分析**。

---
<br/>

## 协议转换

客户端可以使用 HTTP/HTTPS，内部微服务使用 gRPC：

```text
App

HTTP

 |

API Gateway

 |

gRPC

 |

video-service
```

例如客户端请求：

```http
GET /video/123
```

Go 网关转换为 gRPC 调用：

```go
client.GetVideo(ctx, request)
```

最终调用：

```text
video-service
```

---
<br/>

## API 版本管理

旧版 App 使用：

```text
/api/v1/video
```

新版 App 使用：

```text
/api/v2/video
```

网关将不同版本转发到对应服务：

```text
/v1
 |
旧服务


/v2
 |
新服务
```

因此可以兼容旧版客户端，不必强制所有用户立即升级。

***
<br/>

> <h3 id="大型系统架构">大型系统架构</h3>

大型视频平台的架构通常类似：

```text
              App
               |
               |
          CDN / WAF
               |
               |
        API Gateway
               |
 ------------------------------------------------
 |             |             |                  |
用户服务    视频服务      评论服务          推荐服务
 |             |             |
MySQL       MySQL        MySQL
 |
Redis
```

如果使用 Go 设计视频平台，可采用以下架构：

```text
                 App
                  |
                  |
              CDN/WAF
                  |
                  |
          Go API Gateway
                  |
        -------------------
        |        |        |
     User     Video    Comment
     Service Service  Service
        |        |        |
      Redis    Redis   Redis
        |
      MySQL
```

网关技术栈可以包括：

```text
Go
+
Gin/Fiber
+
gRPC Client
+
Redis Token
+
JWT
+
Rate Limit
+
OpenTelemetry
```

***
<br/>

> <h3 id="Go-项目中的实现方案">Go 项目中的实现方案</h3>

## Nginx + Go Gateway

```text
Client

 |
Nginx

 |
Go Gateway

 |
Micro Services
```

**Nginx 负责：**

- TLS
- 静态资源
- 负载均衡

**Go Gateway 负责：**

- 登录认证
- JWT 校验
- 路由转发
- RPC 调用

---
<br/>

## 使用成熟网关

### Kong

Kong 基于 Nginx + Lua，支持 JWT、限流和插件扩展。

### Apache APISIX

APISIX 常用于云原生架构：

```text
Client

 |
APISIX

 |
Kubernetes Service
```

### Envoy

Envoy 常用于 Service Mesh：

```text
App

 |
Envoy Gateway

 |
Service
```

***
<br/>

> <h3 id="API-Gateway-与普通-HTTP-Server-的区别">API Gateway 与普通 HTTP Server 的区别</h3>

| 对比项 | 普通 HTTP 服务 | API Gateway |
| --- | --- | --- |
| 业务逻辑 | 有 | 少 |
| 数据库 | 通常有 | 没有 |
| 用户认证 | 部分 | 大量 |
| 路由 | 简单 | 复杂 |
| 限流 | 少 | 必须 |
| 服务发现 | 无 | 有 |
| RPC 调用 | 少 | 大量 |
| 部署数量 | 少 | 多 |

**API Gateway 是微服务架构的统一入口。客户端请求先进入网关，由网关负责认证、路由、限流、安全、日志和协议转换，再将请求转发给真正处理业务的服务。**

对于包含视频、评论、点赞、关注、弹幕和推荐等多个服务的视频平台，API Gateway 通常是连接客户端与微服务的必要组件。


<br/>

***
<br/><br/><br/>
># <h1 id="API网关生产部署边界">API 网关生产部署边界</h1>

# API 网关生产部署边界

## 已实现能力

- 客户端只访问一个公开域名和 `/api/v1/...` 路径，不感知模块实例地址。
- 网关按模块前缀在启动期编译路由；模块可留在本进程，也可通过 `API_GATEWAY_UPSTREAM_<MODULE>` 转发到 HTTP(S) 上游。
- 上游转发保留原始 method、path、query、body、Authorization、签名和版本 Header，并重建可信 `X-Forwarded-For`。
- 认证接口继续由 API Guard 执行时间戳和 HMAC 签名校验，受保护接口继续执行 JWT 与权限校验，避免网关重复解析 JWT。
- Redis Lua 令牌桶提供跨实例的模块/IP 限流；单实例 `max_in_flight` 舱壁提供过载快速失败。
- URL、Header、Body、慢请求头和连接超时均有硬资源边界；可信代理、防伪造来源 IP 和安全响应头在统一入口执行。
- 请求 ID、状态码、响应字节和耗时由根入口统一记录一次；网关拒绝量和并发水位输出到 `/metrics`。
- `/api/vN` 与 `X-API-Version` 必须一致，未知版本不回退到 v1。
- 外部 HTTP/1.1 请求可由 Go Transport 复用连接并以 HTTP/2 访问支持 HTTP/2 的 HTTPS 上游。业务载荷转换仍必须按具体 API 契约实现，禁止通用猜测式 JSON/gRPC 转换。

## 千万并发部署前提

单个 Go 进程、单个 Redis 节点或一份 Kubernetes YAML 不能证明支撑千万并发。生产架构至少需要：

- CDN/WAF/DDoS 清洗承担公网攻击、Bot、TLS 和静态内容流量。
- 多地域 Anycast/GSLB 和地域内 L4/L7 负载均衡承担连接分发与故障域隔离。
- Kubernetes Service、Envoy 或 APISIX 承担服务发现、健康摘除、熔断、重试预算和跨区域流量治理。
- Redis Cluster 按 key 分片并配置容量、故障切换和热点监控；禁止把全部限流 key 固定到同一 hash slot。
- 业务 Pod 使用 HPA/KEDA 按 CPU、P99、in-flight 和拒绝率扩缩容，扩容速度必须覆盖流量突刺。
- 日志经 stdout/采集 Agent 异步进入日志平台，不能让请求 goroutine 同步写远端日志系统。
- 压测必须覆盖正常流量、热点模块、慢上游、Redis 故障、重试风暴、大请求、长连接和滚动发布。

## 模块拆分示例

```text
API_GATEWAY_UPSTREAM_AUTH=http://mlc-auth.mlc.svc.cluster.local
API_GATEWAY_UPSTREAM_PROFILE=http://mlc-profile.mlc.svc.cluster.local
API_GATEWAY_UPSTREAM_BILIBILI=http://mlc-bilibili.mlc.svc.cluster.local
```

未配置上游的模块继续走本进程 handler。这样客户端地址和 API 签名路径保持不变，可按模块逐步拆分而不要求客户端升级。

## 不应在应用网关实现

- DDoS 清洗、IP 信誉、验证码挑战、TLS 证书和全球流量调度。
- 无契约的 JSON、Protobuf、gRPC 字段转换。
- 对非幂等写请求自动重试。
- 将用户 ID、IP、URL、错误文本作为 Prometheus label。
- 依靠任意客户端可写的 `X-Forwarded-For` 决定限流身份。

<br/>

***
<br/><br/><br/>

># API 网关实施交付记录

‌BóYí (๑•̀ㅂ•́)و✧

## 1. 修改了哪些文件

核心实现：

- `internal/pkg/hg_router/hg_api_gateway.go`
- `internal/pkg/hg_router/hg_api_gateway_metrics.go`
- `internal/pkg/hg_router/hg_api_gateway_test.go`
- `internal/pkg/config/hg_env_config.go`
- `internal/pkg/config/hg_api_gateway_config_test.go`
- `internal/pkg/middleware/hg_middleware.go`
- `internal/pkg/middleware/hg_api_guard.go`
- `internal/pkg/middleware/hg_access_log_test.go`
- `internal/pkg/middleware/hg_api_guard_version_test.go`
- `internal/pkg/hg_router/hg_route_groups.go`
- `internal/handler/hg_root_handler.go`
- `main_mlc_project.go`

配置与部署：

- `config/base/app.yaml`
- `config/prod/app.yaml`
- `deployments/kubernetes/mlc-go-workload.yaml`
- `deployments/monitoring/rules/mlc-api-gateway.rules.yml`
- `HG_Docuemnts/API网关生产部署边界.md`

## 2. 做了什么改动

### 客户端服务地址

已实现。客户端继续只访问一个公开 API Origin 和 `/api/v1/...`，不需要知道认证、用户、Bilibili、评论、互动等模块的服务地址。

模块可采用两种部署方式：

- `upstream_url` 为空：转发到当前进程内注册的模块 Handler。
- 设置 `API_GATEWAY_UPSTREAM_<MODULE>`：透明转发到独立微服务。

示例：

```text
API_GATEWAY_UPSTREAM_AUTH=http://mlc-auth.mlc.svc.cluster.local
API_GATEWAY_UPSTREAM_PROFILE=http://mlc-profile.mlc.svc.cluster.local
API_GATEWAY_UPSTREAM_BILIBILI=http://mlc-bilibili.mlc.svc.cluster.local
```

支持的环境变量包括：

```text
API_GATEWAY_UPSTREAM_AUTH
API_GATEWAY_UPSTREAM_PROFILE
API_GATEWAY_UPSTREAM_VIDEO_UPLOAD
API_GATEWAY_UPSTREAM_BILIBILI
API_GATEWAY_UPSTREAM_VIDEO_INTERACTION
API_GATEWAY_UPSTREAM_VIDEO_COMMENT
API_GATEWAY_UPSTREAM_VIDEO_DANMAKU
API_GATEWAY_UPSTREAM_OPS
```

### 路由转发

已补齐 HTTP 反向代理转发能力：

- 按 `/api/v1/<module>/...` 识别模块。
- 原始 HTTP method、path、query string 和 request body 不变。
- `Authorization`、签名和客户端版本 Header 不变。
- 上游地址只允许无凭据、无 path、无 query 的 HTTP/HTTPS Origin。
- 支持连接池复用，HTTPS 上游支持协商 HTTP/2。
- 上游连接失败统一返回 HTTP `502` 和项目原有 JSON envelope。
- 不自动重试非幂等请求，避免重复写入。
- 未配置上游的模块继续走本地 Handler，便于单体向微服务渐进拆分。

### 身份认证

原有功能已经存在，本次保持其执行位置和语义：

- API Guard 校验 HMAC 请求签名和请求时间戳，降低请求篡改和重放风险。
- 校验 `X-Device-ID`、`X-Client-Type`、`X-Client-Version`、`X-API-Version` 等公共 Header。
- 受保护接口继续执行 JWT 校验，JWT 限制 HS256。
- 校验 token 类型、issuer、subject 和设备指纹。
- Ops 等接口继续执行权限判断。

网关没有重复解析 JWT，避免同一请求执行两次密码学计算。客户端仍只携带原有 Token 和签名，不需要为远程上游改变协议。

### 限流与过载保护

原有 Redis Lua 分布式令牌桶继续保留，并新增单实例舱壁：

- 跨实例限流维度为模块 + 来源 IP。
- Redis Lua 原子令牌桶保证多实例并发正确性。
- Redis key 不保存原始 IP，也未使用固定全局 hash tag，可以由 Redis Cluster 分散 slot。
- 超限返回 HTTP `429` 和 `Retry-After`。
- Redis 限流异常默认 fail-closed，返回 HTTP `503`。
- 每个模块新增 `max_in_flight`，达到上限立即返回 HTTP `503`，不创建无界 goroutine 或请求队列。
- 上传模块使用较低并发舱壁，公开读模块使用较高并发舱壁。
- 业务专用的用户级、幂等和高成本操作限流仍然保留。

默认配置示例：

```yaml
bilibili:
  capacity: 300
  refill_per_second: 100
  max_body_bytes: 1048576
  max_in_flight: 8000
  upstream_url: ""
```

这些是资源硬边界，不代表单 Pod 已经实测支持 8000 个持续执行中的昂贵请求。生产值必须根据 P99、CPU、内存和下游连接池压测调整。

### 网关防攻击

已增加或统一以下应用层防护：

- URL 长度上限，默认 `8192` 字节。
- 每个模块独立 request body 上限。
- 已知 `Content-Length` 超限时在读取 body 前快速返回 HTTP `413`。
- 未知长度 body 使用 `http.MaxBytesReader` 限制实际读取量。
- 保留 HTTP Server 的 `ReadHeaderTimeout`、`ReadTimeout`、`WriteTimeout`、`IdleTimeout` 和 `MaxHeaderBytes`。
- 只有可信代理可以传递客户端来源 IP，并拒绝 `0.0.0.0/0` 和 `::/0` 可信代理配置。
- 重新构建上游 `X-Forwarded-For`，删除非可信 `Forwarded`，不允许客户端伪造来源链。
- Redis 故障和并发舱壁满载时快速失败。
- 添加 `X-Content-Type-Options`、`X-Frame-Options`、`Referrer-Policy` 和 `Permissions-Policy` 安全响应头。
- 认证和 Ops 接口使用 `Cache-Control: no-store`。

DDoS 清洗、IP 信誉、Bot 识别、验证码挑战和 TLS 防护不能由应用进程可靠承担，仍必须部署在 CDN/WAF/L4/L7 边缘层。

### 统一日志

已把请求 ID、恢复和访问日志提升到业务根入口，避免每个模块重复执行。

统一日志现在包含：

```json
{
  "request_id": "...",
  "method": "GET",
  "path": "/api/v1/bilibili/author/homepage",
  "status": 200,
  "bytes": 1024,
  "cost_ms": 12
}
```

同时：

- 每个响应返回 `X-Request-ID`。
- 网关错误响应与业务响应使用同一个请求 ID。
- 保留确定性日志采样。
- 不记录 Token、签名、请求 body、查询参数值或用户隐私。
- 日志响应包装器实现 `Unwrap`，让标准库控制器仍能访问底层 ResponseWriter 能力。
- 移除了模块层和路由清单接口的重复日志、重复 Request ID 和重复 panic 恢复。

### 协议转换

已实现安全、无业务语义变化的传输层适配：

- 客户端 HTTP/1.1 可以通过网关访问支持 HTTP/2 的 HTTPS 上游。
- 网关保持 JSON body、multipart、二进制 body 和流式请求原样转发。
- 不对 payload 做猜测式转换。

没有实现通用 JSON 与 gRPC/Protobuf 字段转换。当前工程没有为这些接口提供 protobuf、字段映射、错误映射和 streaming 契约，直接自动转换会破坏 HMAC body hash、DTO 类型、错误码和上传语义。需要 gRPC 时，应针对具体模块增加 `.proto` 和显式 grpc-gateway 映射。

### API 版本管理

已加强：

```yaml
supported_versions: [v1]
```

- URL `/api/v1/...` 与 `X-API-Version: v1` 必须一致。
- 未携带版本 Header 时保持当前客户端兼容，由 URL 决定版本。
- `X-API-Version: v2` 请求 `/api/v1/...` 会被拒绝。
- `/api/v2/...` 在未声明 `v2` 时会被拒绝。
- API Guard 不再把未知版本静默回退到 `v1`。
- 现有 `/api/v1/...` 路由和签名路径保持不变。

### 可观测和弹性部署

新增低基数 Prometheus 指标：

```text
mlc_api_gateway_requests_total{module="..."}
mlc_api_gateway_rejections_total{module="...",reason="rate_limit"}
mlc_api_gateway_rejections_total{module="...",reason="overload"}
mlc_api_gateway_rejections_total{module="...",reason="redis_failure"}
mlc_api_gateway_in_flight{module="..."}
```

新增 Redis 限流依赖持续失败、模块舱壁持续满载和模块限流拒绝比例过高告警。

Kubernetes 新增基础 HPA：最少 3 个副本，最多 100 个副本，CPU 目标 60%，采用快速扩容和保守缩容策略，并保留现有 PDB、readiness 和滚动发布设置。

## 3. 为什么这样改

目标是保持现有客户端完全兼容，同时让单体模块具备渐进拆分能力：

```text
客户端
  -> CDN/WAF/DDoS
  -> Ingress/L7 LB
  -> HG API Gateway
  -> 本进程模块
  或
  -> Kubernetes Service / 独立上游服务
```

主要设计取舍：

- 路由和策略在启动期构造，请求期不访问 Viper。
- 模块匹配只有固定 8 个入口，不引入动态正则或通用脚本路由。
- 分布式令牌桶解决多 Pod 限流一致性，本地并发舱壁防止单 Pod 无界积压。
- 不自动重试写请求，避免重试风暴和重复写。
- 不重复 JWT 和 HMAC 校验，控制热点路径 CPU 成本。
- 反向代理保持原始公开路径，客户端签名算法不用修改。
- 日志和指标禁止使用 IP、用户 ID、完整 URL 或错误文本作为 label，避免亿级时序。
- 服务发现、健康摘除和故障转移交给 Kubernetes Service、Envoy 或 APISIX。

## 4. 准确性检查结果

已确认：

- `/api/v1/bilibili/author/homepage?userId=...` 经过网关后 path 和 query 不变。
- 反向代理场景下 path、query、`Authorization` 不变。
- 不可信客户端伪造 `X-Forwarded-For` 不会影响来源 IP。
- 转发给上游的 `X-Forwarded-For` 由网关重新构造。
- 本地路由和远程上游可以按模块独立选择。
- URL 版本和 Header 版本不一致时被拒绝，未知版本不再回退到 `v1`。
- body 超限在进入 Handler 前被拒绝。
- 舱壁满载时返回 `503` 和 `Retry-After: 1`。
- Redis 错误返回 `503`，Redis 限流返回 `429`。
- 网关指标只包含固定模块和固定原因标签。
- `X-Request-ID` 会同时写入响应、context 和访问日志。
- CORS 位于网关之前，浏览器预检不会消耗 Redis 令牌。
- `git diff --check` 通过。

## 5. 潜在影响

- 每个业务 API 请求仍会增加一次 Redis Lua 调用。
- Redis 故障时业务 API 默认 fail-closed，优先保护数据库和下游。
- `max_in_flight` 设置过低会产生 `503`；设置过高会把压力继续传给数据库、Redis 和上游服务。
- 模块转发开启后，上游必须识别现有完整路径 `/api/v1/...`。
- 网关当前没有为上游请求启用自动重试，这是为了避免写接口重复执行。
- 远程上游 Header 等待上限为 30 秒，长耗时业务应改成异步任务。
- 生产 HPA 当前使用 CPU 作为基础指标，千万并发场景还应接入 in-flight、P99 和拒绝率。
- 高流量生产日志必须由 stdout/节点 Agent 异步采集，不能同步发送到远端日志服务。
- 抖音体量的视频上传应改为客户端直传对象存储，应用只签发上传凭证和处理完成回调。

## 6. 格式化、编译和测试说明

通过：

```text
gofmt
go test ./internal/pkg/hg_router ./internal/pkg/config ./internal/handler
go test ./internal/pkg/middleware -run 'Test(RequestIDMiddlewareReturnsRequestID|AccessLogResponseWriterCapturesStatusAndBytes|APIGuardDoesNotFallbackUnknownVersion|APIGuard)'
go test -race ./internal/pkg/hg_router ./internal/pkg/config ./internal/handler
go test -race ./internal/pkg/middleware -run 'Test(RequestIDMiddlewareReturnsRequestID|AccessLogResponseWriterCapturesStatusAndBytes|APIGuardDoesNotFallbackUnknownVersion|APIGuard)'
go vet ./internal/pkg/hg_router ./internal/pkg/config ./internal/pkg/middleware ./internal/handler ./internal/pkg/redis
go run ./cmd/hg_config_check --env=debug --config-dir=./config
go build -tags production -o /var/folders/2z/dxhnl1vd6jzdg_70q_2h00bh0000gn/T/opencode/mlc-go-gateway .
git diff --check
```

完整 `internal/pkg/middleware` 测试仍有两个既有失败：

```text
TestAuthMiddleware_MissingTokenReturnStandardError
TestTokenAuthMiddleware_MissingTokenReturnStandardError
```

原因是测试期望旧业务码 `101001`，当前 JWT 实现返回 `300001`。本次没有修改该鉴权错误语义。

根包生产标签测试仍有既有 Kafka 配置失败：

```text
consumer danmaku group_id 不能为空
```

受影响的既有测试：

```text
TestInitKafkaIfConfiguredAllowsDebugWithoutBroker
TestInitKafkaIfConfiguredRejectsBrokerFailureWhenRequired
```

生产标签二进制构建已独立成功。

没有执行真实千万并发压力测试，因此准确结论是：

- 已完成面向高并发的入口资源治理和水平扩展设计。
- 已完成多实例分布式限流、模块舱壁、透明路由、统一日志和版本治理。
- 尚未通过真实集群、真实 Redis Cluster、真实 WAF/Ingress 和压测证明支撑千万并发。
- 抖音体量仍需要多地域边缘网络、服务拆分、对象存储直传、缓存分层、Redis Cluster、消息队列、数据库分片和持续容量工程共同支撑。

## 7. 后续优化建议

1. 部署 CDN/WAF/DDoS 清洗，应用网关只承担可信流量后的业务治理。
2. 将视频上传改为对象存储预签名直传，避免 4GB 请求占用 Go Pod 连接和带宽。
3. 使用 Envoy/APISIX 或云网关承接服务发现、健康摘除、熔断、重试预算和跨地域流量。
4. 为 HPA 接入 `mlc_api_gateway_in_flight`、P99 和拒绝率，而不是只依赖 CPU。
5. 按实际模块拆分规划设置 `API_GATEWAY_UPSTREAM_*`，先从公开读和独立性高的 Bilibili 模块开始。
6. 如果需要 gRPC，先为目标模块建立 `.proto`、错误码和 streaming 契约，再增加显式 grpc-gateway 转换。
7. 在预发布执行阶梯压测、热点压测、Redis 故障、慢上游、重试风暴和滚动发布压测，并根据结果校准 `max_in_flight` 和 Redis 配额。

## 容量参数详细校准注释

`max_in_flight` 和 Redis 令牌桶参数不能根据目标 QPS 直接拍脑袋设置。它们分别保护单实例执行资源和全实例入口流量，必须分开测量。

### `max_in_flight` 校准方法

1. 先固定 Pod CPU、内存、数据库连接池、Redis 连接池和上游连接池，不在同一轮压测中同时修改多个资源变量。
2. 对每个模块分别执行阶梯压测，从稳定低流量开始，每 3-5 分钟提升一档并等待 P99、CPU、GC 和连接池水位稳定。
3. 记录首次出现以下任一拐点时的单 Pod in-flight：CPU 持续高于 70%-80%、P99 明显非线性增长、GC pause 增长、数据库连接池等待、Redis pool timeout 或上游连接等待。
4. 将拐点前最后一个稳定档位的 in-flight 乘以 `0.6-0.8` 作为初始 `max_in_flight`，预留滚动发布、节点抖动、热点 key 和依赖变慢的容量余量。
5. 上传、导出、复杂写入等长耗时接口不能与公开读接口共用同一并发预算，应保持模块级或接口级独立舱壁。
6. 上线后重点观察 `mlc_api_gateway_in_flight`、`reason="overload"` 拒绝率、P99、CPU throttling、内存和下游连接等待，再以小步方式调整，每次建议不超过 20%。

### Redis 配额校准方法

1. `capacity` 控制允许吸收的瞬时突刺，`refill_per_second` 控制单个模块和来源 IP 的长期平均速率。
2. 对公开读接口，应依据正常用户、NAT 出口共享用户和爬虫流量分布设置，避免大型企业网或校园网共享 IP 被误伤。
3. 对认证、评论、互动、上传和 Ops 写接口，应结合用户维度、设备维度和幂等策略使用更严格配额，不能只依靠 IP 限流。
4. `capacity` 通常可从 `refill_per_second` 的 1-5 秒突刺量起步；高成本写接口取较低倍数，缓存命中率高的公开读接口可以取较高倍数。
5. 压测必须验证 Redis Cluster CPU、网络、命令延迟、连接池等待和 key slot 分布。若单 slot、单分片或单来源 IP 形成热点，应拆分限流维度或在边缘层先行限流，而不是无限提高 Redis 配额。
6. 线上调参以 `reason="rate_limit"` 拒绝率、业务成功率、Redis P99 和下游饱和度为依据。正常用户误限且下游有余量时小步提高；下游已接近拐点时保持或降低配额，并优先扩容、缓存或异步削峰。

### 校准通过标准

- 正常流量下没有持续 `overload`，突刺时能快速拒绝且实例不发生 OOM、长 GC 或连接池雪崩。
- Redis 故障演练时入口按预期 fail-closed，数据库和业务上游不会承受失控流量。
- 单 Pod 下线、一个可用区降级和滚动发布期间，剩余容量仍能覆盖目标流量并保持可接受 P99。
- 参数结论必须附带压测模型、Pod 规格、实例数、命中率、请求体大小、下游延迟和 Redis 拓扑，不能脱离测试上下文复用。

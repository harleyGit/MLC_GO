# User 模块分层规范

## 标准分层

User 模块采用大厂常见的四层模型。handler/service 本身不是“大厂专属设计”，但“单向依赖 + 依赖注入 + handler 不写业务 + service 编排业务 + repository/cache 管数据访问”是主流中大型后端团队长期使用的稳定设计。

- `module`：模块装配层，只负责创建依赖、组装 service/handler、注册模块。
- `handler`：HTTP 适配层，只负责解析参数、调用 service、转换错误、写响应。
- `service`：业务编排层，只负责业务流程、校验、缓存清理、调用 repository/cache。
- `repository` / `cache`：数据访问层，只负责 SQL 或 Redis 读写，不承载 HTTP 语义。

## 依赖方向

依赖只能单向流动：

```text
module -> handler -> service -> repository/cache -> infrastructure
```

禁止反向依赖：

- `service` 不允许调用 handler。
- `repository` 不允许调用 service。
- `handler` 不允许直接 new repository/cache/db。
- 新业务不允许直接调用全局 DB/Redis 函数绕过注入依赖。
- 允许底层基础设施包保留全局兼容函数，但 user 新代码必须优先使用 module 注入进来的 `RedisService` / repository。

## 为什么这样设计

- 职责清晰：HTTP、业务、数据访问互不污染。
- 可测试：service 可以用 mock repo/cache 测试，handler 可以用 mock service 测试。
- 可替换：MySQL 改分库分表、Redis 改集群时，尽量只影响 infrastructure/cache/repository。
- 可维护：新增字段或接口时，不会在多个层级散落创建依赖的代码。
- 高并发更稳：依赖统一复用连接池，避免请求期重复创建 DB/Redis 客户端。

## 新功能编码模板

新增一个用户接口时按以下顺序编码：

1. 在 `dto` 中定义请求/响应结构，保持 JSON 字段兼容。
2. 在 `repository` 或 `cache` 中补数据访问方法。
3. 在 `service` 中编排业务流程和缓存清理。
4. 在 `handler` 中解析请求并调用 service。
5. 在 `middleware_group` 中注册路由。
6. 补充受影响包测试并运行 `go test ./internal/modules/user/...`。

## 当前落地文件结构

当前 user 模块按“业务模块内分层 + 能力拆文件”落地。这是字节、阿里等大厂 Go 后端中更常见的中型服务写法：不把一个模块拆成过多微包，也不把所有逻辑塞进一个大文件。

```text
internal/modules/user
├── api/                         # 模块独立启动入口，主服务通常不直接使用
├── cache/                       # Redis 访问，按缓存对象拆文件
├── dto/                         # 请求/响应 DTO，对外字段保持兼容
├── handler/                     # HTTP 适配层，只做参数、错误码、响应
│   ├── hg_user_handler.go       # handler 聚合结构和构造函数
│   ├── hg_auth_handler.go       # 注册、验证码、登录、刷新 token
│   ├── hg_profile_handler.go    # 用户资料、列表、更新
│   └── hg_avatar_handler.go     # 头像上传/获取
├── mapper/                      # DTO/model 转换
├── middleware/                  # user 模块专属中间件，如 JWT
├── model/                       # 数据库模型
├── module/                      # 依赖装配和模块注册
│   ├── hg_user_assembly.go      # repo/cache/service/handler 装配
│   └── hg_user_module.go        # auth/profile 模块注册
├── repository/                  # SQL 访问层
└── service/                     # 业务编排层，按能力拆文件
    ├── hg_user_service.go       # UserService 聚合结构和构造函数
    ├── hg_auth_user_service.go  # 注册、登录、验证码
    ├── hg_auth_service.go       # Token/Claims/刷新/登出兼容能力
    ├── hg_profile_service.go    # 用户资料读写
    ├── hg_user_query_service.go # 用户列表查询和缓存失效
    └── hg_avatar_service.go     # 头像业务
```

## 文件规范

- `module/hg_user_assembly.go`：唯一依赖装配入口，负责 new repo/cache/service/handler。
- `module/hg_user_module.go`：只实现模块注册和路由组接入。
- `handler/hg_user_handler.go`：只定义 handler 聚合结构和构造函数。
- `handler/hg_auth_handler.go`：认证相关 HTTP 接口。
- `handler/hg_profile_handler.go`：用户资料相关 HTTP 接口。
- `handler/hg_avatar_handler.go`：头像相关 HTTP 接口。
- `service/hg_user_service.go`：只定义 UserService 聚合结构和构造函数。
- `service/hg_auth_user_service.go`：注册、登录、验证码业务编排。
- `service/hg_auth_service.go`：Token、Claims、刷新、登出等认证基础能力。

登录会话按当前站点单一应用计数：初次和刷新后的 access JWT 均有效 24 小时；refresh JWT 签名后不轮换、不设固定 `exp`，服务端 Redis 会话和 refresh 状态均按每次成功刷新滑动 30 天。连续 30 天未刷新则状态自然过期，旧 refresh 无法重新生成会话；刷新不删除旧 access，旧 access 仍按自身 `exp` 及会话存活状态鉴权。每用户会话索引最多 100 条，超限原子淘汰最久未续期的会话；用户登出原子删除会话、索引与 refresh 状态，后台强制下线、账号删除仍能立即撤销。Refresh Token 如被泄露，可在最长 30 天内持续续期，必须保护客户端存储和传输；不要将令牌写入日志。

pre/prod 启动前必须通过密钥管理注入 `MLC_AUTH_JWT_SECRET_HEX`（32 字节、64 位十六进制），不能沿用旧代码中的固定占位密钥；debug 未配置时随机生成进程密钥，重启会导致本机令牌失效。浏览器请求 HMAC 与 JWT 私钥分离：`MLC_API_SIGN_SECRET` 可选，必须和前端构建期 `VITE_SIGN_SECRET` 相同；未配置均保持既有公开的 `change-me` 请求签名协议，不得视作安全凭证。签名密钥从旧占位值切换后旧 token 无法验证，需重新登录；原本不含 `sid` 的旧版 access/refresh 也必须重新登录，不能绕过服务端会话撤销。若使用同一密钥滚动升级，发布前签发、含 7 天 `exp` 的 refresh 只有其 Redis 状态仍存在时才可续期，不能恢复已经过期的 7 天状态。旧版会话缺少 ZSET 索引，上线前需先安排受控补录或等待其自然到期；没有补录的过渡期不能声称历史会话也已纳入 100 条上限。
- `service/hg_profile_service.go`：用户资料读写业务。
- `service/hg_user_query_service.go`：用户列表查询、total 查询和列表缓存失效。
- `repository/hg_user_respository.go`：SQL 访问，动态 SQL 字段必须白名单。
- `cache/*.go`：Redis key/value 访问，不承载 HTTP 语义。

## 约束

- Handler 不写业务逻辑。
- Service 不直接写 HTTP 响应。
- Handler 不直接调用全局 `RDB`、不直接 new repository/cache/db。
- Service 不直接使用全局 Redis 函数；必须优先使用 module 注入的 `RedisService` 或 cache 对象。
- Repository 不拼接用户输入以外的动态 SQL 片段，动态字段必须白名单。
- 会影响用户列表的数据写操作后，必须清理用户列表分页缓存和 total 缓存。
- Redis 验证码读取后比较前必须兼容 JSON 字符串值，例如 `"123456"`。

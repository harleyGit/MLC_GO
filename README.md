>#  [接口文档URL](https://app.apifox.com/project/8272891)
- [**‌ 工程启动**](#工程启动)
  - [VSCode 启动](#VSCode启动)
  - [Intel 电脑修改配置启动](#Intel电脑修改配置启动)
  - [终端查看 MySQL 表](#终端查看MySQL表)
  - [redis 启动](#redis启动)
- [表分布](#表分布)
	- [用户模块](#用户模块)
	- [视频模块](#视频模块)
- [**文件结构介绍**](#文件结构介绍)
  - [功能模块文件分布](#功能模块文件分布)
- [**文件规则**](#文件规则)
  - [协议规则](#协议规则)
  - [大厂常见的DDD + Clean Architecture + 微服务预留结构](#大厂常见的DDD+CleanArchitecture+微服务预留结构)
- [Golang 开源项目汇总列表](#Golang开源项目汇总列表)
  - [推荐几个可以写到简历上的 Go 方向优质开源项目（需花点心思研究）](https://juejin.cn/post/7038967716459315208)
  - [golang-gin-realworld-example-app 工程](#golang-gin-realworld-example-app工程)
  - [go-gin-api 全栈项目 ](#go-gin-api全栈项目)
  - [gin-vue-admin 全栈平台 ](#gin-vue-admin全栈平台)
  - [ferry 工单系统 Gorm（ORM 工具）](#ferry工单系统Gorm（ORM工具）)
  - [gin-gorm-restful-api](#gin-gorm-restful-api)
  - [Go-Zero 商城项目](#Go-Zero商城项目)
  - [echo-restful-api](#echo-restful-api)
  - [gorilla-mux-restful-api](#gorilla-mux-restful-api)
  - [beego-restful-api](#beego-restful-api)
  - [kratos-restful-api](#kratos-restful-api)
  - [gin-swagger-restful-api](#gin-swagger-restful-api)
  - [go-kit-restful-api](#go-kit-restful-api)
  - [fiber-restful-api](#fiber-restful-api)
  - [gin-gorm-jwt-restful-api](#gin-gorm-jwt-restful-api)
- [**框架**](#框架)
  - [NSQ 源码阅读](#NSQ源码阅读)
  - [Gin 框架](#Gin框架)
  - [Echo 框架](#Echo框架)
  - [GorillaMux 路由库](#GorillaMux路由库)
  - [Vegeta 负载测试工具](#Vegeta负载测试工具)
  - [Authboss 库-添加认证与授权模块](#Authboss库-添加认证与授权模块)
  - [GoKit 库-网关和分布式追踪](#GoKit库-网关和分布式追踪)
  - [Beego 库](#Beego库)
  - [Fiber 库](#Fiber库)
  - [go-restful 库](#go-restful库)
  - [Chi 库](#Chi库)
  - [Viper 配置管理库](#Viper配置管理库)
- **资料**
  - [浅读 Go 优秀开源项目源码—Gin 框架](https://blog.linganmin.cn/posts/d6715893/)
  - [rickiyang 博客 Go-具体很详细](https://www.cnblogs.com/rickiyang/category/1487722.html)
  - [gorm 库练习](https://www.cnblogs.com/rickiyang/p/11074162.html)
  - [维斯 Echo(博客仔细,不错)-掘金](https://juejin.cn/user/369885757844285/posts)
  - [盘点 7 个优质开源的 Go 项目](https://juejin.cn/post/7092788846781267975)
  - [标准的 Go 项目布局](https://juejin.cn/post/6944649692319842340)
  - [awesome-go 项目](https://github.com/avelino/awesome-go)
  - [Awesome Github REPO](https://github.com/Wechat-ggGitHub/Awesome-GitHub-Repo)
  - [awesome-go 中文介绍](https://github.com/jobbole/awesome-go-cn)
  - [awesome-go 中文介绍 02](https://github.com/hyper0x/awesome-go-China/blob/master/zh_CN/README.md)
  - [超全 golang 面试题合集+golang 学习指南+golang 知识图谱+成长路线](https://github.com/xiaobaiTech/golangFamily?tab=readme-ov-file)
  - [Go 开发者路线图](https://github.com/darius-khll/golang-developer-roadmap/blob/master/i18n/zh-CN/ReadMe-zh-CN.md)
  - [GitHubDaily 已累积分享超过 8000 个开源项目](https://github.com/GitHubDaily/GitHubDaily)
- [优化建议](#优化建议)
  - [免费图片资源](https://picsum.photos)
<br/><br/><br/>

---

<br/>


> <h1 id="工程启动">工程启动</h1>

```sh
cmd/                         # 进程入口
internal/
├── app/                     # 应用启动、生命周期
├── config/                  # 配置
├── infrastructure/          # MySQL、Redis、MQ、第三方基础设施
├── interfaces/              # HTTP/RPC 路由、中间件、presenter
├── modules/                 # 业务模块
│   └── user/
│       ├── module/          # 模块装配
│       ├── handler/         # HTTP/RPC 适配层
│       ├── service/         # 业务编排层
│       ├── repository/      # DB 访问层
│       ├── cache/           # Redis 访问层
│       ├── dto/             # 请求/响应 DTO
│       ├── model/           # 数据模型
│       ├── mapper/          # DTO/model 转换
│       └── middleware/      # 模块私有中间件
└── pkg/                     # 可复用内部工具

```

当前项目更适合采用“业务模块内分层 + 能力拆文件”，我已按这个方向改造 user 模块，没有做跨模块大迁移，避免破坏现有路由和启动链路。
<br/>

> <h2 id="VSCode启动">VSCode启动</h2>

在 **VS Code** 里按下面操作进行启动工程：

- 1.打开左侧“运行和调试”
- 2.在顶部下拉框里选一个配置
- 3.点绿色启动按钮

<br/>
你会看到这三个：

- `🧪 Launch MLC_GO Root main.go (debug)`
- `🧪 Launch MLC_GO Root main.go (pre)`
- `🧪 Launch MLC_GO Root main.go (prod)`

它们分别对应：

- `debug`：本机开发
- `pre`：本机模拟预发
- `prod`：只检查生产配置

如果你不用 VS Code，也可以在终端里手动执行前置脚本：

```bash
./scripts/ensure_debug_deps.sh debug
./scripts/ensure_debug_deps.sh pre
./scripts/ensure_debug_deps.sh prod
```

这个脚本只是做“前置依赖检查/启动”，真正启动 Go 程序还是靠 VS Code 的 debug 配置。

你说的 IP、端口、密码不对，这个改法很简单，主要改这几类文件。

---

<br/>

**改 debug 环境**
MySQL 和 Redis 分别配置在 `config/debug/mysql.yaml`、`config/debug/redis.yaml`。

你可以改这些值：

```yaml
mysql:
  host: 127.0.0.1
  port: "3306"
  user: root
  password: hh109
  database: HG_MLC_DB
  migrate_expect_version: 1

redis:
  host: 127.0.0.1
  port: "6379"
```

如果你本机 MySQL 不是 `3306`，或者密码不是 `hh109`，就在这里改。

---

<br/>

**改 pre 环境**
文件在 `config/pre/mysql.yaml`、`config/pre/redis.yaml`。

目前我给你配的是“本地模拟预发”：

```yaml
mysql:
  host: 127.0.0.1
  port: "3308"
  user: root
  password: hh109
  database: HG_MLC_PRE_DB
  migrate_expect_version: 1

redis:
  host: 127.0.0.1
  port: "6380"
```

<br/>
如果你想让 `pre` 连真实预发机器，比如：

```yaml
mysql:
  host: 10.10.1.25
  port: "3306"
  user: pre_user
  password: 你的密码
  database: pre_db
  migrate_expect_version: 1

redis:
  host: 10.10.1.26
  port: "6379"
```

也可以直接改这里。

但注意一件事：

- 如果 `pre` 改成远端机器地址，那它就不应该再“自动启动 docker compose”
- 它应该只做检查

也就是说：

- 本地模拟预发：`127.0.0.1 + 3308/6380 + compose`
- 真实预发环境：远程 IP + 只检查不启动

---

<br/>

**改 prod 环境**
文件在 `config/prod/mysql.yaml`、`config/prod/redis.yaml`。

例如：

```yaml
mysql:
  host: prod-mysql.internal
  port: "3306"
  user: app
  password: "********"
  database: HG_MLC_PROD_DB
  migrate_expect_version: 1

redis:
  host: prod-redis.internal
  port: "6379"
```

生产环境建议你只改这些连接信息，不要做自动启动。

**改 pre 本地容器端口**
如果你本地 `3308` 或 `6380` 被占用了，就改：
[config/docker/hg_docker_compose.pre.yml](/Users/ganghuang/HGFiles/GitHub/GoProject/src/MLC_GO/config/docker/hg_docker_compose.pre.yml:7)

现在是：

```yaml
ports:
  - "3308:3306"
```

和

```yaml
ports:
  - "6380:6379"
```

如果你想改成 `3310` 和 `6381`，那就同时改两边：

1. 改 compose：

```yaml
- "3310:3306"
- "6381:6379"
```

2. 再同步修改 `config/pre/mysql.yaml` 和 `config/pre/redis.yaml` 中的端口。

一定要两边一致，不然脚本能拉起容器，但程序连不上。

---

<br/>

**改 VS Code 启动配置**
如果你想改调试名、参数、环境名，就看：
[.vscode/launch.json](/Users/ganghuang/HGFiles/GitHub/GoProject/src/MLC_GO/.vscode/launch.json:8)

这里主要是：

- `preLaunchTask`
- `env.SERVER_ENV`
- `args`

通常优先修改当前环境的 `mysql.yaml`、`redis.yaml` 和对应 compose 端口。

<br/>

**最推荐的修改原则**
你可以按这个思路改：

- 本地开发不对：改 `config/debug/mysql.yaml` 和 `redis.yaml`
- 本地模拟预发不对：改 `config/pre` 下的模块配置和 `hg_docker_compose.pre.yml`
- 真实预发地址不对：改 `config/pre` 下的模块配置
- 生产地址不对：改 `config/prod` 下的模块配置

**你现在可以先这样试一次**
如果你要跑本地模拟预发：

1. 先检查 `config/pre/mysql.yaml`、`redis.yaml` 里的地址、端口和密码
2. 再看 `hg_docker_compose.pre.yml` 里的端口映射对不对
3. 然后在 VS Code 选：
   `🧪 Launch MLC_GO Root main.go (pre)`
4. 点运行

如果你要跑本地 debug：

1. 先检查 `config/debug/mysql.yaml` 和 `redis.yaml`
2. 在 VS Code 选：
   `🧪 Launch MLC_GO Root main.go (debug)`
3. 点运行

如果你愿意，我下一步可以直接帮你做一件更省心的事：

你把你真实想用的三套配置发我：

- debug 的 MySQL/Redis 地址、端口、账号、密码
- pre 的地址、端口、账号、密码
- prod 的地址、端口、账号、密码

---

<br/><br/><br/>

> <h2 id="Intel电脑修改配置启动">Intel电脑修改配置启动</h2>

不同开发机器的 MySQL 密码不同时，修改 `config/debug/mysql.yaml` 中的 `mysql.password`。工程不再根据 CPU 架构隐式切换密码。

启动 redis：

```sh
redis-server
```

启动 mysql

```sh
# M2Pro sql 启动
sudo mysql.server start

cd /Users/harleyhuang/HGFiles/GitHub/GoProject/src/MLC_GO/scripts
./db.sh shell
```

```sh
localhost:8080/auth/send_code?phone=17681317668
```

---

<br/><br/><br/>

> <h2 id="终端查看MySQL表">终端查看MySQL表</h2>

### [MySQL 教程](https://www.runoob.com/mysql/mysql-administration.html)

- mysql 启动：

```sh
sudo mysql.server start
```

<br/>
- mysql 关闭:

```sh
mysql.server stop
```

<br/>
- 进入 mysql 指令环境:

```sh
sudo mysql -u root -p
```

<br/>
- 查看数据库:

```sh
show databases;
```

<br/>
- 使用 db_test 数据库:

```sh
use db_test;
```

<br/>
- 查看已有数据表:

```sh
show tables;
```

<br/>
- 数据表结构信息: 
```sh
show columns from 表名;
```
<br/>

### gin 端口占用解决:

- 查找占用 8080 端口的进程 PID: sudo lsof -i :8080
- 终止进程（例如 PID 为 1234）: sudo kill -9 1234

<br/>

### 执行 `migrations`文件夹下的 `xxx.up.sql` 文件,在终端：

```sh
# xxx.sql 文件里已经有使用具体的某个数据库，如：USE HG_MLC_DB;
mysql -uroot -p < xxx.sql.up.sql


# 若是xxx.sql 某有指明使用哪个数据库使用
mysql -uroot -p HG_MLC_DB < xxx.sql.up.sql
```

但是通常使用 ** migrate 工具** 是最主流的，可以使用这个。

<br/><br/>

> <h3 id="redis启动">redis启动</h3>

```sh
# redis 启动
redis-server

# M2Pro sql 启动
sudo mysql.server start

# Intel sql启动 密码：回车即可
mysql -u root -p
```


<br/><br/><br/>

***
<br/>

> <h1 id="表分布">表分布</h1>


***
<br/><br/><br/>
> <h2 id="用户模块">用户模块</h2>


***
<br/><br/><br/>
> <h2 id="视频模块">视频模块</h2>

```sh
--   1. video_submissions      稿件主表（一次投稿）
--   2. video_files            视频文件表（每个视频/分P）
--   3. video_tags             视频标签关联表
--   4. video_scheduled_publish 定时发布表
--   5. video_commercial_promotion 商业推广表
--   6. video_chapters        视频章节表
--   7. video_subtitles       视频字幕表
```

<br/><br/><br/>

---

<br/>

> <h1 id="文件结构介绍">文件结构介绍</h1>

```bash
MLC_GO/
├── Dockerfile
├── Makefile
├── MLC_GO_REMADE.md
├── TestNotes
├── conf
├── cover.out
├── coverage.html
├── docs
├── go.mod
├── go.sum
├── main.go
├── middleware
├── models
├── pkg
├── routers
└── runtime
```

- Dockerfile
- MLC_GO_REMADE.md: 项目介绍
- TestNotes: 测试练习 Go 语法
- conf：用于存储配置文件
- cover.out
- coverage.html
- docs: 基本文件
- go.mod: 模块依赖管理文件
- go.sum: 依赖校验文件
- main.go: 入口文件
- middleware：应用中间件
- models：应用数据库模型
- pkg：第三方包
- routers 路由逻辑处理
- runtime：应用运行时数据

<br/>

安装所有没有安装的库，使用命令：

```bash
go get

# 或者
go mod tidy
```

<br/><br/>

**环境配置文件：**

```sh
├── config/
│   ├── MLC.env                # 唯一环境文件，只提供默认 SERVER_ENV
│   ├── base/                  # 跨环境公共默认值
│   │   ├── app.yaml
│   │   ├── log.yaml
│   │   ├── mysql.yaml
│   │   ├── redis.yaml
│   │   ├── kafka.yaml
│   │   └── tracing.yaml
│   ├── debug/                 # 开发环境覆盖
│   ├── pre/                   # 预发布环境覆盖
│   └── prod/                  # 生产环境覆盖
```

启动时按 `base`、当前环境的顺序合并模块配置，环境配置覆盖公共默认值。默认配置根目录为 `./config`，部署到其他工作目录时可通过 `MLC_CONFIG_DIR` 显式指定；`SERVER_ENV` 仅支持 `debug`、`pre`、`prod`。

本地 `debug` 环境允许 Kafka 未启动时以降级模式运行，便于调试不依赖消息队列的功能；设置 `KAFKA_REQUIRED=true` 可强制使用生产一致的启动检查。`pre` 和 `prod` 环境始终要求 Kafka 可用。

| 环境   | 常用标识          | 用途       |
| ------ | ----------------- | ---------- |
| 开发   | `debug` / `dev`   | 本地开发   |
| 预发布 | `pre` / `staging` | 上线前验证 |
| 正式   | `prod`            | 线上环境   |

---

<br/><br/><br/>

> <h2 id="功能模块文件分布">功能模块文件分布</h2>

```sh
myapp/
├── cmd/
│   └── server/
│       └── main.go               # 程序入口
├── internal/
│   ├── config/                   # 配置加载（环境变量、YAML等）
│   ├── database/                 # DB 数据库连接与初始化
│   ├── cache/                    # Redis 封装（缓存）
│   │
│   ├── models/                   # 所有数据模型（按功能拆分子目录更佳）
│   │   ├── user.go
│   │   └── post.go               # 👈 新增：朋友圈/动态模型
│   │
│   ├── modules/                  # 👈 核心变化：按业务域划分模块（推荐！）
│   │   │
│   │   ├── user/                 # 用户模块（原 auth 相关移入）
│   │   │   ├── repository/      #  数据访问层（DAO）,数据库MySQL操作（Insert / Update / Get）
│   │   │   │   └── user_repository.go
│   │   │   ├── service/         # 业务逻辑层（业务逻辑）
│   │   │   │   └── user_service.go
│   │   │   └── handler/         # HTTP 请求处理（Controller）（HTTP 接口）
│   │   │       └── user_handler.go
│   │   │
│   │   └── post/                 # 👈 新增：朋友圈模块
│   │       ├── repository/
│   │       │   └── post_repository.go
│   │       ├── service/
│   │       │   └── post_service.go
│   │       └── handler/
│   │           └── post_handler.go
│   │
│   └── pkg/                      # 公共工具，可复用的公共包（非本项目独有才放入这里，如 jwt、hash、middleware 等）
│       ├── middleware/
│       │   └── auth.go          # 认证中间件（验证 token）
│       └── utils/
│           └── password.go
│
├── migrations/                   # 数据库迁移脚本（可选，配合 golang-migrate 或 goose，可按模块分文件）
│   ├── 000001_create_users.up.sql
│   └── 000002_create_posts.up.sql
│
├── .env                         # 环境变量文件（不要提交到git）
├── go.mod
└── README.md
```

```sh
大厂标准分层
┌─────────────────────────────────────────────────────────────┐
│                     请求入口                                  │
├─────────────────────────────────────────────────────────────┤
│  基础层（所有请求必经）                                         │
│  ├── CORS          跨域处理                                   │
│  ├── Recovery      panic 恢复                                 │
│  ├── RequestID     请求 ID 生成                                │
│  ├── AccessLog     访问日志                                    │
│  └── JSONHeader    响应头设置                                  │
├─────────────────────────────────────────────────────────────┤
│  安全层（按需组装）                                            │
│  ├── MethodGuard   HTTP 方法校验                               │
│  ├── HeaderGuard   请求头校验 + 签名验证                        │
│  ├── JWTAuth       JWT 认证                                   │
│  └── Permission    权限校验                                    │
├─────────────────────────────────────────────────────────────┤
│  业务层（模块路由组）                                          │
│  ├── AuthModule    公开路由                                    │
│  ├── UserModule    需认证路由                                  │
│  └── OrderModule   订单路由                                    │
└─────────────────────────────────────────────────────────────┘
```

> **说明：** > `internal/`:Go 的约定，该目录下的代码只能**被本项目引用**，防止被外部项目 import；
> 分层架构：**`Handler → Service → Repository → Model + DB/Cache`**，职责分离，便于测试和维护

```txt
问题分析：
1. 职责不清：Handler 层应该只负责 HTTP 请求的解析和响应，不应该包含业务逻辑
2. 业务逻辑泄露：Login 方法中包含了太多的业务逻辑（JWT 生成、Redis 操作等）
3. Service 层不完整：UserService 缺少一些方法，比如 Login、SendCode 等
4. 重复代码：loginHandlerV2、loginHandler 等旧函数与 Login 方法重复
优化方案：
1. 将业务逻辑从 Handler 层移到 Service 层
2. Handler 层只负责请求解析、参数校验、调用 Service、响应返回
3. Service 层负责业务逻辑、数据处理
4. Repository 层负责数据访问
```

<br/><br/>

**设计说明：**

- **1.按业务域（Domain）组织代码 → modules/**
  - 每个核心业务（user, post, comment, message...）是一个独立子模块。
  - 每个模块内部包含自己的 handler → service → repository → model（如果 model 复杂也可放 module 内）。
  - 优点：
    - 高内聚：朋友圈的所有逻辑集中在一起，不污染用户模块。
    - 低耦合：修改朋友圈不影响用户注册逻辑。
    - 易于团队协作：不同人负责不同模块。

> 📌 替代方案：有些人用 features/ 或 domains/，但 modules/ 更通用。

<br/>

- **2.Model 是否放在 modules/xxx/model/？**
  - 如果模型简单且被多个模块共享（如 User 被 Post 引用），建议仍放在顶层 internal/models/。
  - 如果模型高度专属某个模块（如 PostLike 只在 post 模块用），可放入 modules/post/model/。

✅ 推荐初期统一放 internal/models/，后期再按需拆分。

<br/>

- 3.**公共能力下沉到 pkg/**
  - 认证中间件（解析 JWT、查 Redis 验证登录态）
  - 密码哈希工具
  - 分页工具、错误封装等

示例：`pkg/middleware/auth.go`

```go
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		// 用 cache.RDB 查 token 是否有效
		// 若无效，返回 401
		// 若有效，将 userID 注入 context
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
```

在 post_handler.go 中使用：

```go
http.HandleFunc("/posts", middleware.AuthMiddleware(postHandler.CreatePost))
```

<br/>

- **4.据库迁移（Migrations）按功能拆分**
  - 每个新功能对应一个或多个 migration 文件。
  - 工具推荐：golang-migrate

```sql
-- migrations/000002_create_posts.up.sql
CREATE TABLE posts (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

<br/>

**朋友圈功能关键代码示意**

`models/post.go`

```go
type Post struct {
	ID        uint      gorm:"primaryKey"
	UserID    uint      // 关联用户
	Content   string    gorm:"not null"
	CreatedAt time.Time
}
```

<br/>

**`modules/post/repository/post_repository.go `**

```go
func (r *PostRepository) Create(post *models.Post) error {
	return database.DB.Create(post).Error
}

func (r *PostRepository) GetFeedByUserID(userID uint, limit, offset int) ([]models.Post, error) {
	var posts []models.Post
	err := database.DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).Find(&posts).Error
	return posts, err
}
```

<br/>

`modules/post/service/post_service.go`

```go
func (s *PostService) CreatePost(userID uint, content string) error {
	if len(content) == 0 {
		return errors.New("content cannot be empty")
	}
	post := &models.Post{UserID: userID, Content: content}
	return s.repo.Create(post)
}
```

<br/>

`modules/post/handler/post_handler.go `

```go
func (h *PostHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userID").(uint) // 从 auth middleware 注入
	var req struct{ Content string }
	json.NewDecoder(r.Body).Decode(&req)

	err := h.service.CreatePost(userID, req.Content)
	// ...
}
```

<br/>

**✅ 总结：如何应对未来更多功能？**
| 新功能 | 如何扩展目录 |
|------------|----------------------------------|
| 评论 | 新增 `modules/comment/` |
| 点赞 | 在 `post` 模块内加 `LikeService`，或新建 `modules/like/` |
| 私信 | 新增 `modules/message/` |
| 文件上传 | 新增 `modules/storage/` + `pkg/upload/` |




***
<br/><br/><br/>
> <h2 id="大厂常见的DDD+CleanArchitecture+微服务预留结构">大厂常见的 DDD + Clean Architecture + 微服务预留结构</h2>

大厂常见的 **DDD + Clean Architecture + 微服务预留结构**。

---
<br/>

## 推荐目录结构（单体应用阶段）

```text
project/
│
├── cmd/
│   └── api/
│       └── main.go
│
├── configs/
│   ├── config.yaml
│   ├── config.dev.yaml
│   ├── config.test.yaml
│   └── config.prod.yaml
│
├── internal/
│
│   ├── user/
│   │   ├── controller/
│   │   ├── service/
│   │   ├── repository/
│   │   ├── model/
│   │   ├── dto/
│   │   ├── converter/
│   │   └── event/
│   │
│   ├── auth/
│   │   ├── controller/
│   │   ├── service/
│   │   ├── repository/
│   │   ├── model/
│   │   └── dto/
│   │
│   ├── product/
│   │
│   ├── category/
│   │
│   ├── inventory/
│   │
│   ├── cart/
│   │
│   ├── order/
│   │
│   ├── payment/
│   │
│   ├── coupon/
│   │
│   ├── logistics/
│   │
│   ├── notification/
│   │
│   └── analytics/
│
├── pkg/
│
│   ├── jwt/
│   ├── contextx/
│   ├── logger/
│   ├── redis/
│   ├── mysql/
│   ├── kafka/
│   ├── rocketmq/
│   ├── elasticsearch/
│   ├── snowflake/
│   ├── validator/
│   ├── response/
│   ├── middleware/
│   ├── tracing/
│   ├── metrics/
│   ├── cache/
│   ├── lock/
│   ├── rate_limit/
│   ├── crypto/
│   └── utils/
│
├── api/
│   ├── openapi/
│   ├── swagger/
│   └── postman/
│
├── migrations/
│
├── deployments/
│   ├── docker/
│   ├── kubernetes/
│   └── helm/
│
├── scripts/
│
├── docs/
│
├── test/
│
└── third_party/
```

***
<br/>


## 核心模块解释

---

### cmd程序入口

```text
cmd/
└── api/
    └── main.go
```

只做启动。

```go
func main() {
    InitConfig()
    InitMysql()
    InitRedis()
    InitKafka()
    StartHTTPServer()
}
```

不要写业务。

<br/>

### configs**配置中心**

```text
configs/
├── config.dev.yaml
├── config.test.yaml
└── config.prod.yaml
```

例如：

```yaml
mysql:
  host: xxx

redis:
  addr: xxx

jwt:
  secret: xxx
```

<br/>

### internal真正的业务代码。

**user模块**

```text
user/
│
├── controller/
│   └── user_controller.go
│
├── service/
│   └── user_service.go
│
├── repository/
│   └── user_repository.go
│
├── model/
│   └── user.go
│
├── dto/
│   ├── request.go
│   └── response.go
│
├── converter/
│   └── user_converter.go
│
└── event/
    └── user_event.go
```

<br/>

**controller**

HTTP层

```go
POST /user/login
GET /user/profile
```

只负责：

```go
接收参数
调用service
返回结果
```

<br/>

### service业务逻辑层

```go
func Login()
func Register()
func ResetPassword()
```

例如：

```go
验证密码
生成JWT
写Redis
发送MQ
```

<br/>

### repository数据访问层

```go
FindByEmail()
FindByUserID()
CreateUser()
```

只写 SQL。

<br/>

**model** 数据库实体

```go
type User struct {
    ID int64
    UserID string
    Nickname string
}
```

<br/>

**dto前后端交互对象**

```go
type LoginReq struct {
    Email string
}
```

```go
type LoginResp struct {
    Token string
}
```

<br/>

**converter** 转换层

```sh
DTO → Model

Model → DTO
```
<br/>

**event** 领域事件

```go
UserCreated
UserDeleted
UserLogin
```

后面接 Kafka。

<br/>

**pkg公共基础设施**

**jwt**

```text
pkg/jwt
├── claims.go
├── parser.go
└── generator.go
```

<br/>

**contextx统一获取：**

```go
GetUserID(ctx)
GetTraceID(ctx)
GetLang(ctx)
```
<br/>

**logger日志系统,通常：**

```go
Uber Zap
```

<br/>

**redis**

```go
Get()
Set()
Del()
```

统一封装。

<br/>

**mysql**数据库连接池

```go
gorm.Open()
```

或者：

```go
sqlx.Connect()
```

<br/>

**kafka**消息队列

```go
Producer
Consumer
```

<br/>

**lock分布式锁**

```go
AcquireLock()
ReleaseLock()
```

底层：

```go
Redis
```

<br/>

**rate_limit限流**

```go
Token Bucket
Sliding Window
```

<br/>

**cache**缓存组件

```go
Cache Aside
```

<br/>

**tracing链路追踪,一般：**

```text
OpenTelemetry
Jaeger
```

<br/>

**metrics监控指标,一般：**

```text
Prometheus
Grafana
```
<br/>


### **migrations**,数据库迁移

```text
migrations/

000001_init.up.sql
000001_init.down.sql

000002_user.up.sql
000002_user.down.sql
```

你最近接触的就是这里。

<br/>


### deployments部署

<br/>

### Docker

```text
Dockerfile
docker-compose.yml
```

<br/>

### Kubernetes

```text
deployment.yaml
service.yaml
ingress.yaml
```

<br/>

### docs设计文档

```text
架构设计.md
数据库设计.md
接口设计.md
```

<br/>

### 千万级并发还要增加什么？

当用户达到：

```text
100万+
1000万+
```

时目录还会继续扩展：

```text
internal/
├── search/
├── recommendation/
├── risk_control/
├── audit/
├── scheduler/
├── workflow/
├── task/
├── message/
└── ai/
```

基础设施增加：

```text
pkg/
├── kafka/
├── rocketmq/
├── elasticsearch/
├── clickhouse/
├── canal/
├── etcd/
├── nacos/
├── sentinel/
└── grpc/
```

---
<br/>

### 如果是你当前的 GO 工程

结合你之前提到的：

* 用户系统
* JWT
* Redis
* MySQL
* RocketMQ/Kafka
* Docker部署
* 后续微服务

我建议第一版直接采用：

```text
internal/
├── auth/
├── user/
├── device/
├── product/
├── order/
└── notification/

pkg/
├── jwt/
├── contextx/
├── logger/
├── mysql/
├── redis/
├── middleware/
├── response/
├── validator/
├── cache/
├── lock/
└── mq/
```

这个结构足够支撑从 0 到百万级用户，并且未来拆分成独立微服务时基本不需要大规模重构。真正的“千万级并发”更多依赖于架构（负载均衡、缓存、MQ、数据库分库分表、Kubernetes、链路追踪等），而不仅仅是目录结构。



<br/><br/><br/>

---

<br/>

> <h1 id="文件规则">文件规则</h1>



<br/><br/><br/>
> <h2 id="协议规则">协议规则</h2>
> **比如：**

协议的方法前要加入**`协议名_+方法名`**：

```go

type Writer interface {
	Writer_read(text string)
}
```

<br/>

---
<br/><br/><br/><br/><br/>
># <h1 ID="Golang开源项目汇总列表"> [Golang 开源项目汇总列表](https://github.com/hackstoic/golang-open-source-projects)</h1>
<br/>

<br/>

---

<br/><br/><br/>
># <h1 id="golang-gin-realworld-example-app工程">[golang-gin-realworld-example-app 工程](https://github.com/gothinkster/golang-gin-realworld-example-app/tree/master)</h1>

**注册接口测试**

```sh
curl -X POST "http://localhost:8080/api/users/" \
     -H "Content-Type: application/json" \
     -d '{
       "user": {
         "username": "李白",
         "email": "libai@qq.com",
         "password": "mypassword1236789",
         "bio": "Software Developer Golang",
         "image": "https://example.com/avatarPic.jpg"
       }
     }'
```

<br/>
**登录**

```
curl -X POST "http://localhost:8080/api/users/login" \
     -H "Content-Type: application/json" \
     -d '{
       "user": {
         "email": "libai@qq.com",
         "password": "mypassword1236789"
       }
     }'

{"user":{"username":"李白","email":"libai@qq.com","bio":"Software Developer Golang","image":"https://example.com/avatarPic.jpg","token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3NDI3ODQyOTUsImlkIjozfQ.Rkywy09E-iVMmKqyMVIBcEXZtcm4W3x1xatXL6WrxyY"}}
```

> # <h1 id="go-gin-api全栈项目">[go-gin-api 全栈项目](https://github.com/xinliangnote/go-gin-api?tab=readme-ov-file)</h1>

- **简介**：基于 Gin 的模块化 API 框架，封装了 JWT 鉴权、日志管理、数据库操作等常用功能。
- [文档](https://www.yuque.com/xinliangnote/go-gin-api/mb9ad8)
- **特性**：
  - 提供代码生成器，快速生成 CRUD 接口。
  - 集成 Swagger 文档，支持自动化测试。
  - 适合团队协作，规范开发流程。
- **项目地址**：[github.com/xinliangnote/go-gin-api](https://github.com/xinliangnote/go-gin-api)
- **学习价值**：新手友好，适合学习 API 分层设计和工程化实践。

<br/><br/><br/>

---

<br/>

> <h1 id="gin-vue-admin全栈平台">gin-vue-admin全栈平台</h1>

- **简介**：前后端分离的管理系统，后端使用 Gin 实现 RESTful API，前端基于 Vue3。
- **特性**：
  - 支持动态路由、权限控制、文件上传等企业级功能。
  - 集成 ChatGPT 自动生成代码，提升开发效率。
  - 提供完整的 DevOps 工具链（如 CI/CD 配置）。
- **项目地址**：[github.com/flipped-aurora/gin-vue-admin](https://github.com/flipped-aurora/gin-vue-admin)
- **适用场景**：中后台管理系统开发，如 CRM、OA 系统。

<br/><br/><br/>

---

<br/>

> <h1 id="ferry工单系统">ferry工单系统</h1>

- **简介**：基于 Gin 和 Vue 的工单管理系统，后端提供完整的 RESTful API 支持。
- **特性**：
  - 支持自定义审批流程、权限分级。
  - 集成任务钩子和统计功能，适合企业内部流程管理。
- **项目地址**：[github.com/lanyulei/ferry](https://github.com/lanyulei/ferry)
- **学习价值**：了解复杂业务场景下的 API 设计。

<br/><br/><br/>

---

<br/>

> <h1 id="Gorm（ORM工具）">Gorm（ORM 工具）</h1>

- **简介**：Go 生态中最流行的 ORM 库，常与 RESTful API 结合操作数据库。
- **特性**：
  - 支持事务、关联查询、软删除等高级功能。
  - 自动迁移数据库表结构，简化开发流程。
- **项目地址**：[github.com/go-gorm/gorm](https://github.com/go-gorm/gorm)
- **适用场景**：快速实现 CRUD 接口，如用户管理系统。

<br/><br/><br/>

---

<br/>

> # <h1 id="gin-gorm-restful-api">[gin-gorm-restful-api](https://juejin.cn/post/7036011047391592485)</h1>

- **Gin + GORM 项目**
- **简介**: 使用 Gin 框架和 GORM 构建的 RESTful API 项目，结构清晰，模块化设计，适合初学者快速上手。
- **特点**:
  - 清晰的目录结构（controller、service、model 等）。
  - 支持统一的 JSON 响应格式。
  - 使用 GORM 进行数据库操作，支持 MySQL、PostgreSQL 等。
- **GitHub 地址**: [gin-gorm-restful-api](https://github.com/your-repo/gin-gorm-restful-api)

<br/><br/><br/>

---

<br/>

> <h1 id="Go-Zero商城项目">Go-Zero商城项目</h1>

- **项目名称**: `go-zero-mall`
- **简介**: 基于 Go-Zero 框架开发的商城 RESTful API 服务，包含用户、商品、订单等模块。
- **特点**:
  - 使用 Go-Zero 的 `goctl` 工具自动生成代码。
  - 支持 Protobuf 定义 API 接口。
  - 模块化设计，适合中大型项目。
- **GitHub 地址**: [go-zero-mall](https://github.com/your-repo/go-zero-mall)

<br/><br/><br/>

---

<br/>

> <h1 id="echo-restful-api">echo-restful-api</h1>

- **Echo 框架示例**
- **简介**: 使用 Echo 框架构建的高性能 RESTful API 项目，适合需要高性能的场景。
- **特点**:
  - 支持中间件（如日志、认证）。
  - 结构简单，易于扩展。
  - 提供 Swagger 文档支持。
- **GitHub 地址**: [echo-restful-api](https://github.com/your-repo/echo-restful-api)

<br/><br/><br/>

---

<br/>

> <h1 id="gorilla-mux-restful-api">gorilla-mux-restful-api</h1>

- **Gorilla Mux 项目**
- **简介**: 使用 Gorilla Mux 路由库构建的 RESTful API 项目，适合需要灵活路由配置的场景。
- **特点**:
  - 支持复杂的路由匹配规则。
  - 中间件支持（如 CORS、日志）。
  - 适合中小型项目。
- **GitHub 地址**: [gorilla-mux-restful-api](https://github.com/your-repo/gorilla-mux-restful-api)

<br/><br/><br/>

---

<br/>

> <h1 id="beego-restful-api">beego-restful-api</h1>

- **Beego 框架示例**
- **简介**: 使用 Beego 框架构建的 RESTful API 项目，适合需要快速开发的场景。
- **特点**:
  - 内置 ORM、缓存、日志等功能。
  - 提供自动化 API 文档生成。
  - 适合全栈开发。
- **GitHub 地址**: [beego-restful-api](https://github.com/your-repo/beego-restful-api)

<br/><br/><br/>

---

<br/>

> <h1 id="kratos-restful-api">kratos-restful-api</h1>

- **Kratos 微服务框架**
- **简介**: 基于 Bilibili 开源的 Kratos 框架构建的 RESTful API 项目，适合微服务架构。
- **特点**:
  - 支持 gRPC 和 HTTP 双协议。
  - 提供完善的中间件和插件支持。
  - 适合大型分布式系统。
- **GitHub 地址**: [kratos-restful-api](https://github.com/your-repo/kratos-restful-api)

<br/><br/><br/>

---

<br/>

> <h1 id="gin-swagger-restful-api">gin-swagger-restful-api</h1>

- **Gin + Swagger 项目**
- **简介**: 使用 Gin 框架和 Swagger 构建的 RESTful API 项目，提供完整的 API 文档支持。
- **特点**:
  - 自动生成 Swagger 文档。
  - 支持 JWT 认证。
  - 适合需要 API 文档化的项目。
- **GitHub 地址**: [gin-swagger-restful-api](https://github.com/your-repo/gin-swagger-restful-api)

<br/><br/><br/>

---

<br/>

> <h1 id="go-kit-restful-api">go-kit-restful-api</h1>

- **Go-Kit 微服务示例**
- **简介**: 使用 Go-Kit 构建的微服务风格 RESTful API 项目，适合需要高可扩展性的场景。
- **特点**:
  - 支持服务发现、负载均衡。
  - 提供日志、监控等中间件。
  - 适合分布式系统。
- **GitHub 地址**: [go-kit-restful-api](https://github.com/your-repo/go-kit-restful-api)

<br/><br/><br/>

---

<br/>

> <h1 id="fiber-restful-api">fiber-restful-api</h1>

- **Fiber 框架示例**
- **简介**: 使用 Fiber 框架构建的高性能 RESTful API 项目，适合需要极致性能的场景。
- **特点**:
  - 性能接近原生 Go HTTP 服务器。
  - 支持中间件和路由分组。
  - 适合中小型高性能项目。
- **GitHub 地址**: [fiber-restful-api](https://github.com/your-repo/fiber-restful-api)

<br/><br/><br/>

---

<br/>

> <h1 id="gin-gorm-jwt-restful-api">gin-gorm-jwt-restful-api</h1>

- **Gin + GORM + JWT 项目**
- **简介**: 使用 Gin、GORM 和 JWT 构建的 RESTful API 项目，包含用户认证功能。
- **特点**:
  - 支持 JWT 认证。
  - 提供用户注册、登录、权限管理功能。
  - 适合需要认证的 API 项目。
- **GitHub 地址**: [gin-gorm-jwt-restful-api](https://github.com/your-repo/gin-gorm-jwt-restful-api)

<br/><br/><br/>

---

<br/>

> <h1 id="框架">框架</h1>
> <br/>

<br/><br/><br/>

---

<br/>

> <h1 id="NSQ源码阅读">NSQ源码阅读</h1>

很棒的学习计划！NSQ 是一个高性能的分布式消息队列系统，Go 实现、结构清晰、注重工程实践，非常适合学习。

---

## 一、目标拆解：你能从 NSQ 学到什么？

- Go 语言的 **工程化开发实践**（目录结构、模块划分、日志处理等）
- **网络编程（TCP、HTTP）**
- **并发模型（goroutine、channel）**
- **消息队列原理与实现（发布/订阅、消息持久化、ack 机制等）**
- **系统设计理念**（模块解耦、配置热更新、服务注册等）

---

<br/>

** 二、学习前准备（0 基础也能入门）**

**1. 基础技能**

先具备以下基础（可以边学 NSQ 边查）：

- Go 基础语法、interface、goroutine、channel、select
- Go 标准库：`net`, `http`, `os`, `sync`, `log`
- Git 基础操作、源码 clone、`go build` 等

> 学习推荐：

- Go 入门教程：[https://tour.go-zh.org/](https://tour.go-zh.org/)
- 实战 Go： [https://github.com/unknwon/go-fundamental-programming](https://github.com/unknwon/go-fundamental-programming)

---

<br/>

**三、具体学习步骤（建议打印或收藏）**

**✅ Step 1：克隆 NSQ 项目并能运行**

```bash
git clone https://github.com/nsqio/nsq.git
cd nsq
make
./build/nsqd --help   # 查看帮助
```

运行一个简单 demo：

```bash
# 启动 nsqd
./build/nsqd --lookupd-tcp-address=127.0.0.1:4160 &

# 启动 lookupd
./build/nsqlookupd &

# 启动 consumer 测试
curl -d 'hello world' 'http://127.0.0.1:4151/pub?topic=test'
```

<br/>

**顶层目录结构概览：**

```sh
nsq/
├── apps/           ← CLI 命令入口（nsqd、nsqlookupd、nsqadmin）
├── nsqd/           ← 核心服务：接收、处理、转发消息
├── nsqlookupd/     ← 服务发现：记录哪些 topic 存在哪些 nsqd 上
├── nsqadmin/       ← UI 控制台：查看 topic、channel 等
├── protocol/       ← 客户端通信协议（TCP/HTTP）
├── internal/       ← 通用模块（版本号、日志、sync utils）
└── queue/          ← 消息队列底层存储实现（内存/磁盘）
```

---

<br/>

**✅ Step 2：了解整个系统架构（宏观理解）**

> 找到图示：NSQ 架构图：[https://nsq.io/components/](https://nsq.io/components/)

<br/>

**阅读源码建议路线（从易到难）**

**建议顺序：**

- 1.nsqd 的启动流程（main.go）

- 2.Topic 的创建与消息发布：nsqd/internal/topic.go

- 3.channel 的调度逻辑：channel.go

- 4.消息持久化逻辑：diskqueue.go

- 5.客户端连接和协议处理：protocol_v2.go

- 6.nsqlookupd 的注册发现流程：nsqlookupd/server.go

- 7.nsqd 和 lookupd 交互流程：lookup.go

<br/>

**关键组件：**

- `nsqd`：核心组件，负责接收、存储、转发消息
- `nsqlookupd`：服务发现中心，维护 topic 和 channel 映射
- `nsqadmin`：Web UI 管理后台

| 组件           | 作用                                             |
| -------------- | ------------------------------------------------ |
| **nsqd**       | 消息服务的核心：接受生产者消息，投递给消费者     |
| **nsqlookupd** | 服务发现：让消费者找到哪些 `nsqd` 有订阅的 topic |
| **nsqadmin**   | Web UI 控制面板：可监控 topic、channel、消息等   |

重点理解流程：

```sh
Producer --> nsqd --> (Lookupd) --> Consumer
```

<br/>

**运行时调用关系（简化）**

```
Producer → Topic.PutMessageDeferred
        → Channel.PutMessageDeferred
            ├─ atomic.AddUint64   // 计数
            └─ StartDeferredTimeout
                   └─ 放入 delayQueue
                       └─ delayQueue 到期后 → PutMessage → messagePump → Client

```

<br/>

**消息流转过程:**

```sh
Producer
  ↓（发布消息）
 nsqd (核心服务)
  ↓（将消息按 topic 投递到 channel）
 Consumer
```

如果有多个 nsqd，则消费者通过 nsqlookupd 来发现可连接的节点。

---

<br/>

### ✅ Step 3：阅读源码建议路线（从易到难）

建议顺序：

1. `nsqd` 的启动流程（main.go）
2. Topic 的创建与消息发布：`nsqd/internal/topic.go`
3. channel 的调度逻辑：`channel.go`
4. 消息持久化逻辑：`diskqueue.go`
5. 客户端连接和协议处理：`protocol_v2.go`
6. nsqlookupd 的注册发现流程：`nsqlookupd/server.go`
7. nsqd 和 lookupd 交互流程：`lookup.go`

<br/>

- **`nsqd` 的启动流程（main.go）** 文件查找：
- apps/nsqd/main.go 是启动 nsqd 的标准入口，里面会调用 nsqd 包里的核心逻辑（比如 nsqd.New() 等）
- 它是程序真正的 main 包所在位置：
- apps/nsqd/main.go：启动程序入口，负责命令行参数、配置初始化、日志初始化等
- nsqd/ 目录：包含 nsqd 的核心业务代码（消息处理、网络协议、存储等）

---

### ✅ Step 4：使用调试 + 打日志的方式阅读源码

- 使用 VS Code 或 Goland，设置断点调试（如在 `topic.PutMessage()`）
- 插入日志，比如：

```go
fmt.Println("PutMessage:", msg.ID)
```

观察调用路径和数据结构传递。

---

### ✅ Step 5：制作源码学习笔记

每看懂一个文件，做以下输出：

| 文件     | 作用           | 核心函数                    | 涉及模块/调用链          | 你的理解/总结 |
| -------- | -------------- | --------------------------- | ------------------------ | ------------- |
| topic.go | Topic 对象逻辑 | `PutMessage`、`messagePump` | channel.go、diskqueue.go | xxx           |

---

## 四、思路理解技巧

- **看 main 函数**：程序从哪启动，哪些服务注册在哪个模块？
- **找 interface 和 struct 实现**：如 `protocolV2` 是协议实现，了解其输入输出。
- **画图辅助**：将结构体和调用链画成时序图、模块图，有助于理解。
- **对照官方文档看代码**：[https://nsq.io/](https://nsq.io/)

---

## 五、进阶建议：自己实现一个 mini-NSQ

参考 NSQ 结构，自己实现一个小型版本的消息队列（本地内存即可）：

功能：

- topic + channel 架构
- TCP 接收消息
- 消息广播到多个消费者
- goroutine 实现并发处理

你会更理解 “为什么 NSQ 要这么设计”。

---

## 六、额外工具推荐

| 工具                       | 用途                   |
| -------------------------- | ---------------------- |
| Goland / VSCode            | IDE，方便调试          |
| GoLand 插件：Go Call Graph | 可视化调用链           |
| GoDoc / Sourcegraph        | 阅读注释和跳转函数定义 |
| `richgo test` / delve      | 测试和调试             |

---

## 七、社区交流和参考资料

- [NSQ 源码阅读中文系列](https://github.com/denghongcai/nsq-source-code-learning)
- GoCN、NSQ issue 区、知乎：搜“NSQ 源码解析”

---

## 总结：最重要的是「带着目的去看」

每次阅读都要问：

- **这个模块是干嘛的？**
- **它解决了什么问题？**
- **如果我写这个功能，我会怎么做？**

持续记录、总结、对比自己的思路，才是高效学习的关键。

需要我帮你逐步分析 NSQ 某部分源码或画图讲解吗？我可以陪你一起读。

> <h1 id="Gin框架">Gin框架</h1>

- **简介**：高性能 HTTP 框架，轻量且易用，支持中间件、路由分组、参数绑定等功能，适合快速构建 RESTful API。
- **特性**：
  - 集成验证器，支持请求参数自动校验。
  - 路由性能优化，底层基于 `httprouter`，处理速度极快。
  - 支持 Swagger 文档生成（需结合第三方库如 `swaggo`）。
- **项目地址**：[github.com/gin-gonic/gin](https://github.com/gin-gonic/gin)
- **案例参考**：常用于企业级 API 开发，如电商后端和微服务架构。

<br/><br/><br/>

---

<br/>

> <h1 id="Echo框架">Echo框架</h1>

- **简介**：高性能、可扩展的 Web 框架，支持 RESTful 路由设计，提供中间件、模板渲染等功能。
- **特性**：
  - 内置 JSON 序列化、请求验证等实用工具。
  - 支持 WebSocket 和 gRPC 集成，适合复杂场景。
  - 官方维护活跃，社区资源丰富。
- **项目地址**：[github.com/labstack/echo](https://github.com/labstack/echo)
- **案例参考**：适用于高并发 API 服务，如实时数据接口。

<br/><br/><br/>

---

<br/>

> <h1 id="GorillaMux路由库">Gorilla Mux（路由库）</h1>

- **简介**：灵活的路由库，支持 RESTful 路由匹配、中间件链式调用。
- **特性**：
  - 强大的路径参数解析（如正则匹配）。
  - 兼容标准库 `net/http`，适合渐进式升级旧项目。
- **项目地址**：[github.com/gorilla/mux](https://github.com/gorilla/mux)
- **案例参考**：适用于需要精细控制路由逻辑的 API 服务。

<br/><br/><br/>

---

<br/>

> <h1 id="Vegeta负载测试工具">Vegeta负载测试工具</h1>
> **性能优化**：结合 **Vegeta**（负载测试工具）对 API 进行压测。

<br/><br/><br/>

---

<br/>

> <h1 id="Authboss库">Authboss库-添加认证与授权模块</h1>
> **安全增强**：使用 **Authboss** 添加认证与授权模块。

<br/><br/><br/>

---

<br/>

> <h1 id="GoKit库-网关和分布式追踪">GoKit库-网关和分布式追踪</h1>

- **微服务架构**：参考 **GoKit** 实现 API 网关和分布式追踪。

<br/><br/><br/>

---

<br/>

> <h1 id="Beego库">Beego库</h1>
> **Beego** 提供了一个完整的 MVC 框架，除了 RESTful API 支持外，还包括 ORM、定时任务等功能，适合构建大型项目。  
>   项目地址：[github.com/beego/beego/v2](https://github.com/beego/beego/v2) citeturn0search0

<br/><br/><br/>

---

<br/>

> <h1 id="Fiber库">Fiber库</h1>
> **Fiber**灵感来源于 Express（Node.js 框架），追求极致性能和简洁 API，非常适合需要高并发和低延迟的 RESTful API 项目。  
>   项目地址：[github.com/gofiber/fiber](https://github.com/gofiber/fiber) citeturn0search0

<br/><br/><br/>

---

<br/>

> <h1 id="go-restful库">go-restful库</h1>
> **go-restful**  这个项目提供了一套工具来快速构建 REST 风格的 Web 服务，并在设计上借鉴了 Google 风格。  
>   项目地址：[github.com/emicklei/go-restful](https://github.com/emicklei/go-restful) citeturn0search0

<br/><br/><br/>

---

<br/>

> <h1 id="Chi库">Chi库</h1>
> **Chi**  一个轻量级且富有表现力的路由库，注重代码的可组合性与可读性，非常适合构建简单或中型 RESTful API。  
>    GitHub 地址：[github.com/go-chi/chi](https://github.com/go-chi/chi) citeturn0search0

<br/><br/><br/>

---

<br/>

> <h1 id="Viper配置管理库">Viper配置管理库</h1>

Viper 是 Go 语言中一个非常流行的配置管理库，用于处理应用程序的配置信息。它支持多种配置来源（如 JSON、YAML、TOML、环境变量、命令行参数、远程配置系统等），并能自动将它们合并成统一的配置视图。

---

<br/>

**Viper 的主要功能**

- 支持多种格式：JSON、TOML、YAML、HCL、envfile、Java properties。
- 可从文件、环境变量、命令行参数、远程配置（如 etcd、Consul）读取配置。
- 自动监听和重载配置文件（可选）。
- 支持默认值、别名、类型安全读取。
- 灵活的配置优先级（例如：命令行 > 环境变量 > 配置文件 > 默认值）。

<br/>

**安装 Viper**

使用 `go get` 安装：

```bash
go get github.com/spf13/viper
```

> 注意：如果你使用的是 Go Modules（Go 1.11+），会自动在 `go.mod` 中添加依赖。

<br/>

**基本使用示例**

- **1.创建配置文件（比如 config.yaml）**

```yaml
app:
  name: my-app
  port: 8080
database:
  host: localhost
  port: 5432
```

- **2.在 Go 代码中加载并使用**

```go
package main

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

func main() {
	// 设置配置文件名（不带扩展名）和路径
	viper.SetConfigName("config")     // 配置文件名（不带后缀）
	viper.SetConfigType("yaml")       // 配置类型
	viper.AddConfigPath(".")          // 配置文件搜索路径（当前目录）

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	// 读取配置项
	appName := viper.GetString("app.name")
	port := viper.GetInt("app.port")
	dbHost := viper.GetString("database.host")

	fmt.Printf("App Name: %s\n", appName)
	fmt.Printf("App Port: %d\n", port)
	fmt.Printf("DB Host: %s\n", dbHost)
}
```

<br/>

**其他常用方法**

| 方法                                | 说明                   |
| ----------------------------------- | ---------------------- |
| `viper.SetDefault("key", value)`    | 设置默认值             |
| `viper.BindEnv("key")`              | 绑定环境变量           |
| `viper.GetBool/GetString/GetInt...` | 类型安全地获取值       |
| `viper.Unmarshal(&struct)`          | 将配置反序列化到结构体 |
| `viper.WatchConfig()`               | 监听配置文件变化       |

**绑定环境变量**

```go
viper.BindEnv("app.port", "APP_PORT")
// 如果设置了环境变量 APP_PORT=9000，则会覆盖配置文件中的值
```

**反序列化到结构体**

```go
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Database DatabaseConfig `mapstructure:"database"`
}

type AppConfig struct {
	Name string `mapstructure:"name"`
	Port int    `mapstructure:"port"`
}

type DatabaseConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

var config Config
if err := viper.Unmarshal(&config); err != nil {
	log.Fatalf("Unable to decode config: %v", err)
}
```


<br/><br/><br/>

***
<br/>

> <h1 id="优化建议">优化建议</h1>

```sh
很多团队会：

数据库由 DBA / docker-init / k8s-init 创建
migrate 只做 schema migration

这样最稳定。
```

<br/>

后续将下面添加进skill，增加sql表的性能：

```sql
中修改不要增加外键避免后期因为分库分表、性能造成影响，请修改

：避免后期分库分表、跨库关联、写入性能、删除级联、DDL 迁移等问题。
业务关系仍然通过字段和索引保留，后续由 service/repository 层保证数据一致性，更适合高并发和可拆分架构。
```

<br/>

视频上传达到百万请求想要的条件

```sh
百万级并发说明
这次已经补上了大厂上传系统的关键组件接口：
Redis session
Redis 限流
Redis 幂等
异步任务发布接口
有界队列保护
流式 multipart 上传
但是要真正达到百万级上传请求并发，仍需要生产架构支撑：
对象存储直传
CDN/边缘上传加速
MQ 集群
转码/审核 worker 集群
Redis Cluster
MySQL 分库分表或元数据分片
全链路压测与容量评估
当前代码已经把 Go 业务服务从“直接同步做所有事”推进到“元数据 + Redis 状态 + 异步任务”的结构，但还没有把大文件流量从 Go 服务迁移到对象存储直传，所以不能真实宣称已达到百万级大文件上传并发。
后续优化建议
1. 实现 Publisher 的 RocketMQ/Kafka 版本，替换 MemoryPublisher。
2. 增加读取 SaveSubmitResult 的逻辑，重复提交时直接返回上次结果。
3. 把上传文件改成对象存储直传，Go 只发临时凭证和保存回调 metadata。
4. 使用 k6 编写真实压测脚本，覆盖 /upload、/draft、/submit，记录 P50/P95/P99、Redis QPS、DB QPS、磁盘 I/O 和内存
```

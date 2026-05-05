<!--
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-15 08:47:16
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-04-30 22:53:26
 * @FilePath: /MLC_GO/IntroduceREMADE.md
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
-->

> <h5></h5>
- [**‌工程启动**](#工程启动)
	- [VSCode启动](#VSCode启动)
	- [Intel电脑修改配置启动](#Intel电脑修改配置启动)
	- [redis启动](#redis启动)
- [**代码调用举例**](#代码调用举例)
	- [分页model调用](#分页model调用)
- [**高级语法**](#高级语法)
	- [函数签名闭包](#函数签名闭包)
- [**token解析**](#token解析)
- [**文件结构介绍**](#文件结构介绍)
	- [功能模块文件分布](#功能模块文件分布)
- [**文件规则**](#文件规则)
	- [协议规则](#协议规则)
- [SQL](#SQL)
	- [xx.sql问件加注释](#xx.sql问件加注释)
- [TCP通信自己实现](#TCP通信自己实现)
- [Golang 开源项目汇总列表](#Golang开源项目汇总列表)
	- [推荐几个可以写到简历上的Go方向优质开源项目（需花点心思研究）](https://juejin.cn/post/7038967716459315208)
	- [golang-gin-realworld-example-app工程](#golang-gin-realworld-example-app工程)
	-  [go-gin-api全栈项目 ](#go-gin-api全栈项目)
	-  [gin-vue-admin全栈平台 ](#gin-vue-admin全栈平台 )
	-  [ferry工单系统Gorm（ORM工具）](#ferry工单系统Gorm（ORM工具）) 
	-  [gin-gorm-restful-api](#gin-gorm-restful-api) 
	-  [Go-Zero商城项目](#Go-Zero商城项目) 
	-  [echo-restful-api](#echo-restful-api) 
	-  [gorilla-mux-restful-api](#gorilla-mux-restful-api) 
	-  [beego-restful-api](#beego-restful-api) 
	-  [kratos-restful-api](#kratos-restful-api) 
	-  [gin-swagger-restful-api](#gin-swagger-restful-api) 
	-  [go-kit-restful-api](#go-kit-restful-api) 
	-  [fiber-restful-api](#fiber-restful-api) 
	-  [gin-gorm-jwt-restful-api](#gin-gorm-jwt-restful-api)
- [**框架**](#框架)
	-  [NSQ源码阅读](#NSQ源码阅读)
	-  [Gin框架](#Gin框架)  
	-  [Echo框架](#Echo框架)
	-  [GorillaMux路由库](#GorillaMux路由库) 
	-  [Vegeta负载测试工具](#Vegeta负载测试工具) 
	-  [Authboss库-添加认证与授权模块](#Authboss库-添加认证与授权模块) 
	-  [GoKit库-网关和分布式追踪](#GoKit库-网关和分布式追踪) 
	-  [Beego库](#Beego库) 
	-  [Fiber库](#Fiber库) 
	-  [go-restful库](#go-restful库) 
	-  [Chi库](#Chi库)
	-  [Viper配置管理库](#Viper配置管理库)
- **资料**
	- [浅读 Go 优秀开源项目源码—Gin框架](https://blog.linganmin.cn/posts/d6715893/)
	- [rickiyang博客Go-具体很详细](https://www.cnblogs.com/rickiyang/category/1487722.html)
		- [gorm库练习](https://www.cnblogs.com/rickiyang/p/11074162.html)
	- [维斯Echo(博客仔细,不错)-掘金](https://juejin.cn/user/369885757844285/posts)
	- [盘点 7 个优质开源的 Go 项目](https://juejin.cn/post/7092788846781267975)
	- [标准的 Go 项目布局](https://juejin.cn/post/6944649692319842340)
	- [awesome-go项目](https://github.com/avelino/awesome-go)
		- [Awesome Github REPO](https://github.com/Wechat-ggGitHub/Awesome-GitHub-Repo)
		- [awesome-go中文介绍](https://github.com/jobbole/awesome-go-cn)
		- [awesome-go中文介绍02](https://github.com/hyper0x/awesome-go-China/blob/master/zh_CN/README.md)
	- [超全golang面试题合集+golang学习指南+golang知识图谱+成长路线](https://github.com/xiaobaiTech/golangFamily?tab=readme-ov-file)
	- [Go 开发者路线图](https://github.com/darius-khll/golang-developer-roadmap/blob/master/i18n/zh-CN/ReadMe-zh-CN.md)
	- [GitHubDaily 已累积分享超过 8000 个开源项目](https://github.com/GitHubDaily/GitHubDaily)
- [功能模块](#功能模块)
	- [登录注册](#登录注册) 
	- [redis缓存-登录注册](#redis缓存-登录注册)
- [**环境变量传递**](#环境变量传递)
	- [VSCode launch.json 环境变量传递流程](#VSCode-launch.json-环境变量传递流程)
- [**日志系统**](#日志系统)
	- [日志函数添加调用者信息](#日志函数添加调用者信息)
- [**配置加载**](#配置加载)
	- [LoadConfig函数解释](#LoadConfig函数解释)
- [**Redis配置**](#Redis配置)
	- [getRedisAddr函数解释](#getRedisAddr函数解释)
- [**环境配置加载**](#环境配置加载)
	- [Load函数解释](#Load函数解释)
	- [getEnvOrDefault函数解释](#getEnvOrDefault函数解释)
- [**根路由处理**](#根路由处理)
	- [NewRootHandler方法解释](#NewRootHandler方法解释)
- [**路由注册机制**](#路由注册机制)
	- [bindRouteSpecs函数解释](#bindRouteSpecs函数解释)
- [**API规则配置**](#API规则配置)
	- [PublicAPIRules配置](#PublicAPIRules配置)
	- [HGAPIRule使用说明](#HGAPIRule使用说明)
	- [Permissions权限配置](#Permissions权限配置)
- [**Go基础语法**](#Go基础语法)
	- [切片和数组的区别](#切片和数组的区别)
- [**结构体实例化**](#结构体实例化)
	- [HGAPIGuard实例化-map容器](#HGAPIGuard实例化-map容器)
- [**拦截器机制**](#拦截器机制)
	- [Interceptor方法解读](#Interceptor方法解读)
	- [http.HandlerFunc与http.Handler的关系](#http.HandlerFunc与http.Handler的关系)
- [**HTTP请求获取方法**](#HTTP请求获取方法)
	- [r.Header.Get详解](#r.Header.Get详解)
	- [r.URL.Path详解](#r.URL.Path详解)
- [**Go语法特性**](#Go语法特性)
	- [comma-ok模式详解](#comma-ok模式详解)
- [**HTTP响应方法**](#HTTP响应方法)
	- [常用响应方法对照表](#常用响应方法对照表)
	- [前端收到的数据格式](#前端收到的数据格式)
- [**Go标准库**](#Go标准库)
	- [sort.Slice排序详解](#sort.Slice排序详解)
- [**路由分组机制**](#路由分组机制)
	- [buildRouteCatalogGrouped详解](#buildRouteCatalogGrouped详解)
- [**NewRootHandler详解**](#NewRootHandler详解)
	- [各函数调用关系](#各函数调用关系)
	- [路由清单接口对比](#路由清单接口对比)
- [**未完成优秀代码**](#未完成优秀代码)
	- [文件排版和架构](#文件排版和架构)



<br/><br/><br/>

***
<br/>

> <h1 id="常用命令">常用命令</h1>

# Intel
```sh
# Intel进入mysql交互
mysql -u root -p

# 查看所有数据库
show databases;

use  HG_MLC_DB;

# 查看所有表
show tables;

# 查看users表中所有字段数据
select * from users;

```

***
<br>


AI的AGENTS.md配置文件

```sh
.ai_agents/     # Codex 规则（精简版）
├── 00-core.md      # 核心
├── 01-style.md     # 风格
├── 02-go-rules.md     # Go
├── 03-api-rules.md`  # API
├── 04-data-rules.md` # 数据层
├── 05-validation.md` # 验证
├── 06-output.md`     # 输出
├── 07-forbidden.md`  # 禁止项
└── 08-performance.md` # 高并发性能
```

***
<br>

```sh

```

***
<br>

```sh

```



<br/><br/><br/>

***
<br/>

> <h1 id="工程启动">工程启动</h1>

***
<br/><br/><br/>
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

***
<br/>

**改 debug 环境**
文件在：
[config/env_configs/hg_debug.env](/Users/ganghuang/HGFiles/GitHub/GoProject/src/MLC_GO/config/env_configs/hg_debug.env:1)

你可以改这些值：

```env
MYSQL_HOST=127.0.0.1
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=!hxxx9
MYSQL_DB=HG_MLC_DB
REDIS_HOST=127.0.0.1
REDIS_PORT=6379
```

如果你本机 MySQL 不是 `3306`，或者密码不是 `hh109`，就在这里改。

***
<br/>

**改 pre 环境**
文件在：
[config/env_configs/hg_pre.env](/Users/ganghuang/HGFiles/GitHub/GoProject/src/MLC_GO/config/env_configs/hg_pre.env:1)

目前我给你配的是“本地模拟预发”：

```env
MYSQL_HOST=127.0.0.1
MYSQL_PORT=3308
MYSQL_USER=root
MYSQL_PASSWORD=!hxxx9
MYSQL_DB=HG_MLC_PRE_DB
REDIS_HOST=127.0.0.1
REDIS_PORT=6380
```

<br/>
如果你想让 `pre` 连真实预发机器，比如：

```env
MYSQL_HOST=10.10.1.25
MYSQL_PORT=3306
MYSQL_USER=pre_user
MYSQL_PASSWORD=你的密码
MYSQL_DB=pre_db
REDIS_HOST=10.10.1.26
REDIS_PORT=6379
```

也可以直接改这里。

但注意一件事：

- 如果 `pre` 改成远端机器地址，那它就不应该再“自动启动 docker compose”
- 它应该只做检查

也就是说：
- 本地模拟预发：`127.0.0.1 + 3308/6380 + compose`
- 真实预发环境：远程 IP + 只检查不启动

***
<br/>

**改 prod 环境**
文件在：
[config/env_configs/hg_prod.env](/Users/ganghuang/HGFiles/GitHub/GoProject/src/MLC_GO/config/env_configs/hg_prod.env:1)

例如：

```env
MYSQL_HOST=prod-mysql.internal
MYSQL_PORT=3306
MYSQL_USER=app
MYSQL_PASSWORD=*******
MYSQL_DB=HG_MLC_PROD_DB
REDIS_HOST=prod-redis.internal
REDIS_PORT=6379
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

2. 再改 `hg_pre.env`：

```env
MYSQL_PORT=3310
REDIS_PORT=6381
```

一定要两边一致，不然脚本能拉起容器，但程序连不上。

***
<br/>

**改 VS Code 启动配置**
如果你想改调试名、参数、环境名，就看：
[.vscode/launch.json](/Users/ganghuang/HGFiles/GitHub/GoProject/src/MLC_GO/.vscode/launch.json:8)

这里主要是：
- `preLaunchTask`
- `env.SERVER_ENV`
- `args`

通常你现在不用先改这里，优先改 `.env` 和 `compose` 就够了。

<br/>

**最推荐的修改原则**
你可以按这个思路改：

- 本地开发不对：改 `hg_debug.env`
- 本地模拟预发不对：改 `hg_pre.env` 和 `hg_docker_compose.pre.yml`
- 真实预发地址不对：改 `hg_pre.env`
- 生产地址不对：改 `hg_prod.env`

**你现在可以先这样试一次**
如果你要跑本地模拟预发：

1. 先检查 `hg_pre.env` 里的端口和密码对不对
2. 再看 `hg_docker_compose.pre.yml` 里的端口映射对不对
3. 然后在 VS Code 选：
   `🧪 Launch MLC_GO Root main.go (pre)`
4. 点运行

如果你要跑本地 debug：

1. 先检查 `hg_debug.env`
2. 在 VS Code 选：
   `🧪 Launch MLC_GO Root main.go (debug)`
3. 点运行

如果你愿意，我下一步可以直接帮你做一件更省心的事：

你把你真实想用的三套配置发我：
- debug 的 MySQL/Redis 地址、端口、账号、密码
- pre 的地址、端口、账号、密码
- prod 的地址、端口、账号、密码


***
<br/><br/><br/>
> <h2 id="Intel电脑修改配置启动">Intel电脑修改配置启动</h2>


**Intel电脑：**
hg_debug.env文件：

```sh
MYSQL_PASSWORD=
```

M2Pro

hg_debug.env文件：

```sh
MYSQL_PASSWORD=hh109
```

启动redis：

```sh
redis-server
```

启动mysql

```sh
# M2Pro sql 启动
sudo mysql.server start

cd /Users/harleyhuang/HGFiles/GitHub/GoProject/src/MLC_GO/scripts
./db.sh shell
```


```sh
localhost:8080/auth/send_code?phone=17681317668
```

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

> <h1 id="代码调用举例">代码调用举例</h1>


***
<br/><br/><br/>
> <h2 id="分页model调用">分页model调用</h2>

```go
items := []VideoItem{ /* ... */ }

// 最简用法（全用默认值）
pageResp := NewPageResponse(items)

// 带自定义参数
pageResp := NewPageResponse(
    items,
    WithPage(2),
    WithPagesize(42),
    WithTotal(1000),
    WithRqtType("search"),
    // Seid 会自动生成，无需手动传
)
```


**输出：**

```json
PageResponse[VideoItem]{
    Seid: "1738901234567890", // 自动生成的唯一ID
    Page: 2,
    Pagesize: 42,
    NumResults: 1000,
    NumPages: 24,
    RqtType: "search",
    Result: items,
}
```



<br/><br/><br/>

***
<br/>

> <h1 id="高级语法">高级语法</h1>

***
<br/><br/><br/>
> <h2 id="函数签名闭包">函数签名闭包</h2>

你问得非常好！我们来详细解释 **`opts ...PageOption` 是怎么传入的**，以及为什么 `NewPageResponse(items)` 这样写是合法的。

---

### 🔹 1. 函数签名回顾

```go
func NewPageResponse[T any](items []T, opts ...PageOption) PageResponse[T]
```

- `items []T`：必须传，是你的真实数据列表（比如视频、用户等）。
- `opts ...PageOption`：**可变参数（variadic parameter）**，表示“0个或多个 `PageOption` 类型的参数”。

> ✅ 所以你可以：
> - **不传任何 opts** → 使用默认值
> - **传 1 个、2 个、或多个 opts** → 覆盖默认值

---

### 🔹 2. `opts` 到底是什么？

`PageOption` 是一个**函数类型**：

```go
type PageOption func(*PageResponseConfig)
```

而像 `WithPage(2)`、`WithPagesize(42)` 这些函数，**返回的正是 `PageOption` 类型**：

```go
func WithPage(page int) PageOption {
    return func(c *PageResponseConfig) { // ← 这就是一个 PageOption
        if page > 0 {
            c.Page = page
        }
    }
}
```

所以当你写：

```go
WithPage(2)
```

它的值是一个 **函数（闭包）**，类型是 `PageOption`。

---

### 🔹 3. 如何传 `opts`？三种典型用法

#### ✅ 用法 1：不传 opts（全用默认值）

```go
pageResp := NewPageResponse(items)
```

- `opts` 接收到的是一个 **空的 slice**（`[]PageOption{}`）
- 内部循环 `for _, opt := range opts` 不会执行
- 所有字段使用 `defaultPageConfig()` 的默认值
- `Seid` 自动生成，`Page=1`, `Pagesize=20` 等

✅ 完全合法！

---

#### ✅ 用法 2：传 1 个 option

```go
pageResp := NewPageResponse(items, WithPage(3))
```

- `opts` 是 `[WithPage(3)]`
- 内部会调用 `WithPage(3)(config)` → 把 `config.Page` 设为 3

---

#### ✅ 用法 3：传多个 options（推荐写法）

```go
pageResp := NewPageResponse(
    items,
    WithPage(2),
    WithPagesize(42),
    WithTotal(1000),
    WithRqtType("search"),
)
```

Go 会自动把逗号分隔的 `PageOption` 值打包成 `[]PageOption` 传给 `opts`。

> 💡 这就是 Go 中模拟“命名参数 + 默认值”的标准做法！

---

### 🔹 4. 错误写法（常见误区）

❌ 不能这样写（把选项放进 slice 手动传）：
```go
// ❌ 不需要！Go 会自动打包
opts := []PageOption{WithPage(2), WithPagesize(42)}
pageResp := NewPageResponse(items, opts) // 编译错误！
```

✅ 正确做法是用 **展开语法**（但通常没必要）：
```go
opts := []PageOption{WithPage(2), WithPagesize(42)}
pageResp := NewPageResponse(items, opts...) // 注意 ... 
```

但在绝大多数场景下，**直接写 `WithXXX(...)` 更简洁清晰**。

---

### 🔹 5. 实际代码演示

```go
package main

import "fmt"

type VideoItem struct {
	Title string
}

func main() {
	items := []VideoItem{
		{Title: "WebRTC 教程"},
		{Title: "FFmpeg 入门"},
	}

	// ✅ 最简调用：无 opts
	resp1 := NewPageResponse(items)
	fmt.Println("默认 Seid:", resp1.Seid)
	fmt.Println("默认 Page:", resp1.Page) // 输出 1

	// ✅ 带 opts
	resp2 := NewPageResponse(
		items,
		WithPage(2),
		WithPagesize(50),
		WithTotal(123),
	)
	fmt.Println("自定义 Page:", resp2.Page)   // 输出 2
	fmt.Println("总页数:", resp2.NumPages)     // 输出 3 (123/50 向上取整)
}
```

---

### ✅ 总结

| 调用方式 | 是否合法 | 说明 |
|--------|--------|------|
| `NewPageResponse(items)` | ✅ | 使用全部默认值，`opts` 为空 |
| `NewPageResponse(items, WithPage(2))` | ✅ | 覆盖 page |
| `NewPageResponse(items, WithPage(2), WithPagesize(30))` | ✅ | 覆盖多个字段 |
| `NewPageResponse(items, []PageOption{...})` | ❌ | 缺少 `...` 展开符 |
| `NewPageResponse(items, []PageOption{...}...)` | ✅ | 合法但冗余 |

所以你写的：
```go
pageResp := NewPageResponse(items)
```
**完全正确！** 它会生成一个带自动生成 `Seid`、默认分页参数的 `PageResponse` 实例。

如有需要，后续再叠加 `WithXXX` 即可灵活定制。


<br/><br/><br/>

***
<br/>

># <h1 id="token解析">[token解析](https://www.jwt.io)</h1>



<br/><br/><br/>

***
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
- TestNotes: 测试练习Go语法
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
│   ├── config.debug.yaml
│   ├── config.pre.yaml
│   └── config.prod.yaml
```
| 环境  | 常用标识              | 用途    |
| --- | ----------------- | ----- |
| 开发  | `debug` / `dev`   | 本地开发  |
| 预发布 | `pre` / `staging` | 上线前验证 |
| 正式  | `prod`            | 线上环境  |


***
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

> **说明：**
> `internal/`:Go的约定，该目录下的代码只能**被本项目引用**，防止被外部项目import；
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

>📌 替代方案：有些人用 features/ 或 domains/，但 modules/ 更通用。

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

` modules/post/service/post_service.go `

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
| 新功能       | 如何扩展目录                          |
|------------|----------------------------------|
| 评论        | 新增 `modules/comment/`            |
| 点赞        | 在 `post` 模块内加 `LikeService`，或新建 `modules/like/` |
| 私信        | 新增 `modules/message/`            |
| 文件上传     | 新增 `modules/storage/` + `pkg/upload/` |




<br/><br/><br/>

***
<br/>

> <h1 id="文件规则">文件规则</h1>
<br/>

> <h2 id="协议规则">协议规则</h2>
**比如：**

协议的方法前要加入**`协议名_+方法名`**：

```go

type Writer interface {
	Writer_read(text string) 
}
```

<br/><br/><br/>

***
<br/>

> <h1 id="SQL">SQL</h1>

***
<br/><br/><br/>
> <h2 id="xx.sql问件加注释">xx.sql问件加注释</h2>

- **✅ SQL 中的注释语法：**
- **1.单行注释（两种方式）：**

```sql
-- 这是一个单行注释（推荐，可移植性好）
```

或者：

```sql
# 这也是一个单行注释（MySQL 特有）
```
   
<br/>

**2.多行注释：**

```sql
/* 
  这是
  多行注释
*/
```
 
 <br/>     


```sql
CREATE DATABASE HG_MLC_DB DEFAULT CHARACTER SET utf8mb4;
USE HG_MLC_DB;
```

**第一行：**

`CREATE DATABASE HG_MLC_DB DEFAULT CHARACTER SET utf8mb4;`

- 作用：创建一个名为 HG_MLC_DB 的数据库。
- DEFAULT CHARACTER SET utf8mb4：指定该数据库默认使用 utf8mb4 字符集。
  - utf8mb4 是 MySQL 中真正完整的 UTF-8 实现（支持 4 字节字符，比如 emoji 表情 🎉），比旧的 utf8（实际是 utf8mb3）更全面。

<br/>

**第二行：**

`USE HG_MLC_D;`

- 作用：切换当前会话使用的数据库为 HG_MLC_D。
- ⚠️ 注意：这里疑似有笔误！前面创建的是 HG_MLC_DB，但这里却用了 HG_MLC_D（少了个 _DB）。
  - 如果 HG_MLC_D 数据库不存在，这条语句会报错。
  - 很可能是打字错误，应为：
        USE HG_MLC_DB;


<br/><br/><br/>

***
<br/>

> <h1 id="TCP通信自己实现">TCP通信自己实现</h1>

在 iOS 端（客户端）使用 Swift 来和 Go 写的 TCP 服务端进行通信，可以使用 `Network` 框架（iOS 12+）或较低版本使用 `CFStream` 或 `GCDAsyncSocket`（CocoaAsyncSocket）库。下面我会给出一个使用 **`Network` 框架**（推荐）的完整示例，展示如何在 iOS 客户端中连接你提供的 Go TCP 服务端（监听端口为 `8889`）并发送/接收数据。

---

**✅ 使用 Swift + Network 框架实现 TCP 客户端通信（iOS）**

**iOS 客户端Demo：**

```swift
import Foundation
import Network

class TCPClient {
    private var connection: NWConnection?
    private let queue = DispatchQueue(label: "TCPClientQueue")

    init(host: String, port: UInt16) {
        let nwEndpoint = NWEndpoint.Host(host)
        let nwPort = NWEndpoint.Port(rawValue: port)!

        connection = NWConnection(host: nwEndpoint, port: nwPort, using: .tcp)
    }

    func connect() {
        connection?.stateUpdateHandler = { state in
            switch state {
            case .ready:
                print("✅ Connected to server")
                self.receive()
            case .failed(let error):
                print("❌ Failed to connect: \(error)")
            default:
                break
            }
        }

        connection?.start(queue: queue)
    }

    func send(data: Data) {
        connection?.send(content: data, completion: .contentProcessed({ error in
            if let error = error {
                print("❌ Send error: \(error)")
            } else {
                print("📤 Data sent: \(data as NSData)")
            }
        }))
    }

    func receive() {
        connection?.receive(minimumIncompleteLength: 1, maximumLength: 1024) { data, _, isComplete, error in
            if let data = data, !data.isEmpty {
                let str = String(data: data, encoding: .utf8) ?? "(无法解码)"
                print("📥 Received: \(str)")
            }

            if isComplete {
                print("📴 Connection closed by server")
            } else if let error = error {
                print("❌ Receive error: \(error)")
            } else {
                // 继续监听
                self.receive()
            }
        }
    }

    func stop() {
        connection?.cancel()
        print("🔌 Connection stopped")
    }
}
```

#### 使用方式示例：

```swift
let client = TCPClient(host: "127.0.0.1", port: 8889)
client.connect()

DispatchQueue.main.asyncAfter(deadline: .now() + 2) {
    let message = "Hello Go Server"
    client.send(data: message.data(using: .utf8)!)
}
```

---

### 📝 注意事项

1. **本地测试地址**：

   * iOS 模拟器可以连接 `127.0.0.1`（Mac 上的服务）
   * 真机需要连接你 Mac 的局域网 IP，如 `192.168.x.x`

2. **Go 端应对粘包问题**（可以参考 TCP 粘包拆包处理）

3. **权限**：若是用真机调试，确保 Wi-Fi 同网段 & Mac 防火墙允许通信。

---

### 🔁 Go 服务端的 `process(conn net.Conn)` 示例

为了能和 iOS 客户端交互，可以写一个简单的处理逻辑：

```go
func process(conn net.Conn) {
	defer conn.Close()
	for {
		// 创建一个缓冲区
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("客户端退出 err =", err)
			return
		}

		// 显示接收到的内容
		fmt.Printf("收到客户端发来的内容: %s\n", string(buf[:n]))

		// 回写给客户端
		conn.Write([]byte("收到: " + string(buf[:n])))
	}
}
```

---

如果你需要更复杂的数据结构通信（如 JSON），也可以加上 JSON 编解码处理。若需要我可以再补充。

是否需要我再帮你做一个 SwiftUI 页面来演示如何交互？


<br/><br/>

# **提问：** Go的Socket.io框架是开源的吗？ 如何使用？有哪些功能

是的，Go 的 Socket.io 框架是 **开源的**，社区中最常用的是 [github.com/googollee/go-socket.io](https://github.com/googollee/go-socket.io)。

这个库是对 **Socket.IO 协议（基于 WebSocket）** 的 Go 实现，功能和 Node.js 的 socket.io 类似，可用于和网页、移动 App（如 iOS、Android）等客户端进行实时通信。

---

## ✅ 一、是否开源？

是的，`go-socket.io` 是完全开源的，采用的是 MIT 许可证（自由商用、修改、分发）。

GitHub 项目地址：
👉 [https://github.com/googollee/go-socket.io](https://github.com/googollee/go-socket.io)

---

## 🚀 二、如何使用？

### 1. 安装

```bash
go get github.com/googollee/go-socket.io
```

---

### 2. 服务端使用示例（Go）

```go
package main

import (
	"fmt"
	"log"
	"net/http"

	socketio "github.com/googollee/go-socket.io"
)

func main() {
	server := socketio.NewServer(nil)

	// 监听连接
	server.OnConnect("/", func(s socketio.Conn) error {
		fmt.Println("🔌 连接成功:", s.ID())
		s.Emit("reply", "欢迎连接 socket.io 服务")
		return nil
	})

	// 监听自定义事件
	server.OnEvent("/", "message", func(s socketio.Conn, msg string) {
		fmt.Println("📥 接收到消息:", msg)
		s.Emit("reply", "已收到消息："+msg)
	})

	// 断开连接
	server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		fmt.Println("❌ 断开连接:", s.ID(), "原因:", reason)
	})

	go server.Serve()
	defer server.Close()

	// 将 socket.io 服务挂载到 http 路由
	http.Handle("/socket.io/", server)
	http.Handle("/", http.FileServer(http.Dir("./public"))) // 可选：挂静态页面测试

	fmt.Println("✅ Socket.IO 服务运行于 localhost:8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
}
```

---

## 🌐 三、前端或客户端连接示例

### HTML + JS 示例（前端页面连接）

```html
<script src="https://cdn.socket.io/4.7.2/socket.io.min.js"></script>
<script>
  const socket = io("http://localhost:8000");

  socket.on("connect", () => {
    console.log("✅ Connected:", socket.id);
    socket.emit("message", "Hello from frontend!");
  });

  socket.on("reply", (data) => {
    console.log("📩 Server reply:", data);
  });
</script>
```

### iOS 客户端使用

iOS 推荐使用 [Socket.IO-Client-Swift](https://github.com/socketio/socket.io-client-swift)：

```swift
import SocketIO

let manager = SocketManager(socketURL: URL(string: "http://localhost:8000")!, config: [.log(true), .compress])
let socket = manager.defaultSocket

socket.on(clientEvent: .connect) { data, ack in
    print("✅ socket connected")
    socket.emit("message", "Hello from iOS")
}

socket.on("reply") { data, ack in
    print("📩 server says:", data)
}

socket.connect()
```

---

## 🧰 四、支持的功能

`go-socket.io` 支持以下主要功能：

| 功能                           | 说明                                            |
| ---------------------------- | --------------------------------------------- |
| `OnConnect` / `OnDisconnect` | 监听连接 / 断开连接                                   |
| `OnEvent`                    | 监听客户端事件                                       |
| `Emit`                       | 主动向客户端发送事件                                    |
| 多命名空间（namespace）             | 类似 `"/chat"`、`"/news"` 等子频道                   |
| 房间机制（rooms）                  | 类似聊天室，支持广播、单发                                 |
| 支持 WebSocket & 长轮询           | 与 Socket.IO 协议兼容                              |
| 支持 JSON 数据、对象传输              | 序列化支持                                         |
| 支持跨语言客户端连接                   | 与 Node.js、JavaScript、Swift、Java、Python 等客户端兼容 |

---

## ❗️五、注意事项

1. 必须用 Socket.IO 协议的客户端（如 Web、iOS 客户端）对接，不是普通 WebSocket。
2. 若部署在服务器上，务必配置好 CORS。
3. 部署到生产环境建议使用反向代理（如 nginx）处理 WebSocket。
4. 与 Socket.IO v4 客户端兼容较好，v5+ 请注意兼容性问题。

---

如你需要一个完整的 **Go 服务端 + iOS 客户端 Socket.IO 示例项目**，我可以帮你准备一下。

是否继续帮你创建一个完整示例？（比如 iOS + Go 实时通信 demo）


<br/><br/>

# 可以？ 若是阅读这个框架？如何阅读和使用转化成自己想写一个这样的功能框架提高自己

太棒了！你想**通过阅读 `go-socket.io` 框架源码来转化为自己的能力，写一个类似的功能框架**，这是非常正确的提升方式！

我来一步步拆解，带你理解如何去 **阅读、学习、改写、重构** 这样的开源框架，并最终构建出自己的版本。

---

## 🧭 1. 阅读目的明确：你要学会什么？

通过阅读 `go-socket.io` 你可以学到：

| 想学内容                  | 学到的技术                         |
| --------------------- | ----------------------------- |
| 实现一个 TCP/WebSocket 协议 | 网络协议解析、帧格式、状态机                |
| 管理客户端连接和消息分发          | Goroutine、channel、map 管理、线程安全 |
| 构建事件驱动框架（on/emit）     | 事件注册机制、反射、函数缓存                |
| 支持房间、命名空间等            | 多维路由分发、群发设计                   |
| 支持断线重连/心跳检测           | 协议保持、超时检测、错误处理                |

---

## 🔍 2. 阅读前准备：克隆项目并准备调试环境

```bash
git clone https://github.com/googollee/go-socket.io.git
cd go-socket.io
go mod tidy
```

建议使用 **VSCode 或 GoLand**，打开目录，方便跳转查看接口、结构体、依赖。

---

## 📖 3. 阅读路线建议（重要）

### 🔹 入口：`NewServer()`（`server.go`）

```go
func NewServer(opts *engineio.Options) *Server {
    server := &Server{
        namespaceHandlers: make(map[string]*namespaceHandler),
        ...
    }
    ...
    return server
}
```

* 这是核心结构体 `Server`，内部维护了 **namespace、rooms、连接等全局状态**
* 它会启动 `engine.io` 底层监听器（这是 Socket.IO 的基础）

---

### 🔹 核心结构体：`Server`（`server.go`）

```go
type Server struct {
    namespaceHandlers map[string]*namespaceHandler
    connectHandler    func(Conn) error
    rooms             map[string]map[Conn]bool
    ...
}
```

你可以把它当成整个“框架的总调度器”。

---

### 🔹 连接处理流程：`OnConnect` / `ServeHTTP`

看看 `ServeHTTP` 怎么响应 `/socket.io/` 请求并升级协议：

```go
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    s.eio.ServeHTTP(w, r)
}
```

它实际转发给了 `engine.io`，你可以继续跟踪 `engine.io` 是如何实现 WebSocket 协议升级的。

---

### 🔹 消息注册和触发：事件分发器 `OnEvent`

```go
server.OnEvent("/", "message", func(c socketio.Conn, msg string) {
    fmt.Println("收到:", msg)
})
```

你会在代码中看到：

* 事件的注册用 `map[string]func(...)` 存储
* 实际消息触发时会查找 map 并调用对应函数
* 用 `reflect` 解析参数和调用（反射调用函数）

---

### 🔹 房间机制：`Join()`、`Leave()`、`Broadcast()`

这部分通常用一个结构：

```go
map[string]map[Conn]bool // room -> set of connections
```

然后通过 `EmitToRoom(room, event, data)` 实现群发。

---

## 🔧 4. 怎么“转化”为自己的框架？

### 第一步：简化核心模型

你可以先从极简框架开始写，例如支持如下功能：

| 功能点       | 实现方式                             |
| --------- | -------------------------------- |
| 客户端连接     | `net.Conn` 或 `gorilla/websocket` |
| 事件注册      | `map[string]func(data []byte)`   |
| `Emit` 方法 | 封装 `conn.Write()` 发送数据           |
| `On` 注册事件 | 注册回调到 map，接收到消息时分发               |
| 多客户端      | `map[string]Conn` 和 goroutine    |

你可以自己从 0 写一个简单框架如下结构：

```go
type Server struct {
    conns map[string]*ClientConn
    events map[string]func(*ClientConn, []byte)
}

func (s *Server) On(event string, fn func(*ClientConn, []byte)) {
    s.events[event] = fn
}

func (s *Server) Emit(connID string, event string, data []byte) {
    // 封装数据结构 {"event": "...", "data": ...}
    // 写入 conn.Write()
}
```

### 第二步：逐步迭代增加功能

你可以按如下顺序迭代功能：

1. ✅ 支持 WebSocket 协议解析（使用 `gorilla/websocket`）
2. ✅ 支持多连接管理
3. ✅ 支持 `On(event, handler)` 和 `Emit(event, data)`
4. ✅ 支持房间 Room 加入/离开
5. ✅ 支持 JSON 消息格式（可兼容 Socket.IO）
6. ✅ 支持断线重连 / 心跳检测

---

## 🛠 5. 推荐配套项目和知识

| 工具/技术                       | 用途                          |
| --------------------------- | --------------------------- |
| `gorilla/websocket`         | WebSocket 协议支持              |
| `encoding/json`             | 消息格式封装                      |
| `sync.Map` / `sync.RWMutex` | 多连接并发安全                     |
| `context.Context`           | 管理连接生命周期                    |
| `reflect`                   | 实现通用函数注册和调用（像 socket.io 那样） |

---

## 💡 6. Bonus：怎么训练思维写一个框架？

> 借助开源项目思维训练：

1. **先写 demo**（客户端能连、发、收）
2. **抽象接口**（定义 `Server`、`Conn`、`EventHandler` 等接口）
3. **抽取结构体模型**（如 `Conn` 包含 `id`、`ws`、`send chan`）
4. **重构功能模块化**（room、namespace、event、transport）
5. **写注释和文档**（锻炼表达和封装能力）
6. **看 benchmark / test 文件**（学习测试用例如何设计）

---

## ✅ 总结

你可以通过如下路径实现“源码学习 -> 吸收设计思想 -> 重构实现”的目标：

```text
克隆源码 → 阅读入口结构 → 理解数据流转 → 模仿实现一版极简结构 → 逐步丰富功能 → 写自己的 socket.io 框架
```

---

如果你希望，我可以为你：

* 给出一个【从零写 socket.io-mini 框架】的分阶段计划
* 提供代码模板（极简事件通信 + 房间支持）
* 帮你逐步实现每一块功能（如：支持 JSON 消息、房间广播、命名空间）

要继续吗？我可以立即为你生成第一阶段代码框架。


<br/><br/>

# 嗯呢  好的 给一个


太好了，我们就开始动手打造一个你自己的 **SocketIO-mini 框架**！这个框架的目标是“**事件驱动的 WebSocket 通信系统**”，可以让你深入掌握 socket 通信、事件注册、并发控制等核心知识。

---

## ✅ 阶段一：基础框架功能目标

我们要先实现的功能包括：

| 功能                         | 实现方式                      |
| -------------------------- | ------------------------- |
| 支持 WebSocket 通信            | 使用 `gorilla/websocket`    |
| 支持多客户端连接                   | 每个连接用 goroutine 维护        |
| 支持 `On(event, func)` 注册事件  | 使用 `map[string]func(...)` |
| 支持 `Emit(event, data)` 发消息 | JSON 格式包封装                |
| 支持客户端消息分发到对应的事件回调          | 实现事件分发器                   |

---

## 🏗️ 项目结构建议

```
socketio-mini/
├── main.go         # 示例服务端
├── server.go       # Server/Conn 结构体和核心逻辑
├── types.go        # 消息结构体等定义
```

---

## 📄 server.go（核心逻辑）

```go
package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type EventHandler func(c *Conn, data []byte)

type Server struct {
	clients    map[*Conn]bool
	handlers   map[string]EventHandler
	register   chan *Conn
	unregister chan *Conn
	mu         sync.RWMutex
}

func NewServer() *Server {
	return &Server{
		clients:    make(map[*Conn]bool),
		handlers:   make(map[string]EventHandler),
		register:   make(chan *Conn),
		unregister: make(chan *Conn),
	}
}

func (s *Server) On(event string, handler EventHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[event] = handler
}

func (s *Server) Run() {
	for {
		select {
		case client := <-s.register:
			s.clients[client] = true
		case client := <-s.unregister:
			delete(s.clients, client)
			client.Close()
		}
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket Upgrade Error:", err)
		return
	}
	conn := &Conn{ws: ws, server: s, send: make(chan []byte, 256)}
	s.register <- conn

	go conn.readLoop()
	go conn.writeLoop()
}
```

---

## 📄 conn.go（连接读写逻辑）

```go
package main

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

type Conn struct {
	ws     *websocket.Conn
	server *Server
	send   chan []byte
}

type Message struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

func (c *Conn) readLoop() {
	defer func() {
		c.server.unregister <- c
		c.ws.Close()
	}()
	for {
		_, msg, err := c.ws.ReadMessage()
		if err != nil {
			log.Println("Read Error:", err)
			break
		}
		var m Message
		if err := json.Unmarshal(msg, &m); err != nil {
			log.Println("Unmarshal Error:", err)
			continue
		}
		handler, ok := c.server.handlers[m.Event]
		if ok {
			handler(c, m.Data)
		} else {
			log.Println("⚠️ 未注册事件:", m.Event)
		}
	}
}

func (c *Conn) writeLoop() {
	for msg := range c.send {
		c.ws.WriteMessage(websocket.TextMessage, msg)
	}
}

func (c *Conn) Emit(event string, data interface{}) {
	msg := map[string]interface{}{
		"event": event,
		"data":  data,
	}
	encoded, _ := json.Marshal(msg)
	c.send <- encoded
}

func (c *Conn) Close() {
	close(c.send)
	c.ws.Close()
}
```

---

## 📄 main.go（服务端启动）

```go
package main

import (
	"fmt"
	"net/http"
)

func main() {
	server := NewServer()

	server.On("ping", func(c *Conn, data []byte) {
		fmt.Println("收到 ping:", string(data))
		c.Emit("pong", "我是服务器收到你的ping")
	})

	go server.Run()

	http.Handle("/ws", server)

	fmt.Println("🚀 Server started at http://localhost:8080/ws")
	http.ListenAndServe(":8080", nil)
}
```

---

## 🔗 客户端调试（浏览器端）

打开控制台运行以下代码测试：

```js
const ws = new WebSocket("ws://localhost:8080/ws");

ws.onopen = () => {
  ws.send(JSON.stringify({ event: "ping", data: "hello server" }));
};

ws.onmessage = (msg) => {
  console.log("📩 收到服务器消息:", msg.data);
};
```

---

## ✅ 下一阶段建议

| 阶段   | 功能                       |
| ---- | ------------------------ |
| 第二阶段 | 支持房间（Room），广播消息          |
| 第三阶段 | 支持命名空间（Namespace）        |
| 第四阶段 | 支持连接认证、心跳检测              |
| 第五阶段 | 自动重连、客户端库适配（如 Swift 客户端） |

---

## 📦 Bonus

我可以把这份代码整理成一个完整压缩包结构，或者上传到你的 GitHub 仓库。如果你愿意，我们可以一起完成第二阶段的 **房间功能（Join/Leave/Broadcast）**。

是否继续进入第二阶段？还是你要先跑通这个版本？我可以等你测试完再继续扩展。





<br/><br/><br/><br/><br/><br/>

***
<br/>
># <h1 ID="Golang开源项目汇总列表"> [Golang 开源项目汇总列表](https://github.com/hackstoic/golang-open-source-projects)</h1>
<br/>

<br/><br/><br/>

***
<br/>

># <h1 id="golang-gin-realworld-example-app工程">[golang-gin-realworld-example-app工程](https://github.com/gothinkster/golang-gin-realworld-example-app/tree/master)</h1>

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




># <h1 id="go-gin-api全栈项目">[go-gin-api全栈项目](https://github.com/xinliangnote/go-gin-api?tab=readme-ov-file)</h1>
- **简介**：基于 Gin 的模块化 API 框架，封装了 JWT 鉴权、日志管理、数据库操作等常用功能。
- [文档](https://www.yuque.com/xinliangnote/go-gin-api/mb9ad8)
- **特性**：
  - 提供代码生成器，快速生成 CRUD 接口。
  - 集成 Swagger 文档，支持自动化测试。
  - 适合团队协作，规范开发流程。
- **项目地址**：[github.com/xinliangnote/go-gin-api](https://github.com/xinliangnote/go-gin-api)  
- **学习价值**：新手友好，适合学习 API 分层设计和工程化实践。



<br/><br/><br/>

***
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

***
<br/>

> <h1 id="ferry工单系统">ferry工单系统</h1> 
- **简介**：基于 Gin 和 Vue 的工单管理系统，后端提供完整的 RESTful API 支持。
- **特性**：
  - 支持自定义审批流程、权限分级。
  - 集成任务钩子和统计功能，适合企业内部流程管理。
- **项目地址**：[github.com/lanyulei/ferry](https://github.com/lanyulei/ferry)  
- **学习价值**：了解复杂业务场景下的 API 设计。

<br/><br/><br/>

***
<br/>

> <h1 id="Gorm（ORM工具）">Gorm（ORM 工具）</h1>
- **简介**：Go 生态中最流行的 ORM 库，常与 RESTful API 结合操作数据库。
- **特性**：
  - 支持事务、关联查询、软删除等高级功能。
  - 自动迁移数据库表结构，简化开发流程。
- **项目地址**：[github.com/go-gorm/gorm](https://github.com/go-gorm/gorm)  
- **适用场景**：快速实现 CRUD 接口，如用户管理系统。

<br/><br/><br/>

***
<br/>

># <h1 id="gin-gorm-restful-api">[gin-gorm-restful-api](https://juejin.cn/post/7036011047391592485)</h1>
- **Gin + GORM 项目**
- **简介**: 使用 Gin 框架和 GORM 构建的 RESTful API 项目，结构清晰，模块化设计，适合初学者快速上手。
- **特点**:
  - 清晰的目录结构（controller、service、model 等）。
  - 支持统一的 JSON 响应格式。
  - 使用 GORM 进行数据库操作，支持 MySQL、PostgreSQL 等。
- **GitHub 地址**: [gin-gorm-restful-api](https://github.com/your-repo/gin-gorm-restful-api) 

<br/><br/><br/>

***
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

***
<br/>

> <h1 id="echo-restful-api">echo-restful-api</h1>
- **Echo框架示例**
- **简介**: 使用 Echo 框架构建的高性能 RESTful API 项目，适合需要高性能的场景。
- **特点**:
  - 支持中间件（如日志、认证）。
  - 结构简单，易于扩展。
  - 提供 Swagger 文档支持。
- **GitHub 地址**: [echo-restful-api](https://github.com/your-repo/echo-restful-api) 

<br/><br/><br/>

***
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

***
<br/>

> <h1 id="beego-restful-api">beego-restful-api</h1>
- **Beego框架示例**
- **简介**: 使用 Beego 框架构建的 RESTful API 项目，适合需要快速开发的场景。
- **特点**:
  - 内置 ORM、缓存、日志等功能。
  - 提供自动化 API 文档生成。
  - 适合全栈开发。
- **GitHub 地址**: [beego-restful-api](https://github.com/your-repo/beego-restful-api)

<br/><br/><br/>

***
<br/>

> <h1 id="kratos-restful-api">kratos-restful-api</h1>
- **Kratos微服务框架**
- **简介**: 基于 Bilibili 开源的 Kratos 框架构建的 RESTful API 项目，适合微服务架构。
- **特点**:
  - 支持 gRPC 和 HTTP 双协议。
  - 提供完善的中间件和插件支持。
  - 适合大型分布式系统。
- **GitHub 地址**: [kratos-restful-api](https://github.com/your-repo/kratos-restful-api)

<br/><br/><br/>

***
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

***
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

***
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

***
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

***
<br/>

> <h1 id="框架">框架</h1>
<br/>


<br/><br/><br/>

***
<br/>

> <h1 id="NSQ源码阅读">NSQ源码阅读</h1>

很棒的学习计划！NSQ 是一个高性能的分布式消息队列系统，Go 实现、结构清晰、注重工程实践，非常适合学习。

---

## 一、目标拆解：你能从 NSQ 学到什么？

* Go 语言的 **工程化开发实践**（目录结构、模块划分、日志处理等）
* **网络编程（TCP、HTTP）**
* **并发模型（goroutine、channel）**
* **消息队列原理与实现（发布/订阅、消息持久化、ack机制等）**
* **系统设计理念**（模块解耦、配置热更新、服务注册等）

---
<br/>

** 二、学习前准备（0基础也能入门）**

**1. 基础技能**

先具备以下基础（可以边学 NSQ 边查）：

* Go 基础语法、interface、goroutine、channel、select
* Go 标准库：`net`, `http`, `os`, `sync`, `log`
* Git 基础操作、源码 clone、`go build` 等

> 学习推荐：

* Go 入门教程：[https://tour.go-zh.org/](https://tour.go-zh.org/)
* 实战 Go： [https://github.com/unknwon/go-fundamental-programming](https://github.com/unknwon/go-fundamental-programming)

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

* `nsqd`：核心组件，负责接收、存储、转发消息
* `nsqlookupd`：服务发现中心，维护 topic 和 channel 映射
* `nsqadmin`：Web UI 管理后台

| 组件             | 作用                                |
| -------------- | --------------------------------- |
| **nsqd**       | 消息服务的核心：接受生产者消息，投递给消费者            |
| **nsqlookupd** | 服务发现：让消费者找到哪些 `nsqd` 有订阅的 topic   |
| **nsqadmin**   | Web UI 控制面板：可监控 topic、channel、消息等 |


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

* 使用 VS Code 或 Goland，设置断点调试（如在 `topic.PutMessage()`）
* 插入日志，比如：

```go
fmt.Println("PutMessage:", msg.ID)
```

观察调用路径和数据结构传递。

---

### ✅ Step 5：制作源码学习笔记

每看懂一个文件，做以下输出：

| 文件       | 作用        | 核心函数                       | 涉及模块/调用链                | 你的理解/总结 |
| -------- | --------- | -------------------------- | ----------------------- | ------- |
| topic.go | Topic对象逻辑 | `PutMessage`、`messagePump` | channel.go、diskqueue.go | xxx     |

---

## 四、思路理解技巧

* **看 main 函数**：程序从哪启动，哪些服务注册在哪个模块？
* **找 interface 和 struct 实现**：如 `protocolV2` 是协议实现，了解其输入输出。
* **画图辅助**：将结构体和调用链画成时序图、模块图，有助于理解。
* **对照官方文档看代码**：[https://nsq.io/](https://nsq.io/)

---

## 五、进阶建议：自己实现一个 mini-NSQ

参考 NSQ 结构，自己实现一个小型版本的消息队列（本地内存即可）：

功能：

* topic + channel 架构
* TCP 接收消息
* 消息广播到多个消费者
* goroutine 实现并发处理

你会更理解 “为什么 NSQ 要这么设计”。

---

## 六、额外工具推荐

| 工具                      | 用途          |
| ----------------------- | ----------- |
| Goland / VSCode         | IDE，方便调试    |
| GoLand 插件：Go Call Graph | 可视化调用链      |
| GoDoc / Sourcegraph     | 阅读注释和跳转函数定义 |
| `richgo test` / delve   | 测试和调试       |

---

## 七、社区交流和参考资料

* [NSQ 源码阅读中文系列](https://github.com/denghongcai/nsq-source-code-learning)
* GoCN、NSQ issue 区、知乎：搜“NSQ 源码解析”

---

## 总结：最重要的是「带着目的去看」

每次阅读都要问：

* **这个模块是干嘛的？**
* **它解决了什么问题？**
* **如果我写这个功能，我会怎么做？**

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

***
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

***
<br/>

> <h1 id="GorillaMux路由库">Gorilla Mux（路由库）</h1> 
- **简介**：灵活的路由库，支持 RESTful 路由匹配、中间件链式调用。
- **特性**：
  - 强大的路径参数解析（如正则匹配）。
  - 兼容标准库 `net/http`，适合渐进式升级旧项目。
- **项目地址**：[github.com/gorilla/mux](https://github.com/gorilla/mux)  
- **案例参考**：适用于需要精细控制路由逻辑的 API 服务。

<br/><br/><br/>

***
<br/>

> <h1 id="Vegeta负载测试工具">Vegeta负载测试工具</h1>
**性能优化**：结合 **Vegeta**（负载测试工具）对 API 进行压测。

<br/><br/><br/>

***
<br/>

> <h1 id="Authboss库">Authboss库-添加认证与授权模块</h1>
**安全增强**：使用 **Authboss** 添加认证与授权模块。


<br/><br/><br/>

***
<br/>

> <h1 id="GoKit库-网关和分布式追踪">GoKit库-网关和分布式追踪</h1>
- **微服务架构**：参考 **GoKit** 实现 API 网关和分布式追踪。


<br/><br/><br/>

***
<br/>

> <h1 id="Beego库">Beego库</h1>
**Beego** 提供了一个完整的 MVC 框架，除了 RESTful API 支持外，还包括 ORM、定时任务等功能，适合构建大型项目。  
  项目地址：[github.com/beego/beego/v2](https://github.com/beego/beego/v2) citeturn0search0


<br/><br/><br/>

***
<br/>

> <h1 id="Fiber库">Fiber库</h1>
**Fiber**灵感来源于 Express（Node.js 框架），追求极致性能和简洁 API，非常适合需要高并发和低延迟的 RESTful API 项目。  
  项目地址：[github.com/gofiber/fiber](https://github.com/gofiber/fiber) citeturn0search0
  
  
  
<br/><br/><br/>

***
<br/>

> <h1 id="go-restful库">go-restful库</h1>
**go-restful**  这个项目提供了一套工具来快速构建 REST 风格的 Web 服务，并在设计上借鉴了 Google 风格。  
  项目地址：[github.com/emicklei/go-restful](https://github.com/emicklei/go-restful) citeturn0search0


<br/><br/><br/>

***
<br/>

> <h1 id="Chi库">Chi库</h1>
**Chi**  一个轻量级且富有表现力的路由库，注重代码的可组合性与可读性，非常适合构建简单或中型 RESTful API。  
   GitHub 地址：[github.com/go-chi/chi](https://github.com/go-chi/chi) citeturn0search0



<br/><br/><br/>

***
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

| 方法 | 说明 |
|------|------|
| `viper.SetDefault("key", value)` | 设置默认值 |
| `viper.BindEnv("key")` | 绑定环境变量 |
| `viper.GetBool/GetString/GetInt...` | 类型安全地获取值 |
| `viper.Unmarshal(&struct)` | 将配置反序列化到结构体 |
| `viper.WatchConfig()` | 监听配置文件变化 |

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

> <h1 id="功能模块">功能模块</h1>

***
<br/><br/><br/>
> <h2 id="登录注册">登录注册</h2>

<br/><br/>
> <h3 id="简单缓存版本">简单缓存版本 </h3>

**调用代码：**

```go
/* 路由注册 */
func RegisterUserRoutes() {
	http.HandleFunc("/user/send_verify_code", sendVerifyCodeHandler)
	http.HandleFunc("/user/register", registerHandler)
	http.HandleFunc("/user/login", loginHandler)
	http.HandleFunc("/user/profile", PkgMiddlewarePackage.TokenAuthMiddleware(profile)) // 受保护接口
}
```

<br/>

- **1.验证码发送**

```sh
curl -X POST http://localhost:8080/user/send_verify_code \
-H "Content-Type: application/json" \
-d '{"account":"test@example.com"}'

{"message":"验证码已发送"}
```

<br/>

**2.注册接口调用**

```sh
curl -X POST http://localhost:8080/user/register \
-H "Content-Type: application/json" \
-d '{"account":"test@example.com","code":"359203","password":"123456"}'

{"id":1,"message":"注册成功"}
```

<br/>

**3.登录接口调用：**

```go
curl -X POST http://localhost:8080/user/login \
-H "Content-Type: application/json" \
-d '{"account":"test@example.com","password":"123456"}'
{"id":2,"message":"登录成功","token":"96051413223373232403545874607770"}
        
{"id":1,"message":"登录成功"}
```

<br/><br/>
> <h3 id="redis缓存-登录注册">redis缓存-登录注册</h3>
**不要在VSCode中启动Redis的服务，若是杀掉了话可能redis就关了，最好在系统的终端：**

```sh
redis-server
```

<br/>

**工程中路由代码：**

```go
/* 路由注册 */
func RegisterUserRoutesV2() {
	cachePackage.NewRedisService() // 初始化Redis连接
	
	http.HandleFunc("/user/send_verify_code", sendVerifyCodeHandlerV2)
	http.HandleFunc("/user/register", registerHandlerV2)
	http.HandleFunc("/user/login", loginHandlerV2)
	http.HandleFunc("/user/profile", PkgMiddlewarePackage.TokenAuthMiddleware(profile)) // 受保护接口
}
```
**验证码、注册、登录的`curl`指令**还是用[简单缓存版本](#简单缓存版本)中的指令。

**4.访问受保护接口：**

```sh
// 传入 登录使用的token码
 curl -X GET http://localhost:8080/user/profile \
-H "Authorization: 96051413223373232403545874607770"

{"message":"已通过认证"}
```



<br/><br/><br/>

***
<br/>

> <h1 id="未完成优秀代码">未完成优秀代码</h1>

***
<br/><br/><br/>
> <h2 id="文件排版和架构">文件排版和架构</h2>

```sh
package UserHandlerPackage

import (
	UserServicePackage "MLC_GO/internal/modules/user/service"

}

/* 路由注册 */
func RegisterUserRoutes() {
	cachePackage.NewRedis("localhost:6379", "", 0) //redis初始化

	http.HandleFunc("/user/send_verify_code", sendVerifyCodeHandler)
	http.HandleFunc("/user/register", registerHandler)
	http.HandleFunc("/user/login", loginHandler)
	http.HandleFunc("/user/profile", UserServicePackage.TokenAuthMiddleware(profile)) // 受保护接口
}

在 service.go中有

package UserServicePackage

import (
	UserHandlerPackage "MLC_GO/internal/modules/user/handler"
)

然后会编译报错，如何解决
```

对于大型公司项目，我会推荐**六层架构 + 依赖注入 + 接口隔离**的方案。不使用第三方库，仅用Go标准库。

## **大型项目架构设计**

### **目录结构**
```
internal/
├── modules/
│   └── user/
│       ├── domain/          # 领域层 (DDD)
│       │   ├── entity/      # 领域实体
│       │   ├── valueobject/ # 值对象
│       │   └── repository/  # 仓储接口
│       ├── application/     # 应用层 (用例/服务)
│       │   ├── dto/         # 数据传输对象
│       │   └── service/     # 应用服务
│       ├── infrastructure/  # 基础设施层
│       │   ├── persistence/ # 持久化实现
│       │   └── cache/       # 缓存实现
│       ├── interfaces/      # 接口层
│       │   ├── http/        # HTTP处理器
│       │   ├── middleware/  # HTTP中间件
│       │   └── validator/   # 参数验证
│       └── di/             # 依赖注入容器
├── pkg/
│   ├── cache/              # 通用缓存组件
│   ├── database/           # 数据库组件
│   └── errorhandler/       # 错误处理
└── cmd/
    └── api/
        └── main.go         # 应用入口
```

## **详细实现**

### **1. 领域层 (Domain Layer) - 核心业务逻辑**
```go
// internal/modules/user/domain/entity/user.go
package entity

import "time"

type User struct {
    ID        int64
    Username  string
    Email     string
    Password  string  // 加密后的
    Status    int
    CreatedAt time.Time
    UpdatedAt time.Time
}

func (u *User) Validate() error {
    // 业务规则验证
    if len(u.Username) < 3 {
        return errors.New("用户名至少3个字符")
    }
    return nil
}
```

```go
// internal/modules/user/domain/repository/user_repository.go
package repository

type UserRepository interface {
    FindByID(id int64) (*entity.User, error)
    FindByEmail(email string) (*entity.User, error)
    Save(user *entity.User) error
    Update(user *entity.User) error
    Delete(id int64) error
}
```

### **2. 应用层 (Application Layer) - 用例编排**
```go
// internal/modules/user/application/dto/user_dto.go
package dto

type RegisterRequest struct {
    Username string `json:"username" validate:"required,min=3"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=6"`
}

type LoginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

type UserResponse struct {
    ID       int64  `json:"id"`
    Username string `json:"username"`
    Email    string `json:"email"`
}
```

```go
// internal/modules/user/application/service/user_service.go
package service

import (
    "context"
    "MLC_GO/internal/modules/user/application/dto"
    "MLC_GO/internal/modules/user/domain/entity"
    "MLC_GO/internal/modules/user/domain/repository"
)

// 应用服务接口
type UserService interface {
    Register(ctx context.Context, req *dto.RegisterRequest) (*dto.UserResponse, error)
    Login(ctx context.Context, req *dto.LoginRequest) (string, error) // 返回token
    GetProfile(ctx context.Context, userID int64) (*dto.UserResponse, error)
}

// 实现
type userServiceImpl struct {
    userRepo repository.UserRepository
    tokenGen TokenGenerator
}

func NewUserService(repo repository.UserRepository, tokenGen TokenGenerator) UserService {
    return &userServiceImpl{
        userRepo: repo,
        tokenGen: tokenGen,
    }
}

func (s *userServiceImpl) Register(ctx context.Context, req *dto.RegisterRequest) (*dto.UserResponse, error) {
    // 业务逻辑
    user := &entity.User{
        Username: req.Username,
        Email:    req.Email,
        Password: hashPassword(req.Password),
    }
    
    if err := user.Validate(); err != nil {
        return nil, err
    }
    
    if err := s.userRepo.Save(user); err != nil {
        return nil, err
    }
    
    return &dto.UserResponse{
        ID:       user.ID,
        Username: user.Username,
        Email:    user.Email,
    }, nil
}
```

### **3. 基础设施层 (Infrastructure Layer)**
```go
// internal/modules/user/infrastructure/persistence/user_repository_impl.go
package persistence

import (
    "database/sql"
    "MLC_GO/internal/modules/user/domain/entity"
    "MLC_GO/internal/modules/user/domain/repository"
)

type userRepositoryImpl struct {
    db *sql.DB
}

func NewUserRepository(db *sql.DB) repository.UserRepository {
    return &userRepositoryImpl{db: db}
}

func (r *userRepositoryImpl) FindByID(id int64) (*entity.User, error) {
    // 数据库查询实现
    row := r.db.QueryRow("SELECT id, username, email FROM users WHERE id = ?", id)
    
    var user entity.User
    err := row.Scan(&user.ID, &user.Username, &user.Email)
    if err != nil {
        return nil, err
    }
    
    return &user, nil
}
```

### **4. 接口层 (Interface Layer) - HTTP处理器**
```go
// internal/modules/user/interfaces/http/user_handler.go
package http

import (
    "encoding/json"
    "net/http"
    "MLC_GO/internal/modules/user/application/dto"
    "MLC_GO/internal/modules/user/application/service"
)

type UserHandler struct {
    userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
    return &UserHandler{userService: userService}
}

func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
    var req dto.RegisterRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "无效的请求参数", http.StatusBadRequest)
        return
    }
    
    // 参数验证
    if err := validateRegisterRequest(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    resp, err := h.userService.Register(r.Context(), &req)
    if err != nil {
        // 根据错误类型返回不同状态码
        handleError(w, err)
        return
    }
    
    jsonResponse(w, resp, http.StatusCreated)
}

// 辅助函数
func jsonResponse(w http.ResponseWriter, data interface{}, statusCode int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(data)
}
```

### **5. 中间件层 (Middleware)**
```go
// internal/modules/user/interfaces/middleware/auth_middleware.go
package middleware

import (
    "context"
    "net/http"
    "strings"
)

// 上下文Key类型
type contextKey string

const (
    UserIDKey contextKey = "user_id"
)

func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 从Header获取token
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            http.Error(w, "未提供认证令牌", http.StatusUnauthorized)
            return
        }
        
        // Bearer token格式
        parts := strings.Split(authHeader, " ")
        if len(parts) != 2 || parts[0] != "Bearer" {
            http.Error(w, "令牌格式错误", http.StatusUnauthorized)
            return
        }
        
        token := parts[1]
        
        // 验证token (这里可以是JWT验证或查Redis)
        userID, err := validateToken(token)
        if err != nil {
            http.Error(w, "令牌无效或已过期", http.StatusUnauthorized)
            return
        }
        
        // 将用户ID存入上下文
        ctx := context.WithValue(r.Context(), UserIDKey, userID)
        
        // 调用下一个处理器
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func validateToken(token string) (int64, error) {
    // 实现token验证逻辑
    // 可以查Redis或验证JWT
    return 1, nil // 示例
}
```

### **6. 依赖注入容器**
```go
// internal/modules/user/di/container.go
package di

import (
    "database/sql"
    "MLC_GO/internal/modules/user/application/service"
    "MLC_GO/internal/modules/user/infrastructure/persistence"
    "MLC_GO/internal/modules/user/interfaces/http"
)

// Container 依赖注入容器
type Container struct {
    DB          *sql.DB
    UserService service.UserService
    UserHandler *http.UserHandler
}

func NewContainer(db *sql.DB) *Container {
    // 1. 创建仓储
    userRepo := persistence.NewUserRepository(db)
    
    // 2. 创建Token生成器
    tokenGen := NewTokenGenerator()
    
    // 3. 创建应用服务
    userService := service.NewUserService(userRepo, tokenGen)
    
    // 4. 创建HTTP处理器
    userHandler := http.NewUserHandler(userService)
    
    return &Container{
        DB:          db,
        UserService: userService,
        UserHandler: userHandler,
    }
}

// 全局容器实例
var globalContainer *Container

func InitContainer(db *sql.DB) {
    globalContainer = NewContainer(db)
}

func GetContainer() *Container {
    return globalContainer
}
```

### **7. 路由注册**
```go
// internal/modules/user/interfaces/http/router.go
package http

import (
    "net/http"
    "MLC_GO/internal/modules/user/di"
    "MLC_GO/internal/modules/user/interfaces/middleware"
)

func SetupRoutes(mux *http.ServeMux) {
    container := di.GetContainer()
    
    // 公开路由
    mux.HandleFunc("POST /api/users/register", container.UserHandler.Register)
    mux.HandleFunc("POST /api/users/login", container.UserHandler.Login)
    
    // 需要认证的路由
    protectedMux := http.NewServeMux()
    protectedMux.HandleFunc("GET /api/users/profile", container.UserHandler.GetProfile)
    
    // 应用认证中间件
    mux.Handle("/api/users/", middleware.AuthMiddleware(protectedMux))
}
```

### **8. 主程序入口**
```go
// cmd/api/main.go
package main

import (
    "database/sql"
    "log"
    "net/http"
    _ "github.com/go-sql-driver/mysql"
    
    "MLC_GO/internal/modules/user/di"
    userHTTP "MLC_GO/internal/modules/user/interfaces/http"
)

func main() {
    // 1. 初始化数据库
    db, err := sql.Open("mysql", "user:pass@tcp(localhost:3306)/dbname")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // 2. 初始化Redis (标准库实现HTTP客户端连接)
    // redisConn := initRedis()
    
    // 3. 初始化依赖容器
    di.InitContainer(db)
    
    // 4. 设置路由
    mux := http.NewServeMux()
    userHTTP.SetupRoutes(mux)
    
    // 5. 全局中间件链
    handler := applyGlobalMiddleware(mux)
    
    // 6. 启动服务器
    log.Println("Server starting on :8080")
    if err := http.ListenAndServe(":8080", handler); err != nil {
        log.Fatal(err)
    }
}

func applyGlobalMiddleware(h http.Handler) http.Handler {
    // 全局中间件：日志、恢复、CORS等
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. 日志
        log.Printf("%s %s", r.Method, r.URL.Path)
        
        // 2. CORS (使用标准库)
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }
        
        // 3. 恢复panic
        defer func() {
            if err := recover(); err != nil {
                log.Printf("Recovered from panic: %v", err)
                http.Error(w, "Internal Server Error", http.StatusInternalServerError)
            }
        }()
        
        h.ServeHTTP(w, r)
    })
}
```

## **关键设计原则**

### **1. 依赖规则（Clean Architecture）**
```
接口层 → 应用层 → 领域层
        ↑         ↓
基础设施层 ←─────┘
```

### **2. 接口隔离**
```go
// 使用接口而非具体实现
type UserRepository interface { ... }
type TokenGenerator interface { ... }
type Cache interface { ... }
```

### **3. 错误处理标准化**
```go
// internal/pkg/errorhandler/errors.go
package errorhandler

type AppError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Err     error  `json:"-"`
}

func (e *AppError) Error() string {
    if e.Err != nil {
        return e.Err.Error()
    }
    return e.Message
}

func NewBadRequestError(message string) *AppError {
    return &AppError{
        Code:    http.StatusBadRequest,
        Message: message,
    }
}
```

### **4. 配置管理**
```go
// config/config.go
package config

import (
    "encoding/json"
    "os"
)

type Config struct {
    Database DatabaseConfig `json:"database"`
    Redis    RedisConfig    `json:"redis"`
    Server   ServerConfig   `json:"server"`
}

func LoadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    
    var config Config
    if err := json.Unmarshal(data, &config); err != nil {
        return nil, err
    }
    
    return &config, nil
}
```

## **大型项目优势**

1. **分层清晰**：每层职责单一
2. **易于测试**：依赖接口，方便Mock
3. **可维护性**：修改一层不影响其他层
4. **团队协作**：不同团队负责不同层
5. **技术栈升级**：基础设施层可替换实现
6. **代码复用**：领域层与应用层可被多个接口层复用

这种架构虽然初期代码量较多，但能支撑百万行代码的大型项目，保证长期的可维护性和可扩展性。


<br/><br/>

# 多个模块文件、架构设计
对于大型公司项目，不使用第三方库，我推荐以下架构方案：

## **项目结构设计（大型项目）**

```
MLC_GO/
├── cmd/
│   └── api/
│       └── main.go                 # 应用入口
├── internal/
│   ├── app/                        # 应用层（依赖注入）
│   │   ├── container.go            # 依赖容器
│   │   └── wire.go                 # 依赖注入配置（可选）
│   ├── domain/                     # 领域层（核心业务）
│   │   ├── user/
│   │   │   ├── entity.go           # 用户实体
│   │   │   ├── repository.go       # 仓储接口
│   │   │   └── service.go          # 领域服务接口
│   │   └── product/
│   │       ├── entity.go
│   │       ├── repository.go
│   │       └── service.go
│   ├── infrastructure/             # 基础设施层
│   │   ├── persistence/
│   │   │   ├── mysql/
│   │   │   └── redis/
│   │   ├── cache/
│   │   │   └── redis_cache.go
│   │   └── http/
│   │       └── server.go
│   ├── interfaces/                 # 接口层
│   │   ├── handlers/               # HTTP处理器
│   │   │   ├── user_handler.go
│   │   │   └── product_handler.go
│   │   ├── middleware/             # HTTP中间件
│   │   │   ├── auth.go
│   │   │   ├── logging.go
│   │   │   └── recovery.go
│   │   └── presenters/             # 响应格式化
│   │       └── json_presenter.go
│   └── application/                # 应用服务层
│       ├── user/
│       │   ├── service.go          # 应用服务
│       │   └── dto.go              # 数据传输对象
│       └── product/
│           ├── service.go
│           └── dto.go
├── pkg/                            # 公共库（可被外部引用）
│   ├── errors/
│   ├── validator/
│   └── logger/
└── configs/                        # 配置文件
```

## **具体实现方案**

### 1. **领域层（Domain） - 核心业务逻辑**
```go
// internal/domain/user/entity.go
package user

type User struct {
    ID        int64
    Username  string
    Email     string
    Password  string  // 加密后的
    CreatedAt time.Time
}

func (u *User) Validate() error {
    if u.Username == "" {
        return errors.New("username is required")
    }
    // 其他验证逻辑
    return nil
}

// internal/domain/user/repository.go
package user

type Repository interface {
    Save(user *User) error
    FindByID(id int64) (*User, error)
    FindByEmail(email string) (*User, error)
    FindByUsername(username string) (*User, error)
    ExistsByEmail(email string) (bool, error)
}
```

### 2. **接口层（Interfaces）**
```go
// internal/interfaces/middleware/auth.go
package middleware

import (
    "MLC_GO/internal/app"
    "context"
    "net/http"
    "strings"
)

type AuthMiddleware struct {
    app *app.Container
}

func NewAuthMiddleware(app *app.Container) *AuthMiddleware {
    return &AuthMiddleware{app: app}
}

func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. 提取Token
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        
        token := strings.TrimPrefix(authHeader, "Bearer ")
        
        // 2. 验证Token（调用应用服务）
        userID, err := m.app.UserAppService.ValidateToken(r.Context(), token)
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        
        // 3. 将用户信息存入Context
        ctx := context.WithValue(r.Context(), "userID", userID)
        ctx = context.WithValue(ctx, "token", token)
        
        // 4. 继续处理
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 3. **应用服务层（Application）**
```go
// internal/application/user/service.go
package user

import (
    "MLC_GO/internal/domain/user"
    "context"
)

type AppService struct {
    userRepo    user.Repository
    tokenRepo   TokenRepository
}

func NewAppService(userRepo user.Repository, tokenRepo TokenRepository) *AppService {
    return &AppService{
        userRepo:  userRepo,
        tokenRepo: tokenRepo,
    }
}

func (s *AppService) ValidateToken(ctx context.Context, token string) (int64, error) {
    // 验证token逻辑
    userID, err := s.tokenRepo.GetUserIDByToken(ctx, token)
    if err != nil {
        return 0, err
    }
    
    return userID, nil
}

func (s *AppService) Register(ctx context.Context, dto RegisterDTO) (*UserDTO, error) {
    // 1. 验证DTO
    if err := dto.Validate(); err != nil {
        return nil, err
    }
    
    // 2. 检查用户是否存在
    exists, err := s.userRepo.ExistsByEmail(dto.Email)
    if err != nil {
        return nil, err
    }
    if exists {
        return nil, ErrUserAlreadyExists
    }
    
    // 3. 创建领域实体
    u := &user.User{
        Username: dto.Username,
        Email:    dto.Email,
        // 密码加密等操作
    }
    
    // 4. 保存用户
    if err := s.userRepo.Save(u); err != nil {
        return nil, err
    }
    
    // 5. 返回DTO
    return &UserDTO{
        ID:       u.ID,
        Username: u.Username,
        Email:    u.Email,
    }, nil
}
```

### 4. **HTTP处理器**
```go
// internal/interfaces/handlers/user_handler.go
package handlers

import (
    "MLC_GO/internal/application/user"
    "encoding/json"
    "net/http"
)

type UserHandler struct {
    appService *user.AppService
}

func NewUserHandler(appService *user.AppService) *UserHandler {
    return &UserHandler{appService: appService}
}

func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
    // 1. 解析请求
    var req user.RegisterDTO
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    // 2. 调用应用服务
    userDTO, err := h.appService.Register(r.Context(), req)
    if err != nil {
        // 错误处理
        h.handleError(w, err)
        return
    }
    
    // 3. 返回响应
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(userDTO)
}

func (h *UserHandler) handleError(w http.ResponseWriter, err error) {
    // 错误映射逻辑
    switch err {
    case user.ErrUserAlreadyExists:
        http.Error(w, "User already exists", http.StatusConflict)
    default:
        http.Error(w, "Internal server error", http.StatusInternalServerError)
    }
}
```

### 5. **依赖注入容器**
```go
// internal/app/container.go
package app

import (
    "MLC_GO/internal/application/user"
    "MLC_GO/internal/domain/user/repository"
    "MLC_GO/internal/infrastructure/persistence/mysql"
    "MLC_GO/internal/interfaces/handlers"
    "MLC_GO/internal/interfaces/middleware"
    "context"
    "database/sql"
    "sync"
)

type Container struct {
    db *sql.DB
    redisClient interface{} // 自定义Redis客户端
    
    // 服务实例（懒加载）
    userRepo      repository.Repository
    userAppService *user.AppService
    authMiddleware *middleware.AuthMiddleware
    userHandler    *handlers.UserHandler
    
    mu sync.Mutex
}

func NewContainer(db *sql.DB) *Container {
    return &Container{
        db: db,
    }
}

// 懒加载模式获取实例
func (c *Container) GetUserRepository() repository.Repository {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    if c.userRepo == nil {
        c.userRepo = mysql.NewUserRepository(c.db)
    }
    return c.userRepo
}

func (c *Container) GetUserAppService() *user.AppService {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    if c.userAppService == nil {
        c.userAppService = user.NewAppService(
            c.GetUserRepository(),
            c.GetTokenRepository(),
        )
    }
    return c.userAppService
}

func (c *Container) GetAuthMiddleware() *middleware.AuthMiddleware {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    if c.authMiddleware == nil {
        c.authMiddleware = middleware.NewAuthMiddleware(c)
    }
    return c.authMiddleware
}

func (c *Container) GetUserHandler() *handlers.UserHandler {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    if c.userHandler == nil {
        c.userHandler = handlers.NewUserHandler(c.GetUserAppService())
    }
    return c.userHandler
}
```

### 6. **主入口和路由配置**
```go
// cmd/api/main.go
package main

import (
    "MLC_GO/internal/app"
    "MLC_GO/internal/infrastructure/http"
    "database/sql"
    "log"
    _ "github.com/go-sql-driver/mysql"
)

func main() {
    // 1. 初始化基础设施
    db, err := sql.Open("mysql", "user:pass@tcp(localhost:3306)/dbname")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // 2. 创建依赖容器
    container := app.NewContainer(db)
    
    // 3. 创建HTTP服务器
    server := http.NewServer(container)
    
    // 4. 注册路由
    server.RegisterRoutes()
    
    // 5. 启动服务器
    log.Println("Server starting on :8080")
    if err := server.Start(":8080"); err != nil {
        log.Fatal(err)
    }
}

// internal/infrastructure/http/server.go
package http

import (
    "MLC_GO/internal/app"
    "net/http"
)

type Server struct {
    container *app.Container
    mux       *http.ServeMux
}

func NewServer(container *app.Container) *Server {
    return &Server{
        container: container,
        mux:       http.NewServeMux(),
    }
}

func (s *Server) RegisterRoutes() {
    // 获取处理器
    userHandler := s.container.GetUserHandler()
    authMiddleware := s.container.GetAuthMiddleware()
    
    // 公开路由
    s.mux.HandleFunc("/api/v1/register", userHandler.Register)
    s.mux.HandleFunc("/api/v1/login", userHandler.Login)
    
    // 受保护路由（使用中间件）
    protected := http.HandlerFunc(userHandler.Profile)
    s.mux.Handle("/api/v1/profile", authMiddleware.Authenticate(protected))
    
    // 其他模块的路由...
    
    // 健康检查
    s.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    })
}

func (s *Server) Start(addr string) error {
    // 可以在这里添加全局中间件链
    handler := s.withGlobalMiddleware(s.mux)
    
    return http.ListenAndServe(addr, handler)
}

func (s *Server) withGlobalMiddleware(next http.Handler) http.Handler {
    // 应用全局中间件：日志、恢复、CORS等
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. 日志中间件
        logMiddleware(next).ServeHTTP(w, r)
    })
}
```

### 7. **模块化设计支持多个模块**
```go
// 模块A：用户模块
// internal/domain/module_a/...

// 模块B：产品模块  
// internal/domain/module_b/...

// 模块C：订单模块
// internal/domain/module_c/...

// 在container中管理所有模块
type Container struct {
    moduleAService *module_a.AppService
    moduleBService *module_b.AppService
    moduleCService *module_c.AppService
}

// 每个模块独立注册路由
func (s *Server) RegisterModuleRoutes() {
    // 用户模块路由
    s.registerUserRoutes()
    
    // 产品模块路由
    s.registerProductRoutes()
    
    // 订单模块路由
    s.registerOrderRoutes()
}
```

## **关键设计原则**

1. **依赖倒置**：高层模块不依赖低层模块，都依赖抽象接口
2. **单一职责**：每个包、每个结构体、每个函数只做一件事
3. **明确依赖方向**：依赖方向从外向内（外层依赖内层）
4. **懒加载模式**：容器中的服务按需初始化
5. **接口隔离**：使用接口定义契约，而不是具体实现

## **优点**
- **无第三方依赖**：完全使用Go标准库
- **高度可测试**：依赖接口，容易mock
- **模块化**：支持多个团队并行开发
- **可维护性强**：结构清晰，职责明确
- **扩展性好**：新功能容易添加

这个架构适合大型企业级应用，可以支持数百个开发人员协作，代码可维护性和可扩展性都非常好。


<br/><br/><br/>

***
<br/>

> <h1 id="环境变量传递">环境变量传递</h1>

***
<br/><br/><br/>
> <h2 id="VSCode-launch.json-环境变量传递流程">VSCode launch.json 环境变量传递流程</h2>

**问题：** `internal/config/env.go` 中的 `GetEnv()` 函数是如何获取当前环境的？

**答案：** 通过 VSCode 的 launch.json 配置注入环境变量。

---

### 1. launch.json 配置（`.vscode/launch.json`）

```json
{
  "name": "🧪 Launch MLC_GO Debug",
  "type": "go",
  "request": "launch",
  "mode": "auto",
  "program": "${workspaceFolder}",
  "env": {
    "GOPATH": "${env:GOPATH}",
    "MY_ENV": "debug",
    "SERVER_ENV": "debug"  // ← 关键：设置 SERVER_ENV=debug
  }
}
```

---

### 2. VSCode 调试按钮点击流程

```
点击 "🧪 Launch MLC_GO Debug" 
    ↓
VSCode 读取 launch.json
    ↓
启动 Go 调试器 (dlv)
    ↓
dlv 创建子进程，注入 env 中的环境变量
    ↓
Go 程序启动，进程的环境变量包含 SERVER_ENV=debug
```

---

### 3. Go 代码获取环境变量（`internal/config/env.go`）

```go
func GetEnv() Env {
    env := os.Getenv("SERVER_ENV")  // 从进程环境变量中读取
    if env == "" {
        return EnvDebug // 默认环境为 debug
    }
    return Env(env)
}
```

---

### 4. 完整链路

```
launch.json "env.SERVER_ENV" = "debug"
        ↓
VSCode/dlv 注入到进程环境变量
        ↓
os.Getenv("SERVER_ENV") 读取到 "debug"
        ↓
GetEnv() 返回 EnvDebug
        ↓
LoadConfig("debug") 加载 config.debug.yaml
```

---

### 5. 三个环境配置对照表

| 配置名 | SERVER_ENV | 加载配置文件 |
|--------|------------|--------------|
| 🧪 Launch MLC_GO Debug | `debug` | config.debug.yaml |
| 💥 Launch MLC_GO Pre | `pre` | config.pre.yaml |
| 🎾 Launch MLC_GO Prod | `prod` | config.prod.yaml |

---

### 6. 简单理解

- `launch.json` 的 `env` 字段 = 启动时注入的环境变量
- `os.Getenv()` = 从当前进程读取环境变量
- 两者通过**操作系统进程环境变量机制**连接

---

### 7. 相关文件

| 文件 | 作用 |
|------|------|
| `.vscode/launch.json` | VSCode 调试配置，定义环境变量 |
| `internal/config/env.go` | 读取环境变量，决定加载哪个配置文件 |
| `config/config.debug.yaml` | debug 环境配置 |
| `config/config.pre.yaml` | pre 环境配置 |
| `config/config.prod.yaml` | prod 环境配置 |


<br/><br/><br/>

***
<br/>

> <h1 id="日志系统">日志系统</h1>

***
<br/><br/><br/>
> <h2 id="日志函数添加调用者信息">日志函数添加调用者信息</h2>

**文件位置：** `internal/pkg/logHG/hglog.go`

**功能：** 在日志输出中自动添加调用者的文件名、行号和函数名。

---

### 1. getCallerInfo 辅助函数

```go
// getCallerInfo 获取调用者的文件名和函数名
// skip=1 获取直接调用者，skip=2 获取调用者的调用者，依此类推
func getCallerInfo(skip int) string {
    pc, file, line, ok := runtime.Caller(skip)
    if !ok {
        return "???:0"
    }

    // 只保留文件名，去掉路径
    parts := strings.Split(file, "/")
    fileName := parts[len(parts)-1]

    // 获取函数名
    funcName := runtime.FuncForPC(pc).Name()
    // 只保留函数名，去掉包路径
    funcParts := strings.Split(funcName, ".")
    funcName = funcParts[len(funcParts)-1]

    return fmt.Sprintf("%s:%d %s", fileName, line, funcName)
}
```

---

### 2. 日志函数示例

```go
func DebugFInfo(format string, v ...interface{}) {
    caller := getCallerInfo(2)
    log.Printf("🔥 [%s] "+format, append([]interface{}{caller}, v...)...)
}

func ErrFInfo(format string, v ...interface{}) {
    caller := getCallerInfo(2)
    log.Printf("❌ [%s] "+format, append([]interface{}{caller}, v...)...)
}
```

---

### 3. 输出格式

```
🔥 [hg_user_handler.go:123 Login] 用户登录成功
❌ [hg_user_service.go:45 GetUserByID] 查询用户失败
💣 [main.go:30 main] 启动失败
```

---

### 4. 日志函数对照表

| 函数 | 前缀 | 用途 |
|------|------|------|
| `DebugInfo` | 🔥 | 调试信息 |
| `DebugFInfo` | 🔥 | 调试信息（格式化） |
| `ErrInfo` | ❌ | 错误信息 |
| `ErrFInfo` | ❌ | 错误信息（格式化） |
| `FatalFInfo` | 💣 | 致命错误（会退出程序） |


<br/><br/><br/>

***
<br/>

> <h1 id="配置加载">配置加载</h1>

***
<br/><br/><br/>
> <h2 id="LoadConfig函数解释">LoadConfig函数解释</h2>

**文件位置：** `internal/config/env.go`

**功能：** 使用 Viper 库加载不同环境的 YAML 配置文件。

---

### 1. 函数代码

```go
func LoadConfig(env string) error {
    viper.SetConfigName(fmt.Sprintf("config.%s", env))  // 1️⃣ 设置配置文件名
    viper.SetConfigType("yaml")                          // 2️⃣ 设置配置文件类型
    viper.AddConfigPath("./config")                      // 3️⃣ 设置配置文件搜索路径
    err := viper.ReadInConfig()                          // 4️⃣ 读取配置文件
    return err
}
```

---

### 2. 逐行解析

| 行 | 代码 | 作用 |
|---|------|------|
| 1 | `viper.SetConfigName(fmt.Sprintf("config.%s", env))` | 设置配置文件名为 `config.debug`、`config.pre` 或 `config.prod`（不含扩展名） |
| 2 | `viper.SetConfigType("yaml")` | 告诉 viper 配置文件是 YAML 格式 |
| 3 | `viper.AddConfigPath("./config")` | 在 `./config` 目录下搜索配置文件 |
| 4 | `viper.ReadInConfig()` | 实际读取并解析配置文件到内存 |

---

### 3. 调用流程

```
main.go
  ↓
GetEnv() → "debug"
  ↓
LoadConfig("debug")
  ↓
viper 读取 ./config/config.debug.yaml
  ↓
配置加载完成，后续可用 viper.GetString("mysql.host") 等读取
```

---

### 4. 实际效果

当传入 `env="debug"` 时，viper 会去读取 `./config/config.debug.yaml` 文件，将其中的配置项（如数据库连接、Redis 地址等）加载到内存中，供程序后续使用。

---

### 5. 相关配置文件

| 环境 | 配置文件 |
|------|----------|
| debug | `config/config.debug.yaml` |
| pre | `config/config.pre.yaml` |
| prod | `config/config.prod.yaml` |


<br/><br/><br/>

***
<br/>

> <h1 id="Redis配置">Redis配置</h1>

***
<br/><br/><br/>
> <h2 id="getRedisAddr函数解释">getRedisAddr函数解释</h2>

**文件位置：** `internal/infrastructure/persistence/redis/hg_redis.go`

**功能：** 从环境变量获取 Redis 服务器的地址和端口号。

---

### 1. 函数代码

```go
func getRedisAddr() string {
    redisHost := os.Getenv("REDIS_HOST")    // 1️⃣ 从环境变量读取主机
    if redisHost == "" {
        redisHost = "localhost"             // 默认值
    }

    redisPort := os.Getenv("REDIS_PORT")    // 2️⃣ 从环境变量读取端口
    if redisPort == "" {
        redisPort = "6379"                  // 默认值
    }

    return redisHost + ":" + redisPort      // 3️⃣ 拼接成 "host:port" 格式
}
```

---

### 2. 数据来源链路

```
VSCode launch.json
    ↓
env.SERVER_ENV = "debug"
    ↓
Go 程序启动，加载 config.debug.yaml
    ↓
env 文件 (config/env_configs/hg_debug.env) 中定义：
    REDIS_HOST=127.0.0.1
    REDIS_PORT=6379
    ↓
os.Getenv("REDIS_HOST") → "127.0.0.1"
os.Getenv("REDIS_PORT") → "6379"
    ↓
返回 "127.0.0.1:6379"
```

---

### 3. 三个环境的配置对照

| 环境 | REDIS_HOST | REDIS_PORT | 最终地址 |
|------|------------|------------|----------|
| debug | 127.0.0.1 | 6379 | 127.0.0.1:6379 |
| pre | 127.0.0.1 | 6380 | 127.0.0.1:6380 |
| prod | prod-redis.internal | 6379 | prod-redis.internal:6379 |

---

### 4. 相关配置文件

| 文件 | 作用 |
|------|------|
| `config/env_configs/hg_debug.env` | debug 环境的环境变量定义 |
| `config/env_configs/hg_pre.env` | pre 环境的环境变量定义 |
| `config/env_configs/hg_prod.env` | prod 环境的环境变量定义 |


<br/><br/><br/>

***
<br/>

> <h1 id="环境配置加载">环境配置加载</h1>

***
<br/><br/><br/>
> <h2 id="Load函数解释">Load函数解释</h2>

**文件位置：** `internal/config/hg_env_config.go`

**功能：** 从环境变量加载 MySQL 等基础设施配置，返回配置模型。

---

### 1. 函数代码

```go
func Load() *ENVConfigModel {
    // 1️⃣ 读取数据库迁移版本号
    v, err := strconv.Atoi(os.Getenv("MIGRATE_EXPECT_VERSION"))
    if err != nil {
        v = 1  // 默认版本为 1
    }

    // 2️⃣ 返回配置模型
    return &ENVConfigModel{
        MySQLHost:      getEnvOrDefault("MYSQL_HOST", "127.0.0.1"),
        MySQLPort:      getEnvOrDefault("MYSQL_PORT", "3306"),
        MySQLUser:      getEnvOrDefault("MYSQL_USER", "root"),
        MySQLPass:      resolveMySQLPassword(),  // 3️⃣ 特殊处理密码
        MySQLDB:        getEnvOrDefault("MYSQL_DB", "HG_MLC_DB"),
        MAC_TYPE:       getEnvOrDefault("MAC_TYPE", DEV_COMPUTER),
        MigrateVersion: v,
    }
}
```

---

### 2. 核心逻辑

| 函数 | 作用 |
|------|------|
| `getEnvOrDefault(key, fallback)` | 读取环境变量，若为空则返回默认值 |
| `resolveMySQLPassword()` | 根据电脑芯片类型（Intel/M2）选择不同密码策略 |

---

### 3. `resolveMySQLPassword()` 密码策略

```
判断是否 macOS + ARM64 (M1/M2/M3 芯片)
    ↓
是 → 优先读 MYSQL_PASSWORD_ARM，否则默认 "hh109"
    ↓
否 → 直接读 MYSQL_PASSWORD（Intel 电脑）
```

---

### 4. 数据来源

| 配置项 | 环境变量 | 默认值 |
|--------|----------|--------|
| MySQLHost | MYSQL_HOST | 127.0.0.1 |
| MySQLPort | MYSQL_PORT | 3306 |
| MySQLUser | MYSQL_USER | root |
| MySQLPass | MYSQL_PASSWORD 或 MYSQL_PASSWORD_ARM | 视芯片而定 |
| MySQLDB | MYSQL_DB | HG_MLC_DB |
| MigrateVersion | MIGRATE_EXPECT_VERSION | 1 |

---

### 5. 与 `LoadConfig` 的区别

| 函数 | 文件 | 作用 |
|------|------|------|
| `Load()` | hg_env_config.go | 加载 MySQL 等基础设施配置 |
| `LoadConfig()` | env.go | 加载 YAML 配置文件（使用 Viper） |


<br/><br/><br/>

***
<br/>

> <h2 id="getEnvOrDefault函数解释">getEnvOrDefault函数解释</h2>

**文件位置：** `internal/config/hg_env_config.go`

**功能：** 从操作系统的环境变量中读取值，若不存在则返回默认值。

---

### 1. 函数代码

```go
func getEnvOrDefault(key, fallback string) string {
    if value := os.Getenv(key); value != "" {
        return value      // 环境变量存在，返回它
    }
    return fallback       // 环境变量不存在，返回默认值
}
```

---

### 2. 环境变量的来源链路

```
.env 文件 (config/env_configs/hg_debug.env)
    ↓
程序启动时加载到进程环境变量
    ↓
os.Getenv("MYSQL_HOST") 读取
    ↓
getEnvOrDefault 返回值
```

---

### 3. 具体流程

| 步骤 | 说明 |
|------|------|
| 1 | VSCode 启动程序，注入 `SERVER_ENV=debug` |
| 2 | 程序根据环境加载对应的 `.env` 文件（如 `hg_debug.env`） |
| 3 | `.env` 文件中的 `MYSQL_HOST=127.0.0.1` 被加载到进程环境变量 |
| 4 | `os.Getenv("MYSQL_HOST")` 返回 `"127.0.0.1"` |

---

### 4. 相关 .env 文件

| 文件 | 内容示例 |
|------|----------|
| `config/env_configs/hg_debug.env` | `MYSQL_HOST=127.0.0.1` |
| `config/env_configs/hg_pre.env` | `MYSQL_HOST=127.0.0.1` |
| `config/env_configs/hg_prod.env` | `MYSQL_HOST=prod-mysql.internal` |


<br/><br/><br/>

***
<br/>

> <h1 id="根路由处理">根路由处理</h1>

***
<br/><br/><br/>
> <h2 id="NewRootHandler方法解释">NewRootHandler方法解释</h2>

**文件位置：** `internal/handler/hg_root_handler.go`

**功能：** 构建根路由，统一挂载各模块的路由到 `/api/v1` 前缀下。

---

### 1. 函数代码

```go
func NewRootHandler(deps HGRootHandlerDeps) *http.ServeMux {
    rootMux := http.NewServeMux()  // 1️⃣ 创建根路由

    // 2️⃣ 创建各模块的 Handler
    userHandler := UserHandlerPackage.NewUserHandler(...)
    publicHandler := HGMiddlewareGroupPackage.NewAuthRouteInterceptorGroup(userHandler)
    userHandlerWithAuth := HGMiddlewareGroupPackage.NewUserRouteInterceptorGroup(userHandler)
    testHandler := HGTestHandlerPackage.TestModuleHandler()

    // 3️⃣ 挂载模块路由到根路由
    registerRootPrefixRoutes(rootMux, []HGRouteMount{
        {Prefix: "/api/v1/auth/", StripPrefix: "/api/v1/auth", Handler: publicHandler},
        {Prefix: "/api/v1/profile/", StripPrefix: "/api/v1/profile", Handler: userHandlerWithAuth},
        {Prefix: "/api/v1/test/", StripPrefix: "/api/v1/test", Handler: testHandler},
    })

    // 4️⃣ 注册路由清单接口
    rootMux.Handle("/api/v1/routes", ...)
    rootMux.Handle("/api/v1/routes/groups", ...)

    return rootMux
}
```

---

### 2. 核心职责

| 步骤 | 说明 |
|------|------|
| 1 | 创建根路由 `http.ServeMux` |
| 2 | 初始化各模块 Handler（用户、认证、测试） |
| 3 | 使用 `StripPrefix` 挂载模块路由 |
| 4 | 注册路由清单接口（供前端联调） |

---

### 3. StripPrefix 机制

```
请求: GET /api/v1/profile/info
         ↓
rootMux 匹配 /api/v1/profile/
         ↓
StripPrefix("/api/v1/profile")
         ↓
子路由收到: /info
         ↓
匹配到 Profile Handler
```

---

### 4. 模块路由对照表

| 请求 URL | 实际命中 |
|----------|----------|
| `/api/v1/auth/send_code` | publicMux → /send_code |
| `/api/v1/auth/login` | publicMux → /login |
| `/api/v1/auth/register` | publicMux → /register |
| `/api/v1/profile/info` | userMux → /info |
| `/api/v1/profile/list` | userMux → /list |
| `/api/v1/profile/update` | userMux → /update |

---

### 5. 依赖结构体

```go
type HGRootHandlerDeps struct {
    RedisService *PersistenceRedisPackage.RedisService
    SQLManager   *PersistenceSQLPackage.HGSQLManager
    SMSSender    HGSMSPackage.HGSender
}
```

---

### 6. 请求链路

```
Request
  ↓
RequestTIDInterceptor
  ↓
AccessLogInterceptor
  ↓
RecoverInterceptor
  ↓
JSONHeaderInterceptor
  ↓
APIGuardInterceptor   ← Method / Auth / Permission / Version
  ↓
(User 模块额外) JWT AuthInterceptor
  ↓
Handler
  ↓
Service
```


<br/><br/><br/>

***
<br/>

> <h1 id="路由注册机制">路由注册机制</h1>

***
<br/><br/><br/>
> <h2 id="bindRouteSpecs函数解释">bindRouteSpecs函数解释</h2>

**文件位置：** `internal/interfaces/middleware/middleware_group/hg_route_internal.go`

**功能：** 批量注册路由，遍历路由规格列表，将每个子路由注册到 `http.ServeMux`。

---

### 1. 函数代码

```go
func bindRouteSpecs(mux *http.ServeMux, specs []hgRouteSpec) {
    for _, route := range specs {
        if route.Handler == nil {
            continue
        }
        mux.HandleFunc(route.SubPath, route.Handler)  // 注册路由
    }
}
```

---

### 2. 作用

**批量注册路由**：遍历路由规格列表，将每个子路由注册到 `http.ServeMux`。

---

### 3. 举例

```go
// 1. 定义路由规格列表
specs := []hgRouteSpec{
    newRouteSpec("profile", http.MethodGet, "/api/v1/profile", "/info", true, "获取用户信息", userHandler.Profile),
    newRouteSpec("profile", http.MethodGet, "/api/v1/profile", "/list", true, "获取用户列表", userHandler.GetUserList),
    newRouteSpec("profile", http.MethodPut, "/api/v1/profile", "/update", true, "更新用户资料", userHandler.UpdateProfile),
}

// 2. 调用 bindRouteSpecs 批量注册
mux := http.NewServeMux()
bindRouteSpecs(mux, specs)

// 等价于：
mux.HandleFunc("/info", userHandler.Profile)
mux.HandleFunc("/list", userHandler.GetUserList)
mux.HandleFunc("/update", userHandler.UpdateProfile)
```

---

### 4. 配合 `newRouteSpec` 使用

```go
// newRouteSpec 构建路由元信息
newRouteSpec(
    "profile",              // group: 分组名
    http.MethodGet,         // method: HTTP 方法
    "/api/v1/profile",      // basePath: 完整前缀
    "/info",                // subPath: 子路径
    true,                   // needAuth: 是否需要认证
    "获取用户信息",           // summary: 描述
    userHandler.Profile,    // handler: 处理函数
)

// 生成的 hgRouteSpec 结构体：
{
    Group:    "profile",
    Method:   "GET",
    SubPath:  "/info",                    // ← 用于注册到子路由
    FullPath: "/api/v1/profile/info",     // ← 用于对外展示
    NeedAuth: true,
    Summary:  "获取用户信息",
    Handler:  userHandler.Profile,
}
```

---

### 5. 核心价值

| 作用 | 说明 |
|------|------|
| **批量注册** | 一次调用注册多个路由，避免重复代码 |
| **子路径分离** | 子路由只关心 `/info`，完整路径 `/api/v1/profile/info` 用于对外展示 |
| **路由清单** | 通过 `buildRouteCatalogItems` 生成 API 文档 |


<br/><br/><br/>

***
<br/>

> <h1 id="API规则配置">API规则配置</h1>

***
<br/><br/><br/>
> <h2 id="PublicAPIRules配置">PublicAPIRules配置</h2>

**文件位置：** `server/hg_router.go`

**功能：** 定义公开接口的 API 规则（不需要认证的接口）。

---

### 1. 函数代码

```go
func PublicAPIRules() []HGMiddlewarePackage.HGAPIRule {
    return []HGMiddlewarePackage.HGAPIRule{
        {
            Path:    "/send_code",           // 路由路径
            Version: "v1",                   // API 版本
            Methods: map[string]bool{        // 只允许 GET
                http.MethodGet: true,
            },
            NeedAuth: false,                 // 不需要认证
        },
        {
            Path:    "/register",
            Version: "v1",
            Methods: map[string]bool{        // 只允许 POST
                http.MethodPost: true,
            },
            NeedAuth: false,
        },
        {
            Path:    "/login",
            Version: "v1",
            Methods: map[string]bool{        // 只允许 POST
                http.MethodPost: true,
            },
            NeedAuth: false,
        },
    }
}
```

---

### 2. 配置对照表

| Path | Methods | NeedAuth | 说明 |
|------|---------|----------|------|
| `/send_code` | GET | false | 发送验证码（公开接口） |
| `/register` | POST | false | 注册（公开接口） |
| `/login` | POST | false | 登录（公开接口） |

---

### 3. 使用方式

```go
// 在 hg_auth_route_interceptor_group.go 中
guarded := HGMiddlewarePackage.APIGuardInterceptor(HGServerPackage.PublicAPIRules())(publicMux)
```


<br/><br/><br/>

***
<br/>

> <h2 id="HGAPIRule使用说明">HGAPIRule使用说明</h2>

**文件位置：** `internal/interfaces/middleware/hg_api_guard_interceptor.go`

---

### 1. 结构体定义

```go
type HGAPIRule struct {
    Path        string            // 路由路径
    Methods     map[string]bool   // 允许的 HTTP 方法
    NeedAuth    bool              // 是否需要认证
    Permissions []string          // 需要的权限列表
    Version     string            // API 版本
}
```

---

### 2. 字段说明

| 字段 | 类型 | 说明 | 示例 |
|------|------|------|------|
| Path | string | 子路由路径 | `/info`, `/login` |
| Methods | map[string]bool | 允许的 HTTP 方法 | `{GET: true, POST: true}` |
| NeedAuth | bool | 是否需要 JWT 认证 | `true` / `false` |
| Permissions | []string | 需要的权限列表 | `["user:view", "user:update"]` |
| Version | string | API 版本 | `"v1"` |

---

### 3. 三种 Rules 配置对比

| 函数 | 用途 | NeedAuth | 有 Permissions |
|------|------|----------|----------------|
| `PublicAPIRules()` | 公开接口（登录、注册） | false | 无 |
| `UserMethodRules()` | 用户模块接口（需登录） | true | 部分有 |
| `MethdRules()` | 仅校验 HTTP 方法 | 不涉及 | 无 |

---

### 4. 完整配置示例

```go
// 需要认证 + 权限的接口
{
    Path:    "/info",
    Version: "v1",
    Methods: map[string]bool{
        http.MethodGet: true,
    },
    NeedAuth: true,
    Permissions: []string{
        "user:view",
    },
}

// 需要认证但无权限要求的接口
{
    Path:    "/update",
    Version: "v1",
    Methods: map[string]bool{
        http.MethodPut: true,
    },
    NeedAuth: true,
}

// 公开接口（无需认证）
{
    Path:    "/login",
    Version: "v1",
    Methods: map[string]bool{
        http.MethodPost: true,
    },
    NeedAuth: false,
}
```

---

### 5. 拦截器校验流程

```
请求进入
    ↓
1. Method 校验 → 方法不允许返回 405
    ↓
2. Header 校验 → 缺少必要 Header 返回 400
    ↓
3. Auth 校验 → NeedAuth=true 时检查 JWT
    ↓
4. Permission 校验 → 检查 Permissions 列表
    ↓
5. 进入 Handler
```


<br/><br/><br/>

***
<br/>

> <h2 id="Permissions权限配置">Permissions权限配置</h2>

**文件位置：** `internal/interfaces/middleware/hg_api_guard_interceptor.go`

**功能：** 配置接口需要的权限，实现基于角色的访问控制（RBAC）。

---

### 1. 配置方式

```go
// 在 server/hg_router.go 中配置
func UserMethodRules() []HGMiddlewarePackage.HGAPIRule {
    return []HGMiddlewarePackage.HGAPIRule{
        {
            Path:    "/info",
            Version: "v1",
            Methods: map[string]bool{
                http.MethodGet: true,
            },
            NeedAuth: true,
            Permissions: []string{      // ← 配置需要的权限
                "user:view",
            },
        },
        {
            Path:    "/update",
            Version: "v1",
            Methods: map[string]bool{
                http.MethodPut: true,
            },
            NeedAuth: true,
            Permissions: []string{      // ← 配置多个权限（满足任一即可）
                "user:update",
                "admin:update",
            },
        },
    }
}
```

---

### 2. 角色权限映射表

```go
// 在 hg_api_guard_interceptor.go 中定义
var rolePermissions = map[string]map[string]bool{
    "admin": {
        "user:update": true,    // admin 角色有 user:update 权限
        "user:view":   true,    // admin 角色有 user:view 权限
    },
    "user": {
        "user:view": true,      // user 角色只有 user:view 权限
    },
}
```

---

### 3. 权限校验逻辑

```go
func HasPermission(role string, perms []string) bool {
    rolePerms := rolePermissions[role]  // 获取角色的权限列表
    for _, p := range perms {           // 遍历需要的权限
        if rolePerms[p] {               // 只要满足一个权限即可
            return true
        }
    }
    return false
}
```

---

### 4. 完整校验流程

```
请求进入
    ↓
PermissionInterceptor 获取 JWT claims 中的 role
    ↓
查找 rule.Permissions（如 ["user:view"]）
    ↓
调用 HasPermission("user", ["user:view"])
    ↓
rolePermissions["user"]["user:view"] → true
    ↓
校验通过，进入 Handler
```

---

### 5. 配置示例对照表

| 场景 | Permissions 配置 | 说明 |
|------|------------------|------|
| 仅查看 | `["user:view"]` | 需要查看权限 |
| 仅编辑 | `["user:update"]` | 需要编辑权限 |
| 查看或编辑 | `["user:view", "user:update"]` | 满足任一即可 |
| 管理员 | `["admin:manage"]` | 需要管理员权限 |
| 无权限要求 | `[]` 或不配置 | 只需要认证 |

---

### 6. 添加新权限步骤

**步骤 1：在角色权限映射表中添加**

```go
var rolePermissions = map[string]map[string]bool{
    "admin": {
        "user:update": true,
        "user:view":   true,
        "user:delete": true,    // ← 新增
    },
    "user": {
        "user:view": true,
    },
    "editor": {                 // ← 新增角色
        "user:view":   true,
        "user:update": true,
    },
}
```

**步骤 2：在 API 规则中配置**

```go
{
    Path:    "/delete",
    Version: "v1",
    Methods: map[string]bool{
        http.MethodDelete: true,
    },
    NeedAuth: true,
    Permissions: []string{
        "user:delete",          // ← 使用新权限
    },
}
```

**步骤 3：在登录时设置角色**

```go
claims := &UserServicePackage.HGClaims{
    UserID:  userID,
    Role:    "editor",          // ← 设置用户角色
    // ...
}
```


<br/><br/><br/>

***
<br/>

> <h1 id="Go基础语法">Go基础语法</h1>

***
<br/><br/><br/>
> <h2 id="切片和数组的区别">切片和数组的区别</h2>

**核心区别：长度是否固定**

---

### 1. 对比表

| 特性 | 数组 (Array) | 切片 (Slice) |
|------|--------------|--------------|
| 声明语法 | `[3]int` | `[]int` |
| 长度 | **固定**，编译时确定 | **动态**，运行时可变 |
| 是否可追加 | ❌ 不可以 | ✅ 可以用 `append` |

---

### 2. 最简单的区分方法：**看有没有数字**

```go
// 数组：有数字 [3]
var arr [3]int = [3]int{1, 2, 3}

// 切片：没数字 []
var s []int = []int{1, 2, 3}
```

---

### 3. 对比示例

```go
// ❌ 数组：长度固定为 3
arr := [3]int{1, 2, 3}
arr = append(arr, 4)  // 编译错误！数组没有 append

// ✅ 切片：长度动态
s := []int{1, 2, 3}
s = append(s, 4)      // 正确！切片可以追加
fmt.Println(s)         // [1 2 3 4]
```

---

### 4. 常见声明方式对比

```go
// 数组（必须指定长度）
a1 := [3]int{1, 2, 3}
a2 := [...]int{1, 2, 3}  // 自动推断长度，但仍是数组

// 切片（不指定长度）
s1 := []int{1, 2, 3}
s2 := make([]int, 3)      // 长度为 3 的切片
s3 := make([]int, 0, 10)  // 长度为 0，容量为 10 的切片
```

---

### 5. 一句话记忆

```
[3]int  → 数组（有数字，固定长度）
[]int   → 切片（没数字，动态长度）
```

---

### 6. 实际项目中

**99% 用切片**，几乎不用数组：

```go
// 项目中常见的切片用法
users := []User{}                    // 空切片
ids := make([]int, 0, 100)          // 预分配容量
result := append(result, newItem)   // 追加元素
```


<br/><br/><br/>

***
<br/>

> <h1 id="结构体实例化">结构体实例化</h1>

***
<br/><br/><br/>
> <h2 id="HGAPIGuard实例化-map容器"> HGAPIGuard实例化-map容器</h2>

**文件位置：** `internal/interfaces/middleware/hg_api_guard_interceptor.go`

**功能：** 存储和管理 API 路由规则，支持按版本查询。

---

### 1. 结构体定义

```go
type HGAPIGuard struct {
    rulesByVersion map[string]map[string]HGAPIRule  // 嵌套 map：版本 → 路径 → 规则
    legacyRules    map[string]HGAPIRule              // 旧版规则：路径 → 规则
}
```

---

### 2. 实例化方式

```go
guard := &HGAPIGuard{
    rulesByVersion: make(map[string]map[string]HGAPIRule),
    legacyRules:    make(map[string]HGAPIRule),
}
```

---

### 3. 为什么用 `make`？

| 字段 | 类型 | 为什么需要 make |
|------|------|----------------|
| `rulesByVersion` | `map[string]map[string]HGAPIRule` | 嵌套 map，需要初始化外层和内层 |
| `legacyRules` | `map[string]HGAPIRule` | 普通 map，需要初始化才能使用 |

---

### 4. 完整初始化示例

```go
// 1. 创建实例
guard := &HGAPIGuard{
    rulesByVersion: make(map[string]map[string]HGAPIRule),
    legacyRules:    make(map[string]HGAPIRule),
}

// 2. 初始化内层 map
guard.rulesByVersion["v1"] = make(map[string]HGAPIRule)

// 3. 添加规则
guard.rulesByVersion["v1"]["/info"] = HGAPIRule{
    Path:    "/info",
    Version: "v1",
    Methods: map[string]bool{http.MethodGet: true},
    NeedAuth: true,
}

guard.legacyRules["/info"] = HGAPIRule{
    Path:    "/info",
    Methods: map[string]bool{http.MethodGet: true},
}
```

---

### 5. `NewAPIGuard` 函数（推荐方式）

```go
func NewAPIGuard(rules []HGAPIRule) *HGAPIGuard {
    guard := &HGAPIGuard{
        rulesByVersion: make(map[string]map[string]HGAPIRule),
        legacyRules:    make(map[string]HGAPIRule),
    }

    for _, r := range rules {
        version := strings.TrimSpace(r.Version)
        if version == "" {
            version = defaultAPIVersion  // 默认 "v1"
        }

        // 初始化内层 map（如果不存在）
        if _, ok := guard.rulesByVersion[version]; !ok {
            guard.rulesByVersion[version] = make(map[string]HGAPIRule)
        }

        // 存储规则
        guard.rulesByVersion[version][r.Path] = r
        guard.legacyRules[r.Path] = r
    }

    return guard
}
```

---

### 6. 使用示例

```go
// 定义规则
rules := []HGAPIRule{
    {Path: "/info", Version: "v1", Methods: map[string]bool{"GET": true}, NeedAuth: true},
    {Path: "/login", Version: "v1", Methods: map[string]bool{"POST": true}, NeedAuth: false},
}

// 创建 guard
guard := NewAPIGuard(rules)

// 查询规则
rule, ok := guard.lookupRule("v1", "/info")  // 找到规则
rule, ok := guard.lookupRule("v1", "/xxx")   // 未找到
```

---

### 7. 内存结构图

```
guard.rulesByVersion:
{
    "v1": {
        "/info":  {Path: "/info", NeedAuth: true, ...},
        "/login": {Path: "/login", NeedAuth: false, ...},
    },
    "v2": {
        "/info":  {Path: "/info", NeedAuth: true, ...},
    }
}

guard.legacyRules:
{
    "/info":  {Path: "/info", NeedAuth: true, ...},
    "/login": {Path: "/login", NeedAuth: false, ...},
}
```


<br/><br/><br/>

***
<br/>

> <h1 id="拦截器机制">拦截器机制</h1>

***
<br/><br/><br/>
> <h2 id="Interceptor方法解读">Interceptor方法解读</h2>

**文件位置：** `internal/interfaces/middleware/hg_api_guard_interceptor.go`

**功能：** 创建拦截器，执行 Method/Header/Auth 统一拦截。

---

### 1. 方法代码

```go
func (g *HGAPIGuard) Interceptor() HGHTTPInterceptor {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 1. 获取 API 版本
            version := strings.TrimSpace(r.Header.Get("X-API-Version"))
            if version == "" {
                version = defaultAPIVersion
            }

            // 2. 查找路由规则
            rule, ok := g.lookupRule(version, r.URL.Path)
            if !ok {
                http.NotFound(w, r)
                return
            }

            // 3. Method 校验
            if !rule.Methods[r.Method] {
                w.WriteHeader(http.StatusMethodNotAllowed)
                return
            }

            // 4. Header 校验
            ctx := g.checkoutHeader(w, r, rule.NeedAuth)
            if ctx == nil {
                return
            }

            // 5. 传递给下一个处理器
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

---

### 2. 返回类型说明

```go
// HGHTTPInterceptor 类型定义
type HGHTTPInterceptor func(http.Handler) http.Handler

// Interceptor() 返回的是一个函数
// 这个函数接受 http.Handler，返回 http.Handler
```

---

### 3. 执行流程

```
Interceptor() 被调用
    ↓
返回 func(next http.Handler) http.Handler
    ↓
当这个函数被调用时（如 APIGuardInterceptor）
    ↓
返回 http.HandlerFunc(...)
    ↓
当请求到来时，调用 ServeHTTP
    ↓
执行内部的校验逻辑
```


<br/><br/><br/>

***
<br/>

> <h2 id="http.HandlerFunc与http.Handler的关系">http.HandlerFunc与http.Handler的关系</h2>

**核心问题：为什么 `http.HandlerFunc` 就是 `http.Handler`？**

---

### 1. http.Handler 接口定义

```go
type Handler interface {
    ServeHTTP(ResponseWriter, *Request)
}
```

---

### 2. http.HandlerFunc 类型定义

```go
type HandlerFunc func(ResponseWriter, *Request)
```

---

### 3. HandlerFunc 实现了 Handler 接口

```go
func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) {
    f(w, r)  // 调用自身
}
```

---

### 4. 类比理解

```go
// 接口
type Animal interface {
    Speak()
}

// 类型
type Dog func()

// Dog 实现了 Animal 接口
func (d Dog) Speak() {
    d()
}

// 所以 Dog 类型就是 Animal
var a Animal = Dog(func() { fmt.Println("汪汪") })
```

---

### 5. 完整类型推导

```go
// Interceptor() 返回类型是 HGHTTPInterceptor
type HGHTTPInterceptor func(http.Handler) http.Handler

// 返回的是一个函数：func(next http.Handler) http.Handler
// 里面的 return 是：http.HandlerFunc(func(...){...})

// http.HandlerFunc(func(...){...}) 是什么？
// → 是 HandlerFunc 类型的值
// → HandlerFunc 实现了 http.Handler 接口
// → 所以它就是 http.Handler
```

---

### 6. 一句话总结

```
http.HandlerFunc 是一个类型，它实现了 http.Handler 接口
所以 http.HandlerFunc 类型的值可以直接作为 http.Handler 使用
```


<br/><br/><br/>

***
<br/>

> <h1 id="HTTP请求获取方法">HTTP请求获取方法</h1>

***
<br/><br/><br/>
> <h2 id="r.Header.Get详解">r.Header.Get详解</h2>

**功能：** 获取 HTTP 请求头中的值。

---

### 1. 请求头示例

```
GET /api/v1/profile/info HTTP/1.1
Host: localhost:8080
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
Content-Type: application/json
X-API-Version: v1
X-Device-ID: abc123
X-Client-Type: ios
X-Client-Version: 1.0.0
X-Language: zh-CN
X-Request-ID: req-123
X-Timestamp: 1700000000
X-Signature: sha256=8f42a...
```

---

### 2. 获取方式

```go
token := r.Header.Get("Authorization")      // "Bearer eyJhbGciOiJIUzI1NiIs..."
contentType := r.Header.Get("Content-Type")  // "application/json"
version := r.Header.Get("X-API-Version")     // "v1"
deviceID := r.Header.Get("X-Device-ID")      // "abc123"
clientType := r.Header.Get("X-Client-Type")  // "ios"
clientVersion := r.Header.Get("X-Client-Version") // "1.0.0"
language := r.Header.Get("X-Language")       // "zh-CN"
requestID := r.Header.Get("X-Request-ID")    // "req-123"
timestamp := r.Header.Get("X-Timestamp")     // "1700000000"
signature := r.Header.Get("X-Signature")     // "sha256=8f42a..."
```

---

### 3. 常用 Header 对照表

| Header | 说明 | 示例值 |
|--------|------|--------|
| `Authorization` | Token | `"Bearer eyJhbG..."` |
| `Content-Type` | 内容类型 | `"application/json"` |
| `X-API-Version` | API 版本 | `"v1"` |
| `X-Device-ID` | 设备 ID | `"abc123"` |
| `X-Client-Type` | 客户端类型 | `"ios"` / `"android"` |
| `X-Client-Version` | 客户端版本 | `"1.0.0"` |
| `X-Language` | 语言 | `"zh-CN"` |
| `X-Request-ID` | 请求 ID | `"req-123"` |
| `X-Timestamp` | 时间戳 | `"1700000000"` |
| `X-Signature` | 签名 | `"sha256=8f42a..."` |


<br/><br/><br/>

***
<br/>

> <h2 id="r.URL.Path详解">r.URL.Path详解</h2>

**功能：** 获取请求的路径部分。

---

### 1. 示例

```go
// 请求: GET /api/v1/profile/info
r.URL.Path  // "/api/v1/profile/info"

// 请求: POST /api/v1/auth/login
r.URL.Path  // "/api/v1/auth/login"

// 请求: PUT /api/v1/profile/update
r.URL.Path  // "/api/v1/profile/update"
```

---

### 2. 其他 URL 相关方法

```go
// 请求: GET /api/v1/users?page=1&size=10

r.URL.Path       // "/api/v1/users"
r.URL.RawQuery   // "page=1&size=10"

// 获取查询参数
r.URL.Query().Get("page")   // "1"
r.URL.Query().Get("size")   // "10"
```

---

### 3. 其他常用的 r 方法

```go
r.Method           // "GET" / "POST" / "PUT" / "DELETE"
r.Host             // "localhost:8080"
r.RemoteAddr       // "127.0.0.1:54321"
r.RequestURI       // "/api/v1/users?page=1"
```

---

### 4. 读取请求体

```go
// POST 请求的 Body
var req LoginRequest
json.NewDecoder(r.Body).Decode(&req)
```


<br/><br/><br/>

***
<br/>

> <h1 id="Go语法特性">Go语法特性</h1>

***
<br/><br/><br/>
> <h2 id="comma-ok模式详解">comma-ok模式详解</h2>

**功能：** 从 map、type assertion、channel 中获取值时，同时判断是否存在/成功。

---

### 1. 核心概念

```go
// 从 map 中获取值，返回两个值
value, ok := m[key]
//   ↑      ↑
//  值    是否存在
```

---

### 2. 示例

```go
m := map[string]int{
    "a": 1,
    "b": 2,
}

// 返回两个值
v1, ok1 := m["a"]  // v1=1, ok1=true
v2, ok2 := m["c"]  // v2=0, ok2=false（不存在）

// 只返回一个值（忽略 ok）
v3 := m["a"]       // v3=1
v4 := m["c"]       // v4=0（不存在时返回零值）
```

---

### 3. 为什么需要 ok？

```go
// 问题：值为 0 时，无法区分是"不存在"还是"值就是 0"
m := map[string]int{"a": 0}
v := m["a"]  // v=0，但不知道是"不存在"还是"值是 0"

// 解决：用 ok 判断
v, ok := m["a"]  // v=0, ok=true → 存在，值是 0
v, ok := m["b"]  // v=0, ok=false → 不存在
```

---

### 4. 哪些类型支持 "comma ok"？

| 类型 | 示例 | 返回值 |
|------|------|--------|
| **map** | `v, ok := m[key]` | 值 + 是否存在 |
| **type assertion** | `v, ok := x.(Type)` | 值 + 是否是该类型 |
| **channel** | `v, ok := <-ch` | 值 + channel 是否关闭 |

---

### 5. map 的 "comma ok"

```go
m := map[string]int{"a": 1, "b": 2}

// 两个值
v, ok := m["a"]
if ok {
    fmt.Println("存在:", v)
} else {
    fmt.Println("不存在")
}

// 一个值（忽略 ok）
v := m["a"]  // 不存在时返回零值
```

---

### 6. type assertion 的 "comma ok"

```go
var x interface{} = "hello"

// 两个值
s, ok := x.(string)
if ok {
    fmt.Println("是字符串:", s)
} else {
    fmt.Println("不是字符串")
}

// 一个值（不是该类型时会 panic）
s := x.(string)  // 如果 x 不是 string，会 panic
```

---

### 7. channel 的 "comma ok"

```go
ch := make(chan int, 1)
ch <- 1
close(ch)

// 两个值
v, ok := <-ch
if ok {
    fmt.Println("收到:", v)
} else {
    fmt.Println("channel 已关闭")
}

// 一个值（channel 关闭时返回零值）
v := <-ch  // channel 关闭时返回零值
```

---

### 8. lookupRule 中的例子

```go
func (g *HGAPIGuard) lookupRule(version string, path string) (HGAPIRule, bool) {
    routesByPath, ok := g.rulesByVersion[version]
    //      ↑              ↑
    //     值         是否存在这个版本
    
    if !ok {
        // 版本不存在，尝试回退到默认版本
        return HGAPIRule{}, false
    }
    
    rule, ok := routesByPath[path]
    //  ↑              ↑
    // 值         是否存在这个路径
    
    return rule, ok
}
```

---

### 9. 返回值模式总结

| 模式 | 示例 | 说明 |
|------|------|------|
| 只返回值 | `v := m[key]` | 不存在时返回零值 |
| 值 + ok | `v, ok := m[key]` | ok 判断是否存在 |
| 只返回成功/失败 | `_, ok := m[key]` | 忽略值，只判断存在 |


<br/><br/><br/>

***
<br/>

> <h1 id="HTTP响应方法">HTTP响应方法</h1>

***
<br/><br/><br/>
> <h2 id="常用响应方法对照表">常用响应方法对照表</h2>

**功能：** HTTP 响应的各种方式，包括错误处理和成功响应。

---

### 1. 常用响应方法对照表

| 方法 | 状态码 | 说明 | 前端收到的数据 |
|------|--------|------|----------------|
| `http.NotFound(w, r)` | 404 | 资源不存在 | `404 page not found` |
| `w.WriteHeader(405)` | 405 | 方法不允许 | 空 body |
| `w.WriteHeader(400)` | 400 | 请求参数错误 | 空 body |
| `w.WriteHeader(401)` | 401 | 未授权 | 空 body |
| `w.WriteHeader(403)` | 403 | 禁止访问 | 空 body |
| `w.WriteHeader(500)` | 500 | 服务器内部错误 | 空 body |
| `w.Write([]byte)` | 200 | 成功响应 | 原始字节 |
| `json.NewEncoder(w).Encode(data)` | 200 | JSON 响应 | JSON 数据 |

---

### 2. 详细示例

#### 2.1 `http.NotFound(w, r)` - 404

```go
// 代码
http.NotFound(w, r)

// 前端收到
// HTTP/1.1 404 Not Found
// Content-Type: text/plain

// 响应体: "404 page not found"
```

**前端处理：**
```javascript
fetch('/api/v1/xxx')
  .then(res => {
    if (res.status === 404) {
      console.log('资源不存在')
    }
  })
```

---

#### 2.2 `w.WriteHeader(statusCode)` - 设置状态码

```go
// 代码
w.WriteHeader(http.StatusMethodNotAllowed)  // 405
w.WriteHeader(http.StatusBadRequest)        // 400
w.WriteHeader(http.StatusUnauthorized)       // 401
w.WriteHeader(http.StatusForbidden)          // 403
w.WriteHeader(http.StatusInternalServerError) // 500

// 前端收到
// HTTP/1.1 405 Method Not Allowed
// Content-Type: text/plain

// 响应体: 空
```

**前端处理：**
```javascript
fetch('/api/v1/profile/info', { method: 'POST' })
  .then(res => {
    if (res.status === 405) {
      console.log('方法不允许，应该用 GET')
    }
  })
```

---

#### 2.3 `w.Write([]byte)` - 写入响应体

```go
// 代码
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(200)
w.Write([]byte(`{"code":0,"message":"success","data":{}}`))

// 前端收到
// HTTP/1.1 200 OK
// Content-Type: application/json

// 响应体: {"code":0,"message":"success","data":{}}
```

**前端处理：**
```javascript
fetch('/api/v1/profile/info')
  .then(res => res.json())
  .then(data => {
    console.log(data)  // {code: 0, message: "success", data: {}}
  })
```

---

#### 2.4 `json.NewEncoder(w).Encode(data)` - JSON 响应

```go
// 代码
type Response struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data"`
}

resp := Response{
    Code:    0,
    Message: "success",
    Data: map[string]string{
        "user_id": "123",
        "name":    "张三",
    },
}

w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(resp)

// 前端收到
// HTTP/1.1 200 OK
// Content-Type: application/json

// 响应体: {"code":0,"message":"success","data":{"user_id":"123","name":"张三"}}
```

---

#### 2.5 `next.ServeHTTP(w, r.WithContext(ctx))` - 传递给下一个处理器

```go
// 这不是响应给前端，而是传递给下一个中间件或处理器
ctx := context.WithValue(r.Context(), "userID", "123")
next.ServeHTTP(w, r.WithContext(ctx))

// 下一个处理器可以获取这个值
userID := r.Context().Value("userID")  // "123"
```


<br/><br/><br/>

***
<br/>

> <h2 id="前端收到的数据格式">前端收到的数据格式</h2>

---

### 1. 状态码对照表

| 状态码 | 前端收到的 res.status | 响应体 |
|--------|----------------------|--------|
| 200 | 200 | JSON 数据 |
| 400 | 400 | 错误信息 |
| 401 | 401 | "未授权" |
| 403 | 403 | "禁止访问" |
| 404 | 404 | "404 page not found" |
| 405 | 405 | 空 |
| 500 | 500 | "Internal Server Error" |

---

### 2. 完整响应模式示例

```go
// 成功响应
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(200)
json.NewEncoder(w).Encode(map[string]interface{}{
    "code":    0,
    "message": "success",
    "data":    data,
})

// 错误响应
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(400)
json.NewEncoder(w).Encode(map[string]interface{}{
    "code":    1001,
    "message": "参数错误",
    "data":    nil,
})
```

---

### 3. 前端处理示例

```javascript
fetch('/api/v1/profile/info')
  .then(res => {
    if (res.ok) {
      return res.json()
    }
    throw new Error(`HTTP ${res.status}`)
  })
  .then(data => {
    console.log(data)  // {code: 0, message: "success", data: {...}}
  })
  .catch(err => {
    console.error(err)  // Error: HTTP 404
  })
```


<br/><br/><br/>

***
<br/>

> <h1 id="Go标准库">Go标准库</h1>

***
<br/><br/><br/>
> <h2 id="sort.Slice排序详解">sort.Slice排序详解</h2>

**文件位置：** `internal/handler/hg_root_handler.go`

**功能：** 对切片进行自定义排序。

---

### 1. 函数代码

```go
sort.Slice(items, func(i, j int) bool {
    if items[i].Path == items[j].Path {
        return items[i].Method < items[j].Method
    }
    return items[i].Path < items[j].Path
})
```

---

### 2. `sort.Slice` 是什么？

`sort.Slice` 是 Go 标准库的排序函数：
- **参数 1**：要排序的切片
- **参数 2**：比较函数，返回 `true` 表示 `i` 应该排在 `j` 前面

---

### 3. 比较函数解析

```go
func(i, j int) bool {
    // 第一步：先按 Path 排序
    if items[i].Path == items[j].Path {
        // 第二步：Path 相同时，按 Method 排序
        return items[i].Method < items[j].Method
    }
    // 默认：按 Path 字母顺序排序
    return items[i].Path < items[j].Path
}
```

---

### 4. 排序规则

| 优先级 | 排序字段 | 顺序 |
|--------|----------|------|
| 1 | Path | 字母升序 (A → Z) |
| 2 | Method | 字母升序 (DELETE → GET → POST → PUT) |

---

### 5. 排序前后对比

**排序前：**
```
POST   /api/v1/auth/login
GET    /api/v1/profile/info
GET    /api/v1/auth/send_code
PUT    /api/v1/profile/update
POST   /api/v1/auth/register
GET    /api/v1/profile/list
```

**排序后：**
```
POST   /api/v1/auth/login
POST   /api/v1/auth/register
GET    /api/v1/auth/send_code
GET    /api/v1/profile/info
GET    /api/v1/profile/list
PUT    /api/v1/profile/update
```

---

### 6. 一句话总结

```
先按路径字母排序，路径相同时按 HTTP 方法字母排序
```


<br/><br/><br/>

***
<br/>

> <h1 id="路由分组机制">路由分组机制</h1>

***
<br/><br/><br/>
> <h2 id="buildRouteCatalogGrouped详解">buildRouteCatalogGrouped详解</h2>

**文件位置：** `internal/handler/hg_root_handler.go`

**功能：** 把扁平的路由列表按 Group 字段分成多个组，每组内部再按 Path 排序。

---

### 1. 函数代码

```go
func buildRouteCatalogGrouped(catalog []HGMiddlewareGroupPackage.HGRouteCatalogItem) map[string][]HGMiddlewareGroupPackage.HGRouteCatalogItem {
    // 1️⃣ 创建分组 map
    grouped := make(map[string][]HGMiddlewareGroupPackage.HGRouteCatalogItem, 8)
    
    // 2️⃣ 按 Group 字段分组
    for _, item := range catalog {
        grouped[item.Group] = append(grouped[item.Group], item)
    }

    // 3️⃣ 每组内部排序
    for group := range grouped {
        routes := grouped[group]
        sort.Slice(routes, func(i, j int) bool {
            if routes[i].Path == routes[j].Path {
                return routes[i].Method < routes[j].Method
            }
            return routes[i].Path < routes[j].Path
        })
        grouped[group] = routes
    }

    return grouped
}
```

---

### 2. 输入和输出

**输入**：扁平的路由列表
```go
[]HGRouteCatalogItem{
    {Group: "auth",    Method: "POST", Path: "/api/v1/auth/login"},
    {Group: "auth",    Method: "GET",  Path: "/api/v1/auth/send_code"},
    {Group: "profile", Method: "GET",  Path: "/api/v1/profile/info"},
    {Group: "profile", Method: "PUT",  Path: "/api/v1/profile/update"},
    {Group: "meta",    Method: "GET",  Path: "/api/v1/routes"},
}
```

**输出**：按 Group 分组的 map
```go
map[string][]HGRouteCatalogItem{
    "auth": [
        {Group: "auth", Method: "POST", Path: "/api/v1/auth/login"},
        {Group: "auth", Method: "GET",  Path: "/api/v1/auth/send_code"},
    ],
    "profile": [
        {Group: "profile", Method: "GET",  Path: "/api/v1/profile/info"},
        {Group: "profile", Method: "PUT",  Path: "/api/v1/profile/update"},
    ],
    "meta": [
        {Group: "meta", Method: "GET", Path: "/api/v1/routes"},
    ],
}
```

---

### 3. 执行流程图

```
输入：扁平路由列表
    ↓
步骤1：创建空 map
    grouped = {}
    ↓
步骤2：遍历列表，按 Group 分组
    grouped["auth"] = [login, send_code]
    grouped["profile"] = [info, update]
    grouped["meta"] = [routes]
    ↓
步骤3：每组内部排序
    auth 组：按 Path 排序
    profile 组：按 Path 排序
    meta 组：按 Path 排序
    ↓
输出：分组后的 map
```

---

### 4. 为什么要分组？

| 场景 | 用途 |
|------|------|
| 前端展示 | 按模块展示 API 列表（auth 模块、profile 模块） |
| 文档生成 | 自动生成分组的 API 文档 |
| 权限管理 | 按模块配置权限 |

---

### 5. 前端收到的数据格式

```json
{
  "auth": [
    {"method": "POST", "path": "/api/v1/auth/login", "summary": "登录"},
    {"method": "GET", "path": "/api/v1/auth/send_code", "summary": "发送验证码"}
  ],
  "profile": [
    {"method": "GET", "path": "/api/v1/profile/info", "summary": "获取用户信息"},
    {"method": "PUT", "path": "/api/v1/profile/update", "summary": "更新用户资料"}
  ],
  "meta": [
    {"method": "GET", "path": "/api/v1/routes", "summary": "查看完整 API 路由清单"}
  ]
}
```

---

### 6. 一句话总结

```
把扁平的路由列表按 Group 字段分成多个组，每组内部再按 Path 排序
```


<br/><br/><br/>

***
<br/>

> <h1 id="NewRootHandler详解">NewRootHandler详解</h1>

**文件位置：** `internal/handler/hg_root_handler.go`

**功能：** 构建根路由，挂载各模块路由，注册路由清单接口。

---

### 1. 函数调用关系图

```
NewRootHandler(deps)
    │
    ├── registerRootPrefixRoutes()   ← 挂载模块路由
    │
    ├── buildRouteCatalog()          ← 构建路由清单
    │
    ├── buildRouteCatalogGrouped()   ← 按模块分组
    │
    ├── rootMux.Handle()             ← 注册路由清单接口
    │   ├── buildRouteCatalogHandler(newRouteCatalogHandler())
    │   └── buildRouteCatalogHandler(newRouteCatalogGroupedHandler())
    │
    └── logRouteCatalog()            ← 启动时打印路由清单
```

<br/><br/><br/>

***
<br/>

> <h2 id="各函数调用关系">各函数调用关系</h2>

---

### 1. `registerRootPrefixRoutes` - 挂载模块路由

```go
registerRootPrefixRoutes(rootMux, []HGRouteMount{
    {Prefix: "/api/v1/auth/", StripPrefix: "/api/v1/auth", Handler: publicHandler},
    {Prefix: "/api/v1/profile/", StripPrefix: "/api/v1/profile", Handler: userHandlerWithAuth},
    {Prefix: "/api/v1/test/", StripPrefix: "/api/v1/test", Handler: testHandler},
})
```

**作用：** 把各模块的 Handler 挂载到根路由，并用 `StripPrefix` 去掉前缀。

**示例：**
```
请求: GET /api/v1/profile/info
    ↓
rootMux 匹配 /api/v1/profile/
    ↓
StripPrefix("/api/v1/profile")
    ↓
子路由收到: /info
```

---

### 2. `buildRouteCatalog` - 构建路由清单

```go
routeCatalog := buildRouteCatalog()
```

**作用：** 汇总所有模块的路由，生成完整的 API 清单。

**返回值：**
```go
[]HGRouteCatalogItem{
    {Group: "auth", Method: "POST", Path: "/api/v1/auth/login", Summary: "登录"},
    {Group: "auth", Method: "GET", Path: "/api/v1/auth/send_code", Summary: "发送验证码"},
    {Group: "profile", Method: "GET", Path: "/api/v1/profile/info", Summary: "获取用户信息"},
    // ...
}
```

---

### 3. `buildRouteCatalogGrouped` - 按模块分组

```go
routeCatalogGrouped := buildRouteCatalogGrouped(routeCatalog)
```

**作用：** 把扁平的路由清单按 Group 分组。

**返回值：**
```go
map[string][]HGRouteCatalogItem{
    "auth": [
        {Method: "POST", Path: "/api/v1/auth/login"},
        {Method: "GET", Path: "/api/v1/auth/send_code"},
    ],
    "profile": [
        {Method: "GET", Path: "/api/v1/profile/info"},
        {Method: "PUT", Path: "/api/v1/profile/update"},
    ],
}
```

---

### 4. `newRouteCatalogHandler` - 创建路由清单处理器

```go
rootMux.Handle("/api/v1/routes", buildRouteCatalogHandler(newRouteCatalogHandler(routeCatalog)))
```

**作用：** 创建一个 HTTP Handler，返回扁平的路由清单。

**前端调用：**
```javascript
fetch('/api/v1/routes')
  .then(res => res.json())
  .then(data => console.log(data))
// 返回: [{method: "GET", path: "/api/v1/profile/info", ...}, ...]
```

---

### 5. `newRouteCatalogGroupedHandler` - 创建分组路由清单处理器

```go
rootMux.Handle("/api/v1/routes/groups", buildRouteCatalogHandler(newRouteCatalogGroupedHandler(routeCatalogGrouped)))
```

**作用：** 创建一个 HTTP Handler，返回按模块分组的路由清单。

**前端调用：**
```javascript
fetch('/api/v1/routes/groups')
  .then(res => res.json())
  .then(data => console.log(data))
// 返回: {auth: [...], profile: [...], meta: [...]}
```

---

### 6. `buildRouteCatalogHandler` - 添加中间件

```go
func buildRouteCatalogHandler(core http.Handler) http.Handler {
    return HGMiddlewarePackage.ChainInterceptors(
        core,
        HGMiddlewarePackage.RecoverInterceptor,    // panic 恢复
        HGMiddlewarePackage.AccessLogInterceptor,  // 访问日志
        HGMiddlewarePackage.RequestTIDInterceptor, // 请求追踪ID
        HGMiddlewarePackage.JSONHeaderInterceptor, // JSON 响应头
    )
}
```

**作用：** 给路由清单接口添加通用中间件（日志、恢复、追踪等）。

---

### 7. `logRouteCatalog` - 打印路由清单

```go
logRouteCatalog(routeCatalog)
```

**作用：** 服务启动时打印所有路由，方便开发调试。

**输出示例：**
```
API 路由清单如下（完整可调用路径）：
[API] POST /api/v1/auth/login auth=false group=auth summary=登录
[API] GET /api/v1/auth/send_code auth=false group=auth summary=发送验证码
[API] GET /api/v1/profile/info auth=true group=profile summary=获取用户信息
API 路由清单接口：GET /api/v1/routes
API 路由分组接口：GET /api/v1/routes/groups
```

---

### 8. 完整流程图

```
NewRootHandler 被调用
    ↓
1. 创建根路由 rootMux
    ↓
2. 初始化各模块 Handler
    ↓
3. registerRootPrefixRoutes 挂载模块路由
    ↓
4. buildRouteCatalog 构建路由清单
    ↓
5. buildRouteCatalogGrouped 按模块分组
    ↓
6. rootMux.Handle 注册路由清单接口
    ↓
7. logRouteCatalog 打印路由清单
    ↓
返回 rootMux
```


<br/><br/><br/>

***
<br/>

> <h2 id="路由清单接口对比">路由清单接口对比</h2>

**说明：** 两个接口路径不同，返回数据格式不同，不是重复的。

---

### 1. 两个接口对比

| 接口 | 路径 | 返回数据 |
|------|------|----------|
| 路由清单 | `/api/v1/routes` | 扁平列表 |
| 分组路由清单 | `/api/v1/routes/groups` | 按模块分组 |

---

### 2. 常量定义

```go
const routeCatalogPath = "/api/v1/routes"
const routeCatalogGroupsPath = "/api/v1/routes/groups"
```

---

### 3. 返回数据格式不同

**`/api/v1/routes`** - 扁平列表
```json
[
  {"method": "POST", "path": "/api/v1/auth/login", "summary": "登录"},
  {"method": "GET", "path": "/api/v1/profile/info", "summary": "获取用户信息"}
]
```

**`/api/v1/routes/groups`** - 按模块分组
```json
{
  "auth": [
    {"method": "POST", "path": "/api/v1/auth/login", "summary": "登录"}
  ],
  "profile": [
    {"method": "GET", "path": "/api/v1/profile/info", "summary": "获取用户信息"}
  ]
}
```

---

### 4. 代码对应

```go
// 注册扁平路由清单接口
rootMux.Handle(routeCatalogPath, buildRouteCatalogHandler(newRouteCatalogHandler(routeCatalog)))

// 注册分组路由清单接口
rootMux.Handle(routeCatalogGroupsPath, buildRouteCatalogHandler(newRouteCatalogGroupedHandler(routeCatalogGrouped)))
```

---

### 5. 一句话总结

```
两个接口路径不同，返回数据格式不同，不是重复的
- /api/v1/routes → 扁平列表
- /api/v1/routes/groups → 按模块分组
```

<!--
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-15 08:47:16
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-09-11 16:30:44
 * @FilePath: /MLC_GO/IntroduceREMADE.md
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
-->
> <h5></h5>
- [**文件结构介绍**](#文件结构介绍)
- [**文件规则**](#文件规则)
	- [协议规则](#协议规则)
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
	- [NSQ源码阅读](#NSQ源码阅读)
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
```


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

> <h1 id="TCP通信自己实现">TCP通信自己实现</h1>

在 iOS 端（客户端）使用 Swift 来和 Go 写的 TCP 服务端进行通信，可以使用 `Network` 框架（iOS 12+）或较低版本使用 `CFStream` 或 `GCDAsyncSocket`（CocoaAsyncSocket）库。下面我会给出一个使用 **`Network` 框架**（推荐）的完整示例，展示如何在 iOS 客户端中连接你提供的 Go TCP 服务端（监听端口为 `8889`）并发送/接收数据。

---

### ✅ 使用 Swift + Network 框架实现 TCP 客户端通信（iOS）

#### 示例代码（iOS 客户端）：

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

## 二、学习前准备（0基础也能入门）

### 1. 基础技能

先具备以下基础（可以边学 NSQ 边查）：

* Go 基础语法、interface、goroutine、channel、select
* Go 标准库：`net`, `http`, `os`, `sync`, `log`
* Git 基础操作、源码 clone、`go build` 等

> 学习推荐：

* Go 入门教程：[https://tour.go-zh.org/](https://tour.go-zh.org/)
* 实战 Go： [https://github.com/unknwon/go-fundamental-programming](https://github.com/unknwon/go-fundamental-programming)

---

## 三、具体学习步骤（建议打印或收藏）

### ✅ Step 1：克隆 NSQ 项目并能运行

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

---

### ✅ Step 2：了解整个系统架构（宏观理解）

> 找到图示：NSQ 架构图：[https://nsq.io/components/](https://nsq.io/components/)

关键组件：

* `nsqd`：核心组件，负责接收、存储、转发消息
* `nsqlookupd`：服务发现中心，维护 topic 和 channel 映射
* `nsqadmin`：Web UI 管理后台

重点理解流程：

```
Producer --> nsqd --> (Lookupd) --> Consumer
```

---

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


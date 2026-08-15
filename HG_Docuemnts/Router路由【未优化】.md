- [中间件初步设想](#中间件初步设想)
- [大厂方案思路](#大厂方案思路)
- [生产级实现思路代码](#生产级实现思路代码)
	- [可运行的 Radix Tree Router](#可运行的-Radix-Tree-Router)


<br/>

***
<br/><br/><br/>
># <h1 id="中间件初步设想">中间件初步设想</h1>


> **如果不用 Gin、Kratos、Echo、Fiber 这些 Go Web 框架，而是只基于 Go 标准库 `net/http` 的 `http.Handler`，如何设计一个类似 B 站、抖音、小红书这种亿级用户、千万级并发的后端基础中间件层？**

答案是：

**可以，而且很多大型 Go 服务的核心思想就是接近这个模式。**

Go 的 HTTP 核心就是：

```go
type Handler interface {
    ServeHTTP(ResponseWriter, *Request)
}
```

所谓 middleware，本质就是：

```
Request
   |
   v
Middleware A
   |
Middleware B
   |
Middleware C
   |
Business Handler
   |
Response
```

它不是魔法，本质就是 Handler 包装 Handler。([DEV Community][1])

对于 B 站这种规模，不会设计一个巨大 middleware，而是分层：

```
                 CDN
                  |
             Load Balancer
                  |
             API Gateway
                  |
        Go HTTP Server Cluster
                  |
        ======================
        自研 Middleware Layer
        ======================
        |
        |-- Request ID
        |-- Trace
        |-- Access Log
        |-- Recover
        |-- Auth
        |-- Permission
        |-- Rate Limit
        |-- Anti Abuse
        |-- Gray Release
        |-- Circuit Breaker
        |-- Timeout
        |-- Metrics
        |
        Business Handler
        |
        Service
        |
        Repository
        |
        MySQL Redis Kafka
```

---

# 一、基础项目结构设计

类似字节、腾讯内部 Go 服务：

```
video-service

├── cmd
│   └── main.go
│
├── internal
│
│   ├── server
│   │    ├── http_server.go
│   │    └── router.go
│   │
│   ├── middleware
│   │
│   │    ├── chain.go
│   │    ├── recovery.go
│   │    ├── request_id.go
│   │    ├── auth.go
│   │    ├── ratelimit.go
│   │    ├── trace.go
│   │    └── metrics.go
│   │
│   ├── handler
│   │
│   ├── service
│   │
│   ├── repository
│   │
│   └── model
│
└── pkg
```

---

# 二、自研 Middleware 核心接口

定义：

```go
package middleware


import "net/http"


type Middleware func(http.Handler) http.Handler
```

例如：

```
LoggingMiddleware

输入:

Handler


输出:

新的Handler
```

结构：

```go
func Logging(next http.Handler) http.Handler {


    return http.HandlerFunc(
        func(w http.ResponseWriter,
             r *http.Request){


            start:=time.Now()


            next.ServeHTTP(w,r)


            cost:=time.Since(start)


            log.Println(cost)
        })
}
```

---

# 三、Middleware Chain

生产不会这样：

```go
handler =
 A(
   B(
     C(
       api
     )
   )
 )
```

而是：

自己实现 Chain。

```go
type Chain struct {

    middlewares []Middleware

}


func NewChain(
    ms ...Middleware,
)*Chain{

    return &Chain{
        middlewares:ms,
    }
}



func(c *Chain) Handler(
    h http.Handler,
)http.Handler{


    for i:=len(c.middlewares)-1;i>=0;i--{

        h=c.middlewares[i](h)

    }


    return h
}
```

使用：

```go
handler:=chain.Handler(
    videoHandler,
)
```

---

# 四、B站级别必须有哪些 Middleware

## 1. Request ID

每个请求唯一：

```
用户打开视频

request_id:

01J8AF89VIDEO99821
```

用途：

日志链路：

```
API Gateway

request_id=abc


video-service

request_id=abc


comment-service

request_id=abc


mysql

request_id=abc
```

代码：

```go
func RequestID(next http.Handler)
http.Handler{


return http.HandlerFunc(
func(w http.ResponseWriter,r *http.Request){


id:=snowflake()


ctx:=context.WithValue(
r.Context(),
"request_id",
id,
)


next.ServeHTTP(
w,
r.WithContext(ctx),
)

})


}
```

---

# 五、Recover Middleware

线上必须防止 panic。

例如：

```go
func Recovery(next http.Handler)
http.Handler{


return http.HandlerFunc(
func(w http.ResponseWriter,r *http.Request){


defer func(){


if err:=recover();err!=nil{


log.Println(err)


http.Error(
w,
"server error",
500,
)


}

}()



next.ServeHTTP(w,r)



})


}
```

---

# 六、JWT/Auth Middleware

B站：

```
GET /api/video/detail?id=100


Header:

Authorization:
Bearer xxxx
```

流程：

```
Request

 |
解析JWT

 |
得到:

user_id=8888

 |
context保存

 |
handler读取
```

代码：

```go
func Auth(next http.Handler)
http.Handler{


return http.HandlerFunc(
func(w http.ResponseWriter,r *http.Request){



token :=
r.Header.Get(
"Authorization",
)



userID,err :=
verify(token)



if err!=nil{


http.Error(
w,
"unauthorized",
401,
)


return

}



ctx :=
context.WithValue(
r.Context(),
"user_id",
userID,
)



next.ServeHTTP(
w,
r.WithContext(ctx),
)


})

}
```

---

# 七、限流 Middleware（千万并发核心）

比如：

热门视频：

```
视频ID:

BV10001


瞬间:

100万人点赞
```

不能直接打 Redis/MySQL。

链路：

```
Request

 |
Local Token Bucket

 |
Redis Counter

 |
Business
```

实现：

```go
type Limiter struct{


tokens int64

mutex sync.Mutex

}



func(l *Limiter)
Allow()bool{


l.mutex.Lock()

defer l.mutex.Unlock()


if l.tokens<=0{

return false

}


l.tokens--

return true

}
```

生产升级：

```
本机令牌桶

+

Redis分布式限流

+

Kafka削峰
```

---

# 八、Response Cache Middleware

抖音首页：

```
GET

/feed

```

不能每次：

```
MySQL
 ↓
推荐算法
 ↓
排序
```

所以：

```
Request

 |
Cache Middleware

 |
Redis

 |
不存在

 |
Service

 |
写Redis
```

例如：

```go
video_feed:user:10001


value:

[
 video1,
 video2,
 video3
]

ttl:

5s
```

---

# 九、Metrics Middleware

统计：

```
接口:

/video/detail


QPS:

200000/s


平均:

15ms


错误:

0.01%
```

结构：

```go
type Metrics struct{


Request uint64


Error uint64


Latency int64


}
```

---

# 十、Trace Middleware

类似：

OpenTelemetry。

链路：

```
gateway

trace_id=a001


video-service

trace_id=a001


comment-service

trace_id=a001


mysql

trace_id=a001
```

---

# 十一、完整 Middleware 顺序

大型互联网：

一般：

```
最外层


Recovery


↓


RequestID


↓


Trace


↓


AccessLog


↓


CORS


↓


RateLimit


↓


Auth


↓


Permission


↓


Cache


↓


Handler


```

为什么？

例如：

Auth失败：

不用进入业务。

限流失败：

不用查数据库。

Cache命中：

不用执行Service。

---

# 十二、Server设计

不要：

```go
http.ListenAndServe(
":8080",
handler,
)
```

生产：

```go
server:=&http.Server{


Addr:":8080",


Handler:handler,


ReadTimeout:
5*time.Second,


WriteTimeout:
5*time.Second,


IdleTimeout:
60*time.Second,


}


server.ListenAndServe()
```

---

# 十三、B站级服务拆分

不会：

```
app
 |
一个Go服务
 |
全部代码
```

而是：

```
                 Gateway


                    |
 -------------------------------------------------

 video-service

 user-service

 comment-service

 like-service

 follow-service

 recommend-service

 search-service


 -------------------------------------------------


 Redis

 Kafka

 MySQL

 ES

 Object Storage

```

---

# 十四、千万并发下 Middleware 注意点

## 1. 禁止全局锁

错误：

```go
var mutex sync.Mutex
```

每个请求：

```
lock

unlock
```

千万并发直接死。

应该：

```
atomic

sync.Pool

sharding
```

---

## 2. 使用对象池

例如：

Request Context：

```go
var pool=
sync.Pool{

New:
func()any{

return &Context{}

},

}
```

避免：

```
100万QPS

↓

100万个对象

↓

GC压力爆炸
```

---

## 3. 不在 Middleware 查数据库

错误：

```
Auth Middleware

↓

mysql查询用户
```

正确：

```
JWT解析

↓

Redis缓存用户状态

↓

异步刷新
```

---

# 十五、最终生产级架构

```
                 APP

                  |

                 CDN

                  |

             API Gateway

                  |

          Go http.Server


                  |

        Middleware Framework

        =====================

        RequestID

        Trace

        Logger

        Recover

        RateLimit

        Auth

        Permission

        Cache

        Metrics

        Gray

        CircuitBreaker


                  |

              Handler


                  |

              Service


                  |

        ----------------

        Redis

        Kafka

        MySQL

        ES

        OSS

        ----------------

```

---

总结：

对于 B 站、抖音、小红书这种级别：

**不要把 Middleware 理解成 Gin 的插件。**

真正的大厂做法：

> 基于 Go `http.Handler` 自己实现一个内部 HTTP Framework。

核心就是：

1. `http.Handler` 作为唯一抽象
2. Middleware Chain 自研
3. Request Context 传递上下文
4. Middleware 无状态化
5. Redis/Kafka/MySQL 负责数据层扩展
6. Gateway + Service Mesh 做服务治理

Gin、Kratos、Echo 本质上也是在这个模型上封装。Go 官方 `net/http` 已经提供了这个最核心的 Handler 抽象。([pkg.go.dev][2])

如果继续深入，下一步就是设计一个**类似字节内部 Go HTTP Framework 的完整代码结构（包含 router、context、middleware、binding、error handler、response writer、RPC 调用）**。

[1]: https://dev.to/shayan_holakouee/building-a-nethttp-middleware-chain-from-scratch-in-go-346b?utm_source=chatgpt.com "Building a net/http Middleware Chain From Scratch in Go - DEV Community"
[2]: https://pkg.go.dev/net/http?utm_source=chatgpt.com "http package - net/http - Go Packages"


<br/>

***
<br/><br/><br/>
># <h1 id="大厂方案思路">大厂方案思路</h1>

下面给出一个**类似字节、腾讯内部 Go HTTP Framework 的简化生产级设计**。

目标：

* 不使用 Gin / Echo / Fiber
* 基于 Go 原生 `net/http`
* 自己实现：

  * Router
  * Context
  * Middleware
  * Response Writer
  * Request Binding
  * Error Handler
  * RPC Client
  * Service 调用层
* 面向：

  * 千万级 QPS
  * 微服务架构
  * B站/抖音视频服务这种模式

整体架构：

```
                    Client
                       |
                       |
                 Load Balancer
                       |
                       |
                HTTP Gateway
                       |
                       |
              =================
                Go Framework
              =================

                    Server

                       |
                 Router

                       |
                 Context

                       |
              Middleware Chain

                       |
                 Handler

                       |
                 Service

                       |
              Repository

                       |
        ----------------------------
        MySQL Redis Kafka ES ObjectStorage
```

---

# 一、工程目录设计

类似大厂：

```
go-framework-demo/


├── cmd
│
│   └── video-api
│       └── main.go
│
│
├── framework
│
│   ├── server
│   │
│   │   └── server.go
│   │
│   ├── router
│   │
│   │   ├── router.go
│   │   └── tree.go
│   │
│   ├── context
│   │
│   │   └── context.go
│   │
│   ├── middleware
│   │
│   │   ├── chain.go
│   │   ├── recovery.go
│   │   ├── auth.go
│   │   └── trace.go
│   │
│   ├── binding
│   │
│   │   └── json.go
│   │
│   ├── response
│   │
│   │   └── response.go
│   │
│   ├── error
│       │
│       └── error.go
│
│
├── internal
│
│   └── video
│
│       ├── handler.go
│       ├── service.go
│       └── repository.go
│
└── pkg
```

---

# 二、核心 Context

类似 Gin Context。

文件：

```
framework/context/context.go
```

代码：

```go
package context


import (
"net/http"
)


type Context struct {


Writer http.ResponseWriter


Request *http.Request



Params map[string]string



Values map[string]interface{}



}


func New(
w http.ResponseWriter,
r *http.Request,
)*Context{


return &Context{


Writer:w,

Request:r,


Params:
make(map[string]string),


Values:
make(map[string]interface{}),


}

}



func(c *Context)
Set(
key string,
value interface{},
){


c.Values[key]=value

}



func(c *Context)
Get(
key string,
)(
interface{},
bool,
){


v,ok :=
c.Values[key]


return v,ok

}
```

---

# 三、Response封装

大厂不会直接：

```go
json.NewEncoder(w).Encode()
```

统一：

```
{
 code:0,
 message:"success",
 data:{}
}
```

文件：

```
response/response.go
```

代码：

```go
package response


import(
"encoding/json"
"net/http"
)



type Result struct{


Code int `json:"code"`


Message string `json:"message"`


Data interface{} `json:"data"`

}



func JSON(
w http.ResponseWriter,
code int,
data interface{},
){



w.Header()
.Set(
"Content-Type",
"application/json",
)



json.NewEncoder(w)
.Encode(

Result{

Code:0,

Message:"success",

Data:data,

},

)


}
```

---

# 四、Handler抽象

定义：

```go
type HandlerFunc func(
*context.Context
)
```

文件：

```
router/router.go
```

---

# 五、Router实现

## 路由节点

类似 Gin radix tree。

```go
type node struct{


path string


children []*node



handler HandlerFunc


}
```

例如：

```
/video/detail

        /
        |
      video
        |
     detail

```

---

## Router

```go
type Router struct{


trees map[string]*node


}
```

注册：

```go
func(r *Router)
GET(
path string,
handler HandlerFunc,
){


r.add(
"GET",
path,
handler,
)

}
```

---

匹配：

请求：

```
GET

/video/detail?id=100
```

寻找：

```
/
 |
video
 |
detail

```

找到 handler。

---

# 六、Middleware系统

定义：

```go
type Middleware func(
HandlerFunc
)
HandlerFunc
```

例如：

```
Request

 |
Trace

 |
Auth

 |
Handler

```

代码：

```go
type Middleware func(
HandlerFunc
)
HandlerFunc
```

---

Chain:

```go
func Chain(
h HandlerFunc,
ms []Middleware,
)
HandlerFunc{


for i:=len(ms)-1;i>=0;i--{


h=
ms[i](h)


}


return h

}
```

---

# 七、Recovery Middleware

```go
func Recovery()
Middleware{


return func(
next HandlerFunc,
)
HandlerFunc{


return func(
ctx *context.Context,
){


defer func(){


if err:=recover();
err!=nil{


response.JSON(

ctx.Writer,

500,

"server error",

)


}


}()



next(ctx)


}

}

}
```

---

# 八、Trace Middleware

请求：

```
request_id:

01JASDKASD123
```

代码：

```go
func Trace()
Middleware{


return func(
next HandlerFunc,
)
HandlerFunc{


return func(
ctx *context.Context,
){



traceID :=
uuid()



ctx.Set(
"trace_id",
traceID,
)



next(ctx)


}

}

}
```

---

# 九、Auth Middleware

用户请求：

```
GET /video/detail


Authorization:

Bearer xxx

```

解析：

```go
func Auth()
Middleware{


return func(
next HandlerFunc,
)
HandlerFunc{


return func(
ctx *context.Context,
){



token :=
ctx.Request.Header.Get(
"Authorization",
)



uid,err :=
ParseJWT(token)



if err!=nil{


response.JSON(

ctx.Writer,

401,

"unauthorized",

)


return

}



ctx.Set(
"user_id",
uid,
)



next(ctx)


}


}


}
```

---

# 十、JSON Binding

客户端：

```json
{
"title":"hello",
"desc":"test"
}
```

定义：

```go
type CreateVideoRequest struct{


Title string `json:"title"`


Desc string `json:"desc"`

}
```

Binding:

```go
func BindJSON(
r *http.Request,
obj interface{},
)
error{


decoder :=
json.NewDecoder(
r.Body,
)


return decoder.Decode(obj)

}
```

---

# 十一、Error系统

不要：

```go
return errors.New()
```

统一：

```go
type AppError struct{


Code int


Message string


}


func(e *AppError)
Error()
string{


return e.Message

}
```

例如：

```go
ErrUserNotFound=
&AppError{


Code:10001,


Message:"user not found",


}
```

---

# 十二、业务 Handler

video handler:

```go
func Detail(
ctx *context.Context,
){



id :=
ctx.Request.URL.Query()
.Get("id")



video,
err :=
videoService.Detail(
id,
)



if err!=nil{


response.JSON(
ctx.Writer,
500,
err,
)


return

}



response.JSON(
ctx.Writer,
200,
video,
)


}
```

---

# 十三、Service层

```
Handler

 ↓

Service

 ↓

Repository

 ↓

DB
```

代码：

```go
type VideoService struct{


repo VideoRepository


}



func(s *VideoService)
Detail(
id string,
)
(*Video,error){


return s.repo.Find(id)


}
```

---

# 十四、Repository

```go
type VideoRepository interface{


Find(
id string,
)
(*Video,error)


}
```

MySQL实现：

```go
type MysqlVideoRepo struct{


db *sql.DB


}


func(r *MysqlVideoRepo)
Find(
id string,
)
(*Video,error){


row:=

r.db.QueryRow(
`
select *
from videos
where id=?
`,
id,
)


}
```

---

# 十五、RPC Client设计

比如：

video-service 调 recommend-service。

不用业务直接调用 grpc。

封装：

```
framework/rpc
```

接口：

```go
type Client interface{


Invoke(
ctx context.Context,
service string,
method string,
req interface{},
resp interface{},
)
error


}
```

调用：

```go
rpc.Invoke(

ctx,

"recommend-service",

"GetRecommend",

request,

response,

)
```

---

# 十六、Server启动

main.go

```go
func main(){



router:=router.New()



router.GET(

"/video/detail",

video.Detail,

)



handler:=

middleware.Chain(

router,

[]Middleware{


Recovery(),

Trace(),

Auth(),

},


)



server:=

http.Server{


Addr:":8080",


Handler:handler,


ReadTimeout:
5*time.Second,


WriteTimeout:
5*time.Second,


}



server.ListenAndServe()


}
```

---

# 十七、真实 B站级扩展

这个框架继续扩展：

## Gateway层

增加：

```
IP限流

用户限流

黑名单

灰度发布

AB Test

```

---

## Context增加

```go
type Context struct{


TraceID string


UserID int64


DeviceID string


ClientIP string


StartTime time.Time


}
```

---

## ResponseWriter增强

统计：

```
status code

response size

latency

```

---

## 增加对象池

千万请求：

```go
var ctxPool sync.Pool
```

避免：

```
100万请求

↓

100万个Context

↓

GC压力
```

---

# 十八、最终效果

你的 Go Framework：

```
             HTTP Request


                  |
                  
              Server


                  |
                  
              Router


                  |
                  
             Context


                  |
                  
        Middleware Pipeline


 Recovery
 Trace
 Logger
 Auth
 RateLimit
 Cache
 Permission


                  |
                  
              Handler


                  |
                  
              Service


                  |
                  
             Repository


                  |
                  
        MySQL Redis Kafka ES

```

这个就是一个简化版的：

* 字节 Go HTTP Framework
* 腾讯 TARS/Go 服务层思想
* 阿里内部 RPC + HTTP 网关思想

如果继续做生产级，还需要补充：

1. **高性能 Radix Tree Router（类似 Gin 的路由树）**
2. **基于 sync.Pool 的零 GC Context**
3. **千万连接 WebSocket Gateway**
4. **Kafka 异步事件总线**
5. **Redis Cluster 分布式缓存层**
6. **MySQL 分库分表 ID 设计**

这几个才是真正支撑 B 站/抖音亿级数据的核心。

<br/>

***
<br/><br/><br/>
># <h1 id="生产级实现思路代码">生产级实现思路代码</h1>



下面按照**真正生产级实现路线**展开。

目标：

实现一个类似：

* 字节内部 Go HTTP Framework
* B站视频服务基础框架
* 抖音 Feed 服务基础设施

不依赖：

* Gin
* Echo
* Fiber
* gRPC 框架
* ORM

只使用：

* net/http
* sync
* context
* encoding/json
* database/sql
* go 原生能力

整体分 6 个模块：

```
go-video-platform/


framework/

 ├── router
 │    └── radix tree router

 ├── context
 │    └── sync.Pool Context

 ├── websocket
 │    └── connection manager

 ├── event
 │    └── kafka producer wrapper

 ├── cache
 │    └── redis cluster client

 └── id
      └── snowflake business id


services/

 ├── video-service

 ├── user-service

 ├── comment-service

 └── recommend-service

```

---

# 一、高性能 Radix Tree Router

## 1. 为什么不用 map

简单：

```go
routes["/video/detail"]
```

问题：

动态路由：

```
/user/10001/video
```

匹配：

```
/user/:id/video
```

需要树。

Gin 使用：

```
Radix Tree
```

结构：

```
             /
             |
          user
             |
          :id
             |
          video

```

---

## 2. Node设计

router/node.go

```go
package router


type Node struct {


path string


children []*Node


paramChild *Node


handler HandlerFunc


}


```

---

## 3. Router

router/router.go

```go
package router


import (
"net/http"
)


type HandlerFunc func(
http.ResponseWriter,
*http.Request,
)



type Router struct{


root *Node


}



func NewRouter()*Router{


return &Router{


root:&Node{},

}


}

```

---

## 4. 注册路由

例如：

```go
router.GET(
"/video/:id",
handler,
)
```

实现：

```go
func(r *Router)
GET(
path string,
h HandlerFunc,
){


r.add(
path,
h,
)

}




func(r *Router)
add(
path string,
h HandlerFunc,
){


node:=r.root


parts:=split(path)



for _,part:=range parts{


child:=findChild(
node,
part,
)


if child==nil{


child=&Node{
path:part,
}


node.children=
append(
node.children,
child,
)


}



node=child


}


node.handler=h


}

```

---

## 5. 查询

请求：

```
GET /video/10086
```

匹配：

```
/
 |
video
 |
10086

```

代码：

```go
func(r *Router)
Find(
path string,
)
HandlerFunc{


node:=r.root


parts:=split(path)


for _,p:=range parts{


child:=findChild(
node,
p,
)


if child==nil{

return nil

}


node=child


}


return node.handler

}

```

性能：

```
O(path length)

```

百万级 QPS 没问题。

---

# 二、sync.Pool 零 GC Context

千万请求：

如果每次：

```
new Context

new map

new object

```

GC压力巨大。

采用：

```
sync.Pool
```

---

## Context

context/context.go

```go
package context


import (
"net/http"
)



type Context struct{


Writer http.ResponseWriter


Request *http.Request


UserID int64


TraceID string


Data map[string]interface{}


}



func(c *Context)
Reset(){


c.Writer=nil

c.Request=nil


c.UserID=0


c.TraceID=""


for k:=range c.Data{


delete(
c.Data,
k,
)


}

}

```

---

## Pool

```go
var pool sync.Pool


func init(){


pool.New=func()interface{}{



return &Context{


Data:
make(map[string]interface{}),


}



}


}

```

获取：

```go
func Acquire()
*Context{


return pool.Get()
.(*Context)

}

```

释放：

```go
func Release(
c *Context,
){


c.Reset()


pool.Put(c)


}

```

请求流程：

```
request

 |

Get Context

 |

handler

 |

Release

```

---

# 三、千万连接 WebSocket Gateway

B站直播弹幕：

```
1000万连接

```

不能：

```
一个goroutine一个连接
```

需要：

```
connection manager

channel

sharding
```

---

## Connection

```go
type Connection struct{


ID string


Socket net.Conn


Send chan []byte


}

```

---

## Manager

```go
type Manager struct{


connections map[string]*Connection


lock sync.RWMutex


}


```

---

添加连接：

```go
func(m *Manager)
Add(
c *Connection,
){


m.lock.Lock()


defer m.lock.Unlock()


m.connections[c.ID]=c


}

```

---

广播：

错误：

```go
for _,conn:=range all{


conn.Send<-msg

}

```

100万人直接阻塞。

---

正确：

分片。

```
Shard0

10万个连接


Shard1

10万个连接


Shard2

10万个连接

```

代码：

```go
type Shard struct{


conns sync.Map


}



type Gateway struct{


shards []Shard


}

```

广播：

```go
func(g *Gateway)
Broadcast(
msg []byte,
){


for _,s:=range g.shards{


go func(shard Shard){


shard.conns.Range(
func(k,v interface{})bool{


v.(*Connection)
.Send<-msg


return true

})


}(s)


}

}

```

---

# 四、Kafka异步事件总线

B站点赞：

用户：

```
点击点赞

```

不能：

```
update mysql

count++

```

同步。

改：

```
HTTP

 |

Kafka

 |

Consumer

 |

MySQL

```

---

## Producer

event/producer.go

```go
type Producer struct{


client kafka.Client


}


func(p *Producer)
Publish(
topic string,
data []byte,
)
error{


msg:=Message{


Topic:topic,


Value:data,


}



return p.client.Send(msg)


}

```

---

业务：

点赞：

```go
func Like(
videoID string,
){


event.Publish(

"video.like",

[]byte(videoID),

)


}

```

---

Kafka Topic设计：

```
video.like

video.comment

video.follow

video.view

video.share

```

---

Consumer:

```go
for{


msg:=Consume()



switch msg.Topic{


case "video.like":


updateLike()



case "video.view":


updateView()



}


}

```

---

# 五、Redis Cluster缓存层

B站首页：

```
GET /feed

```

千万用户。

不能：

```
MySQL

```

流程：

```
request


 |

redis


 |

miss


 |

mysql


 |

redis set


```

---

## Cache Interface

```go
type Cache interface{


Get(
key string,
)
([]byte,error)



Set(
key string,
value []byte,
ttl time.Duration,
)
error


}

```

---

Redis实现：

```go
type RedisCache struct{


client redis.Client


}


func(r *RedisCache)
Get(
key string,
)
([]byte,error){



return r.client.Get(
key,
)

}

```

---

Key设计：

视频：

```
video:detail:10001

```

用户：

```
user:profile:88888

```

Feed：

```
feed:user:88888

```

---

# 六、MySQL分库分表ID设计

亿级数据：

不能：

```sql
id bigint auto_increment
```

原因：

单库瓶颈。

---

## Snowflake

64 bit:

```
0
|
41 bit 时间
|
10 bit机器
|
12 bit序列

```

例如：

```
188238822882882

```

---

Go实现：

```go
type Snowflake struct{


machineID int64


sequence int64


lastTime int64


mutex sync.Mutex


}


func(s *Snowflake)
NextID()
int64{


s.mutex.Lock()

defer s.mutex.Unlock()


now :=
time.Now()
.UnixMilli()



if now==
s.lastTime{


s.sequence++


}else{


s.sequence=0

}


s.lastTime=now



id:=

(now<<22)
|
(s.machineID<<12)
|
s.sequence



return id


}

```

---

## 分库策略

例如：

用户表：

10亿用户

分：

```
user_db_00

user_db_01

...

user_db_63

```

规则：

```
db = user_id % 64

```

例如：

```
user_id=100001


100001 %64


=33


进入:

user_db_33

```

---

# 七、最终请求链

真实 B站视频详情：

```
APP


 |

Nginx


 |

API Gateway


 |

Go Framework


 |

---------------------

RequestID


Trace


RateLimit


Auth


Cache


---------------------


 |

Video Handler


 |

Video Service


 |

Redis


 |

MySQL


 |

Kafka Event


```

---

# 八、千万并发下关键优化

## 1. 禁止：

```go
map + mutex
```

改：

```
sync.Map

或者

Shard Lock

```

---

## 2. 禁止：

```go
每请求创建对象
```

改：

```
sync.Pool
```

---

## 3. 禁止：

```
同步写数据库
```

改：

```
Kafka Async
```

---

## 4. 禁止：

```
单Redis

```

改：

```
Redis Cluster

```

---

## 5. 禁止：

```
单MySQL

```

改：

```
分库分表 + 读写分离

```

---

这个框架继续完善后，就是一个小型：

* Gin 内核
* 字节 HTTP Infra
* B站业务基础框架

下一步建议继续实现：

1. **完整可运行的 Radix Tree Router（支持 :id、*path、路由分组）**
2. **完整 WebSocket Gateway（百万连接模型：epoll + goroutine + shard）**
3. **Kafka 消息队列完整 Go 实现（生产者、消费者、重试、DLQ）**
4. **B站视频系统完整服务代码（视频详情、点赞、评论、关注）**

这四部分组合起来就是一个真实短视频云平台后端骨架。


***
<br/><br/><br/>
> <h2 id="可运行的-Radix-Tree-Router">可运行的 Radix Tree Router</h2>

https://chatgpt.com/g/g-p-6948f37c9d50819197a136546b06413d-gogong-cheng/c/6a804fc4-0888-83e9-b30e-031174c479b0
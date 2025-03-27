<!--
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2025-03-15 08:47:16
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2025-03-20 16:51:42
 * @FilePath: /MLC_GO/IntroduceREMADE.md
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
-->
> <h5></h5>
- [**文件结构介绍**](#文件结构介绍)
- [**文件规则**](#文件规则)
	- [协议规则](#协议规则)
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


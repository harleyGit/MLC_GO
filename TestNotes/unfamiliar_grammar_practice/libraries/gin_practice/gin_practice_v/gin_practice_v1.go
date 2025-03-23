/**
* @Author: GangHuang harleysor@qq.com
* @Date: 2025-03-19 18:48:05
* @LastEditors: GangHuang harleysor@qq.com
* @LastEditTime: 2025-03-19 18:48:09
* @FilePath: /MLC_GO/TestNotes/unfamiliar_grammar_practice/libraries/gin_practice/gin_practice_v/gin_practice_v1.go
* @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */

package gin_practice_v

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"MLC_GO/pkg/hglog"
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// 定义 User 结构体
type User struct {
	// binding:"required" 是 Gin 框架中用于 数据验证（Validation） 的标签（tag），
	// 	它属于 github.com/go-playground/validator/v10 库，Gin 内部集成了这个库来自动验证请求参数
	// 	如果字段值为空（未提供），Gin 会自动返回 400 状态码，并提示该字段是必填项。
	// 要想检测 exists 方法必须注明,比如 GormPracticeV1_v3中注册了
	Name  string `json:"name" binding:"exists,alphanumunicode,min=2,max=255"`
	Email string `json:"email" binding:"required,email"`
	Age   int    `json:"age" binding:"required"`
}

type GinPracticeV1 struct{}

// 协议
func (ginPracticeV1 *GinPracticeV1) ExecutePracticeNone() {
	hglog.DebugInfo("协议 gin库 GinPracticeV1 ExecutePracticeNone")
}

/*
curl -X GET http://localhost:8080/hello -i
	预期输出（响应头中应包含 CORS 相关信息）

curl -X OPTIONS http://localhost:8080/hello -i \
  -H "Access-Control-Request-Method: GET" \
  -H "Access-Control-Request-Headers: Content-Type"
	预期输出（状态码 204，且包含 CORS 头）：

测试携带自定义 Header 的请求
curl -X GET http://localhost:8080/hello -i \
  -H "Origin: https://www.youdao.com/" \
  -H "X-CSRF-Token: testtoken"
	预期输出（服务器应返回 Access-Control-Allow-Origin: *）
*/
// 中间件解决跨域
func (ginPracticeV1 *GinPracticeV1) GormPracticeV1_v9() {
	r := gin.Default()

	// 使用全局中间件处理跨域问题(局部中间件是加在路由组里面)
	r.Use(corsMiddleware())

	// 其他路由注册
	r.GET("/hello", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Hello, CORS is enabled!"})
	})

	// 启动 Gin 服务器
	r.Run(":8080")
}

// corsMiddleware 中间件处理跨域问题
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// Gin 框架的日志功能:日志输出到指定文件夹
func (ginPracticeV1 *GinPracticeV1) GormPracticeV1_v8() {
	path_1 := "./TestNotes/unfamiliar_grammar_practice/libraries/gin_practice/tmp/gin.log"

	// 将日志输出到文件
	file12, _ := os.Create(path_1)
	gin.DefaultWriter = io.MultiWriter(file12, os.Stdout)

	router := gin.Default()

	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	router.Run(":8080")
}

// 添加中间件处理错误
// 自定义错误处理函数: curl http://localhost:8080/ping
// 创建了一个全局中间件函数来检查处理过程中是否有错误发生，如果有错误则返回自定义的错误响应。
//
//	在路由处理函数中，我们通过 c.Error 方法模拟了一个处理过程中发生的错误。
func (ginPracticeV1 *GinPracticeV1) GormPracticeV1_v7() {
	router := gin.Default()

	// 自定义全局中间件处理错误
	// 中间件代码是一个全局的错误处理器，用于捕捉请求处理过程中产生的错误并返回自定义的错误响应
	// 中间件工作机制：
	// 		c.Next()：这个方法告诉 Gin 继续处理后续的中间件或路由处理函数。
	// 		检查 c.Errors：在执行完后续的处理函数后，代码会检查是否有错误（c.Errors）。
	// 		自定义错误处理：如果 c.Errors 中有错误，说明在请求处理过程中发生了错误，返回一个 500 Internal Server Error 和自定义的错误消息。
	router.Use(func(c *gin.Context) {
		// 这个方法告诉 Gin 继续处理后续的中间件或路由处理函数
		c.Next()

		// 检查是否有发生错误
		if len(c.Errors) > 0 {
			// 自定义错误处理
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
		}
	})

	// 当调用 /ping 路由时，模拟了一个错误。然后通过中间件捕捉并返回一个自定义的错误信息。
	router.GET("/ping", func(c *gin.Context) {
		// 模拟处理过程中发生错误
		c.Error(gin.Error{Err: errors.New("处理过程中发生错误")})
	})

	router.Run(":8080")
}

// 静态文件服务：展示了如何在 Gin 框架中提供静态文件服务，可以方便地将静态资源文件（如图片、样式表、脚本等）提供给客户端。
// 从 ./assets/image.jpg 加载文件，并将其保存为 siShenNet00.jpeg: curl http://localhost:8080/static/siShen00.jpeg --output siShenNet00.jpeg
//
//	curl http://localhost:8080/static2/testDocument01.txt --output document.txt
//
// curl http://localhost:8080/favicon.ico --output favicon.ico
func (ginPracticeV1 *GinPracticeV1) GormPracticeV1_v6() {

	var (
		path_1 string = "./TestNotes/unfamiliar_grammar_practice/libraries/gin_practice/assets"
		path_2 string = "./TestNotes/unfamiliar_grammar_practice/libraries/gin_practice/tmp"
		path_3 string = "./TestNotes/unfamiliar_grammar_practice/libraries/gin_practice/resources/favicon.ico"
	)

	router := gin.Default()

	// 从相对路径 "assets" 提供静态文件
	// 这一行配置了一个路由，允许客户端访问 ./assets 目录下的文件。文件会通过 URL 路径 /static 来访问
	// 使得 ./assets 目录下的文件可以通过 /static 路径访问
	// 例如，./assets 目录中的 image.jpg 文件可以通过访问 http://localhost:8080/static/image.jpg 来加载
	router.Static("/static", path_1)

	// 从绝对路径 "/tmp" 提供静态文件
	// 使得 /tmp 目录下的文件可以通过 /static2 路径访问。
	// 例如，/tmp 目录中的 document.txt 文件可以通过访问 http://localhost:8080/static2/document.txt 来加载。
	router.StaticFS("/static2", http.Dir(path_2))

	// 提供单个静态文件
	// 将 ./resources/favicon.ico 文件作为静态文件提供。
	// 访问 http://localhost:8080/favicon.ico 时，会返回 ./resources/favicon.ico 文件的内容。
	router.StaticFile("/favicon.ico", path_3)

	router.Run(":8080")
}

// 使用 LoadHTMLGlob 方法加载了位于 "templates" 目录下的所有模板文件。
//
//	然后，在 "/hello" 路由处理函数中，我们使用 c.HTML 方法渲染了名为 "hello.tmpl" 的模板，并传递了一个包含标题信息的数据
//
// curl -v http://localhost:8080/hello
// 或者在浏览器: http://localhost:8080/hello
func GormPracticeV1_v5() {
	router := gin.Default()

	// 加载模板文件
	router.LoadHTMLGlob("./TestNotes/unfamiliar_grammar_practice/libraries/gin_practice/templates/*")

	// 定义路由处理函数，渲染模板5
	router.GET("/hello", func(c *gin.Context) {
		c.HTML(http.StatusOK, "hello.html", gin.H{
			"title": "Hello, Gin!-- 定义路由处理函数，渲染模板 -- func GormPracticeV1_v5()",
		})
	})

	router.Run(":8080")
}

/*
GET /users/:id: curl -v GET "http://localhost:8080/users/123"

GET /api/v1/users: curl -X GET "http://localhost:8080/api/v1/users"

POST /api/v1/users:	curl -v -X POST "http://localhost:8080/api/v1/users"

PUT /api/v1/users/:id: curl -v -X PUT "http://localhost:8080/api/v1/users/123"

DELETE /api/v1/users/:id: curl -v -X DELETE "http://localhost:8080/api/v1/users/123"

*/
// 参数化路由和路由组
func (ginPracticeV1 *GinPracticeV1) GormPracticeV1_v4() {
	router := gin.Default()

	// 参数化路由
	router.GET("/users/:id", func(c *gin.Context) {
		id := c.Param("id")
		c.String(200, "User ID: %s", id)
	})

	// 路由组
	v1 := router.Group("/api/v1")
	{
		v1.GET("/users", func(c *gin.Context) {
			c.String(200, "List of users")
		})
		v1.POST("/users", func(c *gin.Context) {
			c.String(200, "Create a user")
		})
		v1.PUT("/users/:id", func(c *gin.Context) {
			id := c.Param("id")
			c.String(200, "Update user with ID: %s", id)
		})
		v1.DELETE("/users/:id", func(c *gin.Context) {
			id := c.Param("id")
			c.String(200, "Delete user with ID: %s", id)
		})
	}

	router.Run(":8080")
}

/* 接收 JSON 格式的请求体，并将其绑定到结构体中进行处理
// 可以反馈错误信息: -v
 curl -X POST http://localhost:8080/users \ 
        -H "Content-Type: application/json" \
        -d '{"name": "陈平安", "email": "alice@gmail.com", "age": 15}'

curl  -X POST http://localhost:8080/users \
	-H "Content-Type: application/json" \
	-d '{"name": "Alice🔥", "email": "alice@example.com", "age": 25}'


发送错误数据（缺少 email 字段）
若是json数据可以用下面进行发送:
curl -X POST http://localhost:8080/users \
     -H "Content-Type: application/json" \
     -d '{"name": "Alice", "age": 25}'

但是若是通过表单格式接受数据需要这样:
curl -X POST http://localhost:8080/users \
     -d "name=Alice🔥" \
     -d "age=17" \
     -d "sex=9" \
     -d "phone=17683837665"

*/
// 引擎验证
func (ginPracticeV1 *GinPracticeV1) GormPracticeV1_v3() {
	router := gin.Default()
	// 获取 Gin 的验证引擎
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		logging.DebugInfo("Gin 验证引擎获取成功")

		// 注册自定义验证规则
		v.RegisterValidation("exists", existsValidator)
	} else {
		logging.ErrInfo("Gin 验证引擎获取失败")
	}

	// GET 请求处理
	router.GET("/hello", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Hello, World!",
		})
	})

	// POST 请求处理
	router.POST("/users", func(c *gin.Context) {
		var user User
		if err := c.ShouldBindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 处理接收到的用户数据
		// ...

		// 验证用户数据
		validate := validator.New()
		if err := validate.Struct(user); err != nil {
			// 如果验证失败，返回错误信息
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "User created successfully",
			"user":    user,
		})
	})

	router.Run(":8080")
}
func existsValidator(fl validator.FieldLevel) bool {
	return fl.Field().String() != ""
}

// 加入中间件、参数解析、日志记录等: curl "http://localhost:8080/hello?name=😂俩百家阿拉斯加了嘎举例"
// 加双引号是防止 Shell 解析特殊符号,用单引号也是可以的
func (ginPracticeV1 *GinPracticeV1) GormPracticeV1_v2() {
	r := gin.Default()

	//Logger 中间件将日志写入 gin.DefaultWriter，即使配置了 GIN_MODE=release
	r.Use(gin.Logger())
	// Recovery 中间件会 recover 任何 panic。如果有 panic 的话，会写入 500 响应码。
	r.Use(gin.Recovery())

	r.GET("/hello", func(c *gin.Context) {
		name := c.Query("name")
		c.JSON(http.StatusOK, gin.H{"message": "Hello, " + name})
	})

	r.Run(":8080")
}

// 测试: curl http://localhost:8080
func (ginPracticeV1 *GinPracticeV1) GormPracticeV1_v1() {
	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		c.String(200, "Hello, Gin!, 我在测试这个方法: GormPracticeV1_v1()")
	})
	r.Run(":8080")
}

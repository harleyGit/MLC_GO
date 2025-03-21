module MLC_GO

go 1.23.0

toolchain go1.23.5

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/BurntSushi/toml v1.3.2 // indirect
	github.com/CloudyKit/fastprinter v0.0.0-20200109182630-33d98a066a53 // indirect
	github.com/CloudyKit/jet/v6 v6.2.0 // indirect
	github.com/Joker/jade v1.1.3 // indirect
	// 爬虫网络
	github.com/PuerkitoBio/goquery v1.10.2
	github.com/Shopify/goreferrer v0.0.0-20220729165902-8cddb4f5de06 // indirect
	github.com/andybalholm/brotli v1.1.0 // indirect
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/antchfx/htmlquery v1.3.4 // indirect
	github.com/antchfx/xmlquery v1.4.3 // indirect
	github.com/antchfx/xpath v1.3.3 // indirect
	// go get -u github.com/astaxie/beego/validation:  Beego 框架 提供的 数据验证（validation）库，用于对结构体字段、表单数据等进行校验，类似于 validator 库。
	github.com/astaxie/beego v1.10.1
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/bytedance/sonic v1.12.10 // indirect
	github.com/bytedance/sonic/loader v0.2.3 // indirect
	github.com/cloudwego/base64x v0.1.5 // indirect
	// ‌认证和授权JWT
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	// Swagger UI文件服务器（对外提供服务）,能够使用go-bindata所生成Swagger UI的Go代码，结合net/http对外提供服务
	github.com/elazarl/go-bindata-assetfs v1.0.1
	github.com/fatih/structs v1.1.0 // indirect
	github.com/flosch/pongo2/v4 v4.0.2 // indirect
	github.com/gabriel-vasile/mimetype v1.4.8 // indirect
	github.com/gin-contrib/sse v1.0.0 // indirect
	// Gin 是一个用 Go 语言编写的高性能 Web 框架，专注于快速开发和高效运行。它被广泛用于构建 RESTful API、Web 应用和微服务。Gin 的设计目标是提供简洁的 API 和出色的性能，同时保持足够的灵活性。
	github.com/gin-gonic/gin v1.10.0
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.25.0 // indirect
	github.com/go-sql-driver/mysql v1.9.0
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/gocolly/colly v1.2.0
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gomarkdown/markdown v0.0.0-20240328165702-4d01890c35c0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/css v1.0.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.1
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/iris-contrib/schema v0.0.6 // indirect
	// GORM 是 Go 语言中最流行的 ORM（Object Relational Mapping，对象关系映射） 库，用于操作数据库。它让开发者可以使用 Go 结构体 代替 SQL 语句 来增删改查数据库数据，提供更清晰、简洁的数据库操作方式。
	github.com/jinzhu/gorm v1.9.16
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kataras/blocks v0.0.8 // indirect
	github.com/kataras/golog v0.1.11 // indirect
	github.com/kataras/iris/v12 v12.2.11
	github.com/kataras/pio v0.0.13 // indirect
	github.com/kataras/sitemap v0.0.6 // indirect
	github.com/kataras/tunnel v0.0.4 // indirect
	github.com/kennygrant/sanitize v1.2.4 // indirect
	github.com/klauspost/compress v1.17.7 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mailgun/raymond/v2 v2.0.48 // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/microcosm-cc/bluemonday v1.0.26 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/saintfish/chardet v0.0.0-20230101081208-5e3ef4b5456d // indirect
	github.com/schollz/closestmatch v2.1.0+incompatible // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	// cora 命令行界面工具
	github.com/spf13/cobra v1.9.1
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/tdewolff/minify/v2 v2.20.19 // indirect
	github.com/tdewolff/parse/v2 v2.7.12 // indirect
	github.com/temoto/robotstxt v1.1.2 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	// com库由 Unknwon（Gogs 和 Gitea 的作者）开发的 Go 语言工具库。它提供了一些常用的工具函数和实用方法，旨在简化 Go 开发中的一些常见任务。
	github.com/unknwon/com v1.0.1
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/yosssi/ace v0.0.5 // indirect
	golang.org/x/arch v0.14.0 // indirect
	golang.org/x/crypto v0.35.0 // indirect
	golang.org/x/exp v0.0.0-20240404231335-c0f41cb1a7a0 // indirect
	golang.org/x/net v0.36.0
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	golang.org/x/time v0.8.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250303144028-a0af3efb3deb // indirect
	// rpc、grpc、protobuf 框架
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250227231956-55c901821b1e // indirect
	google.golang.org/grpc v1.70.0
	google.golang.org/protobuf v1.36.5
	gopkg.in/ini.v1 v1.67.0
	gopkg.in/yaml.v3 v3.0.1
)

// api文档编写swagger：go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger
require github.com/fvbock/endless v0.0.0-20170109170031-447134032cb6

// gin-swagger安装及其UI：go get -u github.com/swaggo/gin-swagger
// go get -u github.com/swaggo/files
require (
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/spec v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/swaggo/files v1.0.1
	github.com/swaggo/gin-swagger v1.6.0
	github.com/swaggo/swag v1.16.4
	golang.org/x/tools v0.30.0 // indirect
)

require (
	github.com/boombuler/barcode v1.0.2
	github.com/fsnotify/fsnotify v1.8.0
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0
	github.com/gomodule/redigo v1.9.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.4.0
	github.com/robfig/cron v1.2.0
	github.com/spf13/viper v1.20.0
	github.com/tealeg/xlsx v1.0.5
	go.uber.org/zap v1.18.1
	gorm.io/driver/mysql v1.5.7
	gorm.io/gorm v1.25.12
)

require (
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/sagikazarmark/locafero v0.7.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.12.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.9.0 // indirect
	golang.org/x/image v0.25.0 // indirect
)

replace (
	//PracticeGenExample下的替代本地文件
	github.com/EDDYCJY/go-gin-example/conf => ./TestNotes/GenPracticeExample/pkg/conf
	github.com/EDDYCJY/go-gin-example/middleware => ./TestNotes/GenPracticeExample/middleware
	github.com/EDDYCJY/go-gin-example/models => ./TestNotes/GenPracticeExample/models
	github.com/EDDYCJY/go-gin-example/pkg/setting => ./TestNotes/GenPracticeExample/pkg/setting
	github.com/EDDYCJY/go-gin-example/routers => ./TestNotes/GenPracticeExample/routers
)

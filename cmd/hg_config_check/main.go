package main

import (
	ConfigPackage "MLC_GO/internal/pkg/config"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

// main 为 VS Code preLaunchTask 提供统一的基础设施配置读取和连通性检查入口。
//
// 命令复用应用真实的模块化配置加载逻辑，避免 Shell 脚本自行解析 YAML 后产生配置偏差：
//  1. --env 选择 debug、pre 或 prod 环境。
//  2. --config-dir 指定 config 根目录，与应用的 MLC_CONFIG_DIR 语义一致。
//  3. --check=mysql/redis 执行真实依赖探活；不传 --check 时只输出非敏感地址。
//
// 该命令只能用于启动前检查，不应在请求路径中执行。
func main() {
	// flag.Parse 必须在读取参数值前执行，否则始终只能得到默认值【在终端如脚本：xxx.sh --env=pre --config-dir=/xxx/config --check=mysql, 第三个参数在 -h、-help时才显示。】。
	env := flag.String("env", "debug", "运行环境：debug、pre、prod")
	configDir := flag.String("config-dir", "./config", "配置根目录")
	check := flag.String("check", "", "依赖检查：mysql 或 redis；为空时只输出非敏感地址")

	// flag.Parse() 和 flag.String() 属于 Go 标准库 flag 包，主要作用是：解析命令行参数，让 Go 程序启动时可以通过命令行传入配置
	flag.Parse()

	// LoadConfig 通过 MLC_CONFIG_DIR 定位 base 和环境目录，因此先写入当前进程环境。
	// 该变量只影响此短生命周期检查进程，不会修改调用方或 VS Code 的环境。
	// 设置环境变量，类似Linux中配置： export MLC_CONFIG_DIR=127.0.0.1 
	if err := os.Setenv("MLC_CONFIG_DIR", *configDir); err != nil {
		exitWithError(err)
	}
	// 先完成 base + 当前环境配置合并，再读取经过类型校验的 MySQL/Redis 配置。
	if err := ConfigPackage.LoadConfig(*env); err != nil {
		exitWithError(err)
	}
	mysqlConfig, err := ConfigPackage.GetMySQLConfig()
	if err != nil {
		exitWithError(err)
	}
	redisConfig, err := ConfigPackage.GetRedisConfig()
	if err != nil {
		exitWithError(err)
	}

	switch *check {
	case "":
		// 输出协议固定为 MySQL host、MySQL port、Redis host、Redis port 四行。
		// scripts/ensure_debug_deps.sh 按该顺序读取，禁止调整顺序或输出用户、密码、DSN。
		fmt.Println(mysqlConfig.Host)
		fmt.Println(mysqlConfig.Port)
		fmt.Println(redisConfig.Host)
		fmt.Println(redisConfig.Port)
	case "mysql":
		checkMySQL(mysqlConfig)
	case "redis":
		checkRedis(redisConfig)
	default:
		exitWithError(fmt.Errorf("不支持的检查类型 %q", *check))
	}
}

// checkMySQL 使用与应用相同的连接参数执行启动前 Ping。
// sql.Open 只创建连接池句柄，PingContext 才会验证地址、认证信息和服务可用性。
func checkMySQL(cfg ConfigPackage.HGMySQLConfig) {
	// 使用驱动提供的 Config 构造 DSN，避免手工拼接时遗漏转义；DSN 不得输出到日志。
	// 将 mysql.Config 结构体中的数据库连接信息安全、规范地转换成 MySQL Driver 能识别的 DSN 字符串
	dsn := (&mysql.Config{User: cfg.User, Passwd: cfg.Password, Net: "tcp", Addr: cfg.Host + ":" + cfg.Port}).FormatDSN()
	// Open方法只是创建数据库连接池对象
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		exitWithError(err)
	}
	defer db.Close()
	// 限制前置检查耗时，防止依赖不可达时 VS Code 调试任务长期阻塞。
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// 真正数据库连接是dp.Ping()
	if err := db.PingContext(ctx); err != nil {
		exitWithError(err)
	}
}

// checkRedis 创建短生命周期客户端并通过 PING 验证 Redis 地址和服务状态。
func checkRedis(cfg ConfigPackage.HGRedisConfig) {
	client := redis.NewClient(&redis.Options{Addr: cfg.Host + ":" + cfg.Port})
	defer client.Close()
	// 与 MySQL 使用相同的快速失败边界，保证 preLaunchTask 可预测退出。
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		exitWithError(err)
	}
}

// exitWithError 将错误写入标准错误并使用非零状态码退出。
// 调用方只依赖退出码判断检查结果，错误内容不得包含密码或完整连接串。
func exitWithError(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

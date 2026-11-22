package main

import (
	NotificationPackage "MLC_GO/internal/modules/notification"
	ClickHouse "MLC_GO/internal/pkg/clickhouse"
	ConfigPackage "MLC_GO/internal/pkg/config"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

// main 为 VS Code preLaunchTask 提供统一的基础设施配置读取和连通性检查入口。
//
// 命令复用应用真实的模块化配置加载逻辑，避免 Shell 脚本自行解析 YAML 后产生配置偏差：
//  1. --env 选择 debug、pre 或 prod 环境。
//  2. --config-dir 指定 config 根目录，与应用的 MLC_CONFIG_DIR 语义一致。
//  3. --check=mysql/redis/clickhouse 执行真实依赖探活；不传 --check 时只输出非敏感地址。
//
// 该命令只能用于启动前检查，不应在请求路径中执行。
func main() {
	// flag.Parse 必须在读取参数值前执行，否则始终只能得到默认值【在终端如脚本：xxx.sh --env=pre --config-dir=/xxx/config --check=mysql, 第三个参数在 -h、-help时才显示。】。
	env := flag.String("env", "debug", "运行环境：debug、pre、prod")
	configDir := flag.String("config-dir", "./config", "配置根目录")
	check := flag.String("check", "", "只读检查：mysql、redis、clickhouse、migration-version、iam-scope-types、iam-role-template、notification-counter-ready、notification-send-schema、notification-send-kafka-schema 或 notification-send-kafka-queries；为空时只输出非敏感地址")
	migrationsDir := flag.String("migrations-dir", "", "执行该目录中的 MySQL migrations；为空时不迁移")

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
	if *migrationsDir != "" {
		if *check != "" {
			exitWithError(fmt.Errorf("--migrations-dir 不能与 --check 同时使用"))
		}
		if err := runMySQLMigrations(mysqlConfig, *migrationsDir); err != nil {
			exitWithError(err)
		}
		return
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
	case "iam-scope-types":
		if hgErr := hgCheckIAMScopeTypes(mysqlConfig); hgErr != nil {
			exitWithError(hgErr)
		}
	case "migration-version":
		// 只读schema_migrations，不执行DDL，不输出连接凭据。
		hgDB, hgErr := sql.Open("mysql", (&mysql.Config{User: mysqlConfig.User, Passwd: mysqlConfig.Password, Net: "tcp", Addr: net.JoinHostPort(mysqlConfig.Host, mysqlConfig.Port), DBName: mysqlConfig.Database}).FormatDSN())
		if hgErr != nil {
			exitWithError(hgErr)
		}
		defer hgDB.Close()
		hgCtx, hgCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer hgCancel()
		var hgVersion uint64
		var hgDirty bool
		if hgErr = hgDB.QueryRowContext(hgCtx, SQLQueriesPackage.HGGetCrawlerMigrationVersionSQL).Scan(&hgVersion, &hgDirty); hgErr != nil {
			exitWithError(hgErr)
		}
		fmt.Printf("migration_version=%d dirty=%t\n", hgVersion, hgDirty)
	case "iam-role-template":
		hgDB, hgErr := sql.Open("mysql", (&mysql.Config{User: mysqlConfig.User, Passwd: mysqlConfig.Password, Net: "tcp", Addr: net.JoinHostPort(mysqlConfig.Host, mysqlConfig.Port), DBName: mysqlConfig.Database}).FormatDSN())
		if hgErr != nil {
			exitWithError(fmt.Errorf("角色模板状态检查不可用"))
		}
		defer hgDB.Close()
		hgCtx, hgCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer hgCancel()
		var latest uint32
		var published sql.NullInt64
		var status string
		var realtime bool
		hgErr = hgDB.QueryRowContext(hgCtx, SQLQueriesPackage.HGIAMRoleTemplateStateSQL).Scan(&latest, &published, &status, &realtime)
		if hgErr == sql.ErrNoRows {
			fmt.Println("iam_role_template_exists=false")
			return
		}
		if hgErr != nil {
			exitWithError(fmt.Errorf("角色模板状态检查失败"))
		}
		var title, body string
		var schema []byte
		hgErr = hgDB.QueryRowContext(hgCtx, SQLQueriesPackage.HGIAMRoleTemplateDraftSQL).Scan(&title, &body, &schema)
		hgValid := hgErr == nil && NotificationPackage.HGValidateRolePublished(title, body, schema, []byte(`["in_app","realtime"]`)) == nil
		fmt.Printf("iam_role_template_exists=true latest_version=%d published_version=%d status=%s latest_realtime=%t publishable_with_realtime=%t\n", latest, published.Int64, status, realtime, hgValid)
	case "notification-counter-ready":
		// 只读验证新投影是否具备完整基线；不查询用户数据，不输出数据库凭据。
		hgDB, hgErr := sql.Open("mysql", (&mysql.Config{User: mysqlConfig.User, Passwd: mysqlConfig.Password, Net: "tcp", Addr: net.JoinHostPort(mysqlConfig.Host, mysqlConfig.Port), DBName: mysqlConfig.Database}).FormatDSN())
		if hgErr != nil {
			exitWithError(hgErr)
		}
		defer hgDB.Close()
		hgCtx, hgCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer hgCancel()
		var hgReady bool
		if hgErr = hgDB.QueryRowContext(hgCtx, SQLQueriesPackage.HGNotificationCounterReadySQL).Scan(&hgReady); hgErr != nil {
			exitWithError(hgErr)
		}
		fmt.Printf("notification_counter_ready=%t\n", hgReady)
	case "notification-send-schema":
		hgDB, hgErr := sql.Open("mysql", (&mysql.Config{User: mysqlConfig.User, Passwd: mysqlConfig.Password, Net: "tcp", Addr: net.JoinHostPort(mysqlConfig.Host, mysqlConfig.Port), DBName: mysqlConfig.Database}).FormatDSN())
		if hgErr != nil {
			exitWithError(fmt.Errorf("通知发送结构检查不可用"))
		}
		defer hgDB.Close()
		hgCtx, hgCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer hgCancel()
		if hgErr = hgCheckNotificationSendSchema(hgCtx, hgDB); hgErr != nil {
			exitWithError(hgErr)
		}
		fmt.Println("notification_send_schema_ready=true")
	case "notification-send-kafka-schema":
		hgDB, hgErr := sql.Open("mysql", (&mysql.Config{User: mysqlConfig.User, Passwd: mysqlConfig.Password, Net: "tcp", Addr: net.JoinHostPort(mysqlConfig.Host, mysqlConfig.Port), DBName: mysqlConfig.Database}).FormatDSN())
		if hgErr != nil {
			exitWithError(fmt.Errorf("通知 Kafka 结构检查不可用"))
		}
		defer hgDB.Close()
		hgCtx, hgCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer hgCancel()
		if hgErr = hgCheckNotificationKafkaSchema(hgCtx, hgDB); hgErr != nil {
			exitWithError(hgErr)
		}
		fmt.Println("notification_send_kafka_schema_ready=true")
	case "notification-send-kafka-queries":
		hgDB, hgErr := sql.Open("mysql", (&mysql.Config{User: mysqlConfig.User, Passwd: mysqlConfig.Password, Net: "tcp", Addr: net.JoinHostPort(mysqlConfig.Host, mysqlConfig.Port), DBName: mysqlConfig.Database}).FormatDSN())
		if hgErr != nil {
			exitWithError(fmt.Errorf("通知 Kafka SQL 检查不可用"))
		}
		defer hgDB.Close()
		hgCtx, hgCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer hgCancel()
		if hgErr = hgCheckNotificationKafkaQueries(hgCtx, hgDB); hgErr != nil {
			exitWithError(hgErr)
		}
		fmt.Println("notification_send_kafka_queries_ready=true")
	case "redis":
		checkRedis(redisConfig)
	case "clickhouse":
		hgCheckClickHouse()
	default:
		exitWithError(fmt.Errorf("不支持的检查类型 %q", *check))
	}
}

// hgCheckNotificationSendSchema 只读校验新版本依赖的结构；无误后仍需隔离环境功能验收。
func hgCheckNotificationSendSchema(ctx context.Context, db *sql.DB) error {
	for _, check := range []struct {
		query string
		want  int
	}{
		{SQLQueriesPackage.HGNotificationSendSchemaColumnsSQL, 3},
		{SQLQueriesPackage.HGNotificationSendSchemaIndexesSQL, 3},
		{SQLQueriesPackage.HGNotificationSendSchemaAuditTablesSQL, 2},
		{SQLQueriesPackage.HGNotificationSendSchemaPermissionSQL, 1},
	} {
		var found int
		if err := db.QueryRowContext(ctx, check.query).Scan(&found); err != nil {
			return fmt.Errorf("通知发送结构检查不可用")
		}
		if found != check.want {
			return fmt.Errorf("通知发送结构未就绪")
		}
	}
	return nil
}

// hgCheckNotificationKafkaSchema 只核对 000086 所需的列与索引，不查询接收人和通知正文。
func hgCheckNotificationKafkaSchema(ctx context.Context, db *sql.DB) error {
	for _, check := range []struct {
		query string
		want  int
	}{
		{SQLQueriesPackage.HGNotificationKafkaSchemaColumnsSQL, 3},
		{SQLQueriesPackage.HGNotificationKafkaSchemaIndexesSQL, 2},
	} {
		var found int
		if err := db.QueryRowContext(ctx, check.query).Scan(&found); err != nil || found != check.want {
			return fmt.Errorf("通知 Kafka 结构未就绪")
		}
	}
	return nil
}

// hgCheckNotificationKafkaQueries 在短事务中仅验证真实 MySQL 的索引查询与行锁语法，绝不更新数据。
func hgCheckNotificationKafkaQueries(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("通知 Kafka SQL 检查不可用")
	}
	defer tx.Rollback()
	for _, query := range []string{
		SQLQueriesPackage.HGNotificationKafkaReadySQL,
		SQLQueriesPackage.HGNotificationKafkaQueuedExpiredSQL,
		SQLQueriesPackage.HGNotificationKafkaLeasedExpiredSQL,
	} {
		var id, generation uint64
		var next, total int
		if err := tx.QueryRowContext(ctx, query).Scan(&id, &next, &generation, &total); err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("通知 Kafka SQL 检查失败")
		}
	}
	var template uint64
	var version uint32
	var audience, variables []byte
	var index, attempts int
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.HGNotificationKafkaClaimSQL, 0, 0, 0).
		Scan(&template, &version, &audience, &variables, &index, &attempts); err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("通知 Kafka 领取 SQL 检查失败")
	}
	return nil
}

// hgCheckClickHouse 复用主服务连接配置与 SELECT 1 探活，不输出认证信息或连接串。
func hgCheckClickHouse() {
	cfg, _, err := ConfigPackage.GetStatisticInfrastructureConfig()
	if err != nil {
		exitWithError(fmt.Errorf("读取 ClickHouse 配置失败"))
	}
	if !cfg.Enabled {
		exitWithError(fmt.Errorf("ClickHouse 未启用"))
	}
	client, err := ClickHouse.NewHGClient(ClickHouse.HGConfig{
		Endpoint: fmt.Sprintf("%s://%s:%s", cfg.Scheme, cfg.Host, cfg.Port),
		Database: cfg.Database, Username: cfg.User, Password: cfg.Password,
		StatisticEventsTable: cfg.StatisticEventsTable, StatisticTotalsTable: cfg.StatisticTotalsTable,
		DanmakuHistoryTable: cfg.DanmakuHistoryTable,
		WriteTimeout:        cfg.WriteTimeoutDuration, QueryTimeout: cfg.QueryTimeoutDuration,
	})
	if err != nil {
		exitWithError(fmt.Errorf("ClickHouse 客户端配置失败"))
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.QueryTimeoutDuration)
	defer cancel()
	if err := client.PingContext(ctx); err != nil {
		exitWithError(fmt.Errorf("ClickHouse 就绪检查失败"))
	}
}

// buildMySQLMigrationURL 构造 golang-migrate 使用的 MySQL URL，并转义账号信息中的保留字符。
func buildMySQLMigrationURL(cfg ConfigPackage.HGMySQLConfig) string {
	// credentials 生成用户名密码部分。避免拼接，避免中间反应偶特殊符号比如@可以自动转义
	credentials := url.UserPassword(cfg.User, cfg.Password).String()
	address := net.JoinHostPort(cfg.Host, cfg.Port)
	// PathEscape 转义数据库名字
	return fmt.Sprintf("mysql://%s@tcp(%s)/%s", credentials, address, url.PathEscape(cfg.Database))
}

// runMySQLMigrations 使用项目约定的 golang-migrate CLI 将数据库升级到 migrations 目录最新版本。
func runMySQLMigrations(cfg ConfigPackage.HGMySQLConfig, migrationsDir string) error {
	// 查找系统有没有 migrate 命令，确保在执行迁移前已经安装了 golang-migrate 工具。
	migratePath, err := exec.LookPath("migrate")
	if err != nil {
		return fmt.Errorf("未找到 migrate 命令，请先安装 golang-migrate: %w", err)
	}

	// exec.Command() 相当于在终端执行相关命令
	// migratePath 是 migrate 命令的路径
	// -path 迁移目录 migrationsDir 迁移目录
	// -database 数据库连接信息 buildMySQLMigrationURL(cfg)
	cmd := exec.Command(migratePath, "-path", migrationsDir, "-database", buildMySQLMigrationURL(cfg), "up")
	// 把 migrate 输出显示到当前终端。
	cmd.Stdout = os.Stdout
	// 错误输出。
	cmd.Stderr = os.Stderr
	// cmd.Run() 运行命令
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("执行数据库迁移失败: %w", err)
	}
	return nil
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

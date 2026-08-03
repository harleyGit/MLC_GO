package main

import (
	VideoCommentBackfillPackage "MLC_GO/internal/modules/video_comment/backfill"
	ConfigPackage "MLC_GO/internal/pkg/config"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
)

func main() {
	// 该命令是发布期一次性数据任务，不由应用副本自动启动；pre/prod 必须先完成 migration 23，并观察复制延迟和 redo/binlog 压力。
	env := flag.String("env", "debug", "运行环境：debug、pre、prod")
	configDir := flag.String("config-dir", "./config", "配置根目录")
	batchSize := flag.Int("batch-size", 10000, "每事务最大 reaction 行数，1-100000")
	pause := flag.Duration("pause", 100*time.Millisecond, "批次间暂停时间")
	flag.Parse()
	if err := os.Setenv("MLC_CONFIG_DIR", *configDir); err != nil {
		exit(err)
	}
	if err := ConfigPackage.LoadConfig(*env); err != nil {
		exit(err)
	}
	cfg, err := ConfigPackage.GetMySQLConfig()
	if err != nil {
		exit(err)
	}
	dsn := (&mysql.Config{User: cfg.User, Passwd: cfg.Password, Net: "tcp", Addr: net.JoinHostPort(cfg.Host, cfg.Port), DBName: cfg.Database, ParseTime: true}).FormatDSN()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		exit(err)
	}
	defer db.Close()
	backfill := VideoCommentBackfillPackage.NewHGReactionBackfill(db)
	for {
		// 每批使用独立超时，慢 SQL 或数据库故障只回滚当前区间；已提交 checkpoint 可供下次进程继续。
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		processed, completed, runErr := backfill.RunBatch(ctx, *batchSize)
		cancel()
		if runErr != nil {
			exit(runErr)
		}
		if completed {
			fmt.Println("video comment reaction shard backfill completed")
			return
		}
		if processed && *pause > 0 {
			// 主动留出复制和前台流量资源；生产值应根据 pre 的 redo、binlog、锁等待和 replica lag 调整。
			time.Sleep(*pause)
		}
	}
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// hg_crawler_backfill 在本机 debug 环境按主键分页补采已存在列表的缺失详情。
package main

import (
	hgRepository "MLC_GO/internal/modules/crawler/repository"
	hgConfig "MLC_GO/internal/pkg/config"
	hgMySQL "MLC_GO/internal/pkg/mysql"
	hgQueries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// 这是一个 B 站历史视频详情补采工具（Backfill），主要用于处理之前已经爬取了视频列表、但还没有采集视频详情的数据。
// 它提供两种运行模式：
// - -apply：补采模式，扫描历史视频列表，向数据库提交视频发现事件，交给后续 Kafka 爬虫流水线处理。
// - -verify：核对模式，只读查询数据库，统计哪些视频已经入库、哪些仍未完成或采集失败。
// 注意：-apply 并不是直接请求 B 站抓取视频详情，而是提交发现事件，触发后续异步处理。
/*执行流程：
	main()
	|
解析命令行参数
	|
检查 debug 环境
	|
	hgRun()
	|
初始化配置和 MySQL
	|
+------+------+
|             |
-apply        -verify
|             |
扫描历史列表    查询入库状态
|             |
分页提交事件    统计成功/失败
|             |
Kafka流水线     输出核对结果
|
抓取 → 清洗 → 存储
*/
func main() {
	// 定义命令行参数；flag 是 Go 标准库，用于读取终端启动程序时传入的参数。
	/* -apply	false	开启补采模式
	-verify	false	开启核对模式
	-after	0	从指定数据库主键之后继续
	-pages	10	最多处理多少页
	例如：go run . -apply -pages=10 -after=0

	表示从主键 0 之后开始，最多处理 10 页历史数据。
	如果每页 100 条，那么最多扫描约 1000 条。
	*/
	hgApply := flag.Bool("apply", false, "明确授权写入本机 debug 详情发现事件")
	hgVerify := flag.Bool("verify", false, "只读核对已有列表详情及统计入库状态")
	hgAfter := flag.Uint64("after", 0, "上次已提交页的主键游标")
	hgPages := flag.Int("pages", 10, "最多处理页数，每页100条，上限100")
	flag.Parse()
	if (!*hgApply && !*hgVerify) || *hgPages < 1 || *hgPages > 100 {
		fmt.Println("需指定 -apply，pages 范围1至100")
		os.Exit(1)
	}
	if hgEnv := os.Getenv("SERVER_ENV"); hgEnv != "" && hgEnv != "debug" {
		fmt.Println("仅允许 debug 环境")
		os.Exit(1)
	}
	os.Setenv("SERVER_ENV", "debug")
	if hgErr := hgRun(*hgAfter, *hgPages, *hgVerify); hgErr != nil {
		fmt.Println("补采失败:", hgErr)
		os.Exit(1)
	}
}

func hgRun(hgAfter uint64, hgPages int, hgVerify bool) error {
	// 初始化运行环境
	if hgErr := hgConfig.InitRuntimeEnv(); hgErr != nil {
		return hgErr
	}
	// 加载 debug 配置。
	if hgErr := hgConfig.LoadConfig("debug"); hgErr != nil {
		return hgErr
	}
	hgDB, hgErr := hgMySQL.NewSQLManager()
	if hgErr != nil {
		return hgErr
	}
	defer hgDB.Close()
	if hgVerify {
		hgCtx, hgCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer hgCancel()
		// 使用 Map 统计各种采集状态。
		hgCounts := map[string]int{}
		for hgPage := 0; hgPage < hgPages; hgPage++ {
			// hgAfter 是游标，表示查询上次处理的主键之后的数据。
			hgRows, hgErr := hgDB.GetSQLDB().QueryContext(hgCtx, hgQueries.HGBackfillVideoVerifySQL, hgAfter)
			if hgErr != nil {
				return hgErr
			}
			hgPrevious := hgAfter
			for hgRows.Next() {
				var hgBVID, hgStatus string
				var hgStored bool
				// hgAfter	当前记录主键，同时更新游标
				// hgBVID	B 站视频 BV 号
				// hgStored	是否已经存储详情
				// hgStatus	当前采集状态
				if hgErr = hgRows.Scan(&hgAfter, &hgBVID, &hgStored, &hgStatus); hgErr != nil {
					hgRows.Close()
					return hgErr
				}
				// 已经入库的记录累计到 stored，其他记录按照状态分别计数。
				if hgStored {
					hgCounts["stored"]++
				} else {
					hgCounts[hgStatus]++
					// 打印失败视频，方便排查。
					if strings.HasPrefix(hgStatus, "failed") {
						fmt.Printf("失败视频 %s %s\n", hgBVID, hgStatus)
					}
				}
			}
			hgErr = hgRows.Err()
			hgRows.Close()
			if hgErr != nil {
				return hgErr
			}
			// 如果查询结束后游标没有变化，说明本页没有读到新记录，核对完成。
			if hgAfter == hgPrevious {
				fmt.Printf("完整核对结果 %v\n", hgCounts)
				return nil
			}
		}
		return fmt.Errorf("核对达到页数上限，部分结果=%v，游标=%d", hgCounts, hgAfter)
	}

	// -apply：补采模式
	// 创建数据库 Repository，并开启视频流水线相关逻辑。
	hgRepo := hgRepository.NewRepository(hgDB.GetSQLDB()).WithHGVideoPipeline(true)
	hgTotal := 0
	// 循环分页补采；每次循环处理一页数据。与核对模式不同，这里每一页都有独立的 15 秒超时时间。
	for hgPage := 0; hgPage < hgPages; hgPage++ {
		hgCtx, hgCancel := context.WithTimeout(context.Background(), 15*time.Second)
		// 结合之前的 hgDiscoverVideos()，这些发现事件通常会写入事务中的事件表，再由后续事件投递机制交给 Kafka。因此，hgCount 不代表视频详情已经成功抓取或存储。
		hgNext, hgCount, hgErr := hgRepo.HGBackfillVideoPage(hgCtx, hgAfter)
		hgCancel()
		if hgErr != nil {
			return hgErr
		}

		// 统计提交数量
		hgTotal += hgCount
		fmt.Printf("已提交 cursor=%d discovered=%d total=%d\n", hgNext, hgCount, hgTotal)
		// 如果新旧游标相同，说明没有继续推进，程序认为扫描完成。否则更新游标，继续下一页。
		if hgNext == hgAfter {
			fmt.Println("历史列表扫描完成")
			return nil
		}
		hgAfter = hgNext
	}
	fmt.Printf("达到页数上限，续跑使用 -after=%d\n", hgAfter)
	return nil
}

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

func main() {
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
	if hgErr := hgConfig.InitRuntimeEnv(); hgErr != nil {
		return hgErr
	}
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
		hgCounts := map[string]int{}
		for hgPage := 0; hgPage < hgPages; hgPage++ {
			hgRows, hgErr := hgDB.GetSQLDB().QueryContext(hgCtx, hgQueries.HGBackfillVideoVerifySQL, hgAfter)
			if hgErr != nil {
				return hgErr
			}
			hgPrevious := hgAfter
			for hgRows.Next() {
				var hgBVID, hgStatus string
				var hgStored bool
				if hgErr = hgRows.Scan(&hgAfter, &hgBVID, &hgStored, &hgStatus); hgErr != nil {
					hgRows.Close()
					return hgErr
				}
				if hgStored {
					hgCounts["stored"]++
				} else {
					hgCounts[hgStatus]++
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
			if hgAfter == hgPrevious {
				fmt.Printf("完整核对结果 %v\n", hgCounts)
				return nil
			}
		}
		return fmt.Errorf("核对达到页数上限，部分结果=%v，游标=%d", hgCounts, hgAfter)
	}
	hgRepo := hgRepository.NewRepository(hgDB.GetSQLDB()).WithHGVideoPipeline(true)
	hgTotal := 0
	for hgPage := 0; hgPage < hgPages; hgPage++ {
		hgCtx, hgCancel := context.WithTimeout(context.Background(), 15*time.Second)
		hgNext, hgCount, hgErr := hgRepo.HGBackfillVideoPage(hgCtx, hgAfter)
		hgCancel()
		if hgErr != nil {
			return hgErr
		}
		hgTotal += hgCount
		fmt.Printf("已提交 cursor=%d discovered=%d total=%d\n", hgNext, hgCount, hgTotal)
		if hgNext == hgAfter {
			fmt.Println("历史列表扫描完成")
			return nil
		}
		hgAfter = hgNext
	}
	fmt.Printf("达到页数上限，续跑使用 -after=%d\n", hgAfter)
	return nil
}

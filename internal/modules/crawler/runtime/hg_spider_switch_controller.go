package runtime

import (
	"context"
	"fmt"
	"log"
	"time"

	CrawlerSpiderPackage "MLC_GO/internal/modules/crawler/spider"
)

// HGSpiderSwitchReader 从数据库读取推荐 worker 的权威期望状态。
type HGSpiderSwitchReader interface {
	HGDesiredRunning(context.Context) (bool, error)
}

// HGRunSpiderSwitch 启动时读取状态、随后每2秒协调一次；数据库不可达时停止 worker，恢复后重新对齐。
// 仅供单实例独立 crawler 服务使用；多副本部署需另行引入分布式租约，不能直接运行多个实例。
func HGRunSpiderSwitch(ctx context.Context, reader HGSpiderSwitchReader, manager *CrawlerSpiderPackage.HGManager) error {
	if reader == nil || manager == nil {
		return fmt.Errorf("推荐爬虫开关依赖不可用")
	}
	// 第一次读取失败必须阻止 HTTP 服务启动，避免迁移缺失时退回无保护的自动采集。
	if err := hgReconcileSpiderSwitch(ctx, reader, manager); err != nil {
		return err
	}
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		var lastError string
		for {
			select {
			case <-ctx.Done():
				manager.Stop()
				return
			case <-ticker.C:
				if err := hgReconcileSpiderSwitch(ctx, reader, manager); err != nil {
					manager.Stop() // 数据库失联时不能继续无限期抓取。
					if err.Error() != lastError {
						log.Printf("推荐爬虫开关同步失败: %v", err)
						lastError = err.Error()
					}
				} else {
					lastError = ""
				}
			}
		}
	}()
	return nil
}

func hgReconcileSpiderSwitch(ctx context.Context, reader HGSpiderSwitchReader, manager *CrawlerSpiderPackage.HGManager) error {
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	desired, err := reader.HGDesiredRunning(readCtx)
	if err != nil {
		return fmt.Errorf("读取推荐爬虫期望状态: %w", err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if desired && !manager.HGRunning() {
		return manager.Start()
	}
	if !desired && manager.HGRunning() {
		manager.Stop()
	}
	return nil
}

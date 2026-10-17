package main

import (
	CrawlerPlatformPackage "MLC_GO/internal/modules/crawler/platform"
	CrawlerRepositoryPackage "MLC_GO/internal/modules/crawler/repository"
	VideoUploadRepositoryPackage "MLC_GO/internal/modules/video_upload/repository"
	ClickHousePackage "MLC_GO/internal/pkg/clickhouse"
	ConfigPackage "MLC_GO/internal/pkg/config"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestHGCrawlerPipelineIntegration 仅显式授权后执行：真实抓取一条推荐并写MySQL/Redis/Kafka/ClickHouse。
// 保留采集记录用于页面验收，不删除业务数据；不在测试内执行迁移或建topic。
func TestHGCrawlerPipelineIntegration(t *testing.T) {
	if os.Getenv("HG_CRAWLER_PIPELINE_INTEGRATION") != "1" {
		t.Skip("真实采集验收未授权")
	}
	t.Setenv("SERVER_ENV", "debug")
	t.Setenv("CRAWLER_VIDEO_PIPELINE_ENABLED", "true")
	if err := ConfigPackage.InitRuntimeEnv(); err != nil {
		t.Fatal(err)
	}
	if err := ConfigPackage.LoadConfig("debug"); err != nil {
		t.Fatal(err)
	}
	db, err := PersistenceSQLPackage.NewSQLManager()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	redis, err := PersistenceRedisPackage.NewRedisServiceWithError(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer redis.Close()
	closeRuntime, _, err := initKafkaWithDependencies(redis, db)
	if err != nil {
		t.Fatal(err)
	}
	defer closeRuntime()
	hgCHConfig, _, err := ConfigPackage.GetStatisticInfrastructureConfig()
	if err != nil {
		t.Fatal(err)
	}
	hgCH, err := ClickHousePackage.NewHGClient(ClickHousePackage.HGConfig{Endpoint: fmt.Sprintf("%s://%s:%s", hgCHConfig.Scheme, hgCHConfig.Host, hgCHConfig.Port), Database: hgCHConfig.Database, Username: hgCHConfig.User, Password: hgCHConfig.Password, WriteTimeout: hgCHConfig.WriteTimeoutDuration, QueryTimeout: hgCHConfig.QueryTimeoutDuration})
	if err != nil {
		t.Fatal(err)
	}
	defer hgCH.Close()
	client, err := CrawlerPlatformPackage.NewHGBilibiliPlatform(nil, CrawlerPlatformPackage.HGBilibiliConfig{MaxItems: 1, RequestTimeout: 10 * time.Second, RetryCount: 0, RatePerSecond: 1})
	if err != nil {
		t.Fatal(err)
	}
	items, err := client.FetchRecommendations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatal("推荐未返回一条有效视频")
	}
	repo := CrawlerRepositoryPackage.NewRepository(db.GetSQLDB()).WithHGVideoPipeline(true)
	hgStartedAt := time.Now().UTC()
	if err := repo.UpsertRecommendations(ctx, items); err != nil {
		t.Fatal(err)
	}
	reader := VideoUploadRepositoryPackage.NewRepository(db.GetSQLDB())
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		item, found, readErr := reader.GetVideoListItemByID(ctx, "external:bilibili:"+items[0].ContentID)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if found && item.HGExternalDetail != nil {
			if item.HGExternalDetail.Status == "failed" {
				t.Fatalf("详情采集失败: %s", item.HGExternalDetail.ErrorMessage)
			}
			if item.HGExternalDetail.Status == "stored" {
				if len(item.HGExternalDetail.Parts) == 0 || item.HGExternalDetail.CollectedAt == "" {
					t.Fatal("已入库状态缺少分P或采集时间")
				}
				hgTime, parseErr := time.Parse(time.RFC3339Nano, item.HGExternalDetail.CollectedAt)
				if parseErr != nil {
					t.Fatal(parseErr)
				}
				hgStored, historyErr := hgCH.HGHasCrawlerHistory(ctx, items[0].ContentID, hgTime.UnixMilli())
				if historyErr != nil {
					t.Fatal(historyErr)
				}
				if hgStored && hgTime.After(hgStartedAt) {
					t.Logf("MySQL详情及ClickHouse历史均已落库 bvid=%s parts=%d description_chars=%d", items[0].ContentID, len(item.HGExternalDetail.Parts), len([]rune(item.Description)))
					return
				}
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal("等待详情流水线超时")
		case <-ticker.C:
		}
	}
}

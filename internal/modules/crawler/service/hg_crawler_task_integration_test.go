package service_test

import (
	CrawlerDtoPackage "MLC_GO/internal/modules/crawler/dto"
	CrawlerLeasePackage "MLC_GO/internal/modules/crawler/lease"
	CrawlerParserPackage "MLC_GO/internal/modules/crawler/parser"
	CrawlerRepositoryPackage "MLC_GO/internal/modules/crawler/repository"
	CrawlerServicePackage "MLC_GO/internal/modules/crawler/service"
	VideoUploadCachePackage "MLC_GO/internal/modules/video_upload/cache"
	ConfigPackage "MLC_GO/internal/pkg/config"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestHGTaskServiceIntegrationSaveAndRun(t *testing.T) {
	if os.Getenv("HG_RUN_CRAWLER_INTEGRATION") != "1" {
		t.Skip("set HG_RUN_CRAWLER_INTEGRATION=1 to use local MySQL, Redis, and Bilibili")
	}
	if err := os.Setenv("MLC_CONFIG_DIR", "../../../../config"); err != nil {
		t.Fatal(err)
	}
	if err := ConfigPackage.LoadConfig("debug"); err != nil {
		t.Fatal(err)
	}
	sqlManager, err := PersistenceSQLPackage.NewSQLManager()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlManager.Close()
	redisService, err := PersistenceRedisPackage.NewRedisServiceWithError(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer redisService.Close()

	config, err := ConfigPackage.GetCrawlerTaskConfig()
	if err != nil {
		t.Fatal(err)
	}
	policy, err := CrawlerServicePackage.NewHGTargetPolicy(config.AllowedHosts, config.AllowHTTP)
	if err != nil {
		t.Fatal(err)
	}
	httpService, err := CrawlerServicePackage.NewHGSafeHTTPService(policy, config.DefaultUserAgent)
	if err != nil {
		t.Fatal(err)
	}
	repository := CrawlerRepositoryPackage.NewRepository(sqlManager.GetSQLDB())
	store := CrawlerServicePackage.NewHGExternalContentStore(repository, VideoUploadCachePackage.NewCache(redisService))
	service, err := CrawlerServicePackage.NewHGTaskService(repository, httpService, CrawlerLeasePackage.NewHGRedisTaskLease(redisService), config.LeaseGrace, store)
	if err != nil {
		t.Fatal(err)
	}

	configuration, err := json.Marshal(CrawlerServicePackage.HGTaskConfiguration{
		Request: CrawlerDtoPackage.HGDebugRequest{
			URL: "https://api.bilibili.com/x/web-interface/wbi/index/top/feed/rcmd", Method: "GET",
			Headers: map[string]string{"Accept": "application/json", "Referer": "https://www.bilibili.com/"},
			Params:  map[string]string{"fresh_type": "3", "ps": "12"}, TimeoutMS: 10000,
		},
		Parser: CrawlerParserPackage.Config{
			Type: CrawlerParserPackage.TypeRestrictedJSONPath, Platform: "bilibili", ItemSelector: "$.data.item[*]",
			Fields: map[string]CrawlerParserPackage.FieldConfig{
				"contentId": {Selector: "$.bvid"}, "title": {Selector: "$.title"},
				"authorId": {Selector: "$.owner.mid"}, "authorName": {Selector: "$.owner.name"},
				"coverUrl": {Selector: "$.pic"}, "targetUrl": {Selector: "$.uri"},
				"durationSeconds": {Selector: "$.duration"}, "viewCount": {Selector: "$.stat.view"},
				"likeCount": {Selector: "$.stat.like"}, "commentCount": {Selector: "$.stat.danmaku"},
				"publishedAt": {Selector: "$.pubdate"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	definition, run, err := service.SaveAndRun(context.Background(), CrawlerDtoPackage.HGTaskDefinitionSaveRequest{
		Name: "B站视频采集", Platform: "bilibili", Enabled: true, Cron: "0 */10 * * * *",
		ParserType: string(CrawlerParserPackage.TypeRestrictedJSONPath), ItemPath: "$.data.item[*]", MaxItems: 12,
		Configuration: configuration,
	}, "integration-test")
	if err != nil {
		t.Fatalf("SaveAndRun() error = %v, definition=%#v run=%#v", err, definition, run)
	}
	if definition == nil || definition.ID == 0 || run == nil || run.Status != "succeeded" || run.ItemCount == 0 {
		t.Fatalf("SaveAndRun() definition=%#v run=%#v", definition, run)
	}
}

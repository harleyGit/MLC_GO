package module

import (
	module "MLC_GO/internal/handler"
	"MLC_GO/internal/modules/commerce"
	iam "MLC_GO/internal/modules/iam"
	config "MLC_GO/internal/pkg/config"
	router "MLC_GO/internal/pkg/hg_router"
	mysql "MLC_GO/internal/pkg/mysql"
	upload "MLC_GO/internal/pkg/upload"
	"net/http"
)

// HGModule 是独立生产 CNY 商品模块；暂不连接沙箱、支付、订单或钱包。
type HGModule struct {
	handler    *commerce.HGHandler      // 商品 HTTP 处理器。
	authorizer router.HGRouteAuthorizer // 复用 IAM 审核与商家角色权限。
}

func (m *HGModule) Name() string     { return "commerce" }
func (m *HGModule) BasePath() string { return router.HGCommerceBasePath }
func (m *HGModule) Handler() http.Handler {
	return router.HGNewCommerceRouteGroup(m.handler, m.authorizer)
}

// HGRegisterModules 使用现有 MySQL 和对象存储配置，不自动迁移、不发布测试商品。
func HGRegisterModules(manager *mysql.HGSQLManager) {
	if manager == nil {
		panic("人民币商品模块需要数据库管理器")
	}
	cfg, err := config.GetVideoCommentConfig()
	if err != nil {
		panic(err)
	}
	storage := upload.DefaultConfig()
	storage.MaxFileSize = 5 << 20
	storage.AllowedTypes = []string{upload.ImageTypeJPG, upload.ImageTypeJPEG, upload.ImageTypePNG, upload.ImageTypeWebP}
	if cfg.Storage.Type == "s3" {
		storage.StorageType = "s3"
		storage.S3Config = &upload.S3Config{Endpoint: cfg.Storage.Endpoint, Region: cfg.Storage.Region, BucketName: cfg.Storage.Bucket, AccessKeyID: cfg.Storage.AccessKeyID, SecretAccessKey: cfg.Storage.SecretAccessKey, CDNBaseURL: cfg.Storage.CDNBaseURL, RequestTimeout: cfg.Storage.RequestTimeout}
	} else {
		storage.BaseURL = "http://localhost:8080"
	}
	uploader, err := upload.NewUploaderStrict(storage)
	if err != nil {
		panic(err)
	}
	db := manager.GetSQLDB()
	checker := iam.NewHGAssetAuthorizer(iam.NewHGService(iam.NewHGRepository(db)))
	service := commerce.HGNewService(commerce.HGNewRepository(db), checker)
	module.RegisterModule(&HGModule{handler: commerce.HGNewHandler(service, commerce.HGNewMediaStore(db, uploader)), authorizer: checker})
}

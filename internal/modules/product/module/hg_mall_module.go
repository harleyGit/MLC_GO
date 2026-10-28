package module

import (
	HGHandlerPackage "MLC_GO/internal/handler"
	IAMPackage "MLC_GO/internal/modules/iam"
	"MLC_GO/internal/modules/product"
	ConfigPackage "MLC_GO/internal/pkg/config"
	HGRouterPackage "MLC_GO/internal/pkg/hg_router"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	HGUploadPackage "MLC_GO/internal/pkg/upload"
	"net/http"
)

// HGModule 挂载独立商城业务前缀，授权与商品目录使用同一个 MySQL 连接池。
type HGModule struct{ hgHandler *product.HGHandler }

func (m *HGModule) Name() string          { return "mall" }
func (m *HGModule) BasePath() string      { return HGRouterPackage.HGMallModuleBasePath }
func (m *HGModule) Handler() http.Handler { return HGRouterPackage.HGNewMallRouteGroup(m.hgHandler) }

// HGRegisterModules 只装配依赖和路由，不执行 DDL 或写入任何演示商品。
func HGRegisterModules(manager *PersistenceSQLPackage.HGSQLManager, caches ...*PersistenceRedisPackage.RedisService) {
	if manager == nil {
		panic("商城模块需要数据库管理器")
	}
	db := manager.GetSQLDB()
	config, err := ConfigPackage.GetVideoCommentConfig()
	if err != nil {
		panic(err)
	}
	uploadConfig := HGUploadPackage.DefaultConfig()
	uploadConfig.MaxFileSize = 5 << 20
	uploadConfig.AllowedTypes = []string{HGUploadPackage.ImageTypeJPG, HGUploadPackage.ImageTypeJPEG, HGUploadPackage.ImageTypePNG, HGUploadPackage.ImageTypeWebP}
	if config.Storage.Type == "s3" {
		uploadConfig.StorageType = "s3"
		uploadConfig.S3Config = &HGUploadPackage.S3Config{Endpoint: config.Storage.Endpoint, Region: config.Storage.Region, BucketName: config.Storage.Bucket, AccessKeyID: config.Storage.AccessKeyID, SecretAccessKey: config.Storage.SecretAccessKey, CDNBaseURL: config.Storage.CDNBaseURL, RequestTimeout: config.Storage.RequestTimeout}
	} else {
		uploadConfig.BaseURL = "http://localhost:8080"
	}
	uploader, err := HGUploadPackage.NewUploaderStrict(uploadConfig)
	if err != nil {
		panic(err)
	}
	iamRepo := IAMPackage.NewHGRepository(db)
	if len(caches) > 0 {
		iamRepo.WithHGPermissionCache(caches[0])
	}
	iam := IAMPackage.NewHGService(iamRepo)
	service := product.HGNewService(product.HGNewRepository(db), IAMPackage.NewHGAssetAuthorizer(iam)).HGWithAuthorizer(iam).HGWithOrders(product.HGNewOrderRepository(db))
	HGHandlerPackage.RegisterModule(&HGModule{hgHandler: product.HGNewHandler(service).HGWithImages(product.HGNewImageStore(db, uploader))})
}

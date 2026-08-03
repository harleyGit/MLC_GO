package VideoCommentModulePackage

import (
	HGHandlerPackage "MLC_GO/internal/handler"
	VideoCommentCachePackage "MLC_GO/internal/modules/video_comment/cache"
	VideoCommentHandlerPackage "MLC_GO/internal/modules/video_comment/handler"
	VideoCommentRepositoryPackage "MLC_GO/internal/modules/video_comment/repository"
	VideoCommentServicePackage "MLC_GO/internal/modules/video_comment/service"
	VideoCommentTaskPackage "MLC_GO/internal/modules/video_comment/task"
	ConfigPackage "MLC_GO/internal/pkg/config"
	HGRouterPackage "MLC_GO/internal/pkg/hg_router"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	HGUploadPackage "MLC_GO/internal/pkg/upload"
	"context"
	"net/http"
)

// Module 向统一模块注册器暴露视频评论路由组。
type Module struct {
	handler *VideoCommentHandlerPackage.Handler
}

// Components 暴露需要由应用统一启动和关闭的评论模块后台组件。
type Components struct {
	Maintenance *VideoCommentTaskPackage.HGVideoCommentMaintenance
}

type hgMaintenanceStorage struct {
	adapter *VideoCommentServicePackage.HGUploadAdapter
}

func (s hgMaintenanceStorage) Delete(ctx context.Context, key string) error {
	return s.adapter.DeleteStorageObject(ctx, key)
}

// Name 返回统一模块注册器使用的模块名。
func (m *Module) Name() string { return "video_comment" }

// BasePath 返回视频评论 API 的统一基础路径。
func (m *Module) BasePath() string { return HGRouterPackage.VideoCommentModuleBasePath }

// Handler 返回已应用认证和方法白名单的视频评论路由组。
func (m *Module) Handler() http.Handler {
	return HGRouterPackage.NewVideoCommentRouteGroup(m.handler)
}

// RegisterModules 组装评论 API、Redis 限流、配置化存储和可选维护 worker。
// 显式选择 S3 时采用 fail-fast，配置或凭据错误不会回退到实例本地磁盘。
func RegisterModules(redisService *PersistenceRedisPackage.RedisService, sqlManager *PersistenceSQLPackage.HGSQLManager, leases ...VideoCommentTaskPackage.HGVideoCommentMaintenanceLease) (Components, error) {
	config, err := ConfigPackage.GetVideoCommentConfig()
	if err != nil {
		return Components{}, err
	}
	repo := VideoCommentRepositoryPackage.NewRepository(sqlManager.GetSQLDB())
	// 评论图片单张限制 5 MiB，禁用 GIF，避免动画解码和存储成本放大。
	uploadConfig := HGUploadPackage.DefaultConfig()
	uploadConfig.MaxFileSize = 5 << 20
	uploadConfig.AllowedTypes = []string{HGUploadPackage.ImageTypeJPG, HGUploadPackage.ImageTypeJPEG, HGUploadPackage.ImageTypePNG, HGUploadPackage.ImageTypeWebP}
	if config.Storage.Type == "s3" {
		uploadConfig.StorageType = "s3"
		uploadConfig.S3Config = &HGUploadPackage.S3Config{Endpoint: config.Storage.Endpoint, Region: config.Storage.Region, BucketName: config.Storage.Bucket, AccessKeyID: config.Storage.AccessKeyID, SecretAccessKey: config.Storage.SecretAccessKey, CDNBaseURL: config.Storage.CDNBaseURL, RequestTimeout: config.Storage.RequestTimeout}
	} else {
		uploadConfig.BaseURL = "http://localhost:8080"
	}
	limiter, err := VideoCommentCachePackage.NewHGRedisImageRateLimiter(redisService, VideoCommentCachePackage.HGImageRateLimitConfig{UserCapacity: config.Image.RateUserCapacity, IPCapacity: config.Image.RateIPCapacity, Window: config.Image.RateWindow})
	if err != nil {
		return Components{}, err
	}
	uploader, err := HGUploadPackage.NewUploaderStrict(uploadConfig)
	if err != nil {
		return Components{}, err
	}
	adapter := VideoCommentServicePackage.NewHGUploadAdapter(uploader)
	service := VideoCommentServicePackage.NewServiceWithImageDependencies(repo, adapter, limiter, repo, config.Image.UserCapacityBytes)
	HGHandlerPackage.RegisterModule(&Module{handler: VideoCommentHandlerPackage.NewHandler(service)})
	var maintenance *VideoCommentTaskPackage.HGVideoCommentMaintenance
	if config.Maintenance.Enabled {
		maintenance, err = VideoCommentTaskPackage.NewHGVideoCommentMaintenance(repo, hgMaintenanceStorage{adapter: adapter}, VideoCommentTaskPackage.HGVideoCommentMaintenanceConfig{Interval: config.Maintenance.Interval, Timeout: config.Maintenance.Timeout, OrphanAge: config.Maintenance.OrphanAge, BatchSize: config.Maintenance.BatchSize}, leases...)
		if err != nil {
			return Components{}, err
		}
	}
	return Components{Maintenance: maintenance}, nil
}

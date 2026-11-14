package media

import (
	HGHandlerRegistry "MLC_GO/internal/handler"
	IAM "MLC_GO/internal/modules/iam"
	Transport "MLC_GO/internal/modules/im/transport"
	UserJWT "MLC_GO/internal/modules/user/middleware"
	HGRouter "MLC_GO/internal/pkg/hg_router"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	Permission "MLC_GO/internal/pkg/permission"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const hgBasePath = "/api/v1/media"

// HGModule 提供独立于旧上传链路的隔离上传 API；部署迁移与审核器前不能替换现有读路径。
type HGModule struct {
	handler    *HGHandler                 // 已验证的隔离上传 handler。
	authorizer HGRouter.HGRouteAuthorizer // 管理盘点只读权限；未配置时路由失败关闭。
}

func (m *HGModule) Name() string     { return "media" }
func (m *HGModule) BasePath() string { return hgBasePath }
func (m *HGModule) Handler() http.Handler {
	return HGRouter.NewRouteGroup(HGRouter.RouteGroupConfig{BasePath: hgBasePath, Authorizer: m.authorizer, AuthMiddleware: UserJWT.AuthMiddleware}, hgRoutes(m.handler))
}

func hgRoutes(h *HGHandler) []HGRouter.RouteSpec {
	var init, complete, status, download http.HandlerFunc
	if h != nil {
		init, complete, status, download = h.HGInitUpload, h.HGCompleteUpload, h.HGAssetStatus, h.HGOwnerDownload
	}
	routes := []HGRouter.RouteSpec{
		HGRouter.NewRouteSpec("media", http.MethodPost, hgBasePath, "/uploads/init", true, "按业务用途创建隔离上传", init),
		HGRouter.NewRouteSpec("media", http.MethodPost, hgBasePath, "/uploads/complete", true, "确认本人隔离对象大小，等待内容审核", complete),
		HGRouter.NewRouteSpec("media", http.MethodGet, hgBasePath, "/assets/status", true, "查询本人资源审核状态", status),
		HGRouter.NewRouteSpec("media", http.MethodGet, hgBasePath, "/assets/download", true, "获取本人已审核私有文件短时读授权", download),
	}
	return append(routes, hgInventoryRoutes(h)...)
}

// HGRouteCatalog 由同一 RouteSpec 构造路由清单及 API Guard。
func HGRouteCatalog() []HGRouter.RouteCatalogItem {
	return HGRouter.BuildRouteCatalogItems(hgRoutes(nil))
}

// HGInventoryModule 仅挂载 IAM 管理元数据盘点；缺少对象存储配置时不启用上传及私有读取。
type HGInventoryModule struct {
	handler    *HGHandler                 // 只使用数据库查询的管理盘点入口。
	authorizer HGRouter.HGRouteAuthorizer // 活动管理员和全局 IAM 权限，缺失时拒绝访问。
}

func (m *HGInventoryModule) Name() string     { return "media" }
func (m *HGInventoryModule) BasePath() string { return hgBasePath }
func (m *HGInventoryModule) Handler() http.Handler {
	return HGRouter.NewRouteGroup(HGRouter.RouteGroupConfig{BasePath: hgBasePath, Authorizer: m.authorizer, AuthMiddleware: UserJWT.AuthMiddleware}, hgInventoryRoutes(m.handler))
}

func hgInventoryRoutes(h *HGHandler) []HGRouter.RouteSpec {
	var inventory http.HandlerFunc
	if h != nil {
		inventory = h.HGInventoryList
	}
	return []HGRouter.RouteSpec{
		HGRouter.NewRouteSpec("media", http.MethodGet, hgBasePath, "/admin/assets/list", true, "管理端分页盘点新旧媒体元数据，不授予下载或删除", inventory, Permission.HGMediaInventoryRead),
	}
}

// HGInventoryRouteCatalog 只声明未启用对象存储时实际挂载的管理路由。
func HGInventoryRouteCatalog() []HGRouter.RouteCatalogItem {
	return HGRouter.BuildRouteCatalogItems(hgInventoryRoutes(nil))
}

// HGConfigured 检查是否显式启用媒体隔离桶；缺失配置时不挂载半初始化模块。
func HGConfigured() bool { return os.Getenv("MLC_MEDIA_S3_ENDPOINT") != "" }

// HGRegister 从进程环境读取受信配置；密钥不得写入配置文件、日志或 API 响应。
func HGRegister(db *sql.DB) (*HGRuntime, error) {
	if !HGConfigured() {
		if db == nil {
			return nil, errors.New("媒体盘点数据库未配置")
		}
		// 只读盘点不依赖隔离桶凭据；不创建 S3 客户端、后台 worker 或上传路由。
		authorizer := IAM.NewHGAssetAuthorizer(IAM.NewHGService(IAM.NewHGRepository(db)))
		HGHandlerRegistry.RegisterModule(&HGInventoryModule{handler: HGNewHandler(&HGService{db: db}), authorizer: authorizer})
		return nil, nil
	}
	bucket := os.Getenv("MLC_MEDIA_S3_BUCKET")
	if bucket == "" {
		return nil, errors.New("媒体隔离桶未配置")
	}
	if db == nil {
		return nil, errors.New("媒体数据库未配置")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for _, query := range []string{SQL.HGMediaAssetSchemaSQL, SQL.HGMediaSessionSchemaSQL, SQL.HGMediaQuotaSchemaSQL, SQL.HGMediaDeleteSchemaSQL, SQL.HGMediaRefsSchemaSQL} {
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("媒体元数据迁移未就绪: %w", err)
		}
		if err = rows.Close(); err != nil {
			return nil, fmt.Errorf("关闭媒体数据库检查失败: %w", err)
		}
	}
	store, err := HGNewSeaweedStore(os.Getenv("MLC_MEDIA_S3_ENDPOINT"), os.Getenv("MLC_MEDIA_S3_REGION"), bucket, os.Getenv("MLC_MEDIA_S3_ACCESS_KEY"), os.Getenv("MLC_MEDIA_S3_SECRET_KEY"))
	if err != nil {
		return nil, err
	}
	cleanupKey, cleanupSecret := os.Getenv("MLC_MEDIA_CLEANUP_S3_ACCESS_KEY"), os.Getenv("MLC_MEDIA_CLEANUP_S3_SECRET_KEY")
	if cleanupKey == "" || cleanupSecret == "" || cleanupKey == os.Getenv("MLC_MEDIA_S3_ACCESS_KEY") || cleanupKey == os.Getenv("MLC_IM_REVIEW_S3_ACCESS_KEY") {
		return nil, errors.New("媒体专用清理身份未配置或与上传/审核身份相同")
	}
	cleanup, err := HGNewSeaweedStore(os.Getenv("MLC_MEDIA_S3_ENDPOINT"), os.Getenv("MLC_MEDIA_S3_REGION"), bucket, cleanupKey, cleanupSecret)
	if err != nil {
		return nil, err
	}
	service, err := HGNewService(db, store, bucket)
	if err != nil {
		return nil, err
	}
	url, token := os.Getenv("MLC_MEDIA_REVIEW_URL"), os.Getenv("MLC_MEDIA_REVIEW_TOKEN")
	if url == "" || token == "" {
		return nil, errors.New("可信媒体审核服务未配置")
	}
	if strings.HasPrefix(url, "http://") && os.Getenv("SERVER_ENV") != "debug" {
		return nil, errors.New("非 debug 环境禁止明文审核端点")
	}
	reviewer, err := Transport.HGNewReviewer(url, token)
	if err != nil {
		return nil, err
	}
	if err = service.HGEnableReview(reviewer, &hgWorkerStore{head: store, cleanup: cleanup}); err != nil {
		reviewer.HGClose()
		return nil, err
	}
	readKey, readSecret := os.Getenv("MLC_MEDIA_READ_S3_ACCESS_KEY"), os.Getenv("MLC_MEDIA_READ_S3_SECRET_KEY")
	if readKey != "" || readSecret != "" {
		if readKey == "" || readSecret == "" || readKey == cleanupKey || readKey == os.Getenv("MLC_MEDIA_S3_ACCESS_KEY") {
			reviewer.HGClose()
			return nil, errors.New("媒体私有读取身份不完整或与写入/清理身份混用")
		}
		readStore, readErr := HGNewSeaweedStore(os.Getenv("MLC_MEDIA_S3_ENDPOINT"), os.Getenv("MLC_MEDIA_S3_REGION"), bucket, readKey, readSecret)
		if readErr != nil {
			reviewer.HGClose()
			return nil, readErr
		}
		if err = service.HGEnableOwnerRead(readStore); err != nil {
			reviewer.HGClose()
			return nil, err
		}
	}
	// 管理接口独立使用 IAM 活动管理员 + 全局权限；不改变普通用户的本人上传和私有读取链路。
	authorizer := IAM.NewHGAssetAuthorizer(IAM.NewHGService(IAM.NewHGRepository(db)))
	HGHandlerRegistry.RegisterModule(&HGModule{handler: HGNewHandler(service), authorizer: authorizer})
	return &HGRuntime{service: service, closeReviewer: reviewer.HGClose}, nil
}

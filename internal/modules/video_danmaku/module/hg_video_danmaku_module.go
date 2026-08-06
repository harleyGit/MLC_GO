package VideoDanmakuModulePackage

import (
	HGHandlerPackage "MLC_GO/internal/handler"
	VideoDanmakuHandlerPackage "MLC_GO/internal/modules/video_danmaku/handler"
	VideoDanmakuRealtimePackage "MLC_GO/internal/modules/video_danmaku/realtime"
	VideoDanmakuRepositoryPackage "MLC_GO/internal/modules/video_danmaku/repository"
	VideoDanmakuServicePackage "MLC_GO/internal/modules/video_danmaku/service"
	ConfigPackage "MLC_GO/internal/pkg/config"
	HGRouterPackage "MLC_GO/internal/pkg/hg_router"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"net/http"
)

// Module 向统一注册器暴露弹幕 HTTP API；WebSocket 由同一组件返回的独立 gnet Server 承载。
type Module struct {
	handler *VideoDanmakuHandlerPackage.Handler
}

func (m *Module) Name() string          { return "video_danmaku" }
func (m *Module) BasePath() string      { return HGRouterPackage.VideoDanmakuModuleBasePath }
func (m *Module) Handler() http.Handler { return HGRouterPackage.NewVideoDanmakuRouteGroup(m.handler) }

// Components 暴露由应用统一监管生命周期的实时网关。
type Components struct {
	Realtime *VideoDanmakuRealtimePackage.Server
}

// RegisterModules 复用共享 MySQL/Redis，组装权威存储、HTTP API 和独立实时网关。
func RegisterModules(redisService *PersistenceRedisPackage.RedisService, sqlManager *PersistenceSQLPackage.HGSQLManager) (Components, error) {
	config, err := ConfigPackage.GetVideoDanmakuConfig()
	if err != nil {
		return Components{}, err
	}
	kafkaConfig, _, err := ConfigPackage.GetKafkaConfig()
	if err != nil {
		return Components{}, err
	}
	danmakuTopic := ""
	if len(kafkaConfig.Business.Consumers.Danmaku.Topics) > 0 {
		danmakuTopic = kafkaConfig.Business.Consumers.Danmaku.Topics[0]
	}
	repo := VideoDanmakuRepositoryPackage.NewRepositoryWithTopic(sqlManager.GetSQLDB(), danmakuTopic)
	service := VideoDanmakuServicePackage.NewService(repo, redisService, config.TicketTTL)
	realtime := VideoDanmakuRealtimePackage.NewServer(service, redisService, config)
	// service 只依赖小接口，不依赖 gnet 细节；先提交 MySQL，再调用本机发布器广播。
	service.SetPublisher(realtime)
	HGHandlerPackage.RegisterModule(&Module{handler: VideoDanmakuHandlerPackage.NewHandler(service)})
	return Components{Realtime: realtime}, nil
}

package VideoInteractionModulePackage

import (
	HGHandlerPackage "MLC_GO/internal/handler"
	EventBusPackage "MLC_GO/internal/infrastructure/eventbus"
	VideoInteractionCachePackage "MLC_GO/internal/modules/video_interaction/cache"
	VideoInteractionHandlerPackage "MLC_GO/internal/modules/video_interaction/handler"
	VideoInteractionServicePackage "MLC_GO/internal/modules/video_interaction/service"
	HGRouterPackage "MLC_GO/internal/pkg/hg_router"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"net/http"
)

const hgInteractionTopic = "mlc.domain.events"

type Module struct {
	handler *VideoInteractionHandlerPackage.Handler
}

func (m *Module) Name() string     { return "video_interaction" }
func (m *Module) BasePath() string { return HGRouterPackage.VideoInteractionModuleBasePath }
func (m *Module) Handler() http.Handler {
	return HGRouterPackage.NewVideoInteractionRouteGroup(m.handler)
}

// RegisterModules 组装并注册视频互动模块。
func RegisterModules(redisService *PersistenceRedisPackage.RedisService) {
	cache := VideoInteractionCachePackage.NewCache(redisService)
	service := VideoInteractionServicePackage.NewService(EventBusPackage.NewKafkaEventBus(hgInteractionTopic), cache)
	HGHandlerPackage.RegisterModule(&Module{handler: VideoInteractionHandlerPackage.NewHandler(service)})
}

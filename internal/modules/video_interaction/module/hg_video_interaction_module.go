package VideoInteractionModulePackage

import (
	HGHandlerPackage "MLC_GO/internal/handler"
	EventBusPackage "MLC_GO/internal/infrastructure/eventbus"
	CoinRepositoryPackage "MLC_GO/internal/modules/coin/repository"
	CoinServicePackage "MLC_GO/internal/modules/coin/service"
	VideoInteractionCachePackage "MLC_GO/internal/modules/video_interaction/cache"
	VideoInteractionHandlerPackage "MLC_GO/internal/modules/video_interaction/handler"
	VideoInteractionServicePackage "MLC_GO/internal/modules/video_interaction/service"
	ConfigPackage "MLC_GO/internal/pkg/config"
	HGRouterPackage "MLC_GO/internal/pkg/hg_router"
	"MLC_GO/internal/pkg/idgen"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	PersistenceRedisPackage "MLC_GO/internal/pkg/redis"
	"fmt"
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
func RegisterModules(redisService *PersistenceRedisPackage.RedisService, sqlManager *PersistenceSQLPackage.HGSQLManager) error {
	cache := VideoInteractionCachePackage.NewCache(redisService)
	var coinStore *CoinServicePackage.HGService
	if sqlManager != nil {
		coinStore = CoinServicePackage.NewHGService(CoinRepositoryPackage.NewHGRepository(sqlManager.GetSQLDB(), hgInteractionTopic))
	}
	idConfig, err := ConfigPackage.GetIDGeneratorConfig()
	if err != nil {
		return fmt.Errorf("读取业务 ID 配置: %w", err)
	}
	snowflake, err := idgen.NewHGSnowflake(idConfig.Epoch, idConfig.WorkerID)
	if err != nil {
		return fmt.Errorf("创建 Snowflake ID 生成器: %w", err)
	}
	idGenerator, err := idgen.NewHGGenerator(snowflake)
	if err != nil {
		return fmt.Errorf("创建业务 ID 生成器: %w", err)
	}
	service := VideoInteractionServicePackage.NewServiceWithIDGenerator(EventBusPackage.NewKafkaEventBus(hgInteractionTopic), cache, idGenerator, coinStore)
	HGHandlerPackage.RegisterModule(&Module{handler: VideoInteractionHandlerPackage.NewHandler(service)})
	return nil
}

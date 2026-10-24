package module

import (
	HGHandlerPackage "MLC_GO/internal/handler"
	OpsRepositoryPackage "MLC_GO/internal/modules/ops/repository"
	"MLC_GO/internal/modules/product"
	HGRouterPackage "MLC_GO/internal/pkg/hg_router"
	PersistenceSQLPackage "MLC_GO/internal/pkg/mysql"
	"net/http"
)

// HGModule 挂载独立商城业务前缀，授权与商品目录使用同一个 MySQL 连接池。
type HGModule struct{ hgHandler *product.HGHandler }

func (m *HGModule) Name() string          { return "mall" }
func (m *HGModule) BasePath() string      { return HGRouterPackage.HGMallModuleBasePath }
func (m *HGModule) Handler() http.Handler { return HGRouterPackage.HGNewMallRouteGroup(m.hgHandler) }

// HGRegisterModules 只装配依赖和路由，不执行 DDL 或写入任何演示商品。
func HGRegisterModules(manager *PersistenceSQLPackage.HGSQLManager) {
	if manager == nil {
		panic("商城模块需要数据库管理器")
	}
	db := manager.GetSQLDB()
	service := product.HGNewService(product.HGNewRepository(db), OpsRepositoryPackage.NewRepository(db)).HGWithOrders(product.HGNewOrderRepository(db))
	HGHandlerPackage.RegisterModule(&HGModule{hgHandler: product.HGNewHandler(service)})
}

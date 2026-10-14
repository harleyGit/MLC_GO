package module

import (
	hgroot "MLC_GO/internal/handler"
	"MLC_GO/internal/modules/recharge/handler"
	"MLC_GO/internal/modules/recharge/repository"
	"MLC_GO/internal/modules/recharge/service"
	hgrouter "MLC_GO/internal/pkg/hg_router"
	hgmysql "MLC_GO/internal/pkg/mysql"
	"net/http"
)

// HGModule 挂载独立的充值记录模块。
type HGModule struct{ hgHandler *handler.HGHandler } // 只读 HTTP 处理器。

func (hgModule *HGModule) Name() string     { return "recharge" }
func (hgModule *HGModule) BasePath() string { return hgrouter.HGRechargeModuleBasePath }
func (hgModule *HGModule) Handler() http.Handler {
	return hgrouter.HGNewRechargeRouteGroup(hgModule.hgHandler)
}

// HGRegisterModules 复用主服务连接池，仅装配依赖，不发起查询。
func HGRegisterModules(hgSQLManager *hgmysql.HGSQLManager) {
	if hgSQLManager == nil {
		panic("充值记录模块需要数据库管理器")
	}
	hgService := service.HGNewService(repository.HGNewRepository(hgSQLManager.GetSQLDB()))
	hgroot.RegisterModule(&HGModule{hgHandler: handler.HGNewHandler(hgService)})
}

package module

import (
	hgroot "MLC_GO/internal/handler"
	hgcoinrepo "MLC_GO/internal/modules/coin/repository"
	hgcoinsvc "MLC_GO/internal/modules/coin/service"
	"MLC_GO/internal/modules/wallet/handler"
	"MLC_GO/internal/modules/wallet/repository"
	"MLC_GO/internal/modules/wallet/service"
	hgrouter "MLC_GO/internal/pkg/hg_router"
	hgmysql "MLC_GO/internal/pkg/mysql"
	"net/http"
)

type HGModule struct{ hgHandler *handler.HGHandler }

func (hgModule *HGModule) Name() string     { return "wallet" }
func (hgModule *HGModule) BasePath() string { return hgrouter.HGWalletModuleBasePath }
func (hgModule *HGModule) Handler() http.Handler {
	return hgrouter.HGNewWalletRouteGroup(hgModule.hgHandler)
}

// HGRegisterModules 注册JWT钱包接口；debug入账由wallet仓储复用coin事务适配器。
func HGRegisterModules(hgSQLManager *hgmysql.HGSQLManager) {
	if hgSQLManager == nil {
		panic("钱包模块需要数据库管理器")
	}
	hgDB := hgSQLManager.GetSQLDB()
	hgCoinService := hgcoinsvc.NewHGService(hgcoinrepo.NewHGRepository(hgDB, "mlc.domain.events"))
	hgWalletService := service.HGNewService(repository.HGNewRepository(hgDB), hgCoinService)
	hgroot.RegisterModule(&HGModule{hgHandler: handler.HGNewHandler(hgWalletService)})
}

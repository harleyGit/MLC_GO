/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-05-05 16:58:46
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-08-26 20:25:00
 * @FilePath: /MLC_GO/internal/handler/hg_module.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGHandlerPackage

import (
	"net/http"
	"sync"
)

// HGModule 定义模块统一接口，每个业务模块需实现此接口。
type HGModule interface {
	// Name 返回模块名称，用于日志和路由清单。
	Name() string
	// BasePath 返回模块的 API 前缀路径。例如 `/api/v1/user`
	BasePath() string
	// Handler 返回模块的 HTTP Handler。模块自己的子 mux，**模块内部已经组装好自己的路由、中间件、拦截器**。
	Handler() http.Handler
}

// moduleRegistry 已注册的模块实例。
//
// 这里使用 RWMutex 的原因：模块注册表是包级全局变量，启动、测试、重复构建应用时都可能访问它。
// 没有锁时，RegisterModule/ClearModules 与 GetRegisteredModules 并发执行会产生数据竞争。
// 返回时复制 slice 快照，是为了避免调用方拿到内部 slice 后修改底层数组，破坏注册表一致性。
var (
	moduleRegistryMu sync.RWMutex
	moduleRegistry   []HGModule // 切片。类型是HGModule
)

// RegisterModule 注册模块到全局注册表。
// 写锁保证多个模块并发注册时 append 操作不会破坏 slice 内部状态。
func RegisterModule(modules ...HGModule) {
	moduleRegistryMu.Lock()
	defer moduleRegistryMu.Unlock()

	// 把 modules 里面的所有元素，追加到 moduleRegistry 这个切片中
	moduleRegistry = append(moduleRegistry, modules...)
}

// GetRegisteredModules 返回所有已注册模块。
// 读锁允许多个读取方并发获取模块列表；返回副本隔离内部状态。
func GetRegisteredModules() []HGModule {
	moduleRegistryMu.RLock()
	defer moduleRegistryMu.RUnlock()

	modules := make([]HGModule, len(moduleRegistry))
	copy(modules, moduleRegistry)
	return modules
}

// ClearModules 清空模块注册表（用于测试）。
// 构建应用前清空注册表可以避免重复调用 buildMLCApplication 时出现重复挂载。
func ClearModules() {
	moduleRegistryMu.Lock()
	defer moduleRegistryMu.Unlock()

	moduleRegistry = nil
}

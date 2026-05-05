/*
 * @Author: GangHuang harleysor@qq.com
 * @Date: 2026-05-05 16:58:46
 * @LastEditors: GangHuang harleysor@qq.com
 * @LastEditTime: 2026-05-05 21:47:11
 * @FilePath: /MLC_GO/internal/handler/hg_module.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package HGHandlerPackage

import (
	"net/http"
)

// HGModule 定义模块统一接口，每个业务模块需实现此接口。
type HGModule interface {
	// Name 返回模块名称，用于日志和路由清单。
	Name() string
	// BasePath 返回模块的 API 前缀路径。
	BasePath() string
	// Handler 返回模块的 HTTP Handler。
	Handler() http.Handler
}

// moduleRegistry 已注册的模块实例。
var moduleRegistry []HGModule

// RegisterModule 注册模块到全局注册表。
func RegisterModule(modules ...HGModule) {
	moduleRegistry = append(moduleRegistry, modules...)
}

// GetRegisteredModules 返回所有已注册模块。
func GetRegisteredModules() []HGModule {
	return moduleRegistry
}

// ClearModules 清空模块注册表（用于测试）。
func ClearModules() {
	moduleRegistry = nil
}

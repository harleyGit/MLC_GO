package HGHandlerPackage

import (
	"net/http"
)

// Module 定义模块统一接口，每个业务模块需实现此接口。
type Module interface {
	// Name 返回模块名称，用于日志和路由清单。
	Name() string
	// BasePath 返回模块的 API 前缀路径。
	BasePath() string
	// Handler 返回模块的 HTTP Handler。
	Handler() http.Handler
}

// moduleRegistry 已注册的模块实例。
var moduleRegistry []Module

// RegisterModule 注册模块到全局注册表。
func RegisterModule(modules ...Module) {
	moduleRegistry = append(moduleRegistry, modules...)
}

// GetRegisteredModules 返回所有已注册模块。
func GetRegisteredModules() []Module {
	return moduleRegistry
}

// ClearModules 清空模块注册表（用于测试）。
func ClearModules() {
	moduleRegistry = nil
}

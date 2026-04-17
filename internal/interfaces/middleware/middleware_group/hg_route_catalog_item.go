package HGMiddlewareGroupPackage

// HGRouteCatalogItem 描述对外可调用的 API 路由，供 App/Web 联调查看。
type HGRouteCatalogItem struct {
	Group    string `json:"group"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	NeedAuth bool   `json:"needAuth"`
	Summary  string `json:"summary"`
}

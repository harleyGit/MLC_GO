package spider

import (
	HGRouterPackage "MLC_GO/internal/pkg/hg_router"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const hgCrawlerModuleBasePath = "/api/v1/crawler"

// hgResponse 对齐 React HttpManagerV1 期望的 code/message/result 响应信封。
// crawler 当前作为独立服务运行，因此不依赖主业务进程的 response 包和请求上下文中间件。
type hgResponse struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Result    interface{} `json:"result,omitempty"`
	TID       string      `json:"tid"`
	Timestamp int64       `json:"timestamp"`
}

// NewHGHTTPHandler 创建 crawler 独立服务的管理 API。
// 路由只暴露固定的 Bilibili worker，不接受调用方传入任意目标 URL，避免被用作开放代理。
func NewHGHTTPHandler(manager *HGManager) http.Handler {
	mux := http.NewServeMux()
	h := &hgHTTPHandler{manager: manager}
	for _, route := range hgCrawlerRoutes(h) {
		mux.HandleFunc(route.Method+" "+route.FullPath, route.Handler)
	}
	return mux
}

// hgHTTPHandler 只负责 HTTP 参数和统一响应，任务校验、并发控制和状态更新由 HGManager 完成。
type hgHTTPHandler struct{ manager *HGManager }

// hgCrawlerRoutes 返回 crawler 独立服务的完整路由定义。
func hgCrawlerRoutes(handler *hgHTTPHandler) []HGRouterPackage.RouteSpec {
	if handler == nil {
		return []HGRouterPackage.RouteSpec{
			HGRouterPackage.NewRouteSpec("crawler", http.MethodGet, hgCrawlerModuleBasePath, "/dashboard", false, "获取爬虫管理看板", nil),
			HGRouterPackage.NewRouteSpec("crawler", http.MethodGet, hgCrawlerModuleBasePath, "/spiders", false, "获取爬虫运行状态", nil),
			HGRouterPackage.NewRouteSpec("crawler", http.MethodPost, hgCrawlerModuleBasePath, "/spiders/bilibili/start", false, "启动 Bilibili 爬虫", nil),
			HGRouterPackage.NewRouteSpec("crawler", http.MethodPost, hgCrawlerModuleBasePath, "/spiders/bilibili/stop", false, "停止 Bilibili 爬虫", nil),
			HGRouterPackage.NewRouteSpec("crawler", http.MethodGet, hgCrawlerModuleBasePath, "/tasks", false, "获取爬虫任务列表", nil),
			HGRouterPackage.NewRouteSpec("crawler", http.MethodPost, hgCrawlerModuleBasePath, "/tasks", false, "创建爬虫任务", nil),
			HGRouterPackage.NewRouteSpec("crawler", http.MethodGet, hgCrawlerModuleBasePath, "/recommendations", false, "获取推荐数据", nil),
			HGRouterPackage.NewRouteSpec("crawler", http.MethodGet, "", "/healthz", false, "检查爬虫服务健康状态", nil),
		}
	}

	return []HGRouterPackage.RouteSpec{
		HGRouterPackage.NewRouteSpec("crawler", http.MethodGet, hgCrawlerModuleBasePath, "/dashboard", false, "获取爬虫管理看板", handler.dashboard),
		HGRouterPackage.NewRouteSpec("crawler", http.MethodGet, hgCrawlerModuleBasePath, "/spiders", false, "获取爬虫运行状态", handler.spiders),
		HGRouterPackage.NewRouteSpec("crawler", http.MethodPost, hgCrawlerModuleBasePath, "/spiders/bilibili/start", false, "启动 Bilibili 爬虫", handler.start),
		HGRouterPackage.NewRouteSpec("crawler", http.MethodPost, hgCrawlerModuleBasePath, "/spiders/bilibili/stop", false, "停止 Bilibili 爬虫", handler.stop),
		HGRouterPackage.NewRouteSpec("crawler", http.MethodGet, hgCrawlerModuleBasePath, "/tasks", false, "获取爬虫任务列表", handler.tasks),
		HGRouterPackage.NewRouteSpec("crawler", http.MethodPost, hgCrawlerModuleBasePath, "/tasks", false, "创建爬虫任务", handler.createTask),
		HGRouterPackage.NewRouteSpec("crawler", http.MethodGet, hgCrawlerModuleBasePath, "/recommendations", false, "获取推荐数据", handler.recommendations),
		HGRouterPackage.NewRouteSpec("crawler", http.MethodGet, "", "/healthz", false, "检查爬虫服务健康状态", handler.health),
	}
}

// health 返回 crawler 独立服务存活状态。
func (h *hgHTTPHandler) health(w http.ResponseWriter, _ *http.Request) {
	hgWriteJSON(w, http.StatusOK, "ok", map[string]string{"status": "healthy"})
}

// dashboard 返回指标、趋势和最近任务的内存快照。
func (h *hgHTTPHandler) dashboard(w http.ResponseWriter, _ *http.Request) {
	hgWriteJSON(w, http.StatusOK, "Success", h.manager.Dashboard())
}

// spiders 返回当前进程内唯一 Bilibili worker 的运行状态。
func (h *hgHTTPHandler) spiders(w http.ResponseWriter, _ *http.Request) {
	hgWriteJSON(w, http.StatusOK, "Success", map[string]interface{}{"list": h.manager.Spiders()})
}

// start 幂等边界由 manager 管理；重复启动使用 HTTP 409 明确表达状态冲突。
func (h *hgHTTPHandler) start(w http.ResponseWriter, _ *http.Request) {
	if err := h.manager.Start(); err != nil {
		hgWriteJSON(w, http.StatusConflict, err.Error(), nil)
		return
	}
	hgWriteJSON(w, http.StatusOK, "Spider started", h.manager.Spiders()[0])
}

// stop 会等待正在执行的抓取观察到取消信号后退出，再返回停止后的状态快照。
func (h *hgHTTPHandler) stop(w http.ResponseWriter, _ *http.Request) {
	h.manager.Stop()
	hgWriteJSON(w, http.StatusOK, "Spider stopped", h.manager.Spiders()[0])
}

// tasks 使用内存下标作为短生命周期 cursor，并限制 pageSize，避免管理端一次拉取全部任务历史。
func (h *hgHTTPHandler) tasks(w http.ResponseWriter, r *http.Request) {
	cursor, _ := strconv.Atoi(r.URL.Query().Get("cursor"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	hgWriteJSON(w, http.StatusOK, "Success", h.manager.Tasks(cursor, pageSize, status))
}

// createTask 同步执行一次任务；16 KiB body 上限足以容纳固定 DTO，同时防止大请求占用内存。
func (h *hgHTTPHandler) createTask(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	var req HGCreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		hgWriteJSON(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}
	task, err := h.manager.RunOnce(r.Context(), req)
	if err != nil {
		hgWriteJSON(w, http.StatusConflict, err.Error(), task)
		return
	}
	hgWriteJSON(w, http.StatusOK, "Task completed", task)
}

// recommendations 返回最近一次成功任务的推荐快照；失败任务不会清空上一份可用数据。
func (h *hgHTTPHandler) recommendations(w http.ResponseWriter, _ *http.Request) {
	hgWriteJSON(w, http.StatusOK, "Success", map[string]interface{}{"list": h.manager.Recommendations()})
}

// hgWriteJSON 写入前端统一响应格式并禁用缓存，防止管理页面展示过期 worker 状态。
func hgWriteJSON(w http.ResponseWriter, status int, message string, result interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	code := 200
	if status >= http.StatusBadRequest {
		code = 500005
	}
	_ = json.NewEncoder(w).Encode(hgResponse{Code: code, Message: message, Result: result, Timestamp: time.Now().UnixMilli()})
}

package spider

import (
	HGRouterPackage "MLC_GO/internal/pkg/hg_router"
	resp "MLC_GO/internal/response"
	"MLC_GO/internal/response/hgerrors"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

const hgCrawlerModuleBasePath = "/api/v1/crawler"

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
type hgHTTPHandler struct {
	manager *HGManager /* 进程内的有界任务管理器，不访问其他平台管理面。 */
}

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
func (h *hgHTTPHandler) health(w http.ResponseWriter, r *http.Request) {
	hgWriteJSON(w, r, http.StatusOK, "ok", map[string]string{"status": "healthy"})
}

// dashboard 返回指标、趋势和最近任务的内存快照。
func (h *hgHTTPHandler) dashboard(w http.ResponseWriter, r *http.Request) {
	hgWriteJSON(w, r, http.StatusOK, "Success", h.manager.Dashboard())
}

// spiders 返回当前进程内唯一 Bilibili worker 的运行状态。
func (h *hgHTTPHandler) spiders(w http.ResponseWriter, r *http.Request) {
	hgWriteJSON(w, r, http.StatusOK, "Success", map[string]interface{}{"list": h.manager.Spiders()})
}

// start 已迁往需要登录和权限校验的 Ops API；拒绝独立服务无鉴权写入。
func (h *hgHTTPHandler) start(w http.ResponseWriter, r *http.Request) {
	hgWriteJSON(w, r, http.StatusForbidden, "请使用已鉴权的 Ops 爬虫开关接口", nil)
}

// stop 与 start 一样禁止绕过数据库权威开关。
func (h *hgHTTPHandler) stop(w http.ResponseWriter, r *http.Request) {
	hgWriteJSON(w, r, http.StatusForbidden, "请使用已鉴权的 Ops 爬虫开关接口", nil)
}

// tasks 使用内存下标作为短生命周期 cursor，并限制 pageSize，避免管理端一次拉取全部任务历史。
func (h *hgHTTPHandler) tasks(w http.ResponseWriter, r *http.Request) {
	cursor, _ := strconv.Atoi(r.URL.Query().Get("cursor"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	hgWriteJSON(w, r, http.StatusOK, "Success", h.manager.Tasks(cursor, pageSize, status))
}

// createTask 同步执行一次任务；16 KiB body 上限足以容纳固定 DTO，同时防止大请求占用内存。
func (h *hgHTTPHandler) createTask(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	var req HGCreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		hgWriteJSON(w, r, http.StatusBadRequest, "Invalid request body", nil)
		return
	}
	task, err := h.manager.RunOnce(r.Context(), req)
	if err != nil {
		w.Header().Set("Cache-Control", "no-store")
		resp.HGWriteCompatibleResult(w, r, http.StatusConflict, resp.HGResultModel[any]{Code: resp.ThirdPartyErrorCode, Message: hgerrors.Resolve(err).Message, Result: task}, err)
		return
	}
	hgWriteJSON(w, r, http.StatusOK, "Task completed", task)
}

// recommendations 返回最近一次成功任务的推荐快照；失败任务不会清空上一份可用数据。
func (h *hgHTTPHandler) recommendations(w http.ResponseWriter, r *http.Request) {
	hgWriteJSON(w, r, http.StatusOK, "Success", map[string]interface{}{"list": h.manager.Recommendations()})
}

// hgWriteJSON 写入前端统一响应格式并禁用缓存，防止管理页面展示过期 worker 状态。
func hgWriteJSON(w http.ResponseWriter, r *http.Request, status int, message string, result any) {
	w.Header().Set("Cache-Control", "no-store")
	legacy := resp.HGResultModel[any]{Code: resp.OKCode, Message: message, Result: result}
	var err error
	switch status {
	case http.StatusForbidden:
		err = hgerrors.HGOPSOperationForbidden
	case http.StatusBadRequest:
		err = hgerrors.HGCommonInvalidParam
	}
	if err != nil {
		legacy.Code = resp.ThirdPartyErrorCode
	}
	resp.HGWriteCompatibleResult(w, r, status, legacy, err)
}

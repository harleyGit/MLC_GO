package spider

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

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
	mux.HandleFunc("GET /api/v1/crawler/dashboard", h.dashboard)
	mux.HandleFunc("GET /api/v1/crawler/spiders", h.spiders)
	mux.HandleFunc("POST /api/v1/crawler/spiders/bilibili/start", h.start)
	mux.HandleFunc("POST /api/v1/crawler/spiders/bilibili/stop", h.stop)
	mux.HandleFunc("GET /api/v1/crawler/tasks", h.tasks)
	mux.HandleFunc("POST /api/v1/crawler/tasks", h.createTask)
	mux.HandleFunc("GET /api/v1/crawler/recommendations", h.recommendations)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		hgWriteJSON(w, http.StatusOK, "ok", map[string]string{"status": "healthy"})
	})
	return mux
}

// hgHTTPHandler 只负责 HTTP 参数和统一响应，任务校验、并发控制和状态更新由 HGManager 完成。
type hgHTTPHandler struct{ manager *HGManager }

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

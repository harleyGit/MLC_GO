package OpsHandlerPackage

import (
	CrawlerDtoPackage "MLC_GO/internal/modules/crawler/dto"
	CrawlerServicePackage "MLC_GO/internal/modules/crawler/service"
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	OpsServicePackage "MLC_GO/internal/modules/ops/service"
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePakcage "MLC_GO/internal/response"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// Handler 是 ops 模块的 HTTP 入口。
// 只负责鉴权上下文读取、HTTP 参数解析、错误码映射和统一响应，不直接写 SQL 或处理业务细节。
type Handler struct {
	service      *OpsServicePackage.Service
	crawlerDebug *CrawlerServicePackage.HGDebugService
	crawlerTasks *CrawlerServicePackage.HGTaskService
	crawlerAuth  interface {
		HasAssetPermission(context.Context, string, string) (bool, error)
	}
}

// NewHandler 创建运维管理处理器，由 module assembly 统一注入 service。
func NewHandler(service *OpsServicePackage.Service, crawlerDebug ...*CrawlerServicePackage.HGDebugService) *Handler {
	handler := &Handler{service: service}
	if len(crawlerDebug) > 0 {
		handler.crawlerDebug = crawlerDebug[0]
	}
	return handler
}

// WithCrawlerTasks injects persisted crawler task management and its database-backed authorizer.
func (h *Handler) WithCrawlerTasks(tasks *CrawlerServicePackage.HGTaskService, authorizer interface {
	HasAssetPermission(context.Context, string, string) (bool, error)
}) *Handler {
	h.crawlerTasks = tasks
	h.crawlerAuth = authorizer
	return h
}

// DebugCrawlerTask 执行一次不落库的受控采集请求并返回响应预览和字段建议。
func (h *Handler) DebugCrawlerTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}
	if !h.hgAuthorizeCrawler(w, r, userID, "crawler.task.run") {
		return
	}
	if h.crawlerDebug == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: "采集测试服务不可用"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var req CrawlerDtoPackage.HGDebugRequest
	if err := decoder.Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "请求体格式错误"})
		return
	}
	result, err := h.crawlerDebug.TestRequest(r.Context(), req)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, CrawlerServicePackage.ErrHGDebugInvalidRequest) || errors.Is(err, CrawlerServicePackage.ErrHGDebugUnsafeTarget) {
			status = http.StatusBadRequest
		} else if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
		}
		w.WriteHeader(status)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: err.Error()})
		return
	}
	HGResponsePakcage.SuccessResult(w, r, result)
}

// SaveCrawlerTask validates and persists a crawler task definition without executing it.
func (h *Handler) SaveCrawlerTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}
	if h.crawlerTasks == nil {
		hgWriteCrawlerUnavailable(w, r)
		return
	}
	if !h.hgAuthorizeCrawler(w, r, userID, "crawler.task.write") {
		return
	}
	var req CrawlerDtoPackage.HGTaskDefinitionSaveRequest
	if !hgDecodeCrawlerBody(w, r, &req) {
		return
	}
	definition, err := h.crawlerTasks.Save(r.Context(), req, userID)
	if err != nil {
		hgWriteCrawlerTaskError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, definition)
}

// SaveAndRunCrawlerTask persists a definition and returns its first terminal run, including failed runs.
func (h *Handler) SaveAndRunCrawlerTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}
	if h.crawlerTasks == nil {
		hgWriteCrawlerUnavailable(w, r)
		return
	}
	if !h.hgAuthorizeCrawler(w, r, userID, "crawler.task.write") || !h.hgAuthorizeCrawler(w, r, userID, "crawler.task.run") {
		return
	}
	var req CrawlerDtoPackage.HGTaskDefinitionSaveRequest
	if !hgDecodeCrawlerBody(w, r, &req) {
		return
	}
	definition, run, err := h.crawlerTasks.SaveAndRun(r.Context(), req, userID)
	if definition != nil && run != nil {
		HGResponsePakcage.SuccessResult(w, r, map[string]any{"task": definition, "run": run})
		return
	}
	if err != nil {
		hgWriteCrawlerTaskError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, map[string]any{"task": definition, "run": run})
}

// ListCrawlerTasks returns a bounded persisted-definition cursor page.
func (h *Handler) ListCrawlerTasks(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}
	if h.crawlerTasks == nil {
		hgWriteCrawlerUnavailable(w, r)
		return
	}
	if !h.hgAuthorizeCrawler(w, r, userID, "crawler.task.read") {
		return
	}
	cursor, _ := strconv.ParseUint(r.URL.Query().Get("cursor"), 10, 64)
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	result, err := h.crawlerTasks.List(r.Context(), CrawlerDtoPackage.HGTaskDefinitionListRequest{Cursor: cursor, Limit: pageSize})
	if err != nil {
		hgWriteCrawlerTaskError(w, r, err)
		return
	}
	HGResponsePakcage.SuccessResult(w, r, map[string]any{"list": result.Items, "nextCursor": result.NextCursor, "hasMore": result.HasMore, "total": -1})
}

func (h *Handler) hgAuthorizeCrawler(w http.ResponseWriter, r *http.Request, userID, permission string) bool {
	if h.crawlerAuth == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: "采集任务服务不可用"})
		return false
	}
	allowed, err := h.crawlerAuth.HasAssetPermission(r.Context(), userID, permission)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: "权限校验失败"})
		return false
	}
	if !allowed {
		w.WriteHeader(http.StatusForbidden)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.Forbidden.Code, Message: "无采集任务操作权限"})
		return false
	}
	return true
}

func hgDecodeCrawlerBody(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "请求体格式错误"})
		return false
	}
	return true
}

func hgWriteCrawlerUnavailable(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusServiceUnavailable)
	HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: "采集任务服务不可用"})
}

func hgWriteCrawlerTaskError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	code := HGResponsePakcage.InternalError.Code
	if errors.Is(err, CrawlerServicePackage.ErrHGTaskInvalidDefinition) || errors.Is(err, CrawlerServicePackage.ErrHGCrawlerInvalidRequest) || errors.Is(err, CrawlerServicePackage.ErrHGCrawlerUnsafeTarget) {
		status = http.StatusBadRequest
		code = HGResponsePakcage.InvalidParam.Code
	} else if errors.Is(err, CrawlerServicePackage.ErrHGTaskLeaseNotAcquired) {
		status = http.StatusConflict
		code = HGResponsePakcage.Conflict.Code
	}
	w.WriteHeader(status)
	HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: code, Message: err.Error()})
}

// CreateRole 创建角色
func (h *Handler) CreateRole(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	var req OpsDtoPackage.CreateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "请求体格式错误"})
		return
	}

	resp, err := h.service.CreateRole(r.Context(), userID, req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// UpdateRole 更新角色
func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	var req OpsDtoPackage.UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "请求体格式错误"})
		return
	}

	resp, err := h.service.UpdateRole(r.Context(), userID, req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// DeleteRole 删除角色
func (h *Handler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	var req OpsDtoPackage.DeleteRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "请求体格式错误"})
		return
	}

	err := h.service.DeleteRole(r.Context(), userID, req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult[interface{}](w, r, nil)
}

// GetRoleList 获取角色列表
func (h *Handler) GetRoleList(w http.ResponseWriter, r *http.Request) {
	_, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	resp, err := h.service.GetRoleList(r.Context(), cursor, pageSize)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// GetAdminUserList 获取管理员列表。
func (h *Handler) GetAdminUserList(w http.ResponseWriter, r *http.Request) {
	_, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	resp, err := h.service.GetAdminUserList(r.Context(), cursor, pageSize)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// SearchAdminUsers 搜索可分配角色的管理员。
func (h *Handler) SearchAdminUsers(w http.ResponseWriter, r *http.Request) {
	_, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	// 获取URL查询参数,根据Get请求的参数名获取对应的值
	keyword := r.URL.Query().Get("keyword")

	// r.URL.Query() 获取所有URL查询参数
	// strconv.Atoi 把字符串转换成int
	limitStr := r.URL.Query().Get("limit")
	limit := 10 // 默认值
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}
	resp, err := h.service.SearchAdminUsers(r.Context(), keyword, limit)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// SearchAdminCandidates 搜索可添加为管理员的注册用户候选。
func (h *Handler) SearchAdminCandidates(w http.ResponseWriter, r *http.Request) {
	_, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	keyword := r.URL.Query().Get("keyword")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	resp, err := h.service.SearchAdminCandidates(r.Context(), keyword, limit)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// AddAdmin 添加管理员。
func (h *Handler) AddAdmin(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	var req OpsDtoPackage.AddAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "请求体格式错误"})
		return
	}

	resp, err := h.service.AddAdmin(r.Context(), userID, req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// AssignUserRoles 分配用户角色
func (h *Handler) AssignUserRoles(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	var req OpsDtoPackage.AssignUserRolesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "请求体格式错误"})
		return
	}

	err := h.service.AssignUserRoles(r.Context(), userID, req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult[interface{}](w, r, nil)
}

// GetUserRoles 获取用户角色
func (h *Handler) GetUserRoles(w http.ResponseWriter, r *http.Request) {
	_, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	userID := r.URL.Query().Get("userId")
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "缺少用户ID"})
		return
	}

	resp, err := h.service.GetUserRoles(r.Context(), userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// CreateMenu 创建菜单
func (h *Handler) CreateMenu(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	var req OpsDtoPackage.CreateMenuRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "请求体格式错误"})
		return
	}

	resp, err := h.service.CreateMenu(r.Context(), userID, req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// GetMenuList 获取菜单列表
func (h *Handler) GetMenuList(w http.ResponseWriter, r *http.Request) {
	_, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	resp, err := h.service.GetMenuList(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// AssignRolePermissions 分配角色权限
func (h *Handler) AssignRolePermissions(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	var req OpsDtoPackage.AssignRolePermissionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "请求体格式错误"})
		return
	}

	err := h.service.AssignRolePermissions(r.Context(), userID, req)
	if err != nil {
		if errors.Is(err, OpsServicePackage.ErrHGOperationsForbidden) {
			w.WriteHeader(http.StatusForbidden)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.Forbidden.Code, Message: "无 RBAC 管理权限"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: "角色权限分配失败"})
		return
	}

	HGResponsePakcage.SuccessResult[interface{}](w, r, nil)
}

// GetRolePermissions 获取角色权限
func (h *Handler) GetRolePermissions(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	roleID := r.URL.Query().Get("roleId")
	if roleID == "" {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "缺少角色ID"})
		return
	}

	resp, err := h.service.GetRolePermissions(r.Context(), userID, roleID)
	if err != nil {
		if errors.Is(err, OpsServicePackage.ErrHGOperationsForbidden) {
			w.WriteHeader(http.StatusForbidden)
			HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.Forbidden.Code, Message: "无 RBAC 管理权限"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: "角色权限查询失败"})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// UploadFile 上传文件
func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	userID, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	// 解析multipart表单
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "文件过大"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: "缺少文件"})
		return
	}
	defer file.Close()

	resp, err := h.service.UploadFile(r.Context(), userID, file, header.Filename, header.Size, header.Header.Get("Content-Type"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

// GetFileList 获取文件列表
func (h *Handler) GetFileList(w http.ResponseWriter, r *http.Request) {
	_, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	resp, err := h.service.GetFileList(r.Context(), page, pageSize)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult(w, r, resp)
}

package OpsHandlerPackage

import (
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	OpsServicePackage "MLC_GO/internal/modules/ops/service"
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePakcage "MLC_GO/internal/response"
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler 是 ops 模块的 HTTP 入口。
// 只负责鉴权上下文读取、HTTP 参数解析、错误码映射和统一响应，不直接写 SQL 或处理业务细节。
type Handler struct {
	service *OpsServicePackage.Service
}

// NewHandler 创建运维管理处理器，由 module assembly 统一注入 service。
func NewHandler(service *OpsServicePackage.Service) *Handler {
	return &Handler{service: service}
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

// SearchAdminUsers 搜索可分配角色的管理员。
func (h *Handler) SearchAdminUsers(w http.ResponseWriter, r *http.Request) {
	_, ok := HGContextPackage.CurrentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePakcage.FailTokenInvalid(w, r, "unauthorized")
		return
	}

	keyword := r.URL.Query().Get("keyword")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	resp, err := h.service.SearchAdminUsers(r.Context(), keyword, limit)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InvalidParam.Code, Message: err.Error()})
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
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: err.Error()})
		return
	}

	HGResponsePakcage.SuccessResult[interface{}](w, r, nil)
}

// GetRolePermissions 获取角色权限
func (h *Handler) GetRolePermissions(w http.ResponseWriter, r *http.Request) {
	_, ok := HGContextPackage.CurrentUserID(r)
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

	resp, err := h.service.GetRolePermissions(r.Context(), roleID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		HGResponsePakcage.FailResult[string](w, r, HGResponsePakcage.HGErrorResult{Code: HGResponsePakcage.InternalError.Code, Message: err.Error()})
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

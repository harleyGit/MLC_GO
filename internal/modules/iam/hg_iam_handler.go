package iam

import (
	HGContextPackage "MLC_GO/internal/pkg/hg_context"
	HGResponsePackage "MLC_GO/internal/response"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HGHandler 是 /api/v1/iam 的管理端 HTTP 边界；JWT 由路由组验证。
type HGHandler struct {
	service *HGService // 负责权限校验及业务事务。
}

// NewHGHandler 创建 IAM HTTP 处理器。
func NewHGHandler(service *HGService) *HGHandler { return &HGHandler{service: service} }

// HGRoleRequest 是创建和修改角色的 JSON 协议；创建时 code 必填，修改时 id 必填。
type HGRoleRequest struct {
	ID          string   `json:"id"`          // 角色主键，十进制字符串。
	Code        string   `json:"code"`        // 稳定角色编码，仅创建时使用。
	Name        string   `json:"name"`        // 管理端角色名称。
	Description string   `json:"description"` // 角色职责说明。
	Permissions []string `json:"permissions"` // 创建时一并授予的权限编码。
}

// HGBindingRequest 是授权与撤销 JSON 协议；撤销时只使用 userId 和 bindingId。
type HGBindingRequest struct {
	UserID    string     `json:"userId"`    // 现有管理员的业务用户 ID。
	RoleID    string     `json:"roleId"`    // 授予的角色主键，十进制字符串。
	BindingID string     `json:"bindingId"` // 撤销的绑定主键，十进制字符串。
	ScopeType string     `json:"scopeType"` // global、shop 或 org。
	ScopeID   string     `json:"scopeId"`   // global 为空，其余为组织标识。
	ExpiresAt *time.Time `json:"expiresAt"` // 可空的 RFC3339 失效时间。
}

// HGPermissionsRequest 是自定义角色权限全量替换 JSON 协议。
type HGPermissionsRequest struct {
	RoleID      string   `json:"roleId"`      // 自定义角色主键，十进制字符串。
	Permissions []string `json:"permissions"` // 最多 50 个已登记权限编码。
}

func hgActor(w http.ResponseWriter, r *http.Request) (string, bool) {
	actor, ok := HGContextPackage.CurrentUserID(r)
	if !ok || actor == "" {
		w.WriteHeader(http.StatusUnauthorized)
		HGResponsePackage.FailTokenInvalid(w, r, "登录状态无效")
		return "", false
	}
	return actor, true
}

func hgDecode(w http.ResponseWriter, r *http.Request, dest any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			HGResponsePackage.FailResult[string](w, r, HGResponsePackage.HGErrorResult{Code: HGResponsePackage.RequestTooLargeCode, Message: "请求体过大"})
			return false
		}
		hgHTTPError(w, r, ErrHGInvalid)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		hgHTTPError(w, r, ErrHGInvalid)
		return false
	}
	return true
}

func hgRequestID(r *http.Request) string {
	id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	valid := id != "" && len(id) <= 64
	for _, ch := range id {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.') {
			valid = false
			break
		}
	}
	if valid {
		return id
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "generated-unavailable"
	}
	return hex.EncodeToString(random[:])
}

func hgHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	message := "IAM 服务暂不可用"
	code := HGResponsePackage.InternalError.Code
	switch {
	case errors.Is(err, ErrHGInvalid):
		status, message = http.StatusBadRequest, "请求参数无效"
		code = HGResponsePackage.InvalidParam.Code
	case errors.Is(err, ErrHGForbidden):
		status, message = http.StatusForbidden, "无权执行此操作"
		code = HGResponsePackage.Forbidden.Code
	case errors.Is(err, ErrHGNotFound):
		status, message = http.StatusNotFound, "目标不存在"
		code = HGResponsePackage.InvalidParam.Code
	case errors.Is(err, ErrHGConflict):
		status, message = http.StatusConflict, "角色或授权状态冲突"
		code = HGResponsePackage.Conflict.Code
	}
	w.WriteHeader(status)
	HGResponsePackage.FailResult[string](w, r, HGResponsePackage.HGErrorResult{Code: code, Message: message})
}

func hgID(value string) (uint64, error) {
	id, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil || id == 0 {
		return 0, ErrHGInvalid
	}
	return id, nil
}

// HGListRoles 读取活动角色游标页。
func (h *HGHandler) HGListRoles(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	cursor := uint64(0)
	if value := r.URL.Query().Get("cursor"); value != "" && value != "0" {
		var err error
		cursor, err = hgID(value)
		if err != nil {
			hgHTTPError(w, r, err)
			return
		}
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	items, next, more, err := h.service.ListRoles(r.Context(), actor, cursor, limit)
	if err != nil {
		hgHTTPError(w, r, err)
		return
	}
	roles := make([]HGRoleResponse, 0, len(items))
	for _, item := range items {
		roles = append(roles, hgRoleDTO(item))
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"list": roles, "nextCursor": next, "hasMore": more})
}

// HGCreateRole 创建自定义角色。
func (h *HGHandler) HGCreateRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req HGRoleRequest
	if !hgDecode(w, r, &req) {
		return
	}
	role, err := h.service.CreateRole(r.Context(), actor, hgRequestID(r), req.Code, req.Name, req.Description, req.Permissions)
	if err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, hgRoleDTO(role))
}

// HGUpdateRole 更新自定义角色展示信息。
func (h *HGHandler) HGUpdateRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req HGRoleRequest
	if !hgDecode(w, r, &req) {
		return
	}
	id, err := hgID(req.ID)
	if err != nil {
		hgHTTPError(w, r, err)
		return
	}
	role, err := h.service.UpdateRole(r.Context(), actor, hgRequestID(r), id, req.Name, req.Description)
	if err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, hgRoleDTO(role))
}

// HGDisableRole 停用未绑定的自定义角色。
func (h *HGHandler) HGDisableRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req HGRoleRequest
	if !hgDecode(w, r, &req) {
		return
	}
	id, err := hgID(req.ID)
	if err != nil {
		hgHTTPError(w, r, err)
		return
	}
	if err = h.service.DisableRole(r.Context(), actor, hgRequestID(r), id); err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"disabled": true})
}

// HGListBindings 读取指定管理员的角色绑定。
func (h *HGHandler) HGListBindings(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	cursor := uint64(0)
	if value := r.URL.Query().Get("cursor"); value != "" && value != "0" {
		var err error
		cursor, err = hgID(value)
		if err != nil {
			hgHTTPError(w, r, err)
			return
		}
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	items, next, more, err := h.service.ListBindings(r.Context(), actor, r.URL.Query().Get("userId"), cursor, limit)
	if err != nil {
		hgHTTPError(w, r, err)
		return
	}
	bindings := make([]HGBindingResponse, 0, len(items))
	for _, item := range items {
		bindings = append(bindings, hgBindingDTO(item))
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"list": bindings, "nextCursor": next, "hasMore": more})
}

// HGGrant 授予一个角色及作用域。
func (h *HGHandler) HGGrant(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req HGBindingRequest
	if !hgDecode(w, r, &req) {
		return
	}
	id, err := hgID(req.RoleID)
	if err != nil {
		hgHTTPError(w, r, err)
		return
	}
	if err = h.service.Grant(r.Context(), actor, hgRequestID(r), req.UserID, id, req.ScopeType, req.ScopeID, req.ExpiresAt); err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"granted": true})
}

// HGRevoke 撤销一条精确绑定。
func (h *HGHandler) HGRevoke(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req HGBindingRequest
	if !hgDecode(w, r, &req) {
		return
	}
	id, err := hgID(req.BindingID)
	if err != nil {
		hgHTTPError(w, r, err)
		return
	}
	if err = h.service.Revoke(r.Context(), actor, hgRequestID(r), req.UserID, id); err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"revoked": true})
}

// HGListPermissions 返回权限目录及可选角色的已选权限。
func (h *HGHandler) HGListPermissions(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	roleID := uint64(0)
	if value := r.URL.Query().Get("roleId"); value != "" {
		var err error
		roleID, err = hgID(value)
		if err != nil {
			hgHTTPError(w, r, err)
			return
		}
	}
	items, codes, err := h.service.ListPermissions(r.Context(), actor, roleID)
	if err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"list": items, "selected": codes})
}

// HGSetRolePermissions 全量替换自定义角色的权限。
func (h *HGHandler) HGSetRolePermissions(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req HGPermissionsRequest
	if !hgDecode(w, r, &req) {
		return
	}
	id, err := hgID(req.RoleID)
	if err != nil {
		hgHTTPError(w, r, err)
		return
	}
	if err = h.service.SetRolePermissions(r.Context(), actor, hgRequestID(r), id, req.Permissions); err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"updated": true})
}

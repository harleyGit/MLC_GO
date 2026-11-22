package iam

import (
	HGResponsePackage "MLC_GO/internal/response"
	"net/http"
	"strconv"
)

// HGCreateScopeResourceRequest 是六类 IAM 占位资源登记协议；不接受客户端自选 ID、owner 或状态。
type HGCreateScopeResourceRequest struct {
	ScopeType string `json:"scopeType"` // 六类已接入 IAM 登记的作用域编码，不能使用 global/shop/org。
	Name      string `json:"name"`      // 资源显示名称；1-128 字符，不能作为业务权威实体证明。
}

// HGCreateBusinessScope 从权威用户/组织验证归属后，原子建立业务实体和 IAM 资源；不转换历史占位记录。
func (h *HGHandler) HGCreateBusinessScope(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req HGBusinessScopeCreate
	if !hgDecode(w, r, &req) {
		return
	}
	item, err := h.service.HGCreateBusinessScope(r.Context(), actor, hgRequestID(r), req)
	if err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, item)
}

// HGDisableBusinessScopeRequest 是按类型和不可变业务 ID 停用单项资源的协议；不接受客户端自报所有者或状态。
type HGDisableBusinessScopeRequest struct {
	ScopeType string `json:"scopeType"` // 六类可管理的权威业务类型。
	ScopeID   string `json:"scopeId"`   // 创建接口返回的 ID，不使用名称或区域代码。
}

// HGDisableBusinessScope 同事务停用业务实体和授权候选；已有绑定保留用于审计但不能继续执行业务权限。
func (h *HGHandler) HGDisableBusinessScope(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req HGDisableBusinessScopeRequest
	if !hgDecode(w, r, &req) {
		return
	}
	if err := h.service.HGDisableBusinessScope(r.Context(), actor, hgRequestID(r), req.ScopeType, req.ScopeID); err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"disabled": true})
}

// HGCreateScopeResource 以服务器生成的 ID 登记可授权候选；业务操作仍需另行接入实际资源鉴权。
func (h *HGHandler) HGCreateScopeResource(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req HGCreateScopeResourceRequest
	if !hgDecode(w, r, &req) {
		return
	}
	item, err := h.service.HGCreateScopeResource(r.Context(), actor, hgRequestID(r), req.ScopeType, req.Name)
	if err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, item)
}

// HGListScopeResources 返回已启用 org 或已审核 shop 的游标页，供即时授权选择具体 scopeId。
func (h *HGHandler) HGListScopeResources(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if r.URL.Query().Get("pageSize") != "" && err != nil {
		hgHTTPError(w, r, ErrHGInvalid)
		return
	}
	items, next, more, err := h.service.HGListScopeResources(r.Context(), actor, r.URL.Query().Get("scopeType"), r.URL.Query().Get("cursor"), limit)
	if err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"list": items, "nextCursor": next, "hasMore": more})
}

// HGScopeTypeRequest 是类型登记/编辑/停用协议；编码只在创建时可设，其他字段由服务端强制约束。
type HGScopeTypeRequest struct {
	Code        string `json:"code"`        // 唯一稳定编码。
	Name        string `json:"name"`        // 中文显示名称。
	Description string `json:"description"` // 业务用途。
	Status      *int   `json:"status"`      // 1 启用、0 停用；省略或 null 时创建默认 0，编辑保留原值，停用接口忽略。
}

// HGListScopeTypes 返回有界字典列表，包含尚未启用的设计类型。
func (h *HGHandler) HGListScopeTypes(w http.ResponseWriter, r *http.Request) {
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
	items, next, more, err := h.service.HGListScopeTypes(r.Context(), actor, cursor, limit)
	if err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]any{"list": items, "nextCursor": next, "hasMore": more})
}

// HGGetScopeType 通过编码查询当前数据库状态供编辑页回填。
func (h *HGHandler) HGGetScopeType(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	item, err := h.service.HGGetScopeType(r.Context(), actor, r.URL.Query().Get("code"))
	if err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, item)
}

// HGCreateScopeType 创建类型及指定的字典状态；省略状态时默认停用。
func (h *HGHandler) HGCreateScopeType(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req HGScopeTypeRequest
	if !hgDecode(w, r, &req) {
		return
	}
	if err := h.service.HGCreateScopeType(r.Context(), actor, hgRequestID(r), req.Code, req.Name, req.Description, req.Status); err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"created": true})
}

// HGUpdateScopeType 更新名称、描述及可选状态；资源 ID 校验规则由服务端保持。
func (h *HGHandler) HGUpdateScopeType(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req HGScopeTypeRequest
	if !hgDecode(w, r, &req) {
		return
	}
	if err := h.service.HGUpdateScopeType(r.Context(), actor, hgRequestID(r), req.Code, req.Name, req.Description, req.Status); err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"updated": true})
}

// HGDisableScopeType 有历史绑定或是现有授权类型时拒绝停用。
func (h *HGHandler) HGDisableScopeType(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req HGScopeTypeRequest
	if !hgDecode(w, r, &req) {
		return
	}
	if err := h.service.HGDisableScopeType(r.Context(), actor, hgRequestID(r), req.Code); err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"disabled": true})
}

// HGGrantDirectBatch 一次 MFA 原子授权多个有界绑定；权限仅可收窄到角色现有权限子集，不接受自行扩张的作用域类型。
func (h *HGHandler) HGGrantDirectBatch(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req HGBatchGrantRequest
	if !hgDecode(w, r, &req) {
		return
	}
	if err := h.service.HGGrantDirectBatch(r.Context(), actor, hgRequestID(r), req); err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"granted": true})
}

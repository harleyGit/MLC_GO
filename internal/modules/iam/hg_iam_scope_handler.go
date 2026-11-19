package iam

import (
	HGResponsePackage "MLC_GO/internal/response"
	"net/http"
	"strconv"
)

// HGScopeTypeRequest 是类型登记/编辑/停用协议；编码只在创建时可设，其他字段由服务端强制约束。
type HGScopeTypeRequest struct {
	Code        string `json:"code"`        // 唯一稳定编码。
	Name        string `json:"name"`        // 中文显示名称。
	Description string `json:"description"` // 业务用途。
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

// HGCreateScopeType 创建默认停用的类型；不允许通过管理页面启用未知资源的授权。
func (h *HGHandler) HGCreateScopeType(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req HGScopeTypeRequest
	if !hgDecode(w, r, &req) {
		return
	}
	if err := h.service.HGCreateScopeType(r.Context(), actor, hgRequestID(r), req.Code, req.Name, req.Description); err != nil {
		hgHTTPError(w, r, err)
		return
	}
	HGResponsePackage.SuccessResult(w, r, map[string]bool{"created": true})
}

// HGUpdateScopeType 不接受客户端指定状态或更改授权 ID 校验规则。
func (h *HGHandler) HGUpdateScopeType(w http.ResponseWriter, r *http.Request) {
	actor, ok := hgActor(w, r)
	if !ok {
		return
	}
	var req HGScopeTypeRequest
	if !hgDecode(w, r, &req) {
		return
	}
	if err := h.service.HGUpdateScopeType(r.Context(), actor, hgRequestID(r), req.Code, req.Name, req.Description); err != nil {
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

// HGGrantDirectBatch 一次 MFA 原子授权多个有界绑定；不接受权限编码或自行扩张的作用域类型。
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

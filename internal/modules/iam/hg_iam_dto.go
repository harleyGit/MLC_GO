package iam

import (
	"strconv"
	"time"
)

// HGRoleResponse 是 IAM 管理 API 的角色 JSON 协议，主键使用字符串避免 JS 精度损失。
type HGRoleResponse struct {
	ID          string    `json:"id"`          // 角色主键，十进制字符串。
	Code        string    `json:"code"`        // 稳定角色编码。
	Name        string    `json:"name"`        // 管理端显示名称。
	Description string    `json:"description"` // 角色职责说明。
	Status      int       `json:"status"`      // 1 启用，0 停用。
	BuiltIn     bool      `json:"builtIn"`     // 内置角色标记。
	CreatedAt   time.Time `json:"createdAt"`   // RFC3339 创建时间。
}

// HGBindingResponse 是 IAM 管理 API 的员工角色绑定 JSON 协议。
type HGBindingResponse struct {
	ID        string     `json:"id"`        // 绑定主键，十进制字符串。
	UserID    string     `json:"userId"`    // 管理员业务用户 ID。
	RoleID    string     `json:"roleId"`    // IAM 角色主键，十进制字符串。
	RoleCode  string     `json:"roleCode"`  // 稳定角色编码。
	ScopeType string     `json:"scopeType"` // global、shop 或 org。
	ScopeID   string     `json:"scopeId"`   // 作用域标识。
	ExpiresAt *time.Time `json:"expiresAt"` // 可空的 RFC3339 失效时间。
}

func hgRoleDTO(role HGRole) HGRoleResponse {
	return HGRoleResponse{ID: strconv.FormatUint(role.ID, 10), Code: role.Code, Name: role.Name, Description: role.Description, Status: role.Status, BuiltIn: role.BuiltIn, CreatedAt: role.CreatedAt}
}

func hgBindingDTO(binding HGBinding) HGBindingResponse {
	return HGBindingResponse{ID: strconv.FormatUint(binding.ID, 10), UserID: binding.UserID, RoleID: strconv.FormatUint(binding.RoleID, 10), RoleCode: binding.RoleCode, ScopeType: binding.ScopeType, ScopeID: binding.ScopeID, ExpiresAt: binding.ExpiresAt}
}

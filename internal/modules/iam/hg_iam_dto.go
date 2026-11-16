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

// HGPendingGrantResponse 是按目标管理员查询待审批角色申请的只读 JSON 协议。
// 与 HGBindingResponse 分离：申请未获第二名管理员批准，绝不能作为已生效绑定或撤销对象。
type HGPendingGrantResponse struct {
	ApprovalID        string     `json:"approvalId"`        // 申请主键十进制字符串，仅用于审批队列定位；不是绑定 ID。
	RoleID            string     `json:"roleId"`            // 申请授予的 IAM 角色主键十进制字符串。
	ScopeType         string     `json:"scopeType"`         // global、shop 或 org，由审批快照读取。
	ScopeID           string     `json:"scopeId"`           // global 时为空，其余为组织/店铺业务 ID。
	ExpiresAt         *time.Time `json:"expiresAt"`         // 拟授予角色的失效时间；null 表示长期有效，审批前不生效。
	ApprovalExpiresAt time.Time  `json:"approvalExpiresAt"` // 审批单自身的截止时间，过期后不再返回。
	CreatedAt         time.Time  `json:"createdAt"`         // 申请创建时间（数据库 DATETIME(3)）；用于只读展示。
	Status            string     `json:"status"`            // 恒为 pending，不代表已授予权限。
}

func hgRoleDTO(role HGRole) HGRoleResponse {
	return HGRoleResponse{ID: strconv.FormatUint(role.ID, 10), Code: role.Code, Name: role.Name, Description: role.Description, Status: role.Status, BuiltIn: role.BuiltIn, CreatedAt: role.CreatedAt}
}

func hgBindingDTO(binding HGBinding) HGBindingResponse {
	return HGBindingResponse{ID: strconv.FormatUint(binding.ID, 10), UserID: binding.UserID, RoleID: strconv.FormatUint(binding.RoleID, 10), RoleCode: binding.RoleCode, ScopeType: binding.ScopeType, ScopeID: binding.ScopeID, ExpiresAt: binding.ExpiresAt}
}

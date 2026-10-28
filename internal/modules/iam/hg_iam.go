// Package iam 提供平台角色、作用域绑定和服务端权限判断。
package iam

import (
	"errors"
	"time"
)

var (
	ErrHGInvalid   = errors.New("IAM 参数无效")
	ErrHGForbidden = errors.New("无 IAM 管理权限")
	ErrHGNotFound  = errors.New("IAM 记录不存在")
	ErrHGConflict  = errors.New("IAM 记录冲突")
)

// HGRole 是 IAM 业务角色；Code 是稳定授权标识，ID 仅用于存储和游标。
type HGRole struct {
	ID          uint64    // 角色主键。
	Code        string    // 跨版本稳定的角色编码。
	Name        string    // 管理端显示名称。
	Description string    // 角色职责说明。
	Status      int       // 1 启用，0 停用。
	BuiltIn     bool      // 内置角色不可删除或停用。
	CreatedAt   time.Time // 创建时间，数据库本地时区。
}

// HGBinding 表示用户在一个确定作用域内的角色授权。
type HGBinding struct {
	ID        uint64     // 绑定主键。
	UserID    string     // 现有 JWT 的业务用户 ID。
	RoleID    uint64     // IAM 角色主键。
	RoleCode  string     // 返回给管理端的角色编码。
	ScopeType string     // global、shop 或 org。
	ScopeID   string     // global 时为空，其余作用域的服务端标识。
	ExpiresAt *time.Time // 可空的授权失效时间。
}

// HGResource 是业务服务从可信存储读取的授权上下文，不能直接使用客户端属性填充。
type HGResource struct {
	ScopeType       string   // 资源作用域类型。
	ScopeID         string   // 资源所属作用域 ID。
	OwnerID         string   // 资源所有者业务用户 ID。
	Status          string   // 资源当前业务状态。
	RequireOwner    bool     // 是否要求当前人是资源所有者。
	AllowedStatuses []string // 允许执行动作的资源状态白名单，空表示不限制。
}

// HGDecision 描述默认拒绝的授权结果。
type HGDecision struct {
	Allowed bool   // 是否允许执行操作。
	Reason  string // 供服务端诊断的拒绝原因。
}

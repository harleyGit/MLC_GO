package iam

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var hgRoleCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)
var hgPermissionCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*:[a-z][a-z0-9_]*:[a-z][a-z0-9_]*$`)

// HGService 对管理端做默认拒绝鉴权，业务授权直接读取 MySQL 权威数据。
type HGService struct {
	repo *HGRepository // IAM 权威存储；失败时不放行。
}

// NewHGService 注入 IAM Repository。
func NewHGService(repo *HGRepository) *HGService { return &HGService{repo: repo} }

func hgValidText(value string, max int) bool {
	value = strings.TrimSpace(value)
	return value != "" && len([]rune(value)) <= max
}

func (s *HGService) hgCanManage(ctx context.Context, actor string, write bool) error {
	if s == nil || s.repo == nil || !hgValidText(actor, 64) {
		return ErrHGForbidden
	}
	admin, err := s.repo.hgActiveAdmin(ctx, actor)
	if err != nil {
		return err
	}
	if !admin {
		return ErrHGForbidden
	}
	permission := "iam:role:read"
	if write {
		permission = "iam:role:grant"
	}
	allowed, err := s.repo.hgHasPermission(ctx, actor, permission, "global", "")
	if err != nil {
		return err
	}
	if !allowed {
		return ErrHGForbidden
	}
	return nil
}

// Authorize 校验权限及可信资源属性；调用方须从数据库读取 OwnerID、ScopeID 和 Status。
func (s *HGService) Authorize(ctx context.Context, userID, permission string, resource HGResource) (HGDecision, error) {
	deny := HGDecision{Reason: "默认拒绝"}
	if s == nil || s.repo == nil || !hgValidText(userID, 64) || !hgPermissionCodePattern.MatchString(permission) {
		return deny, ErrHGInvalid
	}
	if resource.ScopeType != "" && resource.ScopeType != "global" && resource.ScopeType != "shop" && resource.ScopeType != "org" {
		return deny, ErrHGInvalid
	}
	if resource.ScopeType == "global" && resource.ScopeID != "" {
		return deny, ErrHGInvalid
	}
	if resource.RequireOwner && resource.OwnerID != userID {
		return HGDecision{Reason: "资源不属于当前用户"}, nil
	}
	if len(resource.AllowedStatuses) > 0 {
		valid := false
		for _, status := range resource.AllowedStatuses {
			if status == resource.Status {
				valid = true
				break
			}
		}
		if !valid {
			return HGDecision{Reason: "资源状态不允许操作"}, nil
		}
	}
	if resource.ScopeType != "" && resource.ScopeType != "global" {
		if !hgValidText(resource.ScopeID, 64) {
			return deny, ErrHGInvalid
		}
		allowed, err := s.repo.hgHasPermission(ctx, userID, permission, resource.ScopeType, resource.ScopeID)
		if err != nil {
			return deny, err
		}
		if allowed {
			return HGDecision{Allowed: true}, nil
		}
	}
	allowed, err := s.repo.hgHasPermission(ctx, userID, permission, "global", "")
	if err != nil {
		return deny, err
	}
	if allowed {
		return HGDecision{Allowed: true}, nil
	}
	return deny, nil
}

// ListRoles 返回有界角色游标页，不执行 COUNT 或深 OFFSET。
func (s *HGService) ListRoles(ctx context.Context, actor string, cursor uint64, limit int) ([]HGRole, string, bool, error) {
	if err := s.hgCanManage(ctx, actor, false); err != nil {
		return nil, "", false, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	items, more, err := s.repo.hgListRoles(ctx, cursor, limit)
	if err != nil {
		return nil, "", false, err
	}
	next := ""
	if len(items) > 0 {
		next = strconv.FormatUint(items[len(items)-1].ID, 10)
	}
	return items, next, more, nil
}

// CreateRole 创建具有稳定编码的自定义角色；授权变更与审计同事务提交。
func (s *HGService) CreateRole(ctx context.Context, actor, requestID, code, name, description string, permissions []string) (HGRole, error) {
	if err := s.hgCanManage(ctx, actor, true); err != nil {
		return HGRole{}, err
	}
	code, name, description = strings.TrimSpace(code), strings.TrimSpace(name), strings.TrimSpace(description)
	if !hgRoleCodePattern.MatchString(code) || !hgValidText(name, 100) || len([]rune(description)) > 255 {
		return HGRole{}, ErrHGInvalid
	}
	if err := s.hgCanDelegate(ctx, actor, permissions); err != nil {
		return HGRole{}, err
	}
	return s.repo.hgCreateRole(ctx, actor, requestID, code, name, description, permissions)
}

func (s *HGService) hgCanDelegate(ctx context.Context, actor string, permissions []string) error {
	if len(permissions) > 50 {
		return ErrHGInvalid
	}
	seen := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		if !hgPermissionCodePattern.MatchString(permission) {
			return ErrHGInvalid
		}
		if _, ok := seen[permission]; ok {
			return ErrHGInvalid
		}
		seen[permission] = struct{}{}
		allowed, err := s.repo.hgHasPermission(ctx, actor, permission, "global", "")
		if err != nil {
			return err
		}
		if !allowed {
			return ErrHGForbidden
		}
	}
	return nil
}

// UpdateRole 修改自定义角色的显示信息，内置角色由迁移维护。
func (s *HGService) UpdateRole(ctx context.Context, actor, requestID string, id uint64, name, description string) (HGRole, error) {
	if err := s.hgCanManage(ctx, actor, true); err != nil {
		return HGRole{}, err
	}
	name, description = strings.TrimSpace(name), strings.TrimSpace(description)
	if id == 0 || !hgValidText(name, 100) || len([]rune(description)) > 255 {
		return HGRole{}, ErrHGInvalid
	}
	return s.repo.hgUpdateRole(ctx, actor, requestID, id, name, description)
}

// DisableRole 停用没有活动绑定的自定义角色。
func (s *HGService) DisableRole(ctx context.Context, actor, requestID string, id uint64) error {
	if err := s.hgCanManage(ctx, actor, true); err != nil {
		return err
	}
	if id == 0 {
		return ErrHGInvalid
	}
	return s.repo.hgDisableRole(ctx, actor, requestID, id)
}

// ListBindings 返回一个管理员的有界角色绑定；不枚举全量用户。
func (s *HGService) ListBindings(ctx context.Context, actor, userID string, cursor uint64, limit int) ([]HGBinding, string, bool, error) {
	if err := s.hgCanManage(ctx, actor, false); err != nil {
		return nil, "", false, err
	}
	if !hgValidText(userID, 64) {
		return nil, "", false, ErrHGInvalid
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	items, more, err := s.repo.hgBindings(ctx, strings.TrimSpace(userID), cursor, limit)
	if err != nil {
		return nil, "", false, err
	}
	next := ""
	if len(items) > 0 {
		next = strconv.FormatUint(items[len(items)-1].ID, 10)
	}
	return items, next, more, nil
}

// Grant 将一个活动角色授予现有管理员，非全局作用域必须先在 IAM 组织表登记。
func (s *HGService) Grant(ctx context.Context, actor, requestID, userID string, roleID uint64, scopeType, scopeID string, expires *time.Time) error {
	if err := s.hgCanManage(ctx, actor, true); err != nil {
		return err
	}
	userID, scopeType, scopeID = strings.TrimSpace(userID), strings.TrimSpace(scopeType), strings.TrimSpace(scopeID)
	if !hgValidText(userID, 64) || roleID == 0 {
		return ErrHGInvalid
	}
	if expires != nil && !expires.After(time.Now()) {
		return ErrHGInvalid
	}
	if scopeType == "global" {
		if scopeID != "" {
			return ErrHGInvalid
		}
	} else if scopeType == "shop" || scopeType == "org" {
		if !hgValidText(scopeID, 64) {
			return ErrHGInvalid
		}
		active, err := s.repo.hgOrganization(ctx, scopeType, scopeID)
		if err != nil {
			return err
		}
		if !active {
			return ErrHGNotFound
		}
	} else {
		return ErrHGInvalid
	}
	active, err := s.repo.hgActiveAdmin(ctx, userID)
	if err != nil {
		return err
	}
	if !active {
		return ErrHGNotFound
	}
	permissions, err := s.repo.hgRolePermissions(ctx, roleID)
	if err != nil {
		return err
	}
	if err = s.hgCanDelegate(ctx, actor, permissions); err != nil {
		return err
	}
	return s.repo.hgGrant(ctx, actor, requestID, userID, roleID, scopeType, scopeID, expires)
}

// Revoke 撤销精确绑定；超级管理员引导绑定受保护，需受控迁移处理。
func (s *HGService) Revoke(ctx context.Context, actor, requestID, userID string, bindingID uint64) error {
	if err := s.hgCanManage(ctx, actor, true); err != nil {
		return err
	}
	if !hgValidText(userID, 64) || bindingID == 0 {
		return ErrHGInvalid
	}
	return s.repo.hgRevoke(ctx, actor, requestID, strings.TrimSpace(userID), bindingID)
}

// ListPermissions 返回有界权限目录及指定角色的当前权限。
func (s *HGService) ListPermissions(ctx context.Context, actor string, roleID uint64) ([]map[string]string, []string, error) {
	if err := s.hgCanManage(ctx, actor, false); err != nil {
		return nil, nil, err
	}
	items, err := s.repo.hgPermissions(ctx)
	if err != nil {
		return nil, nil, err
	}
	if roleID == 0 {
		return items, []string{}, nil
	}
	codes, err := s.repo.hgRolePermissions(ctx, roleID)
	return items, codes, err
}

// SetRolePermissions 全量替换自定义角色权限；重复或未知编码拒绝并回滚。
func (s *HGService) SetRolePermissions(ctx context.Context, actor, requestID string, roleID uint64, codes []string) error {
	if err := s.hgCanManage(ctx, actor, true); err != nil {
		return err
	}
	if roleID == 0 || len(codes) > 50 {
		return ErrHGInvalid
	}
	if err := s.hgCanDelegate(ctx, actor, codes); err != nil {
		return err
	}
	return s.repo.hgSetRolePermissions(ctx, actor, requestID, roleID, codes)
}

// IsDenied 供 HTTP 层区分业务拒绝和依赖失败。
func IsDenied(err error) bool {
	return errors.Is(err, ErrHGForbidden) || errors.Is(err, ErrHGInvalid) || errors.Is(err, ErrHGNotFound) || errors.Is(err, ErrHGConflict)
}

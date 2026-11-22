package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	HGPermissionPackage "MLC_GO/internal/pkg/permission"
	"context"
	"database/sql"
	"errors"
	"sort"
	"strconv"
	"strings"
)

// HGBatchGrantItem 是即时批量授权的单个绑定；permissions 省略继承整个角色，显式空数组表示无权限。
type HGBatchGrantItem struct {
	RoleID      string    `json:"roleId"`                // IAM 角色主键的十进制字符串。
	ScopeType   string    `json:"scopeType"`             // 字典中的已启用作用域编码。
	ScopeID     string    `json:"scopeId"`               // global 为空；shop 可用已审核商家 ID，org 必须已登记。
	Permissions *[]string `json:"permissions,omitempty"` // 仅本绑定允许的角色权限子集；nil 兼容历史客户端，全选时仍显式保存集合。
}

// HGBatchGrantRequest 是一次 MFA 授予多个绑定的协议；最多 12 项，全部成功或全部回滚。
type HGBatchGrantRequest struct {
	UserID   string             `json:"userId"`   // 目标 users.user_id，不得为操作者本人。
	MFACode  string             `json:"mfaCode"`  // 只消费一次的六位本人 TOTP。
	Bindings []HGBatchGrantItem `json:"bindings"` // 一次请求内不允许重复的角色与作用域组合。
}

// HGGrantDirectBatch 一次 MFA 保护整个有界事务，禁止客户端通过循环单条请求重用验证码。
func (s *HGService) HGGrantDirectBatch(ctx context.Context, actor, requestID string, req HGBatchGrantRequest) error {
	if !hgValidText(requestID, 64) || !hgValidText(req.UserID, 64) || len(req.Bindings) == 0 || len(req.Bindings) > 12 {
		return ErrHGInvalid
	}
	userID := strings.TrimSpace(req.UserID)
	if actor == userID {
		return ErrHGForbidden
	}
	if err := s.hgCanManage(ctx, actor, true); err != nil {
		return err
	}
	allowed, err := s.repo.hgHasPermission(ctx, actor, string(HGPermissionPackage.HGIAMRoleGrantDirect), "global", "")
	if err != nil {
		return err
	}
	if !allowed {
		return ErrHGForbidden
	}
	self, err := s.repo.hgGrantIsSelf(ctx, actor, userID)
	if err != nil {
		return err
	}
	if self {
		return ErrHGForbidden
	}
	seen := make(map[string]struct{}, len(req.Bindings))
	roles := make(map[uint64]struct{}, 4)
	scopes := make(map[string]struct{}, 3)
	for _, item := range req.Bindings {
		if item.Permissions != nil {
			if len(*item.Permissions) > 200 {
				return ErrHGInvalid
			}
			codes := make(map[string]bool, len(*item.Permissions))
			for _, code := range *item.Permissions {
				if !hgValidText(code, 128) || strings.TrimSpace(code) != code || codes[code] {
					return ErrHGInvalid
				}
				codes[code] = true
			}
		}
		id, parseErr := strconv.ParseUint(item.RoleID, 10, 64)
		if parseErr != nil || id == 0 || (item.ScopeType != "global" && item.ScopeType != "shop" && item.ScopeType != "org" && !hgRegisteredScopeType(item.ScopeType)) {
			return ErrHGInvalid
		}
		roles[id] = struct{}{}
		scopes[item.ScopeType] = struct{}{}
		if len(roles) > 4 || len(scopes) > 9 {
			return ErrHGInvalid
		}
		scopeID := strings.TrimSpace(item.ScopeID)
		if (item.ScopeType == "global" && scopeID != "") || (item.ScopeType != "global" && !hgValidText(scopeID, 64)) {
			return ErrHGInvalid
		}
		key := strconv.FormatUint(id, 10) + "\x00" + item.ScopeType + "\x00" + scopeID
		if _, ok := seen[key]; ok {
			return ErrHGInvalid
		}
		seen[key] = struct{}{}
		if err := s.hgValidateGrant(ctx, actor, userID, id, item.ScopeType, scopeID, nil, true); err != nil {
			return err
		}
	}
	if err := s.hgLimitMFA(ctx, actor); err != nil {
		return err
	}
	tx, err := s.repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var one int
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMPermission, actor, "global", "", string(HGPermissionPackage.HGIAMRoleGrantDirect)).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrHGForbidden
	}
	if err != nil {
		return err
	}
	if err = s.hgConsumeMFATx(ctx, tx, actor, req.MFACode); err != nil {
		return err
	}
	// 固定锁顺序降低并发批量授权间的角色行锁死锁风险。
	sort.Slice(req.Bindings, func(i, j int) bool {
		left, _ := strconv.ParseUint(req.Bindings[i].RoleID, 10, 64)
		right, _ := strconv.ParseUint(req.Bindings[j].RoleID, 10, 64)
		if left != right {
			return left < right
		}
		if req.Bindings[i].ScopeType != req.Bindings[j].ScopeType {
			return req.Bindings[i].ScopeType < req.Bindings[j].ScopeType
		}
		return req.Bindings[i].ScopeID < req.Bindings[j].ScopeID
	})
	for _, item := range req.Bindings {
		var needsID bool
		err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMActiveScopeType, item.ScopeType).Scan(&needsID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGInvalid
		}
		if err != nil {
			return err
		}
		if needsID != (item.ScopeType != "global") {
			return ErrHGInvalid
		}
		id, _ := strconv.ParseUint(item.RoleID, 10, 64)
		if err = s.repo.hgGrantTx(ctx, tx, actor, actor, requestID, "iam.binding.grant_direct", userID, id, item.ScopeType, strings.TrimSpace(item.ScopeID), nil); err != nil {
			return err
		}
		if item.Permissions != nil {
			if err = s.repo.hgSetBindingPermissionLimitTx(ctx, tx, actor, requestID, userID, id, item); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// hgSetBindingPermissionLimitTx 在已锁定角色并写入绑定后验证权限子集；失败回滚绑定、MFA、缓存版本与审计。
func (r *HGRepository) hgSetBindingPermissionLimitTx(ctx context.Context, tx *sql.Tx, actor, requestID, userID string, roleID uint64, item HGBatchGrantItem) error {
	rows, err := tx.QueryContext(ctx, SQLQueriesPackage.HGSelectIAMRolePermissions, roleID, 201)
	if err != nil {
		return err
	}
	available := make(map[string]bool)
	for rows.Next() {
		var code string
		if err = rows.Scan(&code); err != nil {
			rows.Close()
			return err
		}
		available[code] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(available) > 200 {
		return ErrHGForbidden
	}
	for _, code := range *item.Permissions {
		if !available[code] {
			return ErrHGInvalid
		}
	}
	var bindingID uint64
	if err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMBindingIDForLimit, userID, roleID, item.ScopeType, strings.TrimSpace(item.ScopeID)).Scan(&bindingID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, SQLQueriesPackage.HGInsertIAMBindingLimit, bindingID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, SQLQueriesPackage.HGDeleteIAMBindingAllowed, bindingID); err != nil {
		return err
	}
	for _, code := range *item.Permissions {
		result, insertErr := tx.ExecContext(ctx, SQLQueriesPackage.HGInsertIAMBindingAllowed, bindingID, code)
		if insertErr != nil {
			return insertErr
		}
		affected, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return rowsErr
		}
		if affected != 1 {
			return ErrHGInvalid
		}
	}
	return hgAudit(ctx, tx, actor, "iam.binding.permissions_limit", "user", userID, requestID, map[string]any{
		"bindingId": bindingID, "permissionCount": len(*item.Permissions), "permissions": *item.Permissions,
	})
}

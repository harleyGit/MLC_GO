package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
)

// HGScopeType 是 IAM 作用域类型字典 API 协议；status=0 的类型只供管理查看，不可用于新授权。
type HGScopeType struct {
	ID              string `json:"id"`              // 数据库主键字符串，避免浏览器整数精度丢失。
	Code            string `json:"code"`            // 稳定作用域编码；更新时不可变。
	Name            string `json:"name"`            // 管理端显示名称。
	Description     string `json:"description"`     // 类型业务用途说明。
	RequiresScopeID bool   `json:"requiresScopeId"` // 是否需要具体已登记的业务资源 ID。
	Status          int    `json:"status"`          // 1 可授权，0 未接入或已停用。
}

func hgScanScope(row interface{ Scan(...any) error }) (HGScopeType, error) {
	var id uint64
	var item HGScopeType
	err := row.Scan(&id, &item.Code, &item.Name, &item.Description, &item.RequiresScopeID, &item.Status)
	item.ID = strconv.FormatUint(id, 10)
	return item, err
}

// HGListScopeTypes 返回完整状态的字典游标页；管理端可展示尚未接通业务校验的类型。
func (s *HGService) HGListScopeTypes(ctx context.Context, actor string, cursor uint64, limit int) ([]HGScopeType, string, bool, error) {
	if err := s.hgCanManage(ctx, actor, false); err != nil {
		return nil, "", false, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	query, args := SQLQueriesPackage.HGSelectIAMScopeTypesFirst, []any{limit + 1}
	if cursor > 0 {
		query, args = SQLQueriesPackage.HGSelectIAMScopeTypesAfter, []any{cursor, limit + 1}
	}
	rows, err := s.repo.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", false, err
	}
	defer rows.Close()
	items := make([]HGScopeType, 0, limit+1)
	for rows.Next() {
		item, scanErr := hgScanScope(rows)
		if scanErr != nil {
			return nil, "", false, scanErr
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, "", false, err
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	next := ""
	if len(items) > 0 {
		next = items[len(items)-1].ID
	}
	return items, next, more, nil
}

// HGGetScopeType 根据稳定编码读取编辑回填；不能从列表旧数据直接提交。
func (s *HGService) HGGetScopeType(ctx context.Context, actor, code string) (HGScopeType, error) {
	if err := s.hgCanManage(ctx, actor, false); err != nil {
		return HGScopeType{}, err
	}
	if !hgRoleCodePattern.MatchString(code) {
		return HGScopeType{}, ErrHGInvalid
	}
	item, err := hgScanScope(s.repo.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMScopeType, code))
	if errors.Is(err, sql.ErrNoRows) {
		return HGScopeType{}, ErrHGNotFound
	}
	return item, err
}

// HGCreateScopeType 登记新类型元数据但默认停用；未接入业务资源校验和权限引擎前不得开放授权。
func (s *HGService) HGCreateScopeType(ctx context.Context, actor, requestID, code, name, description string) error {
	if err := s.hgCanManage(ctx, actor, true); err != nil {
		return err
	}
	code, name, description = strings.TrimSpace(code), strings.TrimSpace(name), strings.TrimSpace(description)
	if !hgValidText(requestID, 64) || !hgRoleCodePattern.MatchString(code) || len(code) > 32 || !hgValidText(name, 64) || len([]rune(description)) > 255 {
		return ErrHGInvalid
	}
	tx, err := s.repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, SQLQueriesPackage.HGInsertIAMScopeType, code, name, description, true)
	if hgDuplicate(err) {
		return ErrHGConflict
	}
	if err != nil {
		return err
	}
	if err = hgAudit(ctx, tx, actor, "iam.scope_type.create", "scope_type", code, requestID, map[string]any{"status": 0}); err != nil {
		return err
	}
	return tx.Commit()
}

// HGUpdateScopeType 只更新名称及描述；不允许后台改编码、scope_id 规则或激活未接通的类型。
func (s *HGService) HGUpdateScopeType(ctx context.Context, actor, requestID, code, name, description string) error {
	if err := s.hgCanManage(ctx, actor, true); err != nil {
		return err
	}
	name, description = strings.TrimSpace(name), strings.TrimSpace(description)
	if !hgValidText(requestID, 64) || !hgRoleCodePattern.MatchString(code) || len(code) > 32 || !hgValidText(name, 64) || len([]rune(description)) > 255 {
		return ErrHGInvalid
	}
	tx, err := s.repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status int
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMScopeTypeForUpdate, code).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrHGNotFound
	}
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, SQLQueriesPackage.HGUpdateIAMScopeType, name, description, code)
	if err != nil {
		return err
	}
	if err = hgAudit(ctx, tx, actor, "iam.scope_type.update", "scope_type", code, requestID, map[string]any{"name": name}); err != nil {
		return err
	}
	return tx.Commit()
}

// HGDisableScopeType 删除操作采用停用；任何历史绑定均拒绝，保留已接入类型以避免中断已有授权。
func (s *HGService) HGDisableScopeType(ctx context.Context, actor, requestID, code string) error {
	if err := s.hgCanManage(ctx, actor, true); err != nil {
		return err
	}
	if !hgValidText(requestID, 64) || !hgRoleCodePattern.MatchString(code) || len(code) > 32 {
		return ErrHGInvalid
	}
	if code == "global" || code == "shop" || code == "org" {
		return ErrHGForbidden
	}
	tx, err := s.repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status int
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMScopeTypeForUpdate, code).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrHGNotFound
	}
	if err != nil {
		return err
	}
	if status != 1 {
		return ErrHGConflict
	}
	var one int
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMScopeBinding, code).Scan(&one)
	if err == nil {
		return ErrHGConflict
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	res, err := tx.ExecContext(ctx, SQLQueriesPackage.HGDisableIAMScopeType, code)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrHGConflict
	}
	if err = hgAudit(ctx, tx, actor, "iam.scope_type.disable", "scope_type", code, requestID, map[string]any{}); err != nil {
		return err
	}
	return tx.Commit()
}

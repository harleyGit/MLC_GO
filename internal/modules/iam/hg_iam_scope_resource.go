package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	HGPermissionPackage "MLC_GO/internal/pkg/permission"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
)

// HGScopeResource 是作用域候选列表协议；ID 是授权时的 scopeId，Name 仅用于展示。
type HGScopeResource struct {
	ID   string `json:"id"`   // 已登记 ID 或已审核商家 merchant_id；六类新作用域由服务端生成不可变的 32 字符十六进制 ID。
	Name string `json:"name"` // 资源名称，仅作选项标签；空值可由前端展示 ID。
}

// hgRegisteredScopeType 仅包含六类受支持的业务作用域；历史 IAM 占位 ID 仍须经过权威实体及状态复核。
func hgRegisteredScopeType(scopeType string) bool {
	switch scopeType {
	case "creator", "organization", "environment", "crawler_project", "content_channel", "region":
		return true
	}
	return false
}

// hgCanRegisterScopeResource 同时复核管理员与专门即时授权权限；不凭路由 Guard 单独放行。
func (s *HGService) hgCanRegisterScopeResource(ctx context.Context, actor string) error {
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
	return nil
}

// HGCreateScopeResource 登记 IAM 占位资源；不创建业务实体、不证明资源归属，也不使业务接口自动接受新作用域。
// ID 由密码学随机数生成，名称不能代替真实业务实体状态；请求 ID 用于审计追踪。
func (s *HGService) HGCreateScopeResource(ctx context.Context, actor, requestID, scopeType, name string) (HGScopeResource, error) {
	if err := s.hgCanRegisterScopeResource(ctx, actor); err != nil {
		return HGScopeResource{}, err
	}
	name = strings.TrimSpace(name)
	if !hgRegisteredScopeType(scopeType) || !hgValidText(requestID, 64) || !hgValidText(name, 128) {
		return HGScopeResource{}, ErrHGInvalid
	}
	var existing HGScopeResource
	var existingType string
	err := s.repo.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMScopeResourceRequest, actor, requestID).
		Scan(&existing.ID, &existingType, &existing.Name)
	if err == nil {
		if existingType != scopeType || existing.Name != name {
			return HGScopeResource{}, ErrHGConflict
		}
		return existing, nil // 同一次请求重试复用已提交的 ID，不再生成第二条资源。
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return HGScopeResource{}, err
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return HGScopeResource{}, err
	}
	item := HGScopeResource{ID: hex.EncodeToString(random[:]), Name: name}
	tx, err := s.repo.db.BeginTx(ctx, nil)
	if err != nil {
		return HGScopeResource{}, err
	}
	defer tx.Rollback()
	// 行锁保证字典被并发停用时不会创建孤立候选；事务提交后不会自行授予任何人角色。
	var needsID bool
	if err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMActiveScopeType, scopeType).Scan(&needsID); errors.Is(err, sql.ErrNoRows) {
		return HGScopeResource{}, ErrHGInvalid
	}
	if err != nil {
		return HGScopeResource{}, err
	}
	if !needsID {
		return HGScopeResource{}, ErrHGInvalid
	}
	if _, err = tx.ExecContext(ctx, SQLQueriesPackage.HGInsertIAMScopeResource, scopeType, item.ID, name, actor, requestID); hgDuplicate(err) {
		tx.Rollback() // 释放事务后读取并发成功请求的权威结果；重复请求不重复审计。
		err = s.repo.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMScopeResourceRequest, actor, requestID).
			Scan(&existing.ID, &existingType, &existing.Name)
		if err == nil && existingType == scopeType && existing.Name == name {
			return existing, nil
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return HGScopeResource{}, err
		}
		return HGScopeResource{}, ErrHGConflict
	} else if err != nil {
		return HGScopeResource{}, err
	}
	if err = hgAudit(ctx, tx, actor, "iam.scope_resource.create", "scope_resource", item.ID, requestID, map[string]any{"scopeType": scopeType}); err != nil {
		return HGScopeResource{}, err
	}
	if err = tx.Commit(); err != nil {
		return HGScopeResource{}, err
	}
	return item, nil
}

// HGListScopeResources 查询受支持作用域的有界候选页；六类业务候选必须有权威实体且启用，最终授权仍需复核。
// shop 游标先遍历登记组织 (r:ID)，再遍历审核商家 (m:主键)；org 以组织 ID 为准。
func (s *HGService) HGListScopeResources(ctx context.Context, actor, scopeType, cursor string, limit int) ([]HGScopeResource, string, bool, error) {
	if err := s.hgCanRegisterScopeResource(ctx, actor); err != nil {
		return nil, "", false, err
	}
	if scopeType != "shop" && scopeType != "org" && !hgRegisteredScopeType(scopeType) {
		return nil, "", false, ErrHGInvalid
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	if hgRegisteredScopeType(scopeType) {
		var needsID bool
		err := s.repo.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMReadableScopeType, scopeType).Scan(&needsID)
		if errors.Is(err, sql.ErrNoRows) || !needsID && err == nil {
			return nil, "", false, ErrHGInvalid
		}
		if err != nil {
			return nil, "", false, err
		}
		var after uint64
		if cursor != "" {
			var err error
			after, err = strconv.ParseUint(cursor, 10, 64)
			if err != nil || after == 0 {
				return nil, "", false, ErrHGInvalid
			}
		}
		query := SQLQueriesPackage.HGSelectIAMScopeResourcesFirst
		args := []any{scopeType, limit + 1}
		if after != 0 {
			query, args = SQLQueriesPackage.HGSelectIAMScopeResourcesAfter, []any{scopeType, after, limit + 1}
		}
		rows, err := s.repo.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, "", false, err
		}
		defer rows.Close()
		items := make([]HGScopeResource, 0, limit+1)
		var last uint64
		for rows.Next() {
			var item HGScopeResource
			if err = rows.Scan(&last, &item.ID, &item.Name); err != nil {
				return nil, "", false, err
			}
			items = append(items, item)
			if len(items) == limit {
				break
			}
		}
		if err = rows.Err(); err != nil {
			return nil, "", false, err
		}
		more := false
		if len(items) == limit && rows.Next() {
			var probeID uint64
			var probe HGScopeResource
			if err = rows.Scan(&probeID, &probe.ID, &probe.Name); err != nil {
				return nil, "", false, err
			}
			more = true
		}
		if err = rows.Err(); err != nil {
			return nil, "", false, err
		}
		next := ""
		if len(items) > 0 {
			next = strconv.FormatUint(last, 10)
		}
		return items, next, more, nil
	}
	registeredShop := scopeType == "shop" && !strings.HasPrefix(cursor, "m:")
	if scopeType == "org" || registeredShop {
		orgCursor := cursor // org 的原有 ID 游标不加前缀。
		if registeredShop && orgCursor != "" {
			if !strings.HasPrefix(orgCursor, "r:") {
				return nil, "", false, ErrHGInvalid
			}
			orgCursor = strings.TrimPrefix(orgCursor, "r:")
		}
		query := SQLQueriesPackage.HGSelectIAMScopeOrgsFirst
		args := []any{scopeType, limit + 1}
		if orgCursor != "" {
			if !hgValidText(orgCursor, 64) {
				return nil, "", false, ErrHGInvalid
			}
			query, args = SQLQueriesPackage.HGSelectIAMScopeOrgsAfter, []any{scopeType, orgCursor, limit + 1}
		}
		rows, err := s.repo.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, "", false, err
		}
		defer rows.Close()
		items := make([]HGScopeResource, 0, limit+1)
		for rows.Next() {
			var item HGScopeResource
			if err := rows.Scan(&item.ID, &item.Name); err != nil {
				return nil, "", false, err
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
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
		if scopeType == "org" {
			return items, next, more, nil
		}
		if len(items) > 0 {
			if more {
				return items, "r:" + next, true, nil
			}
			return items, "m:0", true, nil // 进入商家候选页；该页可能为空，不误称已列全。
		}
		if err := rows.Close(); err != nil {
			return nil, "", false, err
		}
	}
	// shop 的第二段按索引读取已审核商家；与已登记 shop 重复的 ID 由展示层去重，
	// 不使用反关联过滤造成大量已登记商家时的深度扫描。
	var after uint64
	if strings.HasPrefix(cursor, "m:") && cursor != "m:0" {
		var err error
		after, err = strconv.ParseUint(strings.TrimPrefix(cursor, "m:"), 10, 64)
		if err != nil || after == 0 {
			return nil, "", false, ErrHGInvalid
		}
	}
	query := SQLQueriesPackage.HGSelectIAMScopeShopsFirst
	args := []any{limit + 1}
	if after != 0 {
		query, args = SQLQueriesPackage.HGSelectIAMScopeShopsAfter, []any{after, limit + 1}
	}
	rows, err := s.repo.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", false, err
	}
	defer rows.Close()
	items := make([]HGScopeResource, 0, limit+1)
	var last uint64
	for rows.Next() {
		var item HGScopeResource
		if err := rows.Scan(&last, &item.ID, &item.Name); err != nil {
			return nil, "", false, err
		}
		items = append(items, item)
		if len(items) == limit {
			// 下一页游标必须停在本页末项，而不是额外读取的探测项。
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", false, err
	}
	// 探测续页必须读取第 limit+1 项，否则恰好满页时会误报有更多。
	var probeID uint64
	var probe HGScopeResource
	more := false
	if len(items) == limit && rows.Next() {
		if err := rows.Scan(&probeID, &probe.ID, &probe.Name); err != nil {
			return nil, "", false, err
		}
		more = true
	}
	if err := rows.Err(); err != nil {
		return nil, "", false, err
	}
	next := ""
	if len(items) > 0 {
		next = "m:" + strconv.FormatUint(last, 10)
	}
	return items, next, more, nil
}

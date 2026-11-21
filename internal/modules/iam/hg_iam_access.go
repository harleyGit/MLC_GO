package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	HGPermissionPackage "MLC_GO/internal/pkg/permission"
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// HGAuthorizeProductUpdate 从权威商品表读取所有者和状态；只预判已上架商品的编辑能力。
// 草稿、其他商品操作及最终写事务仍由商品业务服务独立校验。
func (s *HGService) HGAuthorizeProductUpdate(ctx context.Context, userID, productID string) (HGDecision, error) {
	deny := HGDecision{Reason: "没有操作权限"}
	if s == nil || s.repo == nil || !hgValidText(userID, 64) || !hgValidText(productID, 64) || strings.TrimSpace(productID) != productID {
		return deny, ErrHGInvalid
	}
	var owner, merchantID string
	var merchantStatus, productStatus, reviewStatus int8
	err := s.repo.db.QueryRowContext(ctx, SQLQueriesPackage.HGMallProductAuthStateSQL, productID).
		Scan(&owner, &merchantID, &merchantStatus, &productStatus, &reviewStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return deny, nil
	}
	if err != nil {
		return deny, err
	}
	if owner != userID {
		return deny, nil
	}
	if merchantStatus != 1 || productStatus != 1 || reviewStatus != 1 {
		return HGDecision{Reason: "资源状态不允许操作"}, nil
	}
	return s.Authorize(ctx, userID, string(HGPermissionPackage.HGMallProductUpdate),
		HGResource{ScopeType: "shop", ScopeID: merchantID, OwnerID: owner, RequireOwner: true, Status: "published", AllowedStatuses: []string{"published"}})
}

// HGAccessGrant 是 Web 权限快照中的单条有效授权，权限与绑定作用域不可拆开合并。
type HGAccessGrant struct {
	Permission string `json:"permission"` // 已登记的 IAM 权限码。
	ScopeType  string `json:"scope_type"` // global、shop 或 org。
	ScopeID    string `json:"scope_id"`   // global 时为空，其他作用域为业务 ID。
}

// HGAccessSnapshot 是 /api/v1/iam/me/access 的展示协议，不能代替服务端逐请求授权。
type HGAccessSnapshot struct {
	UserID        string          `json:"user_id"`        // 当前 JWT 的业务用户 ID。
	Roles         []string        `json:"roles"`          // 活动绑定的去重角色编码，仅用于展示。
	Grants        []HGAccessGrant `json:"grants"`         // 按绑定逐条保留的有效权限与作用域。
	PolicyVersion uint64          `json:"policy_version"` // 权威 IAM 策略修订号。
	ExpiresAt     time.Time       `json:"expires_at"`     // 快照有效期，不晚于最早的授权失效时间。
}

// AccessSnapshot 读取有界的当前用户有效授权；依赖故障和超出上限时拒绝返回部分结果。
func (s *HGService) AccessSnapshot(ctx context.Context, userID string) (HGAccessSnapshot, error) {
	result := HGAccessSnapshot{UserID: userID, Roles: []string{}, Grants: []HGAccessGrant{}}
	if s == nil || s.repo == nil || !hgValidText(userID, 64) {
		return result, ErrHGInvalid
	}
	if err := s.repo.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMPolicyRevision).Scan(&result.PolicyVersion); err != nil {
		return result, err
	}
	rows, err := s.repo.db.QueryContext(ctx, SQLQueriesPackage.HGSelectIAMAccessGrants, userID)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	seenRoles := make(map[string]bool)
	// 仅在实时通道可用时取消一分钟复核；旧环境保持短期快照的默认拒绝语义。
	result.ExpiresAt = time.Now().UTC().Add(time.Minute)
	if s.realtime != nil {
		result.ExpiresAt = time.Date(2099, time.January, 1, 0, 0, 0, 0, time.UTC)
	}
	for rows.Next() {
		var role string
		var grant HGAccessGrant
		var expires sql.NullTime
		if err := rows.Scan(&role, &grant.Permission, &grant.ScopeType, &grant.ScopeID, &expires); err != nil {
			return HGAccessSnapshot{}, err
		}
		if len(result.Grants) == 1000 {
			return HGAccessSnapshot{}, errors.New("IAM 授权超过快照上限")
		}
		result.Grants = append(result.Grants, grant)
		if !seenRoles[role] {
			result.Roles = append(result.Roles, role)
			seenRoles[role] = true
		}
		if expires.Valid && expires.Time.Before(result.ExpiresAt) {
			result.ExpiresAt = expires.Time.UTC()
		}
	}
	if err := rows.Err(); err != nil {
		return HGAccessSnapshot{}, err
	}
	return result, nil
}

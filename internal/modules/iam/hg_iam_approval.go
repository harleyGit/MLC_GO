package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	HGPermissionPackage "MLC_GO/internal/pkg/permission"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

// HGApproval 是 IAM 双人审批列表协议；主键使用字符串避免 JS 整数精度丢失。
type HGApproval struct {
	ID          string          `json:"id"`          // 审批主键。
	RequestID   string          `json:"requestId"`   // 申请幂等标识。
	ApplicantID string          `json:"applicantId"` // 申请人。
	Action      string          `json:"action"`      // grant 或 role_permissions。
	Payload     json.RawMessage `json:"payload"`     // 待批准的非敏感变更参数。
	ExpiresAt   time.Time       `json:"expiresAt"`   // 最晚审批时间。
	CreatedAt   time.Time       `json:"createdAt"`   // 申请时间。
}

// hgGrantApprovalPayload 是审批快照；批准时仍重新验证账户、角色、作用域和授权范围。
type hgGrantApprovalPayload struct {
	UserID    string     `json:"userId"`    // 目标管理员业务 ID。
	RoleID    uint64     `json:"roleId"`    // 目标角色主键。
	ScopeType string     `json:"scopeType"` // 授权作用域类型。
	ScopeID   string     `json:"scopeId"`   // 授权作用域业务 ID。
	ExpiresAt *time.Time `json:"expiresAt"` // 临时授权过期时间。
}

// hgRolePermissionsApprovalPayload 保存全量权限替换和申请时版本，防止延迟审批覆盖新策略。
type hgRolePermissionsApprovalPayload struct {
	RoleID      uint64   `json:"roleId"`      // 被修改的角色主键。
	Permissions []string `json:"permissions"` // 待替换的完整权限集合。
	Version     uint64   `json:"version"`     // 申请时角色版本。
}

func (s *HGService) hgRequestApproval(ctx context.Context, actor, requestID, code, otp string, payload any) (string, error) {
	if !hgValidText(requestID, 64) {
		return "", ErrHGInvalid
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	tx, err := s.repo.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err = s.hgConsumeMFA(ctx, tx, actor, otp); err != nil {
		return "", err
	}
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.HGInsertIAMApproval, requestID, actor, code, data, time.Now().Add(24*time.Hour))
	if hgDuplicate(err) {
		return "", ErrHGConflict
	}
	if err != nil {
		return "", err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return "", err
	}
	if err = hgAudit(ctx, tx, actor, "iam.approval.request", "approval", strconv.FormatInt(id, 10), requestID, map[string]any{"action": code}); err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return strconv.FormatInt(id, 10), nil
}

// RequestGrant 校验最小授权范围并由申请人 MFA 签名，等待另一名管理员审批。
func (s *HGService) RequestGrant(ctx context.Context, actor, requestID, otp string, p hgGrantApprovalPayload) (string, error) {
	if err := s.hgCanManage(ctx, actor, true); err != nil {
		return "", err
	}
	if !hgValidText(p.UserID, 64) || p.RoleID == 0 {
		return "", ErrHGInvalid
	}
	if p.ExpiresAt != nil && !p.ExpiresAt.After(time.Now()) {
		return "", ErrHGInvalid
	}
	if p.ScopeType == "global" {
		if p.ScopeID != "" {
			return "", ErrHGInvalid
		}
	} else if p.ScopeType == "shop" || p.ScopeType == "org" {
		if !hgValidText(p.ScopeID, 64) {
			return "", ErrHGInvalid
		}
		active, err := s.repo.hgOrganization(ctx, p.ScopeType, p.ScopeID)
		if err != nil {
			return "", err
		}
		if !active {
			return "", ErrHGNotFound
		}
	} else {
		return "", ErrHGInvalid
	}
	active, err := s.repo.hgActiveAdmin(ctx, p.UserID)
	if err != nil {
		return "", err
	}
	if !active {
		return "", ErrHGNotFound
	}
	codes, err := s.repo.hgRolePermissions(ctx, p.RoleID)
	if err != nil {
		return "", err
	}
	if err = s.hgCanDelegate(ctx, actor, codes); err != nil {
		return "", err
	}
	return s.hgRequestApproval(ctx, actor, requestID, "grant", otp, p)
}

// RequestRolePermissions 为完整权限替换创建审批，拒绝内置角色和过期角色版本。
func (s *HGService) RequestRolePermissions(ctx context.Context, actor, requestID, otp string, roleID uint64, codes []string) (string, error) {
	if err := s.hgCanManage(ctx, actor, true); err != nil {
		return "", err
	}
	if roleID == 0 {
		return "", ErrHGInvalid
	}
	if err := s.hgCanDelegate(ctx, actor, codes); err != nil {
		return "", err
	}
	var version uint64
	var builtIn bool
	err := s.repo.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMRoleVersion, roleID).Scan(&version, &builtIn)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrHGNotFound
	}
	if err != nil {
		return "", err
	}
	if builtIn {
		return "", ErrHGForbidden
	}
	return s.hgRequestApproval(ctx, actor, requestID, "role_permissions", otp, hgRolePermissionsApprovalPayload{RoleID: roleID, Permissions: codes, Version: version})
}

func hgApprovalActorTx(ctx context.Context, tx *sql.Tx, userID, permission string) error {
	var one int
	err := tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMManager, userID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrHGForbidden
	}
	if err != nil {
		return err
	}
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMPermission, userID, "global", "", permission).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrHGForbidden
	}
	if err != nil {
		return err
	}
	return nil
}

func hgApprovalCanDelegateTx(ctx context.Context, tx *sql.Tx, userID string, codes []string) error {
	for _, code := range codes {
		if !hgPermissionCodePattern.MatchString(code) {
			return ErrHGInvalid
		}
		var one int
		err := tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMPermission, userID, "global", "", code).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHGForbidden
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// DecideApproval 使用行锁、MFA 和同一 MySQL 事务完成裁决与授权变更。
func (s *HGService) DecideApproval(ctx context.Context, approver, requestID, otp string, id uint64, approve bool) error {
	if id == 0 {
		return ErrHGInvalid
	}
	if err := s.hgCanManage(ctx, approver, true); err != nil {
		return err
	}
	decision, err := s.Authorize(ctx, approver, string(HGPermissionPackage.HGIAMApprovalApprove), HGResource{ScopeType: "global"})
	if err != nil {
		return err
	}
	if !decision.Allowed {
		return ErrHGForbidden
	}
	tx, err := s.repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var applicant, action, status string
	var payload []byte
	var expires time.Time
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMApprovalForUpdate, id).Scan(&applicant, &action, &payload, &status, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrHGNotFound
	}
	if err != nil {
		return err
	}
	if applicant == approver || status != "pending" || !expires.After(time.Now()) {
		return ErrHGConflict
	}
	if err = hgApprovalActorTx(ctx, tx, approver, string(HGPermissionPackage.HGIAMApprovalApprove)); err != nil {
		return err
	}
	if err = s.hgConsumeMFA(ctx, tx, approver, otp); err != nil {
		return err
	}
	if approve {
		if err = hgApprovalActorTx(ctx, tx, applicant, string(HGPermissionPackage.HGIAMRoleGrant)); err != nil {
			return err
		}
		switch action {
		case "grant":
			var p hgGrantApprovalPayload
			if err = json.Unmarshal(payload, &p); err != nil {
				return err
			}
			if p.RoleID == 0 || !hgValidText(p.UserID, 64) || (p.ExpiresAt != nil && !p.ExpiresAt.After(time.Now())) {
				return ErrHGInvalid
			}
			err = s.repo.hgGrantTx(ctx, tx, applicant, approver, requestID, p.UserID, p.RoleID, p.ScopeType, p.ScopeID, p.ExpiresAt)
		case "role_permissions":
			var p hgRolePermissionsApprovalPayload
			if err = json.Unmarshal(payload, &p); err != nil {
				return err
			}
			if p.RoleID == 0 || p.Version == 0 || len(p.Permissions) > 50 {
				return ErrHGInvalid
			}
			if err = hgApprovalCanDelegateTx(ctx, tx, applicant, p.Permissions); err != nil {
				return err
			}
			if err = hgApprovalCanDelegateTx(ctx, tx, approver, p.Permissions); err != nil {
				return err
			}
			err = s.repo.hgSetRolePermissionsTx(ctx, tx, approver, requestID, p.RoleID, p.Permissions, p.Version)
		default:
			return ErrHGInvalid
		}
		if err != nil {
			return err
		}
	}
	state := "rejected"
	if approve {
		state = "approved"
	}
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.HGDecideIAMApproval, state, approver, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrHGConflict
	}
	if err = hgAudit(ctx, tx, approver, "iam.approval."+state, "approval", strconv.FormatUint(id, 10), requestID, map[string]any{"applicant": applicant, "action": action}); err != nil {
		return err
	}
	return tx.Commit()
}

// ListApprovals 返回待裁决申请的有界游标页。
func (s *HGService) ListApprovals(ctx context.Context, actor string, cursor uint64, limit int) ([]HGApproval, string, bool, error) {
	if err := s.hgCanManage(ctx, actor, false); err != nil {
		return nil, "", false, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	query := SQLQueriesPackage.HGSelectIAMApprovalsFirst
	args := []any{limit + 1}
	if cursor > 0 {
		query = SQLQueriesPackage.HGSelectIAMApprovalsAfter
		args = []any{cursor, limit + 1}
	}
	rows, err := s.repo.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", false, err
	}
	defer rows.Close()
	items := make([]HGApproval, 0, limit+1)
	for rows.Next() {
		var id uint64
		var item HGApproval
		if err = rows.Scan(&id, &item.RequestID, &item.ApplicantID, &item.Action, &item.Payload, &item.ExpiresAt, &item.CreatedAt); err != nil {
			return nil, "", false, err
		}
		item.ID = strconv.FormatUint(id, 10)
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

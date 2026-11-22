package iam

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
)

var hgBusinessScopeCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

// HGBusinessScopeCreate 是六类业务实体创建协议；ownerUserId 仅适用 creator，code 仅适用 environment/region，
// organizationId 仅适用 crawler_project/content_channel。所有 ID 和状态由服务端验证并生成。
type HGBusinessScopeCreate struct {
	ScopeType      string `json:"scopeType"`      // 六类已登记的业务作用域类型，不接受客户端自造的类型。
	Name           string `json:"name"`           // 1-128 字符显示名，不作为归属或鉴权依据。
	OwnerUserID    string `json:"ownerUserId"`    // 创作者所有者的已有 users.user_id，其余类型必须为空。
	Code           string `json:"code"`           // 环境/区域唯一的小写编码，其余类型必须为空。
	OrganizationID string `json:"organizationId"` // 项目/频道的可选已启用组织 ID，其余类型必须为空。
}

// HGCreateBusinessScope 在一个 MySQL 事务内建立业务实体及同 ID 的 IAM 资源。
// 历史 IAM 占位资源不会在此处自动转换；同管理员 requestID 重放只复用匹配的业务实体。
func (s *HGService) HGCreateBusinessScope(ctx context.Context, actor, requestID string, req HGBusinessScopeCreate) (HGScopeResource, error) {
	if err := s.hgCanRegisterScopeResource(ctx, actor); err != nil {
		return HGScopeResource{}, err
	}
	req.Name = strings.TrimSpace(req.Name)
	req.OwnerUserID = strings.TrimSpace(req.OwnerUserID)
	req.Code = strings.TrimSpace(req.Code)
	req.OrganizationID = strings.TrimSpace(req.OrganizationID)
	if !hgRegisteredScopeType(req.ScopeType) || !hgValidText(requestID, 64) || !hgValidText(req.Name, 128) ||
		!hgValidBusinessScopeRequest(req) {
		return HGScopeResource{}, ErrHGInvalid
	}
	// 平台 IAM 管理员不自动具备他人的创作者身份；代建空间须另有经所有者确认的业务流程。
	if req.ScopeType == "creator" && req.OwnerUserID != actor {
		return HGScopeResource{}, ErrHGForbidden
	}
	if existing, found, err := s.hgReplayBusinessScope(ctx, actor, requestID, req); err != nil || found {
		return existing, err
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return HGScopeResource{}, err
	}
	item := HGScopeResource{ID: hex.EncodeToString(random[:]), Name: req.Name}
	tx, err := s.repo.db.BeginTx(ctx, nil)
	if err != nil {
		return HGScopeResource{}, err
	}
	defer tx.Rollback()
	var needsID bool
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMActiveScopeType, req.ScopeType).Scan(&needsID)
	if errors.Is(err, sql.ErrNoRows) || err == nil && !needsID {
		return HGScopeResource{}, ErrHGInvalid
	}
	if err != nil {
		return HGScopeResource{}, err
	}
	if req.OwnerUserID != "" {
		var one int
		err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMGrantTargetUser, req.OwnerUserID).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return HGScopeResource{}, ErrHGInvalid
		}
		if err != nil {
			return HGScopeResource{}, err
		}
	}
	if req.OrganizationID != "" {
		var one int
		err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectBusinessOrganization, req.OrganizationID).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return HGScopeResource{}, ErrHGInvalid
		}
		if err != nil {
			return HGScopeResource{}, err
		}
	}
	_, err = tx.ExecContext(ctx, SQLQueriesPackage.HGInsertIAMBusinessScopeResource, req.ScopeType, item.ID, item.Name, actor, requestID)
	if err == nil {
		err = hgInsertBusinessScope(ctx, tx, actor, item.ID, req)
	}
	if err != nil {
		if !hgDuplicate(err) {
			return HGScopeResource{}, err
		}
		_ = tx.Rollback()
		if existing, found, replayErr := s.hgReplayBusinessScope(ctx, actor, requestID, req); replayErr != nil || found {
			return existing, replayErr
		}
		return HGScopeResource{}, ErrHGConflict
	}
	if err = hgAudit(ctx, tx, actor, "iam.business_scope.create", req.ScopeType, item.ID, requestID, map[string]any{"scopeType": req.ScopeType}); err != nil {
		return HGScopeResource{}, err
	}
	if err = tx.Commit(); err != nil {
		return HGScopeResource{}, err
	}
	return item, nil
}

func hgValidBusinessScopeRequest(req HGBusinessScopeCreate) bool {
	switch req.ScopeType {
	case "creator":
		return hgValidText(req.OwnerUserID, 64) && req.Code == "" && req.OrganizationID == ""
	case "organization":
		return req.OwnerUserID == "" && req.Code == "" && req.OrganizationID == ""
	case "environment", "region":
		return req.OwnerUserID == "" && hgBusinessScopeCodePattern.MatchString(req.Code) && req.OrganizationID == ""
	case "crawler_project", "content_channel":
		return req.OwnerUserID == "" && req.Code == "" && (req.OrganizationID == "" || hgValidText(req.OrganizationID, 64))
	}
	return false
}

func hgInsertBusinessScope(ctx context.Context, tx *sql.Tx, actor, id string, req HGBusinessScopeCreate) error {
	var query string
	var args []any
	switch req.ScopeType {
	case "creator":
		query, args = SQLQueriesPackage.HGInsertCreatorSpace, []any{id, req.OwnerUserID, req.Name, actor}
	case "organization":
		query, args = SQLQueriesPackage.HGInsertBusinessOrganization, []any{id, req.Name, actor}
	case "environment":
		query, args = SQLQueriesPackage.HGInsertOperationEnvironment, []any{id, req.Code, req.Name, actor}
	case "crawler_project":
		var organizationID any
		if req.OrganizationID != "" {
			organizationID = req.OrganizationID
		}
		query, args = SQLQueriesPackage.HGInsertCrawlerProject, []any{id, organizationID, req.Name, actor}
	case "content_channel":
		var organizationID any
		if req.OrganizationID != "" {
			organizationID = req.OrganizationID
		}
		query, args = SQLQueriesPackage.HGInsertContentChannel, []any{id, organizationID, req.Name, actor}
	case "region":
		query, args = SQLQueriesPackage.HGInsertBusinessRegion, []any{id, req.Code, req.Name, actor}
	default:
		return ErrHGInvalid
	}
	_, err := tx.ExecContext(ctx, query, args...)
	return err
}

func hgBusinessScopeMetaQuery(scopeType string) string {
	switch scopeType {
	case "creator":
		return SQLQueriesPackage.HGSelectBusinessCreator
	case "organization":
		return SQLQueriesPackage.HGSelectBusinessOrganizationMeta
	case "environment":
		return SQLQueriesPackage.HGSelectBusinessEnvironment
	case "crawler_project":
		return SQLQueriesPackage.HGSelectBusinessProject
	case "content_channel":
		return SQLQueriesPackage.HGSelectBusinessChannel
	case "region":
		return SQLQueriesPackage.HGSelectBusinessRegion
	}
	return ""
}

func (s *HGService) hgReplayBusinessScope(ctx context.Context, actor, requestID string, req HGBusinessScopeCreate) (HGScopeResource, bool, error) {
	var existing HGScopeResource
	var scopeType string
	err := s.repo.db.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMScopeResourceRequest, actor, requestID).
		Scan(&existing.ID, &scopeType, &existing.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return HGScopeResource{}, false, nil
	}
	if err != nil {
		return HGScopeResource{}, false, err
	}
	if scopeType != req.ScopeType || existing.Name != req.Name {
		return HGScopeResource{}, false, ErrHGConflict
	}
	var owner, code, organizationID string
	err = s.repo.db.QueryRowContext(ctx, hgBusinessScopeMetaQuery(scopeType), existing.ID).Scan(&owner, &code, &organizationID)
	if errors.Is(err, sql.ErrNoRows) || err == nil && (owner != req.OwnerUserID || code != req.Code || organizationID != req.OrganizationID) {
		return HGScopeResource{}, false, ErrHGConflict
	}
	if err != nil {
		return HGScopeResource{}, false, err
	}
	return existing, true, nil
}

// HGDisableBusinessScope 停用一项权威实体及同 ID 的 IAM 候选，拒绝占位记录及尚有活动子资源的组织。
// 既有绑定不会物理删除，Authorize 在后续请求中点查业务状态失败关闭；变更版本使缓存授权失效。
func (s *HGService) HGDisableBusinessScope(ctx context.Context, actor, requestID, scopeType, scopeID string) error {
	if err := s.hgCanRegisterScopeResource(ctx, actor); err != nil {
		return err
	}
	if !hgRegisteredScopeType(scopeType) || !hgValidText(requestID, 64) || !hgValidText(scopeID, 64) {
		return ErrHGInvalid
	}
	tx, err := s.repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var typeStatus int
	if err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMScopeTypeForUpdate, scopeType).Scan(&typeStatus); errors.Is(err, sql.ErrNoRows) {
		return ErrHGNotFound
	}
	if err != nil {
		return err
	}
	var status, verified int
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGSelectIAMBusinessScopeForUpdate, scopeType, scopeID).Scan(&status, &verified)
	if errors.Is(err, sql.ErrNoRows) || err == nil && verified != 1 {
		return ErrHGNotFound
	}
	if err != nil {
		return err
	}
	if status == 0 {
		return nil
	}
	if scopeType == "organization" {
		for _, query := range []string{SQLQueriesPackage.HGSelectActiveProjectByOrganization, SQLQueriesPackage.HGSelectActiveChannelByOrganization} {
			var one int
			err = tx.QueryRowContext(ctx, query, scopeID).Scan(&one)
			if err == nil {
				return ErrHGConflict
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
	}
	query := hgBusinessScopeDisableQuery(scopeType)
	result, err := tx.ExecContext(ctx, query, scopeID)
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
	result, err = tx.ExecContext(ctx, SQLQueriesPackage.HGDisableIAMBusinessScope, scopeType, scopeID)
	if err != nil {
		return err
	}
	n, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrHGConflict
	}
	if err = hgBumpRevision(ctx, tx); err != nil {
		return err
	}
	if err = hgAudit(ctx, tx, actor, "iam.business_scope.disable", scopeType, scopeID, requestID, map[string]any{"scopeType": scopeType}); err != nil {
		return err
	}
	return tx.Commit()
}

func hgBusinessScopeDisableQuery(scopeType string) string {
	switch scopeType {
	case "creator":
		return SQLQueriesPackage.HGDisableCreatorSpace
	case "organization":
		return SQLQueriesPackage.HGDisableBusinessOrganization
	case "environment":
		return SQLQueriesPackage.HGDisableOperationEnvironment
	case "crawler_project":
		return SQLQueriesPackage.HGDisableCrawlerProject
	case "content_channel":
		return SQLQueriesPackage.HGDisableContentChannel
	case "region":
		return SQLQueriesPackage.HGDisableBusinessRegion
	}
	return ""
}

package OpsServicePackage

import (
	CoinModelPackage "MLC_GO/internal/modules/coin/model"
	CoinServicePackage "MLC_GO/internal/modules/coin/service"
	CoinTaskPackage "MLC_GO/internal/modules/coin/task"
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	VideoInteractionTaskPackage "MLC_GO/internal/modules/video_interaction/task"
	HGKafkaPackage "MLC_GO/internal/pkg/kafka"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrHGOperationsForbidden means the JWT operator lacks the required database-backed permission.
	ErrHGOperationsForbidden            = errors.New("operations permission denied")
	ErrHGOperationsInvalid              = errors.New("invalid operations request")
	ErrHGOperationsRateLimited          = errors.New("operations rate limited")
	ErrHGOperationsRateLimitUnavailable = errors.New("operations rate limiter unavailable")
	ErrHGOperationsInvalidApprover      = OpsDtoPackage.ErrHGAssetCorrectionInvalidApprover
)

type hgOpsAuthorizer interface {
	HasAssetPermission(context.Context, string, string) (bool, error)
	ListAssetPermissions(context.Context, string) ([]string, error)
}

const (
	HGAssetPermissionBalanceRead       = "asset.coin.balance.read"
	HGAssetPermissionTransactionRead   = "asset.coin.transaction.read"
	HGAssetPermissionGrant             = "asset.coin.grant"
	HGAssetPermissionRefund            = "asset.coin.refund"
	HGAssetPermissionCorrectionRequest = "asset.coin.correction.request"
	HGAssetPermissionCorrectionApprove = "asset.coin.correction.approve"
	HGAssetPermissionCorrectionApply   = "asset.coin.correction.apply"
	HGAssetPermissionPipelineRead      = "asset.pipeline.read"
)

type HGAssetOperator = OpsDtoPackage.HGAssetOperator
type HGAssetAuditRecord = OpsDtoPackage.HGAssetAuditRecord

type hgOpsAssetAudit interface {
	AppendAssetAudit(context.Context, HGAssetAuditRecord) error
}
type hgOpsAssetRateLimiter interface {
	Allow(context.Context, string, string) (bool, error)
}
type hgOpsCorrections interface {
	CreateCoinCorrection(context.Context, OpsDtoPackage.HGAssetOperator, OpsDtoPackage.HGCoinCorrectionRequest, int64) (OpsDtoPackage.HGCoinCorrectionResponse, error)
	GetCoinCorrectionForApproval(context.Context, string, string) (OpsDtoPackage.HGCoinCorrectionResponse, error)
	CompleteCoinCorrection(context.Context, string, string, CoinModelPackage.HGMutationResult, string) error
	ListCoinCorrections(context.Context, uint64, int) ([]OpsDtoPackage.HGCoinCorrectionResponse, uint64, bool, error)
}

type hgOpsCoinAssets interface {
	Balance(context.Context, string) (uint64, error)
	Grant(context.Context, CoinServicePackage.HGCreditCommand) (CoinModelPackage.HGMutationResult, error)
	Refund(context.Context, CoinServicePackage.HGRefundCommand) (CoinModelPackage.HGMutationResult, error)
	Correct(context.Context, CoinServicePackage.HGCorrectionCommand) (CoinModelPackage.HGMutationResult, error)
}

type hgOpsCoinQueries interface {
	ListTransactions(context.Context, string, CoinModelPackage.HGTransactionCursor, int) ([]CoinModelPackage.HGTransaction, CoinModelPackage.HGTransactionCursor, bool, error)
	LoadInitializerCheckpoint(context.Context) (uint64, error)
}

// hgOpsCoinUserLookup 只暴露资产目标所需的精确身份点查，避免 service 依赖完整用户仓储能力。
type hgOpsCoinUserLookup interface {
	FindCoinUserByExactIdentity(context.Context, string, string) (*OpsDtoPackage.HGCoinUserSearchItem, error)
}

type hgOpsProjectionCheckpoints interface {
	LoadCheckpoint(context.Context, string) (string, error)
}

// HGOperationalDeps 使用小接口注入资产、状态与授权依赖，便于测试且不让 handler 直接访问基础设施。
type HGOperationalDeps struct {
	Authorizer            hgOpsAuthorizer
	CoinAssets            hgOpsCoinAssets
	CoinQueries           hgOpsCoinQueries
	ProjectionCheckpoints hgOpsProjectionCheckpoints
	Audit                 hgOpsAssetAudit
	RateLimiter           hgOpsAssetRateLimiter
	Corrections           hgOpsCorrections
	UserLookup            hgOpsCoinUserLookup
}

// SearchCoinUser 按操作人明确选择的唯一身份字段精确查找资产目标，不执行模糊搜索。
func (s *HGOperationalService) SearchCoinUser(ctx context.Context, operatorID, field, keyword string) (*OpsDtoPackage.HGCoinUserSearchResponse, error) {
	if err := s.hgAuthorizeCoinUserSearch(ctx, operatorID); err != nil {
		return nil, err
	}
	field, keyword = strings.TrimSpace(field), strings.TrimSpace(keyword)
	maxRunes := 0
	switch field {
	case "userId", "email":
		maxRunes = 255
	case "phone":
		maxRunes = 32
	default:
		return nil, ErrHGOperationsInvalid
	}
	if !hgOpsValidText(keyword, maxRunes) || s.deps.UserLookup == nil {
		return nil, ErrHGOperationsInvalid
	}
	user, err := s.deps.UserLookup.FindCoinUserByExactIdentity(ctx, field, keyword)
	if err != nil {
		return nil, fmt.Errorf("find coin target user: %w", err)
	}
	return &OpsDtoPackage.HGCoinUserSearchResponse{User: user}, nil
}

func (s *HGOperationalService) hgAuthorizeCoinUserSearch(ctx context.Context, operatorID string) error {
	if s == nil || s.deps.Authorizer == nil || strings.TrimSpace(operatorID) == "" {
		return ErrHGOperationsForbidden
	}
	permissions, err := s.deps.Authorizer.ListAssetPermissions(ctx, operatorID)
	if err != nil {
		return fmt.Errorf("authorize coin target search: %w", err)
	}
	// 目标选择同时服务余额、流水和写操作；一次读取固定低基数权限集合，避免逐权限重复查询 RBAC 表。
	allowed := map[string]struct{}{
		HGAssetPermissionBalanceRead: {}, HGAssetPermissionTransactionRead: {}, HGAssetPermissionGrant: {},
		HGAssetPermissionRefund: {}, HGAssetPermissionCorrectionRequest: {},
	}
	for _, permission := range permissions {
		if _, ok := allowed[permission]; ok {
			return nil
		}
	}
	return ErrHGOperationsForbidden
}

// HGOperationalService 编排受信运维资产操作和低成本链路状态读取。
type HGOperationalService struct{ deps HGOperationalDeps }

// NewHGOperationalService 创建运维功能服务；未注入的状态依赖会返回空快照，不影响其他组件展示。
func NewHGOperationalService(deps HGOperationalDeps) *HGOperationalService {
	return &HGOperationalService{deps: deps}
}

func (s *HGOperationalService) hgAuthorize(ctx context.Context, operatorID, permission string) error {
	if s == nil || s.deps.Authorizer == nil || strings.TrimSpace(operatorID) == "" {
		return ErrHGOperationsForbidden
	}
	allowed, err := s.deps.Authorizer.HasAssetPermission(ctx, operatorID, permission)
	if err != nil {
		return fmt.Errorf("authorize operations user: %w", err)
	}
	if !allowed {
		return ErrHGOperationsForbidden
	}
	return nil
}

// GetCoinAccount 返回 MySQL 权威余额，并在查询前执行管理员数据库身份二次校验。
func (s *HGOperationalService) GetCoinAccount(ctx context.Context, operatorID, userID string) (*OpsDtoPackage.HGCoinAccountResponse, error) {
	if err := s.hgAuthorize(ctx, operatorID, HGAssetPermissionBalanceRead); err != nil {
		return nil, err
	}
	userID = strings.TrimSpace(userID)
	if !hgOpsValidText(userID, 255) || s.deps.CoinAssets == nil {
		return nil, ErrHGOperationsInvalid
	}
	balance, err := s.deps.CoinAssets.Balance(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get authoritative coin balance: %w", err)
	}
	return &OpsDtoPackage.HGCoinAccountResponse{UserID: userID, Balance: strconv.FormatUint(balance, 10), Authority: "mysql"}, nil
}

// GetCoinTransactions 按复合 keyset 游标读取最多 100 条流水。
func (s *HGOperationalService) GetCoinTransactions(ctx context.Context, operatorID, userID, encodedCursor string, pageSize int) (*OpsDtoPackage.HGCoinTransactionListResponse, error) {
	if err := s.hgAuthorize(ctx, operatorID, HGAssetPermissionTransactionRead); err != nil {
		return nil, err
	}
	userID = strings.TrimSpace(userID)
	if !hgOpsValidText(userID, 255) || s.deps.CoinQueries == nil {
		return nil, ErrHGOperationsInvalid
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	cursor, err := hgDecodeCoinCursor(encodedCursor)
	if err != nil {
		return nil, ErrHGOperationsInvalid
	}
	transactions, next, hasMore, err := s.deps.CoinQueries.ListTransactions(ctx, userID, cursor, pageSize)
	if err != nil {
		return nil, fmt.Errorf("list authoritative coin transactions: %w", err)
	}
	items := make([]OpsDtoPackage.HGCoinTransactionItem, 0, len(transactions))
	for _, transaction := range transactions {
		reference := ""
		if transaction.ReferenceTransactionID > 0 {
			reference = strconv.FormatUint(transaction.ReferenceTransactionID, 10)
		}
		items = append(items, OpsDtoPackage.HGCoinTransactionItem{TransactionID: strconv.FormatUint(transaction.ID, 10), RequestID: transaction.RequestID, Operation: string(transaction.Operation), Amount: strconv.FormatUint(transaction.Amount, 10), SignedDelta: strconv.FormatInt(transaction.SignedDelta, 10), BalanceAfter: strconv.FormatUint(transaction.BalanceAfter, 10), Reason: transaction.Reason, BusinessType: transaction.BusinessType, BusinessKey: transaction.BusinessKey, ReferenceTransactionID: reference, CreatedAt: transaction.CreatedAt.UTC().Format(time.RFC3339Nano)})
	}
	nextCursor := ""
	if len(items) > 0 {
		nextCursor, err = hgEncodeCoinCursor(next)
		if err != nil {
			return nil, fmt.Errorf("encode coin transaction cursor: %w", err)
		}
	}
	return &OpsDtoPackage.HGCoinTransactionListResponse{UserID: userID, List: items, NextCursor: nextCursor, HasMore: hasMore}, nil
}

// GrantCoin 只开放赠币语义，支付充值仍必须由已验签支付 Adapter 完成。
func (s *HGOperationalService) GrantCoin(ctx context.Context, operator HGAssetOperator, req OpsDtoPackage.HGCoinGrantRequest) (*OpsDtoPackage.HGCoinMutationResponse, error) {
	if err := s.hgAuthorize(ctx, operator.ID, HGAssetPermissionGrant); err != nil {
		return nil, err
	}
	if err := s.hgLimitWrite(ctx, operator); err != nil {
		return nil, err
	}
	amount, err := hgParsePositiveCoinAmount(req.Amount)
	reason, reasonErr := hgOpsAuditReason(req.Reason, operator.ID)
	if err != nil || reasonErr != nil || !hgOpsValidText(req.UserID, 255) || !hgOpsValidText(req.RequestID, 128) || !hgOpsValidText(req.BusinessKey, 255) || s.deps.CoinAssets == nil || s.deps.Audit == nil {
		return nil, ErrHGOperationsInvalid
	}
	oldBalance, err := s.deps.CoinAssets.Balance(ctx, strings.TrimSpace(req.UserID))
	if err != nil {
		return nil, err
	}
	result, err := s.deps.CoinAssets.Grant(ctx, CoinServicePackage.HGCreditCommand{UserID: strings.TrimSpace(req.UserID), RequestID: strings.TrimSpace(req.RequestID), Amount: amount, Reason: reason, BusinessType: "ops_grant", BusinessKey: strings.TrimSpace(req.BusinessKey), ExpiresAt: req.ExpiresAt})
	if err != nil {
		_ = s.hgAudit(ctx, operator, "coin.grant", req.UserID, req.RequestID, oldBalance, oldBalance, "", "", "failed", err)
		return nil, err
	}
	if result.Committed {
		oldBalance = result.BalanceAfter - amount
	} else {
		oldBalance = result.BalanceAfter
	}
	if err := s.hgAudit(ctx, operator, "coin.grant", req.UserID, req.RequestID, oldBalance, result.BalanceAfter, "", "", "succeeded", nil); err != nil {
		return nil, err
	}
	return hgCoinMutationResponse(result), nil
}

// RefundCoin 退款必须引用原 debit transaction，累计上限由权威资产事务校验。
func (s *HGOperationalService) RefundCoin(ctx context.Context, operator HGAssetOperator, req OpsDtoPackage.HGCoinRefundRequest) (*OpsDtoPackage.HGCoinMutationResponse, error) {
	if err := s.hgAuthorize(ctx, operator.ID, HGAssetPermissionRefund); err != nil {
		return nil, err
	}
	if err := s.hgLimitWrite(ctx, operator); err != nil {
		return nil, err
	}
	amount, amountErr := hgParsePositiveCoinAmount(req.Amount)
	reference, referenceErr := strconv.ParseUint(strings.TrimSpace(req.ReferenceTransactionID), 10, 64)
	reason, reasonErr := hgOpsAuditReason(req.Reason, operator.ID)
	if amountErr != nil || referenceErr != nil || reasonErr != nil || reference == 0 || !hgOpsValidText(req.UserID, 255) || !hgOpsValidText(req.RequestID, 128) || s.deps.CoinAssets == nil || s.deps.Audit == nil {
		return nil, ErrHGOperationsInvalid
	}
	oldBalance, err := s.deps.CoinAssets.Balance(ctx, strings.TrimSpace(req.UserID))
	if err != nil {
		return nil, err
	}
	result, err := s.deps.CoinAssets.Refund(ctx, CoinServicePackage.HGRefundCommand{UserID: strings.TrimSpace(req.UserID), RequestID: strings.TrimSpace(req.RequestID), Amount: amount, Reason: reason, ReferenceTransactionID: reference})
	if err != nil {
		_ = s.hgAudit(ctx, operator, "coin.refund", req.UserID, req.RequestID, oldBalance, oldBalance, "", "", "failed", err)
		return nil, err
	}
	if result.Committed {
		oldBalance = result.BalanceAfter - amount
	} else {
		oldBalance = result.BalanceAfter
	}
	if err := s.hgAudit(ctx, operator, "coin.refund", req.UserID, req.RequestID, oldBalance, result.BalanceAfter, "", "", "succeeded", nil); err != nil {
		return nil, err
	}
	return hgCoinMutationResponse(result), nil
}

// CorrectCoin creates a pending correction request; it never mutates the balance.
func (s *HGOperationalService) CorrectCoin(ctx context.Context, operator HGAssetOperator, req OpsDtoPackage.HGCoinCorrectionRequest) (*OpsDtoPackage.HGCoinCorrectionResponse, error) {
	if err := s.hgAuthorize(ctx, operator.ID, HGAssetPermissionCorrectionRequest); err != nil {
		return nil, err
	}
	if err := s.hgLimitWrite(ctx, operator); err != nil {
		return nil, err
	}
	delta, err := strconv.ParseInt(strings.TrimSpace(req.Delta), 10, 64)
	hasTicket := hgOpsValidText(req.TicketID, 128)
	hasWorkOrder := hgOpsValidText(req.WorkOrderID, 128)
	if err != nil || delta == 0 || delta > int64(CoinServicePackage.HGMaxMutationAmount) || delta < -int64(CoinServicePackage.HGMaxMutationAmount) || !hgOpsValidText(req.UserID, 255) || !hgOpsValidText(req.RequestID, 128) || (!hasTicket && !hasWorkOrder) || !hgOpsValidText(req.Reason, 255) || s.deps.Corrections == nil || s.deps.Audit == nil {
		return nil, ErrHGOperationsInvalid
	}
	created, err := s.deps.Corrections.CreateCoinCorrection(ctx, operator, req, delta)
	if err != nil {
		return nil, err
	}
	if err := s.hgAudit(ctx, operator, "coin.correction.request", req.UserID, req.RequestID, 0, 0, operator.ID, "", "pending", nil); err != nil {
		return nil, err
	}
	return &created, nil
}

// ApproveCoinCorrection requires a distinct approver and applies the correction after durable approval claim.
func (s *HGOperationalService) ApproveCoinCorrection(ctx context.Context, operator HGAssetOperator, correctionID string) (*OpsDtoPackage.HGCoinCorrectionResponse, error) {
	if err := s.hgAuthorize(ctx, operator.ID, HGAssetPermissionCorrectionApprove); err != nil {
		return nil, err
	}
	if err := s.hgAuthorize(ctx, operator.ID, HGAssetPermissionCorrectionApply); err != nil {
		return nil, err
	}
	if err := s.hgLimitWrite(ctx, operator); err != nil {
		return nil, err
	}
	if !hgOpsValidText(correctionID, 64) || s.deps.Corrections == nil || s.deps.CoinAssets == nil || s.deps.Audit == nil {
		return nil, ErrHGOperationsInvalid
	}
	pending, err := s.deps.Corrections.GetCoinCorrectionForApproval(ctx, strings.TrimSpace(correctionID), operator.ID)
	if errors.Is(err, ErrHGOperationsInvalidApprover) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	if pending.ApplicantID == operator.ID {
		return nil, ErrHGOperationsInvalidApprover
	}
	oldBalance, err := s.deps.CoinAssets.Balance(ctx, pending.UserID)
	if err != nil {
		return nil, err
	}
	delta, err := strconv.ParseInt(pending.Delta, 10, 64)
	if err != nil {
		return nil, ErrHGOperationsInvalid
	}
	reason, err := hgOpsAuditReason(pending.Reason, operator.ID)
	if err != nil {
		return nil, err
	}
	result, err := s.deps.CoinAssets.Correct(ctx, CoinServicePackage.HGCorrectionCommand{UserID: pending.UserID, RequestID: pending.RequestID, Delta: delta, Reason: reason})
	if err != nil {
		// Do not finalize transient infrastructure failures. The durable approving claim is the recovery worker's retry source.
		_ = s.hgAudit(ctx, operator, "coin.correction.apply", pending.UserID, pending.RequestID, oldBalance, oldBalance, pending.ApplicantID, operator.ID, "failed", err)
		return nil, err
	}
	if result.Committed {
		if delta > 0 {
			oldBalance = result.BalanceAfter - uint64(delta)
		} else {
			oldBalance = result.BalanceAfter + uint64(-delta)
		}
	} else {
		oldBalance = result.BalanceAfter
	}
	if err := s.deps.Corrections.CompleteCoinCorrection(ctx, pending.CorrectionID, operator.ID, result, ""); err != nil {
		return nil, err
	}
	if err := s.hgAudit(ctx, operator, "coin.correction.apply", pending.UserID, pending.RequestID, oldBalance, result.BalanceAfter, pending.ApplicantID, operator.ID, "succeeded", nil); err != nil {
		return nil, err
	}
	pending.ApproverID, pending.Status, pending.TransactionID, pending.BalanceAfter = operator.ID, "applied", strconv.FormatUint(result.TransactionID, 10), strconv.FormatUint(result.BalanceAfter, 10)
	return &pending, nil
}

// ReplayApprovingCoinCorrection resumes one durably claimed correction with its persisted approver and original request ID.
// Transient errors intentionally leave the row in approving so the bounded timeout worker can retry it again.
func (s *HGOperationalService) ReplayApprovingCoinCorrection(ctx context.Context, pending OpsDtoPackage.HGCoinCorrectionResponse) error {
	if s == nil || s.deps.Corrections == nil || s.deps.CoinAssets == nil || s.deps.Audit == nil || pending.Status != "approving" || !hgOpsValidText(pending.CorrectionID, 64) || !hgOpsValidText(pending.UserID, 255) || !hgOpsValidText(pending.RequestID, 128) || !hgOpsValidText(pending.ApproverID, 255) || pending.ApplicantID == pending.ApproverID {
		return ErrHGOperationsInvalid
	}
	oldBalance, err := s.deps.CoinAssets.Balance(ctx, pending.UserID)
	if err != nil {
		return fmt.Errorf("read correction balance for retry: %w", err)
	}
	delta, err := strconv.ParseInt(strings.TrimSpace(pending.Delta), 10, 64)
	if err != nil || delta == 0 {
		return ErrHGOperationsInvalid
	}
	reason, err := hgOpsAuditReason(pending.Reason, pending.ApproverID)
	if err != nil {
		return err
	}
	result, err := s.deps.CoinAssets.Correct(ctx, CoinServicePackage.HGCorrectionCommand{UserID: pending.UserID, RequestID: pending.RequestID, Delta: delta, Reason: reason})
	if err != nil {
		return fmt.Errorf("replay approving coin correction: %w", err)
	}
	if result.Committed {
		if delta > 0 {
			oldBalance = result.BalanceAfter - uint64(delta)
		} else {
			oldBalance = result.BalanceAfter + uint64(-delta)
		}
	} else {
		oldBalance = result.BalanceAfter
	}
	operator := HGAssetOperator{ID: pending.ApproverID, SourceIP: "system:correction-recovery", TID: "correction-recovery"}
	// Audit before finalizing the control-plane row. If audit persistence fails, approving remains retryable and the coin command replays idempotently.
	if err := s.hgAudit(ctx, operator, "coin.correction.apply", pending.UserID, pending.RequestID, oldBalance, result.BalanceAfter, pending.ApplicantID, pending.ApproverID, "succeeded", nil); err != nil {
		return err
	}
	return s.deps.Corrections.CompleteCoinCorrection(ctx, pending.CorrectionID, pending.ApproverID, result, "")
}

func (s *HGOperationalService) ListCoinCorrections(ctx context.Context, operatorID, cursorValue string, pageSize int) (*OpsDtoPackage.HGCoinCorrectionListResponse, error) {
	if err := s.hgAuthorize(ctx, operatorID, HGAssetPermissionCorrectionRequest); err != nil {
		return nil, err
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	cursor, err := strconv.ParseUint(strings.TrimSpace(cursorValue), 10, 64)
	if strings.TrimSpace(cursorValue) == "" {
		cursor, err = 0, nil
	}
	if err != nil || s.deps.Corrections == nil {
		return nil, ErrHGOperationsInvalid
	}
	items, next, more, err := s.deps.Corrections.ListCoinCorrections(ctx, cursor, pageSize)
	if err != nil {
		return nil, err
	}
	nextValue := ""
	if more {
		nextValue = strconv.FormatUint(next, 10)
	}
	return &OpsDtoPackage.HGCoinCorrectionListResponse{List: items, NextCursor: nextValue, HasMore: more}, nil
}

func (s *HGOperationalService) GetCurrentAssetPermissions(ctx context.Context, operatorID string) (*OpsDtoPackage.HGAssetPermissionsResponse, error) {
	if s == nil || s.deps.Authorizer == nil || strings.TrimSpace(operatorID) == "" {
		return nil, ErrHGOperationsForbidden
	}
	permissions, err := s.deps.Authorizer.ListAssetPermissions(ctx, operatorID)
	if err != nil {
		return nil, fmt.Errorf("list asset permissions: %w", err)
	}
	return &OpsDtoPackage.HGAssetPermissionsResponse{Permissions: permissions}, nil
}

// GetAssetPipelineStatus 聚合原子指标和固定 checkpoint，不执行大表扫描或 Kafka Admin 请求。
func (s *HGOperationalService) GetAssetPipelineStatus(ctx context.Context, operatorID string) (*OpsDtoPackage.HGAssetPipelineStatusResponse, error) {
	if err := s.hgAuthorize(ctx, operatorID, HGAssetPermissionPipelineRead); err != nil {
		return nil, err
	}
	response := &OpsDtoPackage.HGAssetPipelineStatusResponse{ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), CoinReconciliationDrifts: strconv.FormatUint(CoinTaskPackage.HGReconciliationMetricsSnapshot(), 10), InteractionStreams: []OpsDtoPackage.HGInteractionStreamStatus{}, Kafka: OpsDtoPackage.HGKafkaStatus{Measurement: "observed_application_processing_lag", AssignedPartitions: strconv.FormatInt(HGKafkaPackage.HGAssignedPartitionsSnapshot(), 10), Items: []OpsDtoPackage.HGKafkaLagItem{}}}
	if s.deps.CoinQueries != nil {
		checkpoint, err := s.deps.CoinQueries.LoadInitializerCheckpoint(ctx)
		if err != nil {
			return nil, fmt.Errorf("load coin initializer checkpoint: %w", err)
		}
		response.CoinInitializerCursor = strconv.FormatUint(checkpoint, 10)
	}
	metrics := VideoInteractionTaskPackage.HGProjectionMetricsSnapshot()
	for _, metric := range metrics {
		checkpoint := ""
		if s.deps.ProjectionCheckpoints != nil {
			value, err := s.deps.ProjectionCheckpoints.LoadCheckpoint(ctx, metric.Stream)
			if err != nil {
				return nil, fmt.Errorf("load %s projection checkpoint: %w", metric.Stream, err)
			}
			checkpoint = value
		}
		response.InteractionStreams = append(response.InteractionStreams, OpsDtoPackage.HGInteractionStreamStatus{Stream: metric.Stream, Checkpoint: checkpoint, Runs: strconv.FormatUint(metric.Runs, 10), Rows: strconv.FormatUint(metric.Rows, 10), Failures: strconv.FormatUint(metric.Failures, 10), LeaseSkips: strconv.FormatUint(metric.LeaseSkips, 10), DurationNanos: strconv.FormatUint(metric.DurationNanos, 10)})
	}
	for _, item := range HGKafkaPackage.HGConsumerLagSnapshots() {
		response.Kafka.Items = append(response.Kafka.Items, OpsDtoPackage.HGKafkaLagItem{Group: item.Group, Topic: item.Topic, LagRecords: strconv.FormatInt(item.LagRecords, 10)})
	}
	return response, nil
}

func (s *HGOperationalService) hgLimitWrite(ctx context.Context, operator HGAssetOperator) error {
	if s.deps.RateLimiter == nil || strings.TrimSpace(operator.ID) == "" || strings.TrimSpace(operator.SourceIP) == "" {
		return ErrHGOperationsRateLimitUnavailable
	}
	allowed, err := s.deps.RateLimiter.Allow(ctx, operator.ID, operator.SourceIP)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrHGOperationsRateLimitUnavailable, err)
	}
	if !allowed {
		return ErrHGOperationsRateLimited
	}
	return nil
}

func (s *HGOperationalService) hgAudit(ctx context.Context, operator HGAssetOperator, action, target, requestID string, oldBalance, newBalance uint64, applicant, approver, outcome string, actionErr error) error {
	action = strings.TrimSpace(action)
	requestID = strings.TrimSpace(requestID)
	outcome = strings.TrimSpace(outcome)
	message := ""
	if actionErr != nil {
		message = actionErr.Error()
		if len(message) > 500 {
			message = message[:500]
		}
	}
	eventKey := fmt.Sprintf("v1|%s|%s|%s", action, outcome, requestID)
	if err := s.deps.Audit.AppendAssetAudit(ctx, HGAssetAuditRecord{EventKey: eventKey, OperatorID: operator.ID, Action: action, TargetUserID: strings.TrimSpace(target), SourceIP: operator.SourceIP, RequestID: requestID, TID: operator.TID, OldBalance: oldBalance, NewBalance: newBalance, ApplicantID: applicant, ApproverID: approver, Outcome: outcome, ErrorMessage: message}); err != nil {
		return fmt.Errorf("append immutable ops asset audit after action boundary: %w", err)
	}
	return nil
}

func hgParsePositiveCoinAmount(value string) (uint64, error) {
	amount, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil || amount == 0 || amount > CoinServicePackage.HGMaxMutationAmount {
		return 0, ErrHGOperationsInvalid
	}
	return amount, nil
}

func hgOpsAuditReason(reason, operatorID string) (string, error) {
	reason = strings.TrimSpace(reason)
	operatorID = strings.TrimSpace(operatorID)
	if reason == "" || operatorID == "" {
		return "", ErrHGOperationsInvalid
	}
	result := fmt.Sprintf("%s [operator=%s]", reason, operatorID)
	if len([]rune(result)) > 255 {
		return "", ErrHGOperationsInvalid
	}
	return result, nil
}

func hgOpsValidText(value string, maxRunes int) bool {
	value = strings.TrimSpace(value)
	return value != "" && len([]rune(value)) <= maxRunes
}

func hgCoinMutationResponse(result CoinModelPackage.HGMutationResult) *OpsDtoPackage.HGCoinMutationResponse {
	return &OpsDtoPackage.HGCoinMutationResponse{Committed: result.Committed, IdempotentReplay: !result.Committed, TransactionID: strconv.FormatUint(result.TransactionID, 10), BalanceAfter: strconv.FormatUint(result.BalanceAfter, 10)}
}

func hgEncodeCoinCursor(cursor CoinModelPackage.HGTransactionCursor) (string, error) {
	encoded, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func hgDecodeCoinCursor(value string) (CoinModelPackage.HGTransactionCursor, error) {
	if strings.TrimSpace(value) == "" {
		return CoinModelPackage.HGTransactionCursor{}, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return CoinModelPackage.HGTransactionCursor{}, err
	}
	var cursor CoinModelPackage.HGTransactionCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil || cursor.CreatedAt.IsZero() || cursor.ID == 0 {
		return CoinModelPackage.HGTransactionCursor{}, ErrHGOperationsInvalid
	}
	return cursor, nil
}

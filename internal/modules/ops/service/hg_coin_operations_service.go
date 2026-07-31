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
	// ErrHGOperationsForbidden 表示当前 JWT 用户未在 admin_user 中处于有效状态。
	ErrHGOperationsForbidden = errors.New("operations permission denied")
	ErrHGOperationsInvalid   = errors.New("invalid operations request")
)

type hgOpsAuthorizer interface {
	IsActiveAdmin(context.Context, string) (bool, error)
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

type hgOpsProjectionCheckpoints interface {
	LoadCheckpoint(context.Context, string) (string, error)
}

// HGOperationalDeps 使用小接口注入资产、状态与授权依赖，便于测试且不让 handler 直接访问基础设施。
type HGOperationalDeps struct {
	Authorizer            hgOpsAuthorizer
	CoinAssets            hgOpsCoinAssets
	CoinQueries           hgOpsCoinQueries
	ProjectionCheckpoints hgOpsProjectionCheckpoints
}

// HGOperationalService 编排受信运维资产操作和低成本链路状态读取。
type HGOperationalService struct{ deps HGOperationalDeps }

// NewHGOperationalService 创建运维功能服务；未注入的状态依赖会返回空快照，不影响其他组件展示。
func NewHGOperationalService(deps HGOperationalDeps) *HGOperationalService {
	return &HGOperationalService{deps: deps}
}

func (s *HGOperationalService) hgAuthorize(ctx context.Context, operatorID string) error {
	if s == nil || s.deps.Authorizer == nil || strings.TrimSpace(operatorID) == "" {
		return ErrHGOperationsForbidden
	}
	allowed, err := s.deps.Authorizer.IsActiveAdmin(ctx, operatorID)
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
	if err := s.hgAuthorize(ctx, operatorID); err != nil {
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
	if err := s.hgAuthorize(ctx, operatorID); err != nil {
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
func (s *HGOperationalService) GrantCoin(ctx context.Context, operatorID string, req OpsDtoPackage.HGCoinGrantRequest) (*OpsDtoPackage.HGCoinMutationResponse, error) {
	if err := s.hgAuthorize(ctx, operatorID); err != nil {
		return nil, err
	}
	amount, err := hgParsePositiveCoinAmount(req.Amount)
	reason, reasonErr := hgOpsAuditReason(req.Reason, operatorID)
	if err != nil || reasonErr != nil || !hgOpsValidText(req.UserID, 255) || !hgOpsValidText(req.RequestID, 128) || !hgOpsValidText(req.BusinessKey, 255) || s.deps.CoinAssets == nil {
		return nil, ErrHGOperationsInvalid
	}
	result, err := s.deps.CoinAssets.Grant(ctx, CoinServicePackage.HGCreditCommand{UserID: strings.TrimSpace(req.UserID), RequestID: strings.TrimSpace(req.RequestID), Amount: amount, Reason: reason, BusinessType: "ops_grant", BusinessKey: strings.TrimSpace(req.BusinessKey), ExpiresAt: req.ExpiresAt})
	if err != nil {
		return nil, err
	}
	return hgCoinMutationResponse(result), nil
}

// RefundCoin 退款必须引用原 debit transaction，累计上限由权威资产事务校验。
func (s *HGOperationalService) RefundCoin(ctx context.Context, operatorID string, req OpsDtoPackage.HGCoinRefundRequest) (*OpsDtoPackage.HGCoinMutationResponse, error) {
	if err := s.hgAuthorize(ctx, operatorID); err != nil {
		return nil, err
	}
	amount, amountErr := hgParsePositiveCoinAmount(req.Amount)
	reference, referenceErr := strconv.ParseUint(strings.TrimSpace(req.ReferenceTransactionID), 10, 64)
	reason, reasonErr := hgOpsAuditReason(req.Reason, operatorID)
	if amountErr != nil || referenceErr != nil || reasonErr != nil || reference == 0 || !hgOpsValidText(req.UserID, 255) || !hgOpsValidText(req.RequestID, 128) || s.deps.CoinAssets == nil {
		return nil, ErrHGOperationsInvalid
	}
	result, err := s.deps.CoinAssets.Refund(ctx, CoinServicePackage.HGRefundCommand{UserID: strings.TrimSpace(req.UserID), RequestID: strings.TrimSpace(req.RequestID), Amount: amount, Reason: reason, ReferenceTransactionID: reference})
	if err != nil {
		return nil, err
	}
	return hgCoinMutationResponse(result), nil
}

// CorrectCoin 通过有界正负 delta 写 correction 流水，不提供余额覆盖接口。
func (s *HGOperationalService) CorrectCoin(ctx context.Context, operatorID string, req OpsDtoPackage.HGCoinCorrectionRequest) (*OpsDtoPackage.HGCoinMutationResponse, error) {
	if err := s.hgAuthorize(ctx, operatorID); err != nil {
		return nil, err
	}
	delta, err := strconv.ParseInt(strings.TrimSpace(req.Delta), 10, 64)
	reason, reasonErr := hgOpsAuditReason(req.Reason, operatorID)
	if err != nil || reasonErr != nil || delta == 0 || delta > int64(CoinServicePackage.HGMaxMutationAmount) || delta < -int64(CoinServicePackage.HGMaxMutationAmount) || !hgOpsValidText(req.UserID, 255) || !hgOpsValidText(req.RequestID, 128) || s.deps.CoinAssets == nil {
		return nil, ErrHGOperationsInvalid
	}
	result, err := s.deps.CoinAssets.Correct(ctx, CoinServicePackage.HGCorrectionCommand{UserID: strings.TrimSpace(req.UserID), RequestID: strings.TrimSpace(req.RequestID), Delta: delta, Reason: reason})
	if err != nil {
		return nil, err
	}
	return hgCoinMutationResponse(result), nil
}

// GetAssetPipelineStatus 聚合原子指标和固定 checkpoint，不执行大表扫描或 Kafka Admin 请求。
func (s *HGOperationalService) GetAssetPipelineStatus(ctx context.Context, operatorID string) (*OpsDtoPackage.HGAssetPipelineStatusResponse, error) {
	if err := s.hgAuthorize(ctx, operatorID); err != nil {
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

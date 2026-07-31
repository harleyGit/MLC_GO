package OpsServicePackage

import (
	CoinModelPackage "MLC_GO/internal/modules/coin/model"
	CoinServicePackage "MLC_GO/internal/modules/coin/service"
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	"context"
	"errors"
	"testing"
	"time"
)

type hgFakeOpsAuthorizer struct {
	permissions map[string]bool
	err         error
}

func (f hgFakeOpsAuthorizer) HasAssetPermission(_ context.Context, _ string, permission string) (bool, error) {
	return f.permissions[permission], f.err
}

func (f hgFakeOpsAuthorizer) ListAssetPermissions(context.Context, string) ([]string, error) {
	permissions := make([]string, 0, len(f.permissions))
	for permission, allowed := range f.permissions {
		if allowed {
			permissions = append(permissions, permission)
		}
	}
	return permissions, f.err
}

type hgFakeOpsCoinAssets struct {
	balance           uint64
	grantCommand      CoinServicePackage.HGCreditCommand
	refundCommand     CoinServicePackage.HGRefundCommand
	correctionCommand CoinServicePackage.HGCorrectionCommand
	result            CoinModelPackage.HGMutationResult
}

func (f *hgFakeOpsCoinAssets) Balance(context.Context, string) (uint64, error) { return f.balance, nil }
func (f *hgFakeOpsCoinAssets) Grant(_ context.Context, command CoinServicePackage.HGCreditCommand) (CoinModelPackage.HGMutationResult, error) {
	f.grantCommand = command
	return f.result, nil
}
func (f *hgFakeOpsCoinAssets) Refund(_ context.Context, command CoinServicePackage.HGRefundCommand) (CoinModelPackage.HGMutationResult, error) {
	f.refundCommand = command
	return f.result, nil
}
func (f *hgFakeOpsCoinAssets) Correct(_ context.Context, command CoinServicePackage.HGCorrectionCommand) (CoinModelPackage.HGMutationResult, error) {
	f.correctionCommand = command
	return f.result, nil
}

type hgFakeOpsCoinQueries struct {
	transactions []CoinModelPackage.HGTransaction
	next         CoinModelPackage.HGTransactionCursor
	hasMore      bool
	cursor       CoinModelPackage.HGTransactionCursor
	limit        int
}

func (f *hgFakeOpsCoinQueries) ListTransactions(_ context.Context, _ string, cursor CoinModelPackage.HGTransactionCursor, limit int) ([]CoinModelPackage.HGTransaction, CoinModelPackage.HGTransactionCursor, bool, error) {
	f.cursor, f.limit = cursor, limit
	return f.transactions, f.next, f.hasMore, nil
}
func (f *hgFakeOpsCoinQueries) LoadInitializerCheckpoint(context.Context) (uint64, error) {
	return 91, nil
}

type hgFakeProjectionCheckpoints map[string]string

func (f hgFakeProjectionCheckpoints) LoadCheckpoint(_ context.Context, stream string) (string, error) {
	return f[stream], nil
}

func TestHGOperationalServiceRejectsNonAdminBeforeAssetAccess(t *testing.T) {
	service := NewHGOperationalService(HGOperationalDeps{Authorizer: hgFakeOpsAuthorizer{permissions: map[string]bool{}}})
	_, err := service.GetCoinAccount(context.Background(), "user", "target")
	if !errors.Is(err, ErrHGOperationsForbidden) {
		t.Fatalf("error=%v, want ErrHGOperationsForbidden", err)
	}
}

func TestHGOperationalServiceUsesSeparateDatabasePermissions(t *testing.T) {
	assets := &hgFakeOpsCoinAssets{balance: 9}
	service := NewHGOperationalService(HGOperationalDeps{
		Authorizer: hgFakeOpsAuthorizer{permissions: map[string]bool{HGAssetPermissionBalanceRead: true}},
		CoinAssets: assets,
	})
	if _, err := service.GetCoinAccount(context.Background(), "admin-1", "user-1"); err != nil {
		t.Fatalf("GetCoinAccount() error=%v", err)
	}
	_, err := service.GrantCoin(context.Background(), HGAssetOperator{ID: "admin-1", SourceIP: "203.0.113.8"}, OpsDtoPackage.HGCoinGrantRequest{UserID: "user-1", RequestID: "grant-1", Amount: "1", Reason: "ticket", BusinessKey: "T-1"})
	if !errors.Is(err, ErrHGOperationsForbidden) {
		t.Fatalf("GrantCoin() error=%v, want ErrHGOperationsForbidden", err)
	}
}

func TestHGOperationalServiceGrantAddsImmutableOperatorAudit(t *testing.T) {
	assets := &hgFakeOpsCoinAssets{result: CoinModelPackage.HGMutationResult{Committed: true, TransactionID: 7, BalanceAfter: 15}}
	service := NewHGOperationalService(HGOperationalDeps{Authorizer: hgFakeOpsAuthorizer{permissions: map[string]bool{HGAssetPermissionGrant: true}}, CoinAssets: assets, RateLimiter: hgFakeAssetRateLimiter{allowed: true}, Audit: &hgFakeAssetAudit{}})

	result, err := service.GrantCoin(context.Background(), HGAssetOperator{ID: "admin-1", SourceIP: "203.0.113.8"}, OpsDtoPackage.HGCoinGrantRequest{
		UserID: "user-1", RequestID: "ticket-7", Amount: "5", Reason: "manual compensation", BusinessKey: "T-7",
	})
	if err != nil {
		t.Fatalf("GrantCoin() error=%v", err)
	}
	if assets.grantCommand.Reason != "manual compensation [operator=admin-1]" || assets.grantCommand.BusinessType != "ops_grant" {
		t.Fatalf("command=%+v", assets.grantCommand)
	}
	if result.TransactionID != "7" || result.BalanceAfter != "15" || result.IdempotentReplay {
		t.Fatalf("result=%+v", result)
	}
}

func TestHGOperationalServiceRejectsEmptyOrOversizedAuditReason(t *testing.T) {
	assets := &hgFakeOpsCoinAssets{}
	service := NewHGOperationalService(HGOperationalDeps{Authorizer: hgFakeOpsAuthorizer{permissions: map[string]bool{HGAssetPermissionGrant: true}}, CoinAssets: assets, RateLimiter: hgFakeAssetRateLimiter{allowed: true}, Audit: &hgFakeAssetAudit{}})
	for _, reason := range []string{"   ", string(make([]byte, 240))} {
		_, err := service.GrantCoin(context.Background(), HGAssetOperator{ID: "admin-identifier", SourceIP: "203.0.113.8"}, OpsDtoPackage.HGCoinGrantRequest{
			UserID: "user-1", RequestID: "ticket-8", Amount: "1", Reason: reason, BusinessKey: "T-8",
		})
		if !errors.Is(err, ErrHGOperationsInvalid) {
			t.Fatalf("reason length=%d error=%v, want ErrHGOperationsInvalid", len(reason), err)
		}
	}
}

func TestHGOperationalServiceRejectsOversizedAssetIdentifiers(t *testing.T) {
	assets := &hgFakeOpsCoinAssets{}
	service := NewHGOperationalService(HGOperationalDeps{Authorizer: hgFakeOpsAuthorizer{permissions: map[string]bool{HGAssetPermissionGrant: true}}, CoinAssets: assets, RateLimiter: hgFakeAssetRateLimiter{allowed: true}, Audit: &hgFakeAssetAudit{}})
	_, err := service.GrantCoin(context.Background(), HGAssetOperator{ID: "admin-1", SourceIP: "203.0.113.8"}, OpsDtoPackage.HGCoinGrantRequest{
		UserID: string(make([]byte, 256)), RequestID: string(make([]byte, 129)), Amount: "1", Reason: "ticket", BusinessKey: string(make([]byte, 256)),
	})
	if !errors.Is(err, ErrHGOperationsInvalid) {
		t.Fatalf("error=%v, want ErrHGOperationsInvalid", err)
	}
}

func TestHGOperationalServiceListsTransactionsWithOpaqueCursor(t *testing.T) {
	createdAt := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
	queries := &hgFakeOpsCoinQueries{
		transactions: []CoinModelPackage.HGTransaction{{ID: 8, UserID: "user-1", Amount: 3, SignedDelta: -3, CreatedAt: createdAt}},
		next:         CoinModelPackage.HGTransactionCursor{CreatedAt: createdAt, ID: 8}, hasMore: true,
	}
	service := NewHGOperationalService(HGOperationalDeps{Authorizer: hgFakeOpsAuthorizer{permissions: map[string]bool{HGAssetPermissionTransactionRead: true}}, CoinQueries: queries})

	result, err := service.GetCoinTransactions(context.Background(), "admin-1", "user-1", "", 20)
	if err != nil {
		t.Fatalf("GetCoinTransactions() error=%v", err)
	}
	if len(result.List) != 1 || result.List[0].TransactionID != "8" || result.NextCursor == "" || !result.HasMore {
		t.Fatalf("result=%+v", result)
	}
	if queries.limit != 20 || !queries.cursor.CreatedAt.IsZero() || queries.cursor.ID != 0 {
		t.Fatalf("cursor=%+v limit=%d", queries.cursor, queries.limit)
	}
}

func TestHGOperationalServiceBuildsBoundedPipelineSnapshot(t *testing.T) {
	service := NewHGOperationalService(HGOperationalDeps{
		Authorizer:            hgFakeOpsAuthorizer{permissions: map[string]bool{HGAssetPermissionPipelineRead: true}},
		CoinQueries:           &hgFakeOpsCoinQueries{},
		ProjectionCheckpoints: hgFakeProjectionCheckpoints{"video_state": `{"updatedAt":"2026-07-31T10:00:00Z","id":8}`},
	})

	result, err := service.GetAssetPipelineStatus(context.Background(), "admin-1")
	if err != nil {
		t.Fatalf("GetAssetPipelineStatus() error=%v", err)
	}
	if result.CoinInitializerCursor != "91" || len(result.InteractionStreams) != 4 || result.Kafka.Measurement == "" {
		t.Fatalf("result=%+v", result)
	}
}

type hgFakeAssetRateLimiter struct {
	allowed bool
	err     error
}

func (f hgFakeAssetRateLimiter) Allow(context.Context, string, string) (bool, error) {
	return f.allowed, f.err
}

type hgFakeAssetAudit struct {
	records []HGAssetAuditRecord
}

func (f *hgFakeAssetAudit) AppendAssetAudit(_ context.Context, record HGAssetAuditRecord) error {
	f.records = append(f.records, record)
	return nil
}

type hgFakeCorrections struct {
	pending OpsDtoPackage.HGCoinCorrectionResponse
}

func (f hgFakeCorrections) CreateCoinCorrection(context.Context, HGAssetOperator, OpsDtoPackage.HGCoinCorrectionRequest, int64) (OpsDtoPackage.HGCoinCorrectionResponse, error) {
	return f.pending, nil
}
func (f hgFakeCorrections) GetCoinCorrectionForApproval(context.Context, string, string) (OpsDtoPackage.HGCoinCorrectionResponse, error) {
	return f.pending, nil
}
func (f hgFakeCorrections) CompleteCoinCorrection(context.Context, string, string, CoinModelPackage.HGMutationResult, string) error {
	return nil
}
func (f hgFakeCorrections) ListCoinCorrections(context.Context, uint64, int) ([]OpsDtoPackage.HGCoinCorrectionResponse, uint64, bool, error) {
	return nil, 0, false, nil
}

func TestHGOperationalServiceFailsClosedWhenRateLimiterFails(t *testing.T) {
	assets := &hgFakeOpsCoinAssets{}
	service := NewHGOperationalService(HGOperationalDeps{
		Authorizer:  hgFakeOpsAuthorizer{permissions: map[string]bool{HGAssetPermissionGrant: true}},
		CoinAssets:  assets,
		RateLimiter: hgFakeAssetRateLimiter{err: errors.New("redis unavailable")},
		Audit:       &hgFakeAssetAudit{},
	})
	_, err := service.GrantCoin(context.Background(), HGAssetOperator{ID: "admin-1", SourceIP: "203.0.113.8"}, OpsDtoPackage.HGCoinGrantRequest{UserID: "user-1", RequestID: "grant-1", Amount: "1", Reason: "ticket", BusinessKey: "T-1"})
	if !errors.Is(err, ErrHGOperationsRateLimitUnavailable) {
		t.Fatalf("error=%v, want ErrHGOperationsRateLimitUnavailable", err)
	}
	if assets.grantCommand.UserID != "" {
		t.Fatalf("asset mutation reached after limiter failure: %+v", assets.grantCommand)
	}
}

func TestHGOperationalServiceCorrectionRequiresDifferentApprover(t *testing.T) {
	service := NewHGOperationalService(HGOperationalDeps{
		Authorizer: hgFakeOpsAuthorizer{permissions: map[string]bool{
			HGAssetPermissionCorrectionApprove: true,
			HGAssetPermissionCorrectionApply:   true,
		}},
		RateLimiter: hgFakeAssetRateLimiter{allowed: true},
		Corrections: hgFakeCorrections{pending: OpsDtoPackage.HGCoinCorrectionResponse{CorrectionID: "COR-1", ApplicantID: "admin-1"}},
		CoinAssets:  &hgFakeOpsCoinAssets{},
		Audit:       &hgFakeAssetAudit{},
	})
	_, err := service.ApproveCoinCorrection(context.Background(), HGAssetOperator{ID: "admin-1", SourceIP: "203.0.113.8"}, "COR-1")
	if !errors.Is(err, ErrHGOperationsInvalidApprover) {
		t.Fatalf("error=%v, want ErrHGOperationsInvalidApprover", err)
	}
}

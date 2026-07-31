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
	allowed bool
	err     error
}

func (f hgFakeOpsAuthorizer) IsActiveAdmin(context.Context, string) (bool, error) {
	return f.allowed, f.err
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
	service := NewHGOperationalService(HGOperationalDeps{Authorizer: hgFakeOpsAuthorizer{allowed: false}})
	_, err := service.GetCoinAccount(context.Background(), "user", "target")
	if !errors.Is(err, ErrHGOperationsForbidden) {
		t.Fatalf("error=%v, want ErrHGOperationsForbidden", err)
	}
}

func TestHGOperationalServiceGrantAddsImmutableOperatorAudit(t *testing.T) {
	assets := &hgFakeOpsCoinAssets{result: CoinModelPackage.HGMutationResult{Committed: true, TransactionID: 7, BalanceAfter: 15}}
	service := NewHGOperationalService(HGOperationalDeps{Authorizer: hgFakeOpsAuthorizer{allowed: true}, CoinAssets: assets})

	result, err := service.GrantCoin(context.Background(), "admin-1", OpsDtoPackage.HGCoinGrantRequest{
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
	service := NewHGOperationalService(HGOperationalDeps{Authorizer: hgFakeOpsAuthorizer{allowed: true}, CoinAssets: assets})
	for _, reason := range []string{"   ", string(make([]byte, 240))} {
		_, err := service.GrantCoin(context.Background(), "admin-identifier", OpsDtoPackage.HGCoinGrantRequest{
			UserID: "user-1", RequestID: "ticket-8", Amount: "1", Reason: reason, BusinessKey: "T-8",
		})
		if !errors.Is(err, ErrHGOperationsInvalid) {
			t.Fatalf("reason length=%d error=%v, want ErrHGOperationsInvalid", len(reason), err)
		}
	}
}

func TestHGOperationalServiceRejectsOversizedAssetIdentifiers(t *testing.T) {
	assets := &hgFakeOpsCoinAssets{}
	service := NewHGOperationalService(HGOperationalDeps{Authorizer: hgFakeOpsAuthorizer{allowed: true}, CoinAssets: assets})
	_, err := service.GrantCoin(context.Background(), "admin-1", OpsDtoPackage.HGCoinGrantRequest{
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
	service := NewHGOperationalService(HGOperationalDeps{Authorizer: hgFakeOpsAuthorizer{allowed: true}, CoinQueries: queries})

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
		Authorizer:            hgFakeOpsAuthorizer{allowed: true},
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

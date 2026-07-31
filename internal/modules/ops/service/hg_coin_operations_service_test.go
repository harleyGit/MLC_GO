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
	balanceErr        error
	grantCommand      CoinServicePackage.HGCreditCommand
	refundCommand     CoinServicePackage.HGRefundCommand
	correctionCommand CoinServicePackage.HGCorrectionCommand
	result            CoinModelPackage.HGMutationResult
	correctionErr     error
}

func (f *hgFakeOpsCoinAssets) Balance(context.Context, string) (uint64, error) {
	return f.balance, f.balanceErr
}
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
	return f.result, f.correctionErr
}

type hgFakeOpsCoinQueries struct {
	transactions []CoinModelPackage.HGTransaction
	next         CoinModelPackage.HGTransactionCursor
	hasMore      bool
	cursor       CoinModelPackage.HGTransactionCursor
	limit        int
}

type hgFakeOpsCoinUserLookup struct {
	field   string
	keyword string
	result  *OpsDtoPackage.HGCoinUserSearchItem
	err     error
	called  bool
}

func (f *hgFakeOpsCoinUserLookup) FindCoinUserByExactIdentity(_ context.Context, field, keyword string) (*OpsDtoPackage.HGCoinUserSearchItem, error) {
	f.field, f.keyword, f.called = field, keyword, true
	return f.result, f.err
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

func TestHGOperationalServiceSearchCoinUserUsesExactSelectedIdentity(t *testing.T) {
	lookup := &hgFakeOpsCoinUserLookup{result: &OpsDtoPackage.HGCoinUserSearchItem{UserID: "UID-101", UserName: "alice", NickName: "Alice", MatchedBy: "email"}}
	service := NewHGOperationalService(HGOperationalDeps{
		Authorizer: hgFakeOpsAuthorizer{permissions: map[string]bool{HGAssetPermissionBalanceRead: true}},
		UserLookup: lookup,
	})

	result, err := service.SearchCoinUser(context.Background(), "admin-1", "email", " alice@example.com ")
	if err != nil {
		t.Fatalf("SearchCoinUser() error=%v", err)
	}
	if lookup.field != "email" || lookup.keyword != "alice@example.com" {
		t.Fatalf("lookup field=%q keyword=%q", lookup.field, lookup.keyword)
	}
	if result.User == nil || result.User.UserID != "UID-101" || result.User.MatchedBy != "email" {
		t.Fatalf("result=%+v", result)
	}
}

func TestHGOperationalServiceSearchCoinUserRejectsInvalidFieldBeforeLookup(t *testing.T) {
	lookup := &hgFakeOpsCoinUserLookup{}
	service := NewHGOperationalService(HGOperationalDeps{
		Authorizer: hgFakeOpsAuthorizer{permissions: map[string]bool{HGAssetPermissionBalanceRead: true}},
		UserLookup: lookup,
	})

	_, err := service.SearchCoinUser(context.Background(), "admin-1", "nickname", "alice")
	if !errors.Is(err, ErrHGOperationsInvalid) {
		t.Fatalf("error=%v, want ErrHGOperationsInvalid", err)
	}
	if lookup.called {
		t.Fatal("invalid field reached user repository")
	}
}

func TestHGOperationalServiceSearchCoinUserRequiresTargetOperationPermission(t *testing.T) {
	lookup := &hgFakeOpsCoinUserLookup{}
	service := NewHGOperationalService(HGOperationalDeps{Authorizer: hgFakeOpsAuthorizer{permissions: map[string]bool{}}, UserLookup: lookup})

	_, err := service.SearchCoinUser(context.Background(), "admin-1", "phone", "13800138000")
	if !errors.Is(err, ErrHGOperationsForbidden) {
		t.Fatalf("error=%v, want ErrHGOperationsForbidden", err)
	}
	if lookup.called {
		t.Fatal("unauthorized search reached user repository")
	}
}

func TestHGOperationalServiceSearchCoinUserAllowsGrantOnlyOperator(t *testing.T) {
	lookup := &hgFakeOpsCoinUserLookup{result: &OpsDtoPackage.HGCoinUserSearchItem{UserID: "UID-202", MatchedBy: "userId"}}
	service := NewHGOperationalService(HGOperationalDeps{
		Authorizer: hgFakeOpsAuthorizer{permissions: map[string]bool{HGAssetPermissionGrant: true}},
		UserLookup: lookup,
	})

	result, err := service.SearchCoinUser(context.Background(), "admin-1", "userId", "UID-202")
	if err != nil {
		t.Fatalf("SearchCoinUser() error=%v", err)
	}
	if result.User == nil || result.User.UserID != "UID-202" {
		t.Fatalf("result=%+v", result)
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
	pending               OpsDtoPackage.HGCoinCorrectionResponse
	completedCorrectionID string
	completedApproverID   string
	completeErr           error
}

func (f *hgFakeCorrections) CreateCoinCorrection(context.Context, HGAssetOperator, OpsDtoPackage.HGCoinCorrectionRequest, int64) (OpsDtoPackage.HGCoinCorrectionResponse, error) {
	return f.pending, nil
}
func (f *hgFakeCorrections) GetCoinCorrectionForApproval(context.Context, string, string) (OpsDtoPackage.HGCoinCorrectionResponse, error) {
	return f.pending, nil
}
func (f *hgFakeCorrections) CompleteCoinCorrection(_ context.Context, correctionID, approverID string, _ CoinModelPackage.HGMutationResult, _ string) error {
	f.completedCorrectionID = correctionID
	f.completedApproverID = approverID
	return f.completeErr
}
func (f *hgFakeCorrections) ListCoinCorrections(context.Context, uint64, int) ([]OpsDtoPackage.HGCoinCorrectionResponse, uint64, bool, error) {
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
		Corrections: &hgFakeCorrections{pending: OpsDtoPackage.HGCoinCorrectionResponse{CorrectionID: "COR-1", ApplicantID: "admin-1"}},
		CoinAssets:  &hgFakeOpsCoinAssets{},
		Audit:       &hgFakeAssetAudit{},
	})
	_, err := service.ApproveCoinCorrection(context.Background(), HGAssetOperator{ID: "admin-1", SourceIP: "203.0.113.8"}, "COR-1")
	if !errors.Is(err, ErrHGOperationsInvalidApprover) {
		t.Fatalf("error=%v, want ErrHGOperationsInvalidApprover", err)
	}
}

func TestHGOperationalServiceRetriesApprovingCorrectionWithOriginalRequestID(t *testing.T) {
	assets := &hgFakeOpsCoinAssets{balance: 10, result: CoinModelPackage.HGMutationResult{Committed: false, TransactionID: 81, BalanceAfter: 13}}
	corrections := &hgFakeCorrections{}
	audit := &hgFakeAssetAudit{}
	service := NewHGOperationalService(HGOperationalDeps{CoinAssets: assets, Corrections: corrections, Audit: audit})
	pending := OpsDtoPackage.HGCoinCorrectionResponse{
		CorrectionID: "COR-81", UserID: "user-81", RequestID: "request-original-81", Delta: "3",
		Reason: "approved ticket", ApplicantID: "admin-applicant", ApproverID: "admin-approver", Status: "approving",
	}

	if err := service.ReplayApprovingCoinCorrection(context.Background(), pending); err != nil {
		t.Fatalf("ReplayApprovingCoinCorrection() error=%v", err)
	}
	if assets.correctionCommand.RequestID != "request-original-81" {
		t.Fatalf("request ID=%q, want original persisted request ID", assets.correctionCommand.RequestID)
	}
	if corrections.completedCorrectionID != "COR-81" || corrections.completedApproverID != "admin-approver" {
		t.Fatalf("completion=%+v", corrections)
	}
	if len(audit.records) != 1 || audit.records[0].Outcome != "succeeded" {
		t.Fatalf("audit=%+v", audit.records)
	}
}

func TestHGOperationalServiceReusesCorrectionAuditEventKeyAfterCompletionFailure(t *testing.T) {
	assets := &hgFakeOpsCoinAssets{balance: 10, result: CoinModelPackage.HGMutationResult{Committed: false, TransactionID: 84, BalanceAfter: 13}}
	corrections := &hgFakeCorrections{completeErr: errors.New("temporary completion timeout")}
	audit := &hgFakeAssetAudit{}
	service := NewHGOperationalService(HGOperationalDeps{CoinAssets: assets, Corrections: corrections, Audit: audit})
	pending := OpsDtoPackage.HGCoinCorrectionResponse{
		CorrectionID: "COR-84", UserID: "user-84", RequestID: "request-original-84", Delta: "3",
		Reason: "approved ticket", ApplicantID: "admin-applicant", ApproverID: "admin-approver", Status: "approving",
	}

	if err := service.ReplayApprovingCoinCorrection(context.Background(), pending); err == nil {
		t.Fatal("expected first completion failure")
	}
	if err := service.ReplayApprovingCoinCorrection(context.Background(), pending); err == nil {
		t.Fatal("expected second completion failure")
	}
	if len(audit.records) != 2 {
		t.Fatalf("audit count=%d, want 2 attempted writes", len(audit.records))
	}
	const wantEventKey = "v1|coin.correction.apply|succeeded|request-original-84"
	if audit.records[0].EventKey != wantEventKey || audit.records[1].EventKey != wantEventKey {
		t.Fatalf("event keys=%q,%q, want %q", audit.records[0].EventKey, audit.records[1].EventKey, wantEventKey)
	}
}

func TestHGOperationalServiceLeavesApprovingCorrectionRetryableOnTransientFailure(t *testing.T) {
	assets := &hgFakeOpsCoinAssets{balance: 10, correctionErr: errors.New("temporary mysql timeout")}
	corrections := &hgFakeCorrections{}
	service := NewHGOperationalService(HGOperationalDeps{CoinAssets: assets, Corrections: corrections, Audit: &hgFakeAssetAudit{}})
	pending := OpsDtoPackage.HGCoinCorrectionResponse{
		CorrectionID: "COR-82", UserID: "user-82", RequestID: "request-original-82", Delta: "-2",
		Reason: "approved ticket", ApplicantID: "admin-applicant", ApproverID: "admin-approver", Status: "approving",
	}

	if err := service.ReplayApprovingCoinCorrection(context.Background(), pending); err == nil {
		t.Fatal("expected transient replay failure")
	}
	if corrections.completedCorrectionID != "" {
		t.Fatalf("transient failure must remain approving, completion=%+v", corrections)
	}
}

func TestHGOperationalServiceApprovalLeavesMutationFailureApprovingForRecovery(t *testing.T) {
	assets := &hgFakeOpsCoinAssets{balance: 10, correctionErr: errors.New("temporary mysql timeout")}
	corrections := &hgFakeCorrections{pending: OpsDtoPackage.HGCoinCorrectionResponse{
		CorrectionID: "COR-83", UserID: "user-83", RequestID: "request-original-83", Delta: "2",
		Reason: "approved ticket", ApplicantID: "admin-applicant", ApproverID: "admin-approver", Status: "approving",
	}}
	service := NewHGOperationalService(HGOperationalDeps{
		Authorizer: hgFakeOpsAuthorizer{permissions: map[string]bool{
			HGAssetPermissionCorrectionApprove: true,
			HGAssetPermissionCorrectionApply:   true,
		}},
		RateLimiter: hgFakeAssetRateLimiter{allowed: true}, CoinAssets: assets, Corrections: corrections, Audit: &hgFakeAssetAudit{},
	})

	if _, err := service.ApproveCoinCorrection(context.Background(), HGAssetOperator{ID: "admin-approver", SourceIP: "203.0.113.8"}, "COR-83"); err == nil {
		t.Fatal("expected correction mutation failure")
	}
	if corrections.completedCorrectionID != "" {
		t.Fatalf("mutation failure must remain approving for timeout recovery, completion=%+v", corrections)
	}
}

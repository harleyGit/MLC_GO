package CoinServicePackage

import (
	CoinEventsPackage "MLC_GO/internal/events/coin"
	CoinModelPackage "MLC_GO/internal/modules/coin/model"
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

type hgFakeCoinRepository struct {
	command CoinModelPackage.HGCommand
	result  CoinModelPackage.HGMutationResult
	err     error
}

func (f *hgFakeCoinRepository) Mutate(_ context.Context, command CoinModelPackage.HGCommand) (CoinModelPackage.HGMutationResult, error) {
	f.command = command
	return f.result, f.err
}

func (f *hgFakeCoinRepository) Balance(context.Context, string) (uint64, error) { return 0, nil }

func TestHGServiceGrantBuildsAuditedExpiringCredit(t *testing.T) {
	repository := &hgFakeCoinRepository{result: CoinModelPackage.HGMutationResult{Committed: true, TransactionID: 7, BalanceAfter: 12}}
	service := NewHGService(repository)
	expiresAt := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	result, err := service.Grant(context.Background(), HGCreditCommand{
		UserID: "user-1", RequestID: "grant-1", Amount: 12, Reason: "campaign", ExpiresAt: &expiresAt,
	})
	if err != nil {
		t.Fatalf("Grant() error = %v", err)
	}
	if !result.Committed || repository.command.Operation != CoinModelPackage.HGOperationGrant || repository.command.ExpiresAt != &expiresAt {
		t.Fatalf("result=%+v command=%+v", result, repository.command)
	}
	if repository.command.Event == nil || repository.command.Event.EventName() != CoinEventsPackage.HGAssetChangedEventName {
		t.Fatalf("event = %#v", repository.command.Event)
	}
}

func TestHGServiceRejectsInvalidMutationBeforeRepository(t *testing.T) {
	repository := &hgFakeCoinRepository{}
	service := NewHGService(repository)

	_, err := service.Debit(context.Background(), HGDebitCommand{UserID: "user-1", RequestID: "debit-1"})
	if !errors.Is(err, ErrHGInvalidAmount) {
		t.Fatalf("error = %v, want ErrHGInvalidAmount", err)
	}
	if repository.command.Operation != "" {
		t.Fatalf("repository called with %+v", repository.command)
	}
}

func TestHGServiceCorrectionIsExplicitAuditedMutation(t *testing.T) {
	repository := &hgFakeCoinRepository{result: CoinModelPackage.HGMutationResult{Committed: true}}
	service := NewHGService(repository)

	_, err := service.Correct(context.Background(), HGCorrectionCommand{
		UserID: "user-1", RequestID: "correction-1", Delta: -3, Reason: "ticket-42",
	})
	if err != nil {
		t.Fatalf("Correct() error = %v", err)
	}
	if repository.command.Operation != CoinModelPackage.HGOperationCorrection || repository.command.Reason != "ticket-42" {
		t.Fatalf("command = %+v", repository.command)
	}
}

func TestHGServiceRejectsAmountAboveLotReadBound(t *testing.T) {
	service := NewHGService(&hgFakeCoinRepository{})
	_, err := service.Grant(context.Background(), HGCreditCommand{UserID: "u-1", RequestID: "r-1", Amount: HGMaxMutationAmount + 1})
	if !errors.Is(err, ErrHGInvalidAmount) {
		t.Fatalf("error = %v, want ErrHGInvalidAmount", err)
	}
}

func TestHGServiceRejectsCorrectionOutsideLotReadBound(t *testing.T) {
	service := NewHGService(&hgFakeCoinRepository{})
	for _, delta := range []int64{int64(HGMaxMutationAmount) + 1, -int64(HGMaxMutationAmount) - 1, math.MinInt64} {
		_, err := service.Correct(context.Background(), HGCorrectionCommand{
			UserID: "u-1", RequestID: "correction-bound", Delta: delta, Reason: "ticket-42",
		})
		if !errors.Is(err, ErrHGInvalidAmount) {
			t.Fatalf("delta=%d error=%v, want ErrHGInvalidAmount", delta, err)
		}
	}
}

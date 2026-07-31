package OpsTaskPackage

import (
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	"context"
	"testing"
	"time"
)

type hgFakeCorrectionRecoveryRepository struct {
	cutoff time.Time
	limit  int
	items  []OpsDtoPackage.HGCoinCorrectionResponse
}

func (f *hgFakeCorrectionRecoveryRepository) ListStaleApprovingCoinCorrections(_ context.Context, cutoff time.Time, limit int) ([]OpsDtoPackage.HGCoinCorrectionResponse, error) {
	f.cutoff = cutoff
	f.limit = limit
	return f.items, nil
}

type hgFakeCorrectionReplayer struct {
	requestIDs []string
}

func (f *hgFakeCorrectionReplayer) ReplayApprovingCoinCorrection(_ context.Context, item OpsDtoPackage.HGCoinCorrectionResponse) error {
	f.requestIDs = append(f.requestIDs, item.RequestID)
	return nil
}

func TestHGCorrectionRecoveryScansBoundedStaleRowsAndReplaysOriginalRequestIDs(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	repository := &hgFakeCorrectionRecoveryRepository{items: []OpsDtoPackage.HGCoinCorrectionResponse{
		{CorrectionID: "COR-1", RequestID: "REQ-ORIGINAL-1", Status: "approving"},
		{CorrectionID: "COR-2", RequestID: "REQ-ORIGINAL-2", Status: "approving"},
	}}
	replayer := &hgFakeCorrectionReplayer{}
	worker, err := NewHGCorrectionRecovery(repository, replayer, HGCorrectionRecoveryConfig{
		Interval: time.Minute, Timeout: 10 * time.Second, ApprovingTimeout: 5 * time.Minute, BatchSize: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	worker.now = func() time.Time { return now }

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error=%v", err)
	}
	if !repository.cutoff.Equal(now.Add(-5*time.Minute)) || repository.limit != 2 {
		t.Fatalf("scan cutoff=%s limit=%d", repository.cutoff, repository.limit)
	}
	if len(replayer.requestIDs) != 2 || replayer.requestIDs[0] != "REQ-ORIGINAL-1" || replayer.requestIDs[1] != "REQ-ORIGINAL-2" {
		t.Fatalf("replayed request IDs=%v", replayer.requestIDs)
	}
}

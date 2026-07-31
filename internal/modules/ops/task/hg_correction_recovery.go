package OpsTaskPackage

import (
	OpsDtoPackage "MLC_GO/internal/modules/ops/dto"
	"MLC_GO/internal/pkg/logHG"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const hgCorrectionRecoveryLeaseName = "ops_correction_recovery"

// HGCorrectionRecoveryConfig bounds timeout detection and per-run replay work.
type HGCorrectionRecoveryConfig struct {
	Interval         time.Duration
	Timeout          time.Duration
	ApprovingTimeout time.Duration
	BatchSize        int
}

type hgCorrectionRecoveryRepository interface {
	ListStaleApprovingCoinCorrections(context.Context, time.Time, int) ([]OpsDtoPackage.HGCoinCorrectionResponse, error)
}

type hgCorrectionReplayer interface {
	ReplayApprovingCoinCorrection(context.Context, OpsDtoPackage.HGCoinCorrectionResponse) error
}

type hgCorrectionRecoveryLease interface {
	Acquire(context.Context, string, time.Duration) (string, bool, error)
	Release(context.Context, string, string) error
}

// HGCorrectionRecovery scans one bounded stale approving batch and retries each item with its persisted request ID.
type HGCorrectionRecovery struct {
	repository hgCorrectionRecoveryRepository
	replayer   hgCorrectionReplayer
	lease      hgCorrectionRecoveryLease
	config     HGCorrectionRecoveryConfig
	now        func() time.Time
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.Mutex
}

// NewHGCorrectionRecovery validates strict capacity and timeout bounds.
func NewHGCorrectionRecovery(repository hgCorrectionRecoveryRepository, replayer hgCorrectionReplayer, config HGCorrectionRecoveryConfig, leases ...hgCorrectionRecoveryLease) (*HGCorrectionRecovery, error) {
	if repository == nil || replayer == nil || config.Interval <= 0 || config.Timeout <= 0 || config.Timeout >= config.Interval || config.ApprovingTimeout <= config.Timeout || config.BatchSize < 1 || config.BatchSize > 100 {
		return nil, errors.New("correction recovery configuration is invalid")
	}
	var lease hgCorrectionRecoveryLease
	if len(leases) > 0 {
		lease = leases[0]
	}
	return &HGCorrectionRecovery{repository: repository, replayer: replayer, lease: lease, config: config, now: time.Now}, nil
}

// Start launches one immediate run followed by periodic bounded recovery runs.
func (w *HGCorrectionRecovery) Start(parent context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	w.cancel = cancel
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer func() {
			if recovered := recover(); recovered != nil {
				logHG.ErrFInfo("Correction recovery panic: %v", recovered)
			}
		}()
		w.hgRunAndLog(ctx)
		ticker := time.NewTicker(w.config.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.hgRunAndLog(ctx)
			}
		}
	}()
}

func (w *HGCorrectionRecovery) hgRunAndLog(ctx context.Context) {
	if err := w.RunOnce(ctx); err != nil && ctx.Err() == nil {
		logHG.ErrFInfo("Correction recovery run failed: %v", err)
	}
}

// RunOnce acquires the global lease, scans one timeout-index batch, and retries sequentially to cap database pressure.
func (w *HGCorrectionRecovery) RunOnce(ctx context.Context) error {
	runCtx, cancel := context.WithTimeout(ctx, w.config.Timeout)
	defer cancel()
	if w.lease != nil {
		token, acquired, err := w.lease.Acquire(runCtx, hgCorrectionRecoveryLeaseName, w.config.Timeout+time.Second)
		if err != nil || !acquired {
			return err
		}
		defer func() { _ = w.lease.Release(context.WithoutCancel(runCtx), hgCorrectionRecoveryLeaseName, token) }()
	}
	items, err := w.repository.ListStaleApprovingCoinCorrections(runCtx, w.now().UTC().Add(-w.config.ApprovingTimeout), w.config.BatchSize)
	if err != nil {
		return err
	}
	var joined error
	for _, item := range items {
		if err := w.replayer.ReplayApprovingCoinCorrection(runCtx, item); err != nil {
			joined = errors.Join(joined, fmt.Errorf("replay correction %s: %w", item.CorrectionID, err))
		}
	}
	return joined
}

// Close stops the worker and waits for the active bounded run to return.
func (w *HGCorrectionRecovery) Close() {
	w.mu.Lock()
	cancel := w.cancel
	w.cancel = nil
	w.mu.Unlock()
	if cancel != nil {
		cancel()
		w.wg.Wait()
	}
}

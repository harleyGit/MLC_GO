package CoinTaskPackage

import (
	CoinModelPackage "MLC_GO/internal/modules/coin/model"
	"MLC_GO/internal/pkg/logHG"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// HGMaxJobBatchSize 限制单轮初始化、过期和对账访问量，避免后台补偿占满数据库连接池。
const HGMaxJobBatchSize = 1000

// HGJobConfig 定义任务周期、单轮超时和数据库批量上限。
type HGJobConfig struct {
	Interval                  time.Duration
	Timeout                   time.Duration
	BatchSize                 int
	ConsolidationBatchSize    int
	ConsolidationSourceLimit  int
	ConsolidationMaxLotAmount uint64
}

type hgJobRepository interface {
	LoadInitializerCheckpoint(context.Context) (uint64, error)
	ListUsersAfter(context.Context, uint64, int) ([]CoinModelPackage.HGUserCursor, error)
	EnsureWallet(context.Context, string) error
	SaveInitializerCheckpoint(context.Context, uint64) error
	ExpireBatch(context.Context, int, time.Time) (int, error)
	ReconcileBatch(context.Context, uint64, int) ([]CoinModelPackage.HGWalletDrift, uint64, error)
	LoadReconciliationCheckpoint(context.Context) (uint64, error)
	SaveReconciliationCheckpoint(context.Context, uint64) error
	LoadConsolidationCheckpoint(context.Context) (uint64, error)
	SaveConsolidationCheckpoint(context.Context, uint64) error
	ConsolidateBatch(context.Context, uint64, int, int, uint64) (int, uint64, error)
}

type hgJobLease interface {
	Acquire(context.Context, string, time.Duration) (string, bool, error)
	Release(context.Context, string, string) error
}

// HGJobs 运行有界钱包初始化、lot 过期和只检测不修复的资产对账。
// 多副本通过 Redis lease 选出单轮 owner；任务失败不推进 checkpoint，后续轮次可安全重放。
type HGJobs struct {
	repository hgJobRepository
	config     HGJobConfig
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.Mutex
	lease      hgJobLease
}

// NewHGJobs 校验容量边界并创建停止状态的任务集合。
func NewHGJobs(repository hgJobRepository, config HGJobConfig, leases ...hgJobLease) (*HGJobs, error) {
	if repository == nil || config.Interval <= 0 || config.Timeout <= 0 || config.Timeout >= config.Interval || config.BatchSize < 1 || config.BatchSize > HGMaxJobBatchSize {
		return nil, errors.New("coin job configuration is invalid")
	}
	if config.ConsolidationBatchSize < 0 || config.ConsolidationBatchSize > HGMaxJobBatchSize || (config.ConsolidationBatchSize > 0 && (config.ConsolidationSourceLimit < 2 || config.ConsolidationSourceLimit > HGMaxJobBatchSize || config.ConsolidationMaxLotAmount == 0)) {
		return nil, errors.New("coin consolidation configuration is invalid")
	}
	var lease hgJobLease
	if len(leases) > 0 {
		lease = leases[0]
	}
	return &HGJobs{repository: repository, config: config, lease: lease}, nil
}

// Start 启动一个受 parent context 管理的周期 goroutine；重复调用不会创建第二个 worker。
func (j *HGJobs) Start(parent context.Context) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	j.cancel = cancel
	j.wg.Add(1)
	go func() {
		defer j.wg.Done()
		defer func() {
			if recovered := recover(); recovered != nil {
				logHG.ErrFInfo("Coin jobs panic: %v", recovered)
			}
		}()
		if err := j.hgRunSafely(ctx); err != nil && ctx.Err() == nil {
			logHG.ErrFInfo("Coin jobs startup run failed: %v", err)
		}
		ticker := time.NewTicker(j.config.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := j.hgRunSafely(ctx); err != nil && ctx.Err() == nil {
					logHG.ErrFInfo("Coin jobs run failed: %v", err)
				}
			}
		}
	}()
}

func (j *HGJobs) hgRunSafely(ctx context.Context) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("coin jobs panic: %v", recovered)
		}
	}()
	return j.RunOnce(ctx)
}

// Close 取消任务并等待当前有界数据库操作观察到 context 退出。
func (j *HGJobs) Close() {
	j.mu.Lock()
	cancel := j.cancel
	j.cancel = nil
	j.mu.Unlock()
	if cancel != nil {
		cancel()
		j.wg.Wait()
	}
}

// RunOnce 在分布式 lease 下依次执行初始化、过期和对账，每项失败都会保留完整错误上下文。
func (j *HGJobs) RunOnce(ctx context.Context) error {
	runCtx, cancel := context.WithTimeout(ctx, j.config.Timeout)
	defer cancel()
	var joined error
	if err := j.hgRunTask(runCtx, "wallet_initializer", j.hgInitialize); err != nil {
		joined = errors.Join(joined, fmt.Errorf("initialize wallets: %w", err))
	}
	if err := j.hgRunTask(runCtx, "lot_expiration", func(ctx context.Context) error {
		_, err := j.repository.ExpireBatch(ctx, j.config.BatchSize, time.Now().UTC())
		return err
	}); err != nil {
		joined = errors.Join(joined, fmt.Errorf("expire lots: %w", err))
	}
	if err := j.hgRunTask(runCtx, "wallet_reconciliation", j.hgReconcile); err != nil {
		joined = errors.Join(joined, fmt.Errorf("reconcile wallets: %w", err))
	}
	if j.config.ConsolidationBatchSize > 0 {
		if err := j.hgRunTask(runCtx, "lot_consolidation", j.hgConsolidate); err != nil {
			joined = errors.Join(joined, fmt.Errorf("consolidate lots: %w", err))
		}
	}
	return joined
}

func (j *HGJobs) hgRunTask(ctx context.Context, task string, run func(context.Context) error) error {
	if j.lease == nil {
		return run(ctx)
	}
	token, acquired, err := j.lease.Acquire(ctx, task, j.config.Timeout+time.Second)
	if err != nil || !acquired {
		return err
	}
	defer func() { _ = j.lease.Release(context.WithoutCancel(ctx), task, token) }()
	return run(ctx)
}

func (j *HGJobs) hgReconcile(ctx context.Context) error {
	cursor, err := j.repository.LoadReconciliationCheckpoint(ctx)
	if err != nil {
		return err
	}
	drifts, next, err := j.repository.ReconcileBatch(ctx, cursor, j.config.BatchSize)
	if err != nil {
		return err
	}
	if len(drifts) > 0 {
		hgObserveReconciliationDrift(len(drifts))
	}
	return j.repository.SaveReconciliationCheckpoint(ctx, next)
}

func (j *HGJobs) hgConsolidate(ctx context.Context) error {
	cursor, err := j.repository.LoadConsolidationCheckpoint(ctx)
	if err != nil {
		return err
	}
	_, next, err := j.repository.ConsolidateBatch(ctx, cursor, j.config.ConsolidationBatchSize, j.config.ConsolidationSourceLimit, j.config.ConsolidationMaxLotAmount)
	if err != nil {
		return err
	}
	return j.repository.SaveConsolidationCheckpoint(ctx, next)
}

func (j *HGJobs) hgInitialize(ctx context.Context) error {
	cursor, err := j.repository.LoadInitializerCheckpoint(ctx)
	if err != nil {
		return err
	}
	users, err := j.repository.ListUsersAfter(ctx, cursor, j.config.BatchSize)
	if err != nil {
		return err
	}
	for _, user := range users {
		if err := j.repository.EnsureWallet(ctx, user.UserID); err != nil {
			return err
		}
		cursor = user.ID
	}
	if len(users) > 0 {
		return j.repository.SaveInitializerCheckpoint(ctx, cursor)
	}
	return nil
}

package send

import (
	notificationevents "MLC_GO/internal/events/notification"
	notification "MLC_GO/internal/modules/notification"
	"MLC_GO/internal/pkg/logHG"
	queries "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
)

type hgJob struct {
	id         uint64            // 任务主键。
	template   uint64            // 创建时选择的模板主键。
	version    uint32            // 已发布的不可变版本号。
	users      []string          // 有界接收人数组。
	vars       map[string]string // 共享字符串变量。
	index      int               // 下一位接收人的下标。
	attempts   uint8             // 当前接收人的连续失败次数。
	token      string            // 当前工作者的独占租约。
	mode       string            // mysql/kafka；只影响租约回执 SQL，不改变业务通知 ID。
	generation uint64            // Kafka 当前分发代次；MySQL 模式为零。
}

// HGWorker 从 MySQL 持久队列串行取单个接收人；每 Pod 最多一个工作协程，不在事务内投递。
type HGWorker struct {
	repo    *HGRepository           // 任务仓储与事务租约。
	notices *notification.HGService // 站内信幂等写入与偏好处理。
	cancel  context.CancelFunc      // 停止接收下一份任务。
	done    chan struct{}           // 等待当前单次投递完成。
	once    sync.Once               // Close 可以重复调用。
}

// HGNewWorker 构建工作者，调用 HGStart 后才处理队列。
func HGNewWorker(repo *HGRepository, notices *notification.HGService) *HGWorker {
	return &HGWorker{repo: repo, notices: notices}
}

// HGStart 以固定单工速率轮询；任务超过限制一律失败而不扩张 goroutine/队列。
func (worker *HGWorker) HGStart() {
	ctx, cancel := context.WithCancel(context.Background())
	worker.cancel, worker.done = cancel, make(chan struct{})
	go func() {
		defer close(worker.done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		lastErrorLog := time.Time{}
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				callCtx, stop := context.WithTimeout(ctx, 10*time.Second)
				if err := worker.HGDispatchOnce(callCtx); err != nil && time.Since(lastErrorLog) >= time.Minute {
					// 只输出错误类型且限速，禁止记录接收人、模板正文与数据库连接参数。
					logHG.ErrFInfo("通知发送任务处理失败（限速日志）：%T", err)
					lastErrorLog = time.Now()
				}
				stop()
			}
		}
	}()
}

// Close 等待正在运行的有界单次投递退出，随后才允许关闭数据库连接池。
func (worker *HGWorker) Close() {
	if worker == nil {
		return
	}
	worker.once.Do(func() {
		if worker.cancel != nil {
			worker.cancel()
			<-worker.done
		}
	})
}

func (worker *HGWorker) hgClaim(ctx context.Context) (*hgJob, error) {
	tx, err := worker.repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	job := new(hgJob)
	var audience, variables []byte
	scan := func(query string) error {
		return tx.QueryRowContext(ctx, query).Scan(&job.id, &job.template, &job.version, &audience, &variables, &job.index, &job.attempts)
	}
	// 在新请求持续涌入时仍能轮流恢复超时租约，避免永远饿死已持久化的旧任务。
	first, second := queries.HGNotificationSendClaimReadySQL, queries.HGNotificationSendClaimExpiredSQL
	if time.Now().Unix()%2 == 0 {
		first, second = second, first
	}
	err = scan(first)
	if errors.Is(err, sql.ErrNoRows) {
		err = scan(second)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	job.token = hex.EncodeToString(nonce[:])
	job.mode = "mysql"
	if _, err = tx.ExecContext(ctx, queries.HGNotificationSendLeaseSQL, job.token, job.id); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if json.Unmarshal(audience, &job.users) != nil || json.Unmarshal(variables, &job.vars) != nil ||
		len(job.users) == 0 || len(job.users) > 100 || job.index < 0 || job.index >= len(job.users) {
		// 快照损坏不能反复租约重试，也不能尝试发送给未知用户。
		_, _ = worker.repo.db.ExecContext(ctx, queries.HGNotificationSendCorruptSQL, job.id, job.token)
		return nil, HGErrInvalid
	}
	return job, nil
}

func (worker *HGWorker) hgFinish(ctx context.Context, job *hgJob, delivered bool) error {
	var result sql.Result
	var err error
	if delivered {
		status := "pending"
		if job.index+1 == len(job.users) {
			status = "done"
		}
		if job.mode == "kafka" {
			result, err = worker.repo.db.ExecContext(ctx, queries.HGNotificationKafkaAdvanceSQL,
				job.index+1, status, job.id, job.token, job.generation, job.index)
		} else {
			result, err = worker.repo.db.ExecContext(ctx, queries.HGNotificationSendAdvanceSQL,
				job.index+1, status, job.id, job.token, job.index)
		}
	} else {
		next := job.attempts + 1
		status := "pending"
		if next >= 5 {
			status = "failed" // 永久失败等待人工排查，不无限重试。
		}
		delay := 1 << min(int(next), 5)
		if job.mode == "kafka" {
			result, err = worker.repo.db.ExecContext(ctx, queries.HGNotificationKafkaRetrySQL,
				status, next, delay, job.id, job.token, job.generation, job.index)
		} else {
			result, err = worker.repo.db.ExecContext(ctx, queries.HGNotificationSendRetrySQL,
				status, next, delay, job.id, job.token, job.index)
		}
	}
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return HGErrConflict
	}
	return nil
}

// HGDispatchOnce 只处理一个用户；崩溃重试时稳定 event_id 与既有 ledger 唯一键共同防重复入箱。
func (worker *HGWorker) HGDispatchOnce(ctx context.Context) error {
	if worker == nil || worker.repo == nil || worker.repo.db == nil || worker.notices == nil {
		return errors.New("通知发送依赖未就绪")
	}
	job, err := worker.hgClaim(ctx)
	if err != nil || job == nil {
		return err
	}
	_, err = worker.hgProcessJob(ctx, job)
	return err
}

// hgProcessJob 返回是否已持久化任务回执；Kafka 只在状态回执成功后提交当前 offset。
func (worker *HGWorker) hgProcessJob(ctx context.Context, job *hgJob) (bool, error) {
	version, err := hgScanVersion(worker.repo.db.QueryRowContext(ctx, queries.HGNotificationSendBoundVersionSQL, job.version, job.template))
	if err == nil {
		var notice notification.HGNotice
		notice, err = HGRender(ctx, version, job.vars)
		if err == nil {
			var exists int
			err = worker.repo.db.QueryRowContext(ctx, queries.HGNotificationSendUserExistsSQL, job.users[job.index]).Scan(&exists)
			if err == nil {
				hash := sha256.Sum256([]byte(job.users[job.index]))
				eventID := fmt.Sprintf("ntf-%s-%x", strconv.FormatUint(job.id, 10), hash[:8])
				err = worker.notices.HGDeliver(ctx, eventID, job.users[job.index], notice)
			}
		}
	}
	if err != nil {
		if finishErr := worker.hgFinish(ctx, job, false); finishErr != nil {
			return false, finishErr
		}
		return true, err
	}
	if err := worker.hgFinish(ctx, job, true); err != nil {
		return false, err
	}
	return true, nil
}

// HGDispatchReady 只消费单一 Kafka 代次；业务失败已登记重试后提交 offset，DB 回执失败则保持重放。
func (worker *HGWorker) HGDispatchReady(ctx context.Context, event notificationevents.HGSendReadyEvent) error {
	if worker == nil || worker.repo == nil || worker.repo.db == nil || worker.notices == nil {
		return errors.New("通知发送依赖未就绪")
	}
	if err := event.HGValidate(); err != nil {
		return err
	}
	job, err := worker.hgClaimReady(ctx, event)
	if err != nil || job == nil {
		return err
	}
	recorded, err := worker.hgProcessJob(ctx, job)
	if recorded {
		return nil
	}
	return err
}

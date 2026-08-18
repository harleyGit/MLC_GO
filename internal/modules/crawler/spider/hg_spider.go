package spider

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	CrawlerPlatformPackage "MLC_GO/internal/modules/crawler/platform"
)

// hgMaxTaskHistory 限制单进程任务快照数量，防止常驻 worker 随运行时间无界增长内存。
const hgMaxTaskHistory = 500

// HGManager 管理单个平台 worker、任务历史和最新推荐快照。
// 内存快照有严格上限，生产持久化应由独立 repository 批量写入专用外部内容表。
type HGManager struct {
	// platform、interval、timeout 构造后只读，不需要锁保护。
	platform CrawlerPlatformPackage.HGPlatform
	interval time.Duration
	timeout  time.Duration

	// mu 保护以下运行状态和切片；任何外部 HTTP I/O 都必须在释放锁后执行。
	mu              sync.RWMutex
	running         bool
	stopping        bool
	executing       bool
	cancel          context.CancelFunc
	workerDone      chan struct{}
	tasks           []HGTaskSnapshot
	recommendations []CrawlerPlatformPackage.HGRecommendation
	lastSuccessAt   time.Time
	lastError       string

	nextTaskID atomic.Int64 // 进程内单调任务 ID，不作为跨实例全局主键。
}

// NewHGManager 创建单 worker 管理器，避免同一进程内任务重入和无界堆积。
// interval 小于 10 秒时回落到 5 分钟，防止误配置形成第三方请求风暴；单轮超时最大 1 分钟。
func NewHGManager(platform CrawlerPlatformPackage.HGPlatform, interval, timeout time.Duration) (*HGManager, error) {
	if platform == nil {
		return nil, errors.New("crawler platform is required")
	}
	if interval < 10*time.Second {
		interval = 5 * time.Minute
	}
	if timeout <= 0 || timeout > time.Minute {
		timeout = 10 * time.Second
	}
	m := &HGManager{platform: platform, interval: interval, timeout: timeout}
	m.nextTaskID.Store(time.Now().UnixMilli())
	return m, nil
}

// Start 启动周期 worker；重复启动返回冲突错误。
// running 和 cancel 在启动 goroutine 前写入，确保并发 Stop 可以可靠取得取消函数。
func (m *HGManager) Start() error {
	m.mu.Lock()
	if m.running || m.stopping {
		m.mu.Unlock()
		return errors.New("spider is already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	m.running = true
	m.cancel = cancel
	// 每轮 worker 使用独立完成 channel。相比复用 WaitGroup，它允许旧 Stop 等待旧 channel 的同时，
	// 下一轮 Start 在旧 channel 关闭后安全创建新 worker，不存在 Add/Wait 并发复用窗口。
	m.workerDone = workerDone
	m.mu.Unlock()

	go m.loop(ctx, workerDone)
	return nil
}

// loop 启动后立即执行一次任务，再按固定周期串行执行。
// RunOnce 自身有不可重入保护，因此手动任务与周期 tick 同时发生时只会有一个真正访问上游。
func (m *HGManager) loop(ctx context.Context, workerDone chan struct{}) {
	defer func() {
		// 后台 goroutine 的 panic 不能扩散导致整个 crawler 进程退出；管理端会看到 lastError。
		if recover() != nil {
			m.mu.Lock()
			m.lastError = "crawler worker recovered from panic"
			m.mu.Unlock()
		}
		// Worker 退出是 RUNNING/STOPPING 到 STOPPED 的唯一状态提交点。
		// 状态和完成信号在同一临界区提交，确保 Start 看到 STOPPED 时旧 worker 已完成全部状态清理。
		m.mu.Lock()
		m.running = false
		m.stopping = false
		m.cancel = nil
		m.workerDone = nil
		close(workerDone)
		m.mu.Unlock()
	}()

	_, _ = m.RunOnce(ctx, HGCreateTaskRequest{Platform: m.platform.Name(), Type: "recommendation", Priority: 5})
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = m.RunOnce(ctx, HGCreateTaskRequest{Platform: m.platform.Name(), Type: "recommendation", Priority: 5})
		}
	}
}

// Stop 取消 worker 并等待当前请求在超时或取消后退出。
// 取消函数在锁外调用、Wait 也在锁外执行，避免 worker 收尾更新状态时发生死锁。
func (m *HGManager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	cancel := m.cancel
	workerDone := m.workerDone
	// running 在 worker 真正退出前保持 true，stopping 用于阻止并发 Start 创建第二个 worker。
	// 多个并发 Stop 只读取并等待同一个完成 channel；channel 仅由 worker 关闭，不会重复释放资源。
	m.stopping = true
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if workerDone != nil {
		<-workerDone
	}
}

// RunOnce 同步执行一次推荐抓取，适用于手动任务和 --once 命令。
// 状态流程：校验请求 -> 锁内登记 RUNNING -> 锁外抓取 -> 锁内更新 SUCCESS/FAILED 与最新快照。
func (m *HGManager) RunOnce(parent context.Context, req HGCreateTaskRequest) (task HGTaskSnapshot, runErr error) {
	if req.Platform == "" {
		req.Platform = m.platform.Name()
	}
	if req.Type == "" {
		req.Type = "recommendation"
	}
	if req.Platform != m.platform.Name() || req.Type != "recommendation" {
		return HGTaskSnapshot{}, errors.New("only bilibili recommendation tasks are supported")
	}
	if req.Priority < 1 || req.Priority > 10 {
		req.Priority = 5
	}

	m.mu.Lock()
	if m.executing {
		m.mu.Unlock()
		return HGTaskSnapshot{}, errors.New("a crawler task is already running")
	}
	m.executing = true
	// 新任务放在切片头部，使默认列表天然按创建时间倒序；超过上限时丢弃最旧记录。
	task = HGTaskSnapshot{ID: m.nextTaskID.Add(1), Type: req.Type, Platform: req.Platform, Status: "RUNNING", Priority: req.Priority, CreatedAt: time.Now().UTC(), StartedAt: time.Now().UTC()}
	m.tasks = append([]HGTaskSnapshot{task}, m.tasks...)
	if len(m.tasks) > hgMaxTaskHistory {
		m.tasks = m.tasks[:hgMaxTaskHistory]
	}
	m.mu.Unlock()

	// 外部 I/O 严格放在锁外，避免 Dashboard/Tasks 查询被第三方延迟阻塞。
	ctx, cancel := context.WithTimeout(parent, m.timeout)
	defer cancel()
	var items []CrawlerPlatformPackage.HGRecommendation
	defer func() {
		if recovered := recover(); recovered != nil {
			// 平台适配器属于可扩展边界，未来实现即使 panic，也必须把当前 RUNNING 任务收敛为 FAILED，
			// 同时释放 executing，避免整个采集进程永久拒绝后续任务。对外不暴露 panic 内容。
			runErr = errors.New("crawler task recovered from panic")
		}
		finishedAt := time.Now().UTC()
		task.FinishedAt = finishedAt
		task.CostMillis = finishedAt.Sub(task.StartedAt).Milliseconds()
		task.ItemCount = len(items)
		if runErr != nil {
			task.Status = "FAILED"
			task.Error = fmt.Sprintf("fetch failed: %v", runErr)
		} else {
			task.Status = "SUCCESS"
		}

		m.mu.Lock()
		m.executing = false
		for i := range m.tasks {
			if m.tasks[i].ID == task.ID {
				m.tasks[i] = task
				break
			}
		}
		if runErr != nil {
			m.lastError = task.Error
		} else {
			// 只有成功任务替换推荐快照；上游临时失败时保留上一份可用数据用于管理端降级展示。
			m.lastError = ""
			m.lastSuccessAt = finishedAt
			m.recommendations = append([]CrawlerPlatformPackage.HGRecommendation(nil), items...)
		}
		m.mu.Unlock()
	}()

	items, runErr = m.platform.FetchRecommendations(ctx)
	return task, runErr
}

// Spiders 返回管理端使用的平台 worker 状态。
// 返回新 map，不暴露内部可变字段；当前进程只管理一个 Bilibili worker，因此列表固定一项。
func (m *HGManager) Spiders() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status := "STOPPED"
	workers := 0
	if m.stopping {
		status = "STOPPING"
		workers = 1
	} else if m.running {
		status = "RUNNING"
		workers = 1
	}
	return []map[string]interface{}{{
		"id": "bilibili", "name": "Bilibili Recommendation Spider", "type": "video",
		"status": status, "workers": workers, "qpsLimit": 0.2, "intervalSeconds": int64(m.interval.Seconds()),
		"lastSuccessAt": m.lastSuccessAt, "lastError": m.lastError,
	}}
}

// Tasks 返回有界、稳定的任务分页快照。
// cursor 是过滤后切片下标，仅保证当前进程生命周期内有效；持久化版本应改用任务 ID 复合游标。
func (m *HGManager) Tasks(cursor, pageSize int, status string) map[string]interface{} {
	m.mu.RLock()
	// 锁内只复制切片头，过滤和分页在锁外完成，缩短 Dashboard/RunOnce 的锁竞争时间。
	tasks := append([]HGTaskSnapshot(nil), m.tasks...)
	m.mu.RUnlock()
	if status != "" {
		filtered := tasks[:0]
		for _, task := range tasks {
			if task.Status == status {
				filtered = append(filtered, task)
			}
		}
		tasks = filtered
	}
	if cursor < 0 {
		cursor = 0
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	if cursor > len(tasks) {
		cursor = len(tasks)
	}
	end := min(cursor+pageSize, len(tasks))
	nextCursor := 0
	hasMore := end < len(tasks)
	if hasMore {
		nextCursor = end
	}
	return map[string]interface{}{"list": tasks[cursor:end], "nextCursor": nextCursor, "hasMore": hasMore, "total": len(tasks)}
}

// Recommendations 返回最新一轮成功抓取结果副本，防止调用方修改 manager 内部切片。
func (m *HGManager) Recommendations() []CrawlerPlatformPackage.HGRecommendation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]CrawlerPlatformPackage.HGRecommendation(nil), m.recommendations...)
}

// Dashboard 返回实时指标、最近任务和最近 12 个任务趋势点。
// 所有聚合基于最多 500 条内存快照，避免每次请求执行数据库全表统计。
func (m *HGManager) Dashboard() map[string]interface{} {
	m.mu.RLock()
	tasks := append([]HGTaskSnapshot(nil), m.tasks...)
	running := m.running
	recommendationCount := len(m.recommendations)
	m.mu.RUnlock()

	var success, failed int
	for _, task := range tasks {
		if task.Status == "SUCCESS" {
			success++
		} else if task.Status == "FAILED" {
			failed++
		}
	}
	recent := tasks
	if len(recent) > 8 {
		recent = recent[:8]
	}
	trendTasks := append([]HGTaskSnapshot(nil), tasks...)
	sort.Slice(trendTasks, func(i, j int) bool { return trendTasks[i].CreatedAt.Before(trendTasks[j].CreatedAt) })
	if len(trendTasks) > 12 {
		trendTasks = trendTasks[len(trendTasks)-12:]
	}
	trend := make([]map[string]interface{}, 0, len(trendTasks))
	for _, task := range trendTasks {
		value := 0
		if task.Status == "SUCCESS" {
			value = task.ItemCount
		}
		trend = append(trend, map[string]interface{}{"time": task.CreatedAt.Format("15:04:05"), "value": value, "costMillis": task.CostMillis})
	}
	workers := 0
	if running {
		workers = 1
	}
	return map[string]interface{}{
		"stats": map[string]interface{}{"total": len(tasks), "success": success, "failed": failed, "workers": workers, "recommendations": recommendationCount},
		"tasks": recent, "trend": trend,
	}
}

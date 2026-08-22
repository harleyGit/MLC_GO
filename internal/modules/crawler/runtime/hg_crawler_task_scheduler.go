package runtime

import (
	CrawlerModelPackage "MLC_GO/internal/modules/crawler/model"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron"
)

// HGTaskSchedulerRepository provides the bounded enabled-definition snapshot used during refresh.
type HGTaskSchedulerRepository interface {
	ListEnabledTaskDefinitions(context.Context, int) ([]CrawlerModelPackage.HGTaskDefinition, error)
}

// HGTaskRunner executes one persisted definition; its implementation owns the per-task Redis lease.
type HGTaskRunner interface {
	RunByID(context.Context, uint64) (*CrawlerModelPackage.HGTaskRun, error)
}

// HGTaskScheduler dynamically rebuilds a UTC cron v1 schedule when the enabled task digest changes.
type HGTaskScheduler struct {
	repository      HGTaskSchedulerRepository
	runner          HGTaskRunner
	enabled         bool
	refreshInterval time.Duration
	maxTasks        int

	mu        sync.Mutex
	cron      *cron.Cron
	digest    string
	started   bool
	closed    bool
	ctx       context.Context
	cancel    context.CancelFunc
	waitGroup sync.WaitGroup
}

// NewHGTaskScheduler constructs a stopped scheduler. Start and Close are safe to call once through application lifecycle wiring.
func NewHGTaskScheduler(repository HGTaskSchedulerRepository, runner HGTaskRunner, enabled bool, refreshInterval time.Duration, maxTasks int) (*HGTaskScheduler, error) {
	if repository == nil || runner == nil || refreshInterval <= 0 || maxTasks < 1 || maxTasks > 500 {
		return nil, errors.New("crawler task scheduler dependencies or limits are invalid")
	}
	return &HGTaskScheduler{repository: repository, runner: runner, enabled: enabled, refreshInterval: refreshInterval, maxTasks: maxTasks}, nil
}

// Start loads the first snapshot before starting cron and the periodic refresh loop.
func (s *HGTaskScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("crawler task scheduler is closed")
	}
	if s.started || !s.enabled {
		s.started = true
		s.mu.Unlock()
		return nil
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.started = true
	s.mu.Unlock()

	if err := s.refresh(s.ctx); err != nil {
		s.mu.Lock()
		s.started = false
		s.cancel()
		s.mu.Unlock()
		return err
	}
	s.waitGroup.Add(1)
	go s.refreshLoop()
	return nil
}

// Close stops refresh work and waits for cron v1 jobs currently running in this process.
func (s *HGTaskScheduler) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	if s.cancel != nil {
		s.cancel()
	}
	current := s.cron
	s.cron = nil
	s.mu.Unlock()
	if current != nil {
		current.Stop()
	}
	s.waitGroup.Wait()
	return nil
}

func (s *HGTaskScheduler) refreshLoop() {
	defer s.waitGroup.Done()
	defer func() { _ = recover() }()
	ticker := time.NewTicker(s.refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			_ = s.refresh(s.ctx)
		}
	}
}

func (s *HGTaskScheduler) refresh(ctx context.Context) error {
	definitions, err := s.repository.ListEnabledTaskDefinitions(ctx, s.maxTasks)
	if err != nil {
		return fmt.Errorf("list enabled crawler tasks: %w", err)
	}
	digest := hgCrawlerTaskDigest(definitions)
	s.mu.Lock()
	if s.closed || digest == s.digest {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	next := cron.NewWithLocation(time.UTC)
	for _, definition := range definitions {
		if definition.ID == 0 || strings.TrimSpace(definition.Cron) == "" {
			return fmt.Errorf("enabled crawler task %d has no valid identity or cron", definition.ID)
		}
		taskID := definition.ID
		if err := next.AddFunc(definition.Cron, func() {
			defer func() { _ = recover() }()
			s.mu.Lock()
			if s.closed {
				s.mu.Unlock()
				return
			}
			s.waitGroup.Add(1)
			runCtx := s.ctx
			s.mu.Unlock()
			defer s.waitGroup.Done()
			_, _ = s.runner.RunByID(runCtx, taskID)
		}); err != nil {
			return fmt.Errorf("add crawler task %d cron: %w", taskID, err)
		}
	}
	next.Start()

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		next.Stop()
		return nil
	}
	previous := s.cron
	s.cron = next
	s.digest = digest
	s.mu.Unlock()
	if previous != nil {
		previous.Stop()
	}
	return nil
}

func hgCrawlerTaskDigest(definitions []CrawlerModelPackage.HGTaskDefinition) string {
	values := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		values = append(values, strconv.FormatUint(definition.ID, 10)+"\x00"+definition.Cron+"\x00"+strconv.FormatUint(definition.Version, 10))
	}
	sort.Strings(values)
	digest := sha256.Sum256([]byte(strings.Join(values, "\x01")))
	return hex.EncodeToString(digest[:])
}

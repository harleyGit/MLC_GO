package service

import (
	CrawlerDtoPackage "MLC_GO/internal/modules/crawler/dto"
	CrawlerLeasePackage "MLC_GO/internal/modules/crawler/lease"
	CrawlerModelPackage "MLC_GO/internal/modules/crawler/model"
	CrawlerParserPackage "MLC_GO/internal/modules/crawler/parser"
	CrawlerPlatformPackage "MLC_GO/internal/modules/crawler/platform"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/robfig/cron"
)

var (
	// ErrHGTaskInvalidDefinition indicates invalid scheduling, request, or parser configuration.
	ErrHGTaskInvalidDefinition = errors.New("crawler task definition is invalid")
	// ErrHGTaskLeaseNotAcquired indicates that another instance currently owns the task execution.
	ErrHGTaskLeaseNotAcquired = errors.New("crawler task lease was not acquired")
)

// HGTaskConfiguration is the immutable JSON snapshot used by one formal crawler execution.
type HGTaskConfiguration struct {
	Request CrawlerDtoPackage.HGDebugRequest `json:"request"`
	Parser  CrawlerParserPackage.Config      `json:"parser"`
}

// HGTaskRepository is the persistence surface required by configurable task management and execution.
type HGTaskRepository interface {
	SaveTaskDefinition(context.Context, *CrawlerModelPackage.HGTaskDefinition) error
	GetTaskDefinitionByID(context.Context, uint64) (*CrawlerModelPackage.HGTaskDefinition, error)
	ListTaskDefinitions(context.Context, uint64, int) ([]CrawlerModelPackage.HGTaskDefinition, uint64, bool, error)
	CreateTaskRun(context.Context, *CrawlerModelPackage.HGTaskRun) error
	CompleteTaskRun(context.Context, *CrawlerModelPackage.HGTaskRun) error
}

// HGRecommendationStore persists parsed external content and maintains its read-side caches.
type HGRecommendationStore interface {
	UpsertRecommendations(context.Context, []CrawlerPlatformPackage.HGRecommendation) error
}

// HGTaskHTTPExecutor returns the bounded raw upstream response used by the parser.
type HGTaskHTTPExecutor interface {
	ValidateRequest(CrawlerDtoPackage.HGDebugRequest) error
	Execute(context.Context, CrawlerDtoPackage.HGDebugRequest) (HGHTTPResult, error)
}

// HGTaskListResult is a stable cursor page of persisted task definitions.
type HGTaskListResult struct {
	Items      []CrawlerModelPackage.HGTaskDefinition
	NextCursor uint64
	HasMore    bool
}

// HGTaskService validates, persists, leases, executes, parses, and records configurable crawler tasks.
type HGTaskService struct {
	repository HGTaskRepository
	store      HGRecommendationStore
	http       HGTaskHTTPExecutor
	lease      CrawlerLeasePackage.HGTaskLease
	leaseGrace time.Duration
}

// NewHGTaskService creates the task runtime without starting any scheduler or application lifecycle hooks.
func NewHGTaskService(repository HGTaskRepository, httpExecutor HGTaskHTTPExecutor, taskLease CrawlerLeasePackage.HGTaskLease, leaseGrace time.Duration, stores ...HGRecommendationStore) (*HGTaskService, error) {
	if repository == nil || httpExecutor == nil || taskLease == nil || leaseGrace <= 0 {
		return nil, errors.New("crawler task service dependencies are required")
	}
	var store HGRecommendationStore
	if len(stores) > 0 {
		store = stores[0]
	}
	if store == nil {
		if repositoryStore, ok := repository.(HGRecommendationStore); ok {
			store = repositoryStore
		}
	}
	if store == nil {
		return nil, errors.New("crawler recommendation store is required")
	}
	return &HGTaskService{repository: repository, store: store, http: httpExecutor, lease: taskLease, leaseGrace: leaseGrace}, nil
}

// Save validates and persists a definition without executing it.
func (s *HGTaskService) Save(ctx context.Context, req CrawlerDtoPackage.HGTaskDefinitionSaveRequest, actor string) (*CrawlerModelPackage.HGTaskDefinition, error) {
	configuration, err := s.validateSaveRequest(req)
	if err != nil {
		return nil, err
	}
	normalizedConfiguration, err := json.Marshal(configuration)
	if err != nil {
		return nil, fmt.Errorf("encode crawler task configuration: %w", err)
	}
	actor = strings.TrimSpace(actor)
	definition := &CrawlerModelPackage.HGTaskDefinition{
		ID: req.ID, Name: strings.TrimSpace(req.Name), Platform: strings.TrimSpace(req.Platform), Enabled: req.Enabled,
		Cron: strings.TrimSpace(req.Cron), ParserType: string(configuration.Parser.Type), ItemPath: configuration.Parser.ItemSelector,
		MaxItems: req.MaxItems, Configuration: normalizedConfiguration, Version: req.Version,
		CreatedBy: actor, UpdatedBy: actor,
	}
	if err := s.repository.SaveTaskDefinition(ctx, definition); err != nil {
		return nil, fmt.Errorf("save crawler task definition: %w", err)
	}
	return definition, nil
}

// List returns a bounded primary-key cursor page.
func (s *HGTaskService) List(ctx context.Context, req CrawlerDtoPackage.HGTaskDefinitionListRequest) (HGTaskListResult, error) {
	items, nextCursor, hasMore, err := s.repository.ListTaskDefinitions(ctx, req.Cursor, req.Limit)
	if err != nil {
		return HGTaskListResult{}, fmt.Errorf("list crawler task definitions: %w", err)
	}
	return HGTaskListResult{Items: items, NextCursor: nextCursor, HasMore: hasMore}, nil
}

// SaveAndRun persists the definition first, then executes its saved snapshot. Execution failure never rolls back the definition.
func (s *HGTaskService) SaveAndRun(ctx context.Context, req CrawlerDtoPackage.HGTaskDefinitionSaveRequest, actor string) (*CrawlerModelPackage.HGTaskDefinition, *CrawlerModelPackage.HGTaskRun, error) {
	definition, err := s.Save(ctx, req, actor)
	if err != nil {
		return nil, nil, err
	}
	run, runErr := s.RunByID(ctx, definition.ID)
	return definition, run, runErr
}

// RunByID obtains a per-task lease, creates a run snapshot, and completes it for success or failure.
func (s *HGTaskService) RunByID(ctx context.Context, taskID uint64) (*CrawlerModelPackage.HGTaskRun, error) {
	if taskID == 0 {
		return nil, fmt.Errorf("%w: task id is required", ErrHGTaskInvalidDefinition)
	}
	definition, err := s.repository.GetTaskDefinitionByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get crawler task definition: %w", err)
	}
	configuration, timeout, err := s.validateDefinition(definition)
	if err != nil {
		return nil, err
	}
	token, acquired, err := s.lease.Acquire(ctx, taskID, timeout+s.leaseGrace)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, ErrHGTaskLeaseNotAcquired
	}
	defer func() { _ = s.lease.Release(context.WithoutCancel(ctx), taskID, token) }()

	run := &CrawlerModelPackage.HGTaskRun{TaskDefinitionID: taskID, Status: "running", Configuration: append(json.RawMessage(nil), definition.Configuration...), StartedAt: time.Now().UTC()}
	if err := s.repository.CreateTaskRun(ctx, run); err != nil {
		return run, fmt.Errorf("create crawler task run: %w", err)
	}
	itemCount, executionErr := s.execute(ctx, definition, configuration)
	run.ItemCount = uint32(itemCount)
	run.FinishedAt = hgCrawlerTimePointer(time.Now().UTC())
	if executionErr != nil {
		run.Status = "failed"
		run.ErrorMessage = hgCrawlerBoundError(executionErr)
	} else {
		run.Status = "succeeded"
	}
	completionCtx, cancelCompletion := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	completionErr := s.repository.CompleteTaskRun(completionCtx, run)
	cancelCompletion()
	if executionErr != nil && completionErr != nil {
		return run, errors.Join(executionErr, fmt.Errorf("complete crawler task run: %w", completionErr))
	}
	if completionErr != nil {
		return run, fmt.Errorf("complete crawler task run: %w", completionErr)
	}
	if executionErr != nil {
		return run, executionErr
	}
	return run, nil
}

func (s *HGTaskService) validateSaveRequest(req CrawlerDtoPackage.HGTaskDefinitionSaveRequest) (HGTaskConfiguration, error) {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Platform) == "" || req.MaxItems < 1 || req.MaxItems > 50 {
		return HGTaskConfiguration{}, fmt.Errorf("%w: name, platform, and maxItems are invalid", ErrHGTaskInvalidDefinition)
	}
	cronSpec := strings.TrimSpace(req.Cron)
	if req.Enabled && cronSpec == "" {
		return HGTaskConfiguration{}, fmt.Errorf("%w: enabled task cron is required", ErrHGTaskInvalidDefinition)
	}
	if cronSpec != "" {
		if _, err := cron.Parse(cronSpec); err != nil {
			return HGTaskConfiguration{}, fmt.Errorf("%w: invalid cron: %v", ErrHGTaskInvalidDefinition, err)
		}
	}
	configuration, _, err := hgDecodeTaskConfiguration(req.Configuration)
	if err != nil {
		return HGTaskConfiguration{}, err
	}
	if strings.TrimSpace(configuration.Parser.Platform) == "" {
		configuration.Parser.Platform = strings.TrimSpace(req.Platform)
	}
	if configuration.Parser.Platform != strings.TrimSpace(req.Platform) || string(configuration.Parser.Type) != strings.TrimSpace(req.ParserType) || configuration.Parser.ItemSelector != strings.TrimSpace(req.ItemPath) {
		return HGTaskConfiguration{}, fmt.Errorf("%w: scalar parser fields do not match configuration", ErrHGTaskInvalidDefinition)
	}
	if err := CrawlerParserPackage.Validate(configuration.Parser); err != nil {
		return HGTaskConfiguration{}, fmt.Errorf("%w: %v", ErrHGTaskInvalidDefinition, err)
	}
	if err := s.http.ValidateRequest(configuration.Request); err != nil {
		return HGTaskConfiguration{}, fmt.Errorf("%w: %v", ErrHGTaskInvalidDefinition, err)
	}
	return configuration, nil
}

func (s *HGTaskService) validateDefinition(definition *CrawlerModelPackage.HGTaskDefinition) (HGTaskConfiguration, time.Duration, error) {
	if definition == nil || definition.ID == 0 || definition.MaxItems < 1 || definition.MaxItems > 50 {
		return HGTaskConfiguration{}, 0, fmt.Errorf("%w: persisted definition is invalid", ErrHGTaskInvalidDefinition)
	}
	configuration, timeout, err := hgDecodeTaskConfiguration(definition.Configuration)
	if err != nil {
		return HGTaskConfiguration{}, 0, err
	}
	if configuration.Parser.Platform == "" {
		configuration.Parser.Platform = definition.Platform
	}
	if configuration.Parser.Platform != definition.Platform || string(configuration.Parser.Type) != definition.ParserType || configuration.Parser.ItemSelector != definition.ItemPath {
		return HGTaskConfiguration{}, 0, fmt.Errorf("%w: persisted scalar parser fields do not match configuration", ErrHGTaskInvalidDefinition)
	}
	if err := CrawlerParserPackage.Validate(configuration.Parser); err != nil {
		return HGTaskConfiguration{}, 0, fmt.Errorf("%w: %v", ErrHGTaskInvalidDefinition, err)
	}
	if err := s.http.ValidateRequest(configuration.Request); err != nil {
		return HGTaskConfiguration{}, 0, fmt.Errorf("%w: %v", ErrHGTaskInvalidDefinition, err)
	}
	return configuration, timeout, nil
}

func (s *HGTaskService) execute(ctx context.Context, definition *CrawlerModelPackage.HGTaskDefinition, configuration HGTaskConfiguration) (int, error) {
	response, err := s.http.Execute(ctx, configuration.Request)
	if err != nil {
		return 0, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return 0, fmt.Errorf("crawler target returned status %d", response.StatusCode)
	}
	items, err := CrawlerParserPackage.Parse(configuration.Parser, response.URL, response.Body)
	if err != nil {
		return 0, fmt.Errorf("parse crawler response: %w", err)
	}
	if len(items) > int(definition.MaxItems) {
		items = items[:definition.MaxItems]
	}
	if len(items) == 0 {
		return 0, errors.New("crawler parser returned no valid items")
	}
	if err := s.store.UpsertRecommendations(ctx, items); err != nil {
		return 0, fmt.Errorf("upsert crawler recommendations: %w", err)
	}
	return len(items), nil
}

func hgDecodeTaskConfiguration(raw json.RawMessage) (HGTaskConfiguration, time.Duration, error) {
	var configuration HGTaskConfiguration
	if !json.Valid(raw) || len(raw) > 128<<10 {
		return configuration, 0, fmt.Errorf("%w: configuration JSON is invalid", ErrHGTaskInvalidDefinition)
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&configuration); err != nil {
		return configuration, 0, fmt.Errorf("%w: decode configuration: %v", ErrHGTaskInvalidDefinition, err)
	}
	timeoutMS := configuration.Request.TimeoutMS
	if timeoutMS == 0 {
		timeoutMS = 10000
	}
	if timeoutMS < 500 || timeoutMS > 10000 {
		return configuration, 0, fmt.Errorf("%w: request timeoutMs must be 500-10000", ErrHGTaskInvalidDefinition)
	}
	if strings.TrimSpace(configuration.Request.URL) == "" {
		return configuration, 0, fmt.Errorf("%w: request URL is required", ErrHGTaskInvalidDefinition)
	}
	return configuration, time.Duration(timeoutMS) * time.Millisecond, nil
}

func hgCrawlerBoundError(err error) string {
	message := err.Error()
	if len(message) > 2048 {
		return message[:2048]
	}
	return message
}

func hgCrawlerTimePointer(value time.Time) *time.Time { return &value }

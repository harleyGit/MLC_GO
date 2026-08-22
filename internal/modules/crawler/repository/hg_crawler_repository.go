package repository

import (
	CrawlerModelPackage "MLC_GO/internal/modules/crawler/model"
	CrawlerPlatformPackage "MLC_GO/internal/modules/crawler/platform"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	hgMaxRecommendationBatchSize = 50
	hgDefaultTaskListLimit       = 20
	hgMaxTaskListLimit           = 100
	hgDefaultTaskContentLimit    = 20
	hgMaxTaskContentLimit        = 100
	hgMaxEnabledTaskListLimit    = 500
)

var (
	// ErrHGTaskDefinitionNotFound indicates that an ID lookup did not match a persisted definition.
	ErrHGTaskDefinitionNotFound = errors.New("crawler task definition not found")
	// ErrHGTaskDefinitionVersionConflict indicates a stale optimistic version or a missing update target.
	ErrHGTaskDefinitionVersionConflict = errors.New("crawler task definition version conflict")
	// ErrHGTaskRunStateConflict indicates that a run is no longer in the running state.
	ErrHGTaskRunStateConflict = errors.New("crawler task run state conflict")
)

// Repository 持久化第三方公开内容元数据，不写入本站投稿和媒体文件表。
type Repository struct {
	db *sql.DB
}

// NewRepository 创建 crawler 外部内容仓储。
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// SaveTaskDefinition inserts a new definition or updates one row using its optimistic version.
func (r *Repository) SaveTaskDefinition(ctx context.Context, definition *CrawlerModelPackage.HGTaskDefinition) error {
	if r == nil || r.db == nil {
		return errors.New("crawler repository database is required")
	}
	if definition == nil {
		return errors.New("crawler task definition is required")
	}
	if err := hgValidateTaskDefinition(definition); err != nil {
		return err
	}
	if definition.ID == 0 {
		result, err := r.db.ExecContext(ctx, SQLQueriesPackage.InsertCrawlerTaskDefinitionSQL,
			definition.Name, definition.Platform, definition.Enabled, definition.Cron, definition.ParserType,
			definition.ItemPath, definition.MaxItems, []byte(definition.Configuration), definition.CreatedBy, definition.UpdatedBy,
		)
		if err != nil {
			return fmt.Errorf("inserting crawler task definition: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("reading crawler task definition insert id: %w", err)
		}
		definition.ID = uint64(id)
		definition.Version = 1
		return nil
	}
	if definition.Version == 0 {
		return errors.New("crawler task definition version is required for update")
	}
	result, err := r.db.ExecContext(ctx, SQLQueriesPackage.UpdateCrawlerTaskDefinitionSQL,
		definition.Name, definition.Platform, definition.Enabled, definition.Cron, definition.ParserType,
		definition.ItemPath, definition.MaxItems, []byte(definition.Configuration), definition.UpdatedBy,
		definition.ID, definition.Version,
	)
	if err != nil {
		return fmt.Errorf("updating crawler task definition: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("reading crawler task definition update result: %w", err)
	}
	if affected != 1 {
		return ErrHGTaskDefinitionVersionConflict
	}
	definition.Version++
	return nil
}

// GetTaskDefinitionByID performs a primary-key lookup for one task definition.
func (r *Repository) GetTaskDefinitionByID(ctx context.Context, id uint64) (*CrawlerModelPackage.HGTaskDefinition, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("crawler repository database is required")
	}
	definition, err := hgScanTaskDefinition(r.db.QueryRowContext(ctx, SQLQueriesPackage.GetCrawlerTaskDefinitionByIDSQL, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrHGTaskDefinitionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting crawler task definition: %w", err)
	}
	return &definition, nil
}

// ListTaskDefinitions returns a bounded primary-key cursor page and the next cursor.
func (r *Repository) ListTaskDefinitions(ctx context.Context, cursor uint64, limit int) ([]CrawlerModelPackage.HGTaskDefinition, uint64, bool, error) {
	if r == nil || r.db == nil {
		return nil, 0, false, errors.New("crawler repository database is required")
	}
	limit = hgBoundTaskListLimit(limit, hgMaxTaskListLimit)
	definitions, err := r.hgQueryTaskDefinitions(ctx, SQLQueriesPackage.ListCrawlerTaskDefinitionsSQL, cursor, limit+1)
	if err != nil {
		return nil, 0, false, err
	}
	hasMore := len(definitions) > limit
	if hasMore {
		definitions = definitions[:limit]
	}
	nextCursor := uint64(0)
	if len(definitions) > 0 {
		nextCursor = definitions[len(definitions)-1].ID
	}
	return definitions, nextCursor, hasMore, nil
}

// ListTaskExternalContents returns a bounded newest-first association cursor page for one task.
func (r *Repository) ListTaskExternalContents(ctx context.Context, taskID, cursor uint64, limit int) ([]CrawlerModelPackage.HGTaskExternalContent, uint64, bool, error) {
	if r == nil || r.db == nil {
		return nil, 0, false, errors.New("crawler repository database is required")
	}
	if taskID == 0 {
		return nil, 0, false, errors.New("crawler task definition id is required")
	}
	limit = hgBoundListLimit(limit, hgDefaultTaskContentLimit, hgMaxTaskContentLimit)
	query := SQLQueriesPackage.ListCrawlerTaskExternalContentsFirstSQL
	args := []any{taskID, limit + 1}
	if cursor > 0 {
		query = SQLQueriesPackage.ListCrawlerTaskExternalContentsByCursorSQL
		args = []any{taskID, cursor, limit + 1}
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, false, fmt.Errorf("listing crawler task external contents: %w", err)
	}
	defer rows.Close()
	items := make([]CrawlerModelPackage.HGTaskExternalContent, 0, limit+1)
	for rows.Next() {
		item, err := hgScanTaskExternalContent(rows)
		if err != nil {
			return nil, 0, false, fmt.Errorf("scanning crawler task external content: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, false, fmt.Errorf("iterating crawler task external contents: %w", err)
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	nextCursor := uint64(0)
	if len(items) > 0 {
		nextCursor = items[len(items)-1].AssociationID
	}
	return items, nextCursor, hasMore, nil
}

// ListEnabledTaskDefinitions returns an explicitly bounded scheduler snapshot using (enabled,id).
func (r *Repository) ListEnabledTaskDefinitions(ctx context.Context, limit int) ([]CrawlerModelPackage.HGTaskDefinition, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("crawler repository database is required")
	}
	limit = hgBoundTaskListLimit(limit, hgMaxEnabledTaskListLimit)
	return r.hgQueryTaskDefinitions(ctx, SQLQueriesPackage.ListEnabledCrawlerTaskDefinitionsSQL, limit)
}

// CreateTaskRun inserts a running execution with the exact configuration snapshot used by the worker.
func (r *Repository) CreateTaskRun(ctx context.Context, run *CrawlerModelPackage.HGTaskRun) error {
	if r == nil || r.db == nil {
		return errors.New("crawler repository database is required")
	}
	if run == nil || run.TaskDefinitionID == 0 || !json.Valid(run.Configuration) {
		return errors.New("valid crawler task run and configuration are required")
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now().UTC()
	}
	if run.Status == "" {
		run.Status = "running"
	}
	if run.Status != "running" {
		return errors.New("new crawler task run status must be running")
	}
	result, err := r.db.ExecContext(ctx, SQLQueriesPackage.InsertCrawlerTaskRunSQL,
		run.TaskDefinitionID, run.Status, []byte(run.Configuration), run.StartedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("creating crawler task run: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("reading crawler task run insert id: %w", err)
	}
	run.ID = uint64(id)
	return nil
}

// CompleteTaskRun atomically completes one running row and updates the definition's latest-run summary.
func (r *Repository) CompleteTaskRun(ctx context.Context, run *CrawlerModelPackage.HGTaskRun) error {
	if r == nil || r.db == nil {
		return errors.New("crawler repository database is required")
	}
	if run == nil || run.ID == 0 || run.TaskDefinitionID == 0 || run.StartedAt.IsZero() {
		return errors.New("crawler task run identity and start time are required")
	}
	if run.Status == "" || run.Status == "running" {
		return errors.New("crawler task run terminal status is required")
	}
	finishedAt := time.Now().UTC()
	if run.FinishedAt != nil {
		finishedAt = run.FinishedAt.UTC()
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("beginning crawler task run completion: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.CompleteCrawlerTaskRunSQL,
		run.Status, finishedAt, run.ItemCount, run.ErrorMessage, run.ID, run.TaskDefinitionID,
	)
	if err != nil {
		return fmt.Errorf("completing crawler task run: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("reading crawler task run completion result: %w", err)
	}
	if affected != 1 {
		return ErrHGTaskRunStateConflict
	}
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.UpdateCrawlerTaskDefinitionLastRunSQL,
		run.ID, run.Status, run.StartedAt.UTC(), finishedAt, run.ItemCount, run.ErrorMessage,
		run.TaskDefinitionID, run.ID,
	); err != nil {
		return fmt.Errorf("updating crawler task definition last-run summary: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing crawler task run completion: %w", err)
	}
	run.FinishedAt = &finishedAt
	return nil
}

// UpsertRecommendations 保持 HGRecommendationStore 的兼容签名。
func (r *Repository) UpsertRecommendations(ctx context.Context, items []CrawlerPlatformPackage.HGRecommendation) error {
	_, err := r.UpsertRecommendationsWithInserted(ctx, items)
	return err
}

// UpsertTaskRecommendations preserves the task service store interface while associating the committed batch.
func (r *Repository) UpsertTaskRecommendations(ctx context.Context, taskID, runID uint64, items []CrawlerPlatformPackage.HGRecommendation) error {
	_, err := r.UpsertTaskRecommendationsWithInserted(ctx, taskID, runID, items)
	return err
}

// UpsertRecommendationsWithInserted 在短事务中返回本批次实际新增的外部内容数。
// 第一条有界 insert-noop 仅补缺并计数，第二条 full upsert 刷新全部字段；唯一键 (platform, content_id) 限制锁范围。
func (r *Repository) UpsertRecommendationsWithInserted(ctx context.Context, items []CrawlerPlatformPackage.HGRecommendation) (int64, error) {
	return r.hgUpsertRecommendations(ctx, 0, 0, items)
}

// UpsertTaskRecommendationsWithInserted commits the global upsert and task associations in one short transaction.
func (r *Repository) UpsertTaskRecommendationsWithInserted(ctx context.Context, taskID, runID uint64, items []CrawlerPlatformPackage.HGRecommendation) (int64, error) {
	if taskID == 0 || runID == 0 {
		return 0, errors.New("crawler task and run ids are required")
	}
	return r.hgUpsertRecommendations(ctx, taskID, runID, items)
}

func (r *Repository) hgUpsertRecommendations(ctx context.Context, taskID, runID uint64, items []CrawlerPlatformPackage.HGRecommendation) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("crawler repository database is required")
	}
	if len(items) == 0 {
		return 0, nil
	}
	if len(items) > hgMaxRecommendationBatchSize {
		return 0, fmt.Errorf("crawler recommendation batch exceeds %d items", hgMaxRecommendationBatchSize)
	}

	values := make([]string, 0, len(items))
	args := make([]any, 0, len(items)*13)
	seenAt := time.Now().UTC()
	for _, item := range items {
		if err := hgValidateRecommendation(item); err != nil {
			return 0, err
		}
		values = append(values, SQLQueriesPackage.UpsertCrawlerExternalContentsValue)
		args = append(args,
			item.Platform, item.ContentID, item.Title, item.AuthorID, item.AuthorName,
			item.CoverURL, item.TargetURL, item.Duration, item.ViewCount, item.LikeCount,
			item.CommentCount, nullableTime(item.PublishedAt), seenAt,
		)
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, fmt.Errorf("beginning crawler recommendation upsert: %w", err)
	}
	defer tx.Rollback()

	valuesSQL := strings.Join(values, ",")
	insertNoopQuery := SQLQueriesPackage.UpsertCrawlerExternalContentsPrefix + valuesSQL + SQLQueriesPackage.InsertCrawlerExternalContentsNoopSuffix
	result, err := tx.ExecContext(ctx, insertNoopQuery, args...)
	if err != nil {
		return 0, fmt.Errorf("inserting new crawler recommendations: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("reading inserted crawler recommendation count: %w", err)
	}

	upsertQuery := SQLQueriesPackage.UpsertCrawlerExternalContentsPrefix + valuesSQL + SQLQueriesPackage.UpsertCrawlerExternalContentsSuffix
	if _, err := tx.ExecContext(ctx, upsertQuery, args...); err != nil {
		return 0, fmt.Errorf("upserting crawler recommendations: %w", err)
	}
	if taskID > 0 {
		associationKeys := make([]string, 0, len(items))
		associationArgs := make([]any, 0, 2+len(items)*2)
		associationArgs = append(associationArgs, taskID, runID)
		for _, item := range items {
			associationKeys = append(associationKeys, SQLQueriesPackage.UpsertCrawlerTaskExternalContentsKey)
			associationArgs = append(associationArgs, item.Platform, item.ContentID)
		}
		associationQuery := SQLQueriesPackage.UpsertCrawlerTaskExternalContentsPrefix + strings.Join(associationKeys, " OR ") + SQLQueriesPackage.UpsertCrawlerTaskExternalContentsSuffix
		if _, err := tx.ExecContext(ctx, associationQuery, associationArgs...); err != nil {
			return 0, fmt.Errorf("associating crawler task recommendations: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing crawler recommendation upsert: %w", err)
	}
	return inserted, nil
}

func hgValidateRecommendation(item CrawlerPlatformPackage.HGRecommendation) error {
	if strings.TrimSpace(item.Platform) == "" || strings.TrimSpace(item.ContentID) == "" || strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.TargetURL) == "" {
		return errors.New("crawler recommendation required fields are missing")
	}
	if len(item.Platform) > 32 || len(item.ContentID) > 128 || len(item.Title) > 255 || len(item.AuthorID) > 128 || len(item.AuthorName) > 255 || len(item.CoverURL) > 1024 || len(item.TargetURL) > 2048 {
		return errors.New("crawler recommendation field exceeds storage limit")
	}
	if item.Duration < 0 || item.Duration > int64(^uint32(0)) || item.ViewCount < 0 || item.LikeCount < 0 || item.CommentCount < 0 {
		return errors.New("crawler recommendation counters must be non-negative")
	}
	return nil
}

func hgValidateTaskDefinition(definition *CrawlerModelPackage.HGTaskDefinition) error {
	definition.Name = strings.TrimSpace(definition.Name)
	definition.Platform = strings.TrimSpace(definition.Platform)
	definition.Cron = strings.TrimSpace(definition.Cron)
	definition.ParserType = strings.TrimSpace(definition.ParserType)
	definition.ItemPath = strings.TrimSpace(definition.ItemPath)
	definition.CreatedBy = strings.TrimSpace(definition.CreatedBy)
	definition.UpdatedBy = strings.TrimSpace(definition.UpdatedBy)
	if definition.Name == "" || definition.Platform == "" || definition.ParserType == "" || !json.Valid(definition.Configuration) {
		return errors.New("crawler task definition required fields or configuration are invalid")
	}
	if len(definition.Name) > 128 || len(definition.Platform) > 32 || len(definition.Cron) > 128 ||
		len(definition.ParserType) > 32 || len(definition.ItemPath) > 512 || len(definition.CreatedBy) > 64 || len(definition.UpdatedBy) > 64 {
		return errors.New("crawler task definition field exceeds storage limit")
	}
	return nil
}

func hgBoundTaskListLimit(limit, maximum int) int {
	return hgBoundListLimit(limit, hgDefaultTaskListLimit, maximum)
}

func hgBoundListLimit(limit, defaultLimit, maximum int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maximum {
		return maximum
	}
	return limit
}

func hgScanTaskExternalContent(scanner hgTaskDefinitionScanner) (CrawlerModelPackage.HGTaskExternalContent, error) {
	var item CrawlerModelPackage.HGTaskExternalContent
	var publishedAt sql.NullTime
	if err := scanner.Scan(
		&item.AssociationID, &item.TaskDefinitionID, &item.LastRunID,
		&item.ExternalContentID, &item.Platform, &item.ContentID, &item.Title, &item.AuthorID,
		&item.AuthorName, &item.CoverURL, &item.TargetURL, &item.DurationSeconds, &item.ViewCount,
		&item.LikeCount, &item.CommentCount, &publishedAt, &item.FirstSeenAt, &item.LastSeenAt,
		&item.ContentCreatedAt, &item.ContentUpdatedAt, &item.AssociatedAt, &item.AssociationUpdatedAt,
	); err != nil {
		return item, err
	}
	if publishedAt.Valid {
		value := publishedAt.Time.UTC()
		item.PublishedAt = &value
	}
	item.FirstSeenAt = item.FirstSeenAt.UTC()
	item.LastSeenAt = item.LastSeenAt.UTC()
	item.ContentCreatedAt = item.ContentCreatedAt.UTC()
	item.ContentUpdatedAt = item.ContentUpdatedAt.UTC()
	item.AssociatedAt = item.AssociatedAt.UTC()
	item.AssociationUpdatedAt = item.AssociationUpdatedAt.UTC()
	return item, nil
}

func (r *Repository) hgQueryTaskDefinitions(ctx context.Context, query string, args ...any) ([]CrawlerModelPackage.HGTaskDefinition, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing crawler task definitions: %w", err)
	}
	defer rows.Close()
	definitions := make([]CrawlerModelPackage.HGTaskDefinition, 0)
	for rows.Next() {
		definition, err := hgScanTaskDefinition(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning crawler task definition: %w", err)
		}
		definitions = append(definitions, definition)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating crawler task definitions: %w", err)
	}
	return definitions, nil
}

type hgTaskDefinitionScanner interface {
	Scan(...any) error
}

func hgScanTaskDefinition(scanner hgTaskDefinitionScanner) (CrawlerModelPackage.HGTaskDefinition, error) {
	var definition CrawlerModelPackage.HGTaskDefinition
	var configuration []byte
	var lastRunID sql.Null[uint64]
	var lastRunStartedAt, lastRunFinishedAt sql.NullTime
	if err := scanner.Scan(
		&definition.ID, &definition.Name, &definition.Platform, &definition.Enabled, &definition.Cron,
		&definition.ParserType, &definition.ItemPath, &definition.MaxItems, &configuration,
		&lastRunID, &definition.LastRunStatus, &lastRunStartedAt, &lastRunFinishedAt,
		&definition.LastRunItemCount, &definition.LastRunError, &definition.Version,
		&definition.CreatedBy, &definition.UpdatedBy, &definition.CreatedAt, &definition.UpdatedAt,
	); err != nil {
		return definition, err
	}
	definition.Configuration = append(definition.Configuration[:0], configuration...)
	if lastRunID.Valid {
		definition.LastRunID = lastRunID.V
	}
	if lastRunStartedAt.Valid {
		value := lastRunStartedAt.Time.UTC()
		definition.LastRunStartedAt = &value
	}
	if lastRunFinishedAt.Valid {
		value := lastRunFinishedAt.Time.UTC()
		definition.LastRunFinishedAt = &value
	}
	definition.CreatedAt = definition.CreatedAt.UTC()
	definition.UpdatedAt = definition.UpdatedAt.UTC()
	return definition, nil
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}

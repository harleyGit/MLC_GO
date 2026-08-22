package repository

import (
	CrawlerModelPackage "MLC_GO/internal/modules/crawler/model"
	CrawlerPlatformPackage "MLC_GO/internal/modules/crawler/platform"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSaveTaskDefinitionInsertAndOptimisticUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewRepository(db)
	definition := &CrawlerModelPackage.HGTaskDefinition{
		Name: "recommendations", Platform: "bilibili", Enabled: true, Cron: "*/5 * * * *",
		ParserType: "json", ItemPath: "data.items", MaxItems: 50, Configuration: []byte(`{"url":"https://example.test"}`),
		CreatedBy: "admin", UpdatedBy: "admin",
	}
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCrawlerTaskDefinitionSQL)).
		WithArgs(definition.Name, definition.Platform, true, definition.Cron, definition.ParserType, definition.ItemPath, definition.MaxItems, []byte(definition.Configuration), "admin", "admin").
		WillReturnResult(sqlmock.NewResult(41, 1))
	if err := repo.SaveTaskDefinition(context.Background(), definition); err != nil {
		t.Fatalf("SaveTaskDefinition(insert) error = %v", err)
	}
	if definition.ID != 41 || definition.Version != 1 {
		t.Fatalf("insert result = id %d version %d", definition.ID, definition.Version)
	}
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpdateCrawlerTaskDefinitionSQL)).
		WithArgs(definition.Name, definition.Platform, true, definition.Cron, definition.ParserType, definition.ItemPath, definition.MaxItems, []byte(definition.Configuration), "admin", uint64(41), uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := repo.SaveTaskDefinition(context.Background(), definition); err != nil {
		t.Fatalf("SaveTaskDefinition(update) error = %v", err)
	}
	if definition.Version != 2 {
		t.Fatalf("version = %d, want 2", definition.Version)
	}
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpdateCrawlerTaskDefinitionSQL)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	if err := repo.SaveTaskDefinition(context.Background(), definition); !errors.Is(err, ErrHGTaskDefinitionVersionConflict) {
		t.Fatalf("stale update error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListTaskDefinitionsUsesIDCursorAndBoundedLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	columns := hgTaskDefinitionColumns()
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows(columns).
		AddRow(11, "one", "bilibili", true, "* * * * *", "json", "data", 10, []byte(`{}`), nil, "", nil, nil, 0, "", 1, "a", "a", now, now).
		AddRow(12, "two", "bilibili", true, "* * * * *", "json", "data", 10, []byte(`{}`), nil, "", nil, nil, 0, "", 1, "a", "a", now, now)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.ListCrawlerTaskDefinitionsSQL)).WithArgs(uint64(10), 2).WillReturnRows(rows)
	items, next, more, err := NewRepository(db).ListTaskDefinitions(context.Background(), 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != 11 || next != 11 || !more {
		t.Fatalf("page = %#v next=%d more=%v", items, next, more)
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.ListEnabledCrawlerTaskDefinitionsSQL)).WithArgs(hgMaxEnabledTaskListLimit).WillReturnRows(sqlmock.NewRows(columns))
	if _, err := NewRepository(db).ListEnabledTaskDefinitions(context.Background(), hgMaxEnabledTaskListLimit+1); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetTaskDefinitionByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.GetCrawlerTaskDefinitionByIDSQL)).WithArgs(uint64(7)).
		WillReturnRows(sqlmock.NewRows(hgTaskDefinitionColumns()).
			AddRow(7, "task", "bilibili", true, "* * * * *", "json", "data", 10, []byte(`{"page":1}`), 15, "succeeded", now, now, 3, "", 4, "a", "b", now, now))
	definition, err := NewRepository(db).GetTaskDefinitionByID(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if definition.ID != 7 || definition.LastRunID != 15 || definition.Version != 4 {
		t.Fatalf("definition = %#v", definition)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateAndCompleteTaskRunUpdatesSummaryTransactionally(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewRepository(db)
	startedAt := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	run := &CrawlerModelPackage.HGTaskRun{TaskDefinitionID: 7, Configuration: []byte(`{"page":1}`), StartedAt: startedAt}
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.InsertCrawlerTaskRunSQL)).
		WithArgs(uint64(7), "running", []byte(run.Configuration), startedAt).
		WillReturnResult(sqlmock.NewResult(81, 1))
	if err := repo.CreateTaskRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	finishedAt := startedAt.Add(time.Minute)
	run.Status, run.ItemCount, run.FinishedAt = "succeeded", 25, &finishedAt
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.CompleteCrawlerTaskRunSQL)).
		WithArgs("succeeded", finishedAt, uint32(25), "", uint64(81), uint64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.UpdateCrawlerTaskDefinitionLastRunSQL)).
		WithArgs(uint64(81), "succeeded", startedAt, finishedAt, uint32(25), "", uint64(7), uint64(81)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repo.CompleteTaskRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func hgTaskDefinitionColumns() []string {
	return []string{
		"id", "name", "platform", "enabled", "cron", "parser_type", "item_path", "max_items", "configuration",
		"last_run_id", "last_run_status", "last_run_started_at", "last_run_finished_at", "last_run_item_count",
		"last_run_error", "version", "created_by", "updated_by", "created_at", "updated_at",
	}
}

func TestUpsertRecommendationsWithInsertedUsesShortTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()
	publishedAt := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	insertQuery := SQLQueriesPackage.UpsertCrawlerExternalContentsPrefix + SQLQueriesPackage.UpsertCrawlerExternalContentsValue + SQLQueriesPackage.InsertCrawlerExternalContentsNoopSuffix
	upsertQuery := SQLQueriesPackage.UpsertCrawlerExternalContentsPrefix + SQLQueriesPackage.UpsertCrawlerExternalContentsValue + SQLQueriesPackage.UpsertCrawlerExternalContentsSuffix
	args := []driver.Value{"bilibili", "BV1", "title", "1", "author", "https://cover", "https://www.bilibili.com/video/BV1", int64(10), int64(20), int64(3), int64(4), publishedAt, sqlmock.AnyArg()}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(insertQuery)).
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(upsertQuery)).
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repo := NewRepository(db)
	inserted, err := repo.UpsertRecommendationsWithInserted(context.Background(), []CrawlerPlatformPackage.HGRecommendation{{
		Platform: "bilibili", ContentID: "BV1", Title: "title", AuthorID: "1", AuthorName: "author",
		CoverURL: "https://cover", TargetURL: "https://www.bilibili.com/video/BV1", Duration: 10,
		ViewCount: 20, LikeCount: 3, CommentCount: 4, PublishedAt: publishedAt,
	}})
	if err != nil {
		t.Fatalf("UpsertRecommendationsWithInserted() error = %v", err)
	}
	if inserted != 1 {
		t.Fatalf("inserted = %d, want 1", inserted)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestUpsertRecommendationsWithInsertedRollsBackFullUpsertFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()
	insertQuery := SQLQueriesPackage.UpsertCrawlerExternalContentsPrefix + SQLQueriesPackage.UpsertCrawlerExternalContentsValue + SQLQueriesPackage.InsertCrawlerExternalContentsNoopSuffix
	upsertQuery := SQLQueriesPackage.UpsertCrawlerExternalContentsPrefix + SQLQueriesPackage.UpsertCrawlerExternalContentsValue + SQLQueriesPackage.UpsertCrawlerExternalContentsSuffix
	args := []driver.Value{"bilibili", "BV1", "title", "", "", "", "https://www.bilibili.com/video/BV1", int64(0), int64(0), int64(0), int64(0), nil, sqlmock.AnyArg()}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(insertQuery)).WithArgs(args...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(upsertQuery)).WithArgs(args...).WillReturnError(errors.New("upsert failed"))
	mock.ExpectRollback()

	inserted, err := NewRepository(db).UpsertRecommendationsWithInserted(context.Background(), []CrawlerPlatformPackage.HGRecommendation{{
		Platform: "bilibili", ContentID: "BV1", Title: "title", TargetURL: "https://www.bilibili.com/video/BV1",
	}})
	if err == nil || inserted != 0 {
		t.Fatalf("inserted = %d, error = %v", inserted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestUpsertTaskRecommendationsAssociatesBatchInSameTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	insertQuery := SQLQueriesPackage.UpsertCrawlerExternalContentsPrefix + SQLQueriesPackage.UpsertCrawlerExternalContentsValue + SQLQueriesPackage.InsertCrawlerExternalContentsNoopSuffix
	upsertQuery := SQLQueriesPackage.UpsertCrawlerExternalContentsPrefix + SQLQueriesPackage.UpsertCrawlerExternalContentsValue + SQLQueriesPackage.UpsertCrawlerExternalContentsSuffix
	associationQuery := SQLQueriesPackage.UpsertCrawlerTaskExternalContentsPrefix + SQLQueriesPackage.UpsertCrawlerTaskExternalContentsKey + SQLQueriesPackage.UpsertCrawlerTaskExternalContentsSuffix
	args := []driver.Value{"bilibili", "BV1", "title", "", "", "", "https://www.bilibili.com/video/BV1", int64(0), int64(0), int64(0), int64(0), nil, sqlmock.AnyArg()}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(insertQuery)).WithArgs(args...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(upsertQuery)).WithArgs(args...).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(associationQuery)).WithArgs(uint64(9), uint64(11), "bilibili", "BV1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	inserted, err := NewRepository(db).UpsertTaskRecommendationsWithInserted(context.Background(), 9, 11, []CrawlerPlatformPackage.HGRecommendation{{
		Platform: "bilibili", ContentID: "BV1", Title: "title", TargetURL: "https://www.bilibili.com/video/BV1",
	}})
	if err != nil || inserted != 1 {
		t.Fatalf("inserted=%d err=%v", inserted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListTaskExternalContentsUsesReverseAssociationCursor(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	columns := []string{"association_id", "task_definition_id", "last_run_id", "external_content_id", "platform", "content_id", "title", "author_id", "author_name", "cover_url", "target_url", "duration_seconds", "view_count", "like_count", "comment_count", "published_at", "first_seen_at", "last_seen_at", "content_created_at", "content_updated_at", "associated_at", "association_updated_at"}
	rows := sqlmock.NewRows(columns).
		AddRow(8, 9, 11, 4, "bilibili", "BV1", "one", "1", "author", "cover", "target", 10, 20, 3, 4, now, now, now, now, now, now, now).
		AddRow(7, 9, 10, 3, "bilibili", "BV2", "two", "2", "author", "cover", "target", 10, 20, 3, 4, nil, now, now, now, now, now, now)
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.ListCrawlerTaskExternalContentsByCursorSQL)).WithArgs(uint64(9), uint64(10), 2).WillReturnRows(rows)
	items, next, more, err := NewRepository(db).ListTaskExternalContents(context.Background(), 9, 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].AssociationID != 8 || next != 8 || !more {
		t.Fatalf("items=%#v next=%d more=%v", items, next, more)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

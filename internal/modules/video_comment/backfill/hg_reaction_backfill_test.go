package VideoCommentBackfillPackage

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestReactionBackfillCommitsAggregateAndCheckpointTogether(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error=%v", err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(hgEnsureCheckpointSQL)).WithArgs(hgReactionBackfillJob).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(hgSelectCheckpointSQL)).WithArgs(hgReactionBackfillJob).WillReturnRows(sqlmock.NewRows([]string{"cursor_id", "completed"}).AddRow(100, false))
	mock.ExpectQuery(regexp.QuoteMeta(hgSelectBatchEndSQL)).WithArgs(uint64(100), uint64(100), 1000).WillReturnRows(sqlmock.NewRows([]string{"batch_end"}).AddRow(200))
	mock.ExpectExec(regexp.QuoteMeta(hgAggregateBatchSQL)).WithArgs(uint64(100), uint64(200)).WillReturnResult(sqlmock.NewResult(0, 32))
	mock.ExpectExec(regexp.QuoteMeta(hgAdvanceCheckpointSQL)).WithArgs(uint64(200), false, hgReactionBackfillJob, uint64(100)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	processed, completed, err := NewHGReactionBackfill(db).RunBatch(context.Background(), 1000)
	if err != nil || !processed || completed {
		t.Fatalf("RunBatch() processed=%v completed=%v error=%v", processed, completed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

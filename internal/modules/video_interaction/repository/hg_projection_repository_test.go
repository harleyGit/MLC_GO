package VideoInteractionRepositoryPackage

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestListVideoStatesUsesUpdatedAtAndIDKeyset(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cursorTime := time.Unix(100, 0).UTC()
	cutoff := time.Unix(200, 0).UTC()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoStateProjectionPageSQL)).
		WithArgs(cutoff, cursorTime, cursorTime, uint64(7), 100).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "submission_id", "interaction_type", "active", "quantity", "updated_at"}).
			AddRow(8, "user-1", "submission-1", "like", true, 0, time.Unix(101, 0).UTC()))

	rows, err := NewRepository(db).ListVideoStates(context.Background(), HGProjectionCursor{UpdatedAt: cursorTime, RowID: 7}, cutoff, 100)
	if err != nil {
		t.Fatalf("ListVideoStates() error = %v", err)
	}
	if len(rows) != 1 || rows[0].Cursor.RowID != 8 || rows[0].InteractionType != "like" {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestListVideoCountsReturnsAbsoluteAggregateAndCompositeCursor(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cursorTime := time.Unix(100, 0).UTC()
	cutoff := time.Unix(200, 0).UTC()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.SelectVideoCountProjectionPageSQL)).
		WithArgs(cutoff, cursorTime, cursorTime, "submission-0", cursorTime, "submission-0", uint16(3), 50).
		WillReturnRows(sqlmock.NewRows([]string{"updated_at", "submission_id", "shard_id", "like_count", "coin_count", "favorite_count", "share_count"}).
			AddRow(time.Unix(101, 0).UTC(), "submission-1", 4, 10, 3, 5, 2))

	rows, err := NewRepository(db).ListVideoCounts(context.Background(), HGProjectionCursor{UpdatedAt: cursorTime, EntityID: "submission-0", ShardID: 3}, cutoff, 50)
	if err != nil {
		t.Fatalf("ListVideoCounts() error = %v", err)
	}
	if len(rows) != 1 || rows[0].LikeCount != 10 || rows[0].Cursor.ShardID != 4 {
		t.Fatalf("rows = %#v", rows)
	}
}

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
		WithArgs(uint16(0), uint16(512), cutoff, uint16(4), uint16(4), cursorTime, uint16(4), cursorTime, uint64(7), 100).
		WillReturnRows(sqlmock.NewRows([]string{"reproject_bucket", "id", "user_id", "submission_id", "interaction_type", "active", "quantity", "updated_at"}).
			AddRow(4, 8, "user-1", "submission-1", "like", true, 0, time.Unix(101, 0).UTC()))

	rows, err := NewRepository(db).ListVideoStates(context.Background(), HGProjectionCursor{Bucket: 4, UpdatedAt: cursorTime, RowID: 7}, cutoff, 100, HGProjectionHashRange{Start: 0, End: 512})
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
		WithArgs(uint16(512), uint16(1024), cutoff, uint16(600), uint16(600), cursorTime, uint16(600), cursorTime, "submission-0", uint16(600), cursorTime, "submission-0", uint16(3), 50).
		WillReturnRows(sqlmock.NewRows([]string{"reproject_bucket", "updated_at", "submission_id", "shard_id", "like_count", "coin_count", "favorite_count", "share_count"}).
			AddRow(600, time.Unix(101, 0).UTC(), "submission-1", 4, 10, 3, 5, 2))

	rows, err := NewRepository(db).ListVideoCounts(context.Background(), HGProjectionCursor{Bucket: 600, UpdatedAt: cursorTime, EntityID: "submission-0", ShardID: 3}, cutoff, 50, HGProjectionHashRange{Start: 512, End: 1024})
	if err != nil {
		t.Fatalf("ListVideoCounts() error = %v", err)
	}
	if len(rows) != 1 || rows[0].LikeCount != 10 || rows[0].Cursor.ShardID != 4 {
		t.Fatalf("rows = %#v", rows)
	}
}

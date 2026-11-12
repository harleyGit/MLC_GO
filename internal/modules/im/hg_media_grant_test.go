package im

import (
	"MLC_GO/internal/modules/im/transport"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"encoding/hex"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGMediaReadGrantMemberAndReview(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	storage, err := transport.HGNewSeaweedFS("http://127.0.0.1:8333", "us-east-1", "im-private", "fixture", strings.Repeat("s", 32))
	if err != nil {
		t.Fatal(err)
	}
	defer storage.HGClose()
	media := HGNewPrivateMedia(db, storage, nil, "im-private")
	mediaID := strings.Repeat("a", 32)
	id, _ := hex.DecodeString(mediaID)
	columns := []string{"owner_id", "conversation_id", "object_key", "immutable_key", "media_type", "mime_type", "byte_size", "sha256", "state", "width", "height", "duration_ms"}
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGIMMediaGetSQL)).WithArgs(id).WillReturnRows(sqlmock.NewRows(columns).AddRow("owner", 7, "quarantine/"+mediaID, "sealed/"+mediaID, "image", "image/png", 100, []byte{1}, "ready", 1, 1, 0))
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGIMConversationMemberSQL)).WithArgs(uint64(7), "peer").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGIMMediaReadableSQL)).WithArgs(id, uint64(7)).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	url, err := media.HGReadGrant(context.Background(), "peer", mediaID)
	if err != nil || !strings.Contains(url, "X-Amz-Expires=15") {
		t.Fatalf("已关联消息的成员应得到短时签名: %v", err)
	}
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGIMMediaGetSQL)).WithArgs(id).WillReturnRows(sqlmock.NewRows(columns).AddRow("owner", 7, "quarantine/"+mediaID, "sealed/"+mediaID, "image", "image/png", 100, []byte{1}, "deleted", 1, 1, 0))
	if _, err = media.HGReadGrant(context.Background(), "peer", mediaID); err != HGErrForbidden {
		t.Fatal("删除的媒体不得继续签发")
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

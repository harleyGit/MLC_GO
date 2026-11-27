package im

import (
	q "MLC_GO/internal/pkg/mysql/queries"
	"MLC_GO/internal/response/hgerrors"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHGIMSendReplayUsesOriginalReceipt(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		if err != nil {
			t.Fatal(err)
		}
		cmd := HGSendCommand{ConversationID: "17", ClientMsgID: "first", MessageType: HGText, Content: []byte(`{"text":"你好"}`)}
		hash := sha256.Sum256(append([]byte("text\x00"), cmd.Content...))
		if conflict {
			hash[0] ^= 1
		}
		mock.ExpectBegin()
		mock.ExpectQuery(q.HGIMLockConversationSQL).WillReturnRows(sqlmock.NewRows([]string{"low", "high", "seq"}).AddRow("A", "B", 4))
		mock.ExpectQuery(q.HGIMContactPairSQL).WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(1))
		mock.ExpectQuery(q.HGIMContactBlockPairSQL).WillReturnError(sql.ErrNoRows)
		mock.ExpectQuery(q.HGIMConversationMemberSQL).WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(1))
		mock.ExpectQuery(q.HGIMExistingMessageSQL).WithArgs("A", "first").WillReturnRows(sqlmock.NewRows([]string{"msg", "conv", "seq", "type", "hash"}).AddRow(19, 17, 4, "text", hash[:]))
		mock.ExpectRollback()
		got, err := HGNewService(db, "im", nil).HGSend(context.Background(), "A", cmd)
		if conflict && (!errors.Is(err, HGErrConflict) || hgerrors.Resolve(err).Code != 1302003) || !conflict && (err != nil || got.MessageID != "19" || got.Seq != 4) {
			t.Fatalf("冲突=%v，回执=%+v，错误=%v", conflict, got, err)
		}
		if err = mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}

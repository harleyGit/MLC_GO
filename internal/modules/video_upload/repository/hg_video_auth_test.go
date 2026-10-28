package VideoUploadRepositoryPackage

import (
	VideoUploadDtoPackage "MLC_GO/internal/modules/video_upload/dto"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// 已存在的投稿 ID 属于其他用户时，上传事务必须在任何 INSERT 前拒绝。
func TestHGCreateUploadedVideoRejectsForeignSubmission(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectVideoSubmissionAuthForUpdate)).WithArgs("SUB1").WillReturnRows(sqlmock.NewRows([]string{"user_id", "status"}).AddRow("OTHER", "draft"))
	mock.ExpectRollback()
	err = NewRepository(db).CreateUploadedVideo(context.Background(), UploadedVideo{SubmissionID: "SUB1", UserID: "USER", VideoID: "VID1"})
	if !errors.Is(err, ErrHGVideoForbidden) {
		t.Fatalf("跨用户上传未拒绝: %v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 已提交审核的稿件不能通过保存草稿接口重新写入，且不得触碰标签或 Outbox。
func TestHGSaveSubmissionRejectsReviewedStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectVideoSubmissionAuthForUpdate)).WithArgs("SUB1").WillReturnRows(sqlmock.NewRows([]string{"user_id", "status"}).AddRow("USER", "published"))
	mock.ExpectRollback()
	err = NewRepository(db).SaveSubmissionWithEvents(context.Background(), "USER", VideoUploadDtoPackage.SaveSubmissionRequest{SubmissionID: "SUB1"})
	if !errors.Is(err, ErrHGVideoForbidden) {
		t.Fatalf("已发布投稿未拒绝: %v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

// 稿件本人提交他人 video_id 时，配置更新和标签删除都不能执行。
func TestHGSaveSubmissionRejectsForeignVideoIDBeforeTagWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectVideoSubmissionAuthForUpdate)).WithArgs("SUB1").WillReturnRows(sqlmock.NewRows([]string{"user_id", "status"}).AddRow("USER", "draft"))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.GetSubmissionTotalsSQL)).WithArgs("SUB1", "USER").WillReturnRows(sqlmock.NewRows([]string{"count", "size"}).AddRow(1, 123))
	mock.ExpectExec(regexp.QuoteMeta(SQLQueriesPackage.SaveSubmissionSQL)).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(regexp.QuoteMeta(SQLQueriesPackage.HGSelectVideoFileAuthForUpdate)).WithArgs("FOREIGN_VIDEO", "SUB1", "USER").WillReturnRows(sqlmock.NewRows([]string{"1"}))
	mock.ExpectRollback()
	err = NewRepository(db).SaveSubmissionWithEvents(context.Background(), "USER", VideoUploadDtoPackage.SaveSubmissionRequest{SubmissionID: "SUB1", Videos: []VideoUploadDtoPackage.VideoConfigRequest{{VideoID: "FOREIGN_VIDEO"}}})
	if !errors.Is(err, ErrHGVideoForbidden) {
		t.Fatalf("跨视频标签写未拒绝: %v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

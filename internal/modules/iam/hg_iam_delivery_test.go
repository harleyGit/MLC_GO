package iam

import (
	Notification "MLC_GO/internal/modules/notification"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

type hgTestPublisher struct {
	user    string          // 收到发布的业务用户，空串表示尚未发布。
	payload json.RawMessage // 网关收到的协议原文，仅供断言，不含真实身份或凭据。
}

// 模板未发布时不发送无文案通知，也不能把授权变更任务标记完成。
func TestHGAccessDeliveryRetriesWithoutPublishedTemplate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	publisher := new(hgTestPublisher)
	worker := HGNewAccessDelivery(db, NewHGService(NewHGRepository(db)),
		Notification.HGNewService(Notification.HGNewRepository(db)), publisher)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGClaimIAMDeliveryJob)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "target_type", "target_id", "role_name", "last_user_id"}).
			AddRow(12, "user", "U1", "reviewer", ""))
	mock.ExpectExec(regexp.QuoteMeta(SQL.HGLeaseIAMDeliveryJob)).
		WithArgs(sqlmock.AnyArg(), uint64(12)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGNotificationRolePublishedSQL)).WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(regexp.QuoteMeta(SQL.HGRetryIAMDeliveryJob)).
		WithArgs(uint64(12), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := worker.HGDispatchOnce(context.Background()); err == nil || publisher.user != "" {
		t.Fatal("模板未发布时不能确认任务或投递权限")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func (publisher *hgTestPublisher) HGPublishUser(_ context.Context, user string, payload json.RawMessage) error {
	publisher.user, publisher.payload = user, payload
	return nil
}

// 工作任务先完成站内入箱，再发布当前权威快照；所有 SQL 及租约更新可观察。
func TestHGAccessDeliveryPublishesCurrentSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	publisher := new(hgTestPublisher)
	worker := HGNewAccessDelivery(db, NewHGService(NewHGRepository(db)),
		Notification.HGNewService(Notification.HGNewRepository(db)), publisher)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGClaimIAMDeliveryJob)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "target_type", "target_id", "role_name", "last_user_id"}).
			AddRow(11, "user", "U1", "reviewer", ""))
	mock.ExpectExec(regexp.QuoteMeta(SQL.HGLeaseIAMDeliveryJob)).
		WithArgs(sqlmock.AnyArg(), uint64(11)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGNotificationRolePublishedSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"title", "body", "schema", "channels"}).
			AddRow("角色信息已更新", "您的角色 {{role_name}} 已更新。", []byte(`{"role_name":{"type":"string","required":true,"max_length":128}}`), []byte(`["in_app","realtime"]`)))
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGUserDeliveryName)).WithArgs("reviewer").
		WillReturnRows(sqlmock.NewRows([]string{"role_name"}).AddRow("审核员"))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(SQL.HGNotificationLedgerInsertSQL)).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "U1", "system").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGNotificationCounterReadySQL)).
		WillReturnRows(sqlmock.NewRows([]string{"ready"}).AddRow(false))
	mock.ExpectExec(regexp.QuoteMeta(SQL.HGNotificationInsertSQL)).
		WithArgs(sqlmock.AnyArg(), "U1", "system", "角色信息已更新", "您的角色 审核员 已更新。", "", "").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGSelectIAMPolicyRevision)).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(42))
	mock.ExpectQuery(regexp.QuoteMeta(SQL.HGSelectIAMAccessGrants)).WithArgs("U1").
		WillReturnRows(sqlmock.NewRows([]string{"role", "permission", "scope_type", "scope_id", "expires_at"}).
			AddRow("reviewer", "iam:role:read", "global", "", nil))
	mock.ExpectExec(regexp.QuoteMeta(SQL.HGFinishIAMDeliveryJob)).
		WithArgs("U1", "done", uint64(11), sqlmock.AnyArg(), "").
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := worker.HGDispatchOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	var event struct {
		EventType string           `json:"event_type"` // 实时事件名称，不接受聊天正文。
		Snapshot  HGAccessSnapshot `json:"snapshot"`   // 仅当前用户权威授权快照。
	}
	if err := json.Unmarshal(publisher.payload, &event); err != nil ||
		publisher.user != "U1" || event.EventType != "iam.access.updated.v1" ||
		event.Snapshot.UserID != "U1" || event.Snapshot.PolicyVersion != 42 {
		t.Fatalf("实时用户频道投递协议不正确: %v %s", err, publisher.user)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

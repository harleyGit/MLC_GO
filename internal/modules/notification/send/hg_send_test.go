package send

import (
	notification "MLC_GO/internal/modules/notification"
	queries "MLC_GO/internal/pkg/mysql/queries"
	utilspkg "MLC_GO/internal/pkg/utils"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
)

func hgVersion() HGVersion {
	return HGVersion{
		Category: "system", Title: "维护 {{name}}", Body: "您好 {{name}}，维护将在 {{when}} 开始。",
		Schema: json.RawMessage(`{"name":{"type":"string","required":true,"max_length":8},"when":{"type":"string","required":true,"max_length":32}}`),
		Action: "none", Config: json.RawMessage(`{}`), Channels: json.RawMessage(`["in_app"]`),
	}
}

func TestHGRequestAuditIgnoresForgedHeader(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/notification/send-tasks", nil)
	request.Header.Set("X-Request-ID", "forged-identifier")
	request.Header.Set("X-Forwarded-For", "203.0.113.10")
	request.RemoteAddr = "192.0.2.15:1234"
	meta := hgRequestAudit(request)
	if meta.RequestID != "" || meta.SourceIP != "192.0.2.15" {
		t.Fatalf("不可相信请求头: %+v", meta)
	}
	request = request.WithContext(utilspkg.InjectTID(request.Context(), "server-trace"))
	meta = hgRequestAudit(request)
	if meta.RequestID != "server-trace" || meta.SourceIP != "192.0.2.15" {
		t.Fatalf("应保留中间件注入的追踪 ID 与直连 IP: %+v", meta)
	}
}

func TestHGRenderStrictText(t *testing.T) {
	v := hgVersion()
	notice, err := HGRender(context.Background(), v, map[string]string{"name": "用户A", "when": "02:00"})
	if err != nil || notice.Category != "system" || notice.Title != "维护 用户A" || !strings.Contains(notice.Content, "02:00") {
		t.Fatalf("渲染不正确: %+v %v", notice, err)
	}
	for _, item := range []struct {
		name   string
		change func(*HGVersion, map[string]string)
	}{
		{"缺少变量", func(_ *HGVersion, vars map[string]string) { delete(vars, "when") }},
		{"额外变量", func(_ *HGVersion, vars map[string]string) { vars["secret"] = "x" }},
		{"长度溢出", func(_ *HGVersion, vars map[string]string) { vars["name"] = "123456789" }},
		{"注入文本", func(_ *HGVersion, vars map[string]string) { vars["name"] = "<script>" }},
		{"未声明占位符", func(v *HGVersion, _ map[string]string) { v.Title = "{{oops}}" }},
		{"不完整占位符", func(v *HGVersion, _ map[string]string) { v.Body = "{{name" }},
		{"未接实时渠道", func(v *HGVersion, _ map[string]string) { v.Channels = []byte(`["in_app","realtime"]`) }},
		{"安全类别禁止运营发送", func(v *HGVersion, _ map[string]string) { v.Category = "security" }},
		{"外部跳转禁止", func(v *HGVersion, _ map[string]string) { v.Action = "external_url" }},
	} {
		t.Run(item.name, func(t *testing.T) {
			version := hgVersion()
			vars := map[string]string{"name": "用户A", "when": "02:00"}
			item.change(&version, vars)
			if _, err := HGRender(context.Background(), version, vars); !errors.Is(err, HGErrInvalid) {
				t.Fatalf("必须拒绝: %v", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := HGRender(ctx, v, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("应响应取消: %v", err)
	}
}

func TestHGPrepareBoundedAndDeterministic(t *testing.T) {
	cmd := HGCommand{TemplateID: "7", TemplateVersion: 1, UserIDs: []string{"user_2", "user_1"}, Variables: map[string]string{"name": "用户"}}
	_, audience, _, err := hgPrepare(&cmd)
	if err != nil || string(audience) != `["user_1","user_2"]` {
		t.Fatalf("接收人顺序不稳定: %s %v", audience, err)
	}
	for _, users := range [][]string{nil, {"a", "a"}, {"user id"}, make([]string, 101)} {
		cmd.UserIDs = users
		if _, _, _, err := hgPrepare(&cmd); !errors.Is(err, HGErrInvalid) {
			t.Fatalf("必须限制接收人: %v", err)
		}
	}
}

func TestHGCreateIdempotencyAndConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := HGNewRepository(db)
	cmd := HGCommand{TemplateID: "7", TemplateVersion: 1, UserIDs: []string{"user_1"}, Variables: map[string]string{"name": "用户A", "when": "02:00"}}
	_, audience, variables, err := hgPrepare(&cmd)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d:%s:%s", "ADMIN", 7, 1, audience, variables)))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendByRequestSQL)).WithArgs("req-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "hash"}).AddRow(9, hex.EncodeToString(sum[:])))
	if id, err := repo.HGCreate(context.Background(), "req-1", "ADMIN", cmd); err != nil || id != "9" {
		t.Fatalf("相同请求应返回原任务: %q %v", id, err)
	}
	// 已存在请求先按唯一键返回，无需再读取可能已经更新的模板。
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendByRequestSQL)).WithArgs("req-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "hash"}).AddRow(9, strings.Repeat("x", 64)))
	if _, err := repo.HGCreate(context.Background(), "req-1", "ADMIN", cmd); !errors.Is(err, HGErrConflict) {
		t.Fatalf("请求键不同负载必须冲突: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGPublishRejectsSelfReview(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendPublishLockSQL)).WithArgs(uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"version", "status", "code", "author", "category", "title", "body", "schema", "action", "config", "channels"}).
			AddRow(2, "draft", "system_notice", "ADMIN", "system", "标题", "正文", []byte(`{}`), "none", []byte(`{}`), []byte(`["in_app"]`)))
	mock.ExpectRollback()
	if err := HGNewRepository(db).HGPublish(context.Background(), 7, 2, "ADMIN"); !errors.Is(err, HGErrConflict) {
		t.Fatalf("作者不得自行发布: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGPublishOtherReviewer(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendPublishLockSQL)).WithArgs(uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"version", "status", "code", "author", "category", "title", "body", "schema", "action", "config", "channels"}).
			AddRow(2, "draft", "system_notice", "AUTHOR", "system", "标题", "正文", []byte(`{}`), "none", []byte(`{}`), []byte(`["in_app"]`)))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendPublishVersionSQL)).
		WithArgs(uint64(7), uint32(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationTemplatePublishSQL)).
		WithArgs(uint32(2), "REVIEWER", uint64(7), uint32(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	version := HGVersion{
		Category: "system", Title: "标题", Body: "正文", Schema: json.RawMessage(`{}`),
		Action: "none", Config: json.RawMessage(`{}`), Channels: json.RawMessage(`["in_app"]`),
	}
	content, err := json.Marshal(version)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(content)
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationTemplateAuditSQL)).
		WithArgs(uint64(7), uint32(2), "published", "REVIEWER", "", "", hex.EncodeToString(hash[:])).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := HGNewRepository(db).HGPublish(context.Background(), 7, 2, "REVIEWER"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGPublishAuditFailureRollsBack(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendPublishLockSQL)).WithArgs(uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"version", "status", "code", "author", "category", "title", "body", "schema", "action", "config", "channels"}).
			AddRow(2, "draft", "system_notice", "AUTHOR", "system", "标题", "正文", []byte(`{}`), "none", []byte(`{}`), []byte(`["in_app"]`)))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendPublishVersionSQL)).
		WithArgs(uint64(7), uint32(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationTemplatePublishSQL)).
		WithArgs(uint32(2), "REVIEWER", uint64(7), uint32(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationTemplateAuditSQL)).
		WithArgs(uint64(7), uint32(2), "published", "REVIEWER", "", "", sqlmock.AnyArg()).
		WillReturnError(errors.New("审计不可用"))
	mock.ExpectRollback()
	if err := HGNewRepository(db).HGPublish(context.Background(), 7, 2, "REVIEWER"); err == nil {
		t.Fatal("模板审计失败不得发布")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGCreateTaskTransactional(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := HGNewRepository(db)
	cmd := HGCommand{TemplateID: "7", TemplateVersion: 2, UserIDs: []string{"user_2", "user_1"}, Variables: map[string]string{"name": "用户A", "when": "02:00"}}
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendByRequestSQL)).WithArgs("req-1").WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendTemplateSQL)).WithArgs(uint32(2), uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"category", "title", "body", "schema", "action", "config", "channels"}).
			AddRow("system", "维护 {{name}}", "您好 {{name}}，维护将在 {{when}} 开始。",
				[]byte(hgVersion().Schema), "none", []byte(`{}`), []byte(`["in_app"]`)))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendUsersPrefixSQL)).
		WithArgs("user_1", "user_2").WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("user_1").AddRow("user_2"))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendAdminLockSQL)).WithArgs("ADMIN").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendPendingLimitSQL)).WithArgs("ADMIN").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendInsertSQL)).
		WithArgs("req-1", sqlmock.AnyArg(), uint64(7), uint32(2), []byte(`["user_1","user_2"]`), sqlmock.AnyArg(), "ADMIN").
		WillReturnResult(sqlmock.NewResult(55, 1))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendAuditSQL)).
		WithArgs(int64(55), "created", "ADMIN", uint32(2), "", "").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	id, err := repo.HGCreate(context.Background(), "req-1", "ADMIN", cmd)
	if err != nil || id != "55" {
		t.Fatalf("任务必须原子写入: %q %v", id, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGCreateBackpressureDoesNotInsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cmd := HGCommand{TemplateID: "7", TemplateVersion: 2, UserIDs: []string{"user_1"}, Variables: map[string]string{"name": "用户A", "when": "02:00"}}
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendByRequestSQL)).WithArgs("req-1").WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendTemplateSQL)).WithArgs(uint32(2), uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"category", "title", "body", "schema", "action", "config", "channels"}).
			AddRow("system", "维护 {{name}}", "您好 {{name}}，维护将在 {{when}} 开始。",
				[]byte(hgVersion().Schema), "none", []byte(`{}`), []byte(`["in_app"]`)))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendUsersPrefixSQL)).
		WithArgs("user_1").WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("user_1"))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendAdminLockSQL)).WithArgs("ADMIN").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendPendingLimitSQL)).WithArgs("ADMIN").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(20))
	mock.ExpectRollback()
	if _, err := HGNewRepository(db).HGCreate(context.Background(), "req-1", "ADMIN", cmd); !errors.Is(err, HGErrBusy) {
		t.Fatalf("满载时不能增加任务: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type hgSendHintPublisher struct {
	user    string          // 仅测试用户的本人频道目标。
	payload json.RawMessage // 网关收到的分类提示，无通知正文。
}

func (publisher *hgSendHintPublisher) HGPublishUser(_ context.Context, user string, payload json.RawMessage) error {
	publisher.user, publisher.payload = user, payload
	return nil
}

func TestHGDispatchOncePersistsBeforeAdvancing(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hints := notification.HGNewHints()
	publisher := new(hgSendHintPublisher)
	hints.HGUsePublisher(publisher)
	worker := HGNewWorker(HGNewRepository(db), notification.HGNewService(notification.HGNewRepository(db), hints))
	mock.ExpectBegin()
	// ready 与过期租约会公平轮换；两条索引查询均必须只选择一行并锁定。
	mock.ExpectQuery(`SELECT id,template_id,template_version,audience,variables,next_index,attempts`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "template_id", "version", "audience", "variables", "index", "attempts"}).
			AddRow(12, 7, 2, []byte(`["user_1"]`), []byte(`{"name":"用户A","when":"02:00"}`), 0, 0))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendLeaseSQL)).
		WithArgs(sqlmock.AnyArg(), uint64(12)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendBoundVersionSQL)).WithArgs(uint32(2), uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"category", "title", "body", "schema", "action", "config", "channels"}).
			AddRow("system", "维护 {{name}}", "您好 {{name}}，维护将在 {{when}} 开始。",
				[]byte(hgVersion().Schema), "none", []byte(`{}`), []byte(`["in_app"]`)))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendUserExistsSQL)).WithArgs("user_1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationLedgerInsertSQL)).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "user_1", "system").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationCounterReadySQL)).
		WillReturnRows(sqlmock.NewRows([]string{"ready"}).AddRow(false))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationInsertSQL)).
		WithArgs(sqlmock.AnyArg(), "user_1", "system", "维护 用户A", "您好 用户A，维护将在 02:00 开始。", "", "").
		WillReturnResult(sqlmock.NewResult(55, 1))
	mock.ExpectCommit()
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendAdvanceSQL)).
		WithArgs(1, "done", uint64(12), sqlmock.AnyArg(), 0).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := worker.HGDispatchOnce(context.Background()); err != nil {
		t.Fatalf("已持久化站内通知后应推进任务: %v", err)
	}
	var hint struct {
		EventType string `json:"event_type"` // 仅供 Web 判断需要补拉收件箱。
		Category  string `json:"category"`   // 提示的分类，不包含已读数。
	}
	if err := json.Unmarshal(publisher.payload, &hint); err != nil || publisher.user != "user_1" ||
		hint.EventType != "notification.inbox.updated.v1" || hint.Category != "system" {
		t.Fatalf("任务完成后须向本人发送无正文分类提示: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGDispatchOnceReplaySkipsDuplicateInbox(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	worker := HGNewWorker(HGNewRepository(db), notification.HGNewService(notification.HGNewRepository(db)))
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id,template_id,template_version,audience,variables,next_index,attempts`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "template_id", "version", "audience", "variables", "index", "attempts"}).
			AddRow(12, 7, 2, []byte(`["user_1"]`), []byte(`{"name":"用户A","when":"02:00"}`), 0, 0))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendLeaseSQL)).
		WithArgs(sqlmock.AnyArg(), uint64(12)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendBoundVersionSQL)).WithArgs(uint32(2), uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"category", "title", "body", "schema", "action", "config", "channels"}).
			AddRow("system", "维护 {{name}}", "您好 {{name}}，维护将在 {{when}} 开始。",
				[]byte(hgVersion().Schema), "none", []byte(`{}`), []byte(`["in_app"]`)))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendUserExistsSQL)).WithArgs("user_1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationLedgerInsertSQL)).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "user_1", "system").
		WillReturnError(&mysql.MySQLError{Number: 1062, Message: "duplicate key"})
	mock.ExpectRollback()
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendAdvanceSQL)).
		WithArgs(1, "done", uint64(12), sqlmock.AnyArg(), 0).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := worker.HGDispatchOnce(context.Background()); err != nil {
		t.Fatalf("租约重放不得写第二条通知: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGMinePageOnlyCreatorAndBoundedCursor(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows := sqlmock.NewRows([]string{"id", "template_id", "version", "status", "next_index", "attempts", "created_at", "total", "scheduled_at"})
	for id := 30; id >= 10; id-- {
		rows.AddRow(id, 7, 2, "pending", 0, 0, time.Now().UTC(), 2, nil)
	}
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendMinePageSQL)).
		WithArgs("ADMIN", uint64(31), 21).WillReturnRows(rows)
	items, next, err := HGNewRepository(db).HGMinePage(context.Background(), "ADMIN", 31)
	if err != nil || len(items) != 20 || items[0].ID != "30" || next != "11" {
		t.Fatalf("本人任务游标错误: %d %q %v", len(items), next, err)
	}
	if _, _, err := HGNewRepository(db).HGMinePage(context.Background(), "bad user", 31); !errors.Is(err, HGErrInvalid) {
		t.Fatalf("非法身份必须拒绝: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGPublishedPageUsesPublishedSnapshotNotLatestDraft(t *testing.T) {
	if !strings.Contains(queries.HGNotificationSendPublishedPageSQL, "v.version=t.published_version") ||
		!strings.Contains(queries.HGNotificationSendPublishedPageSQL, "v.status='published'") {
		t.Fatal("发布版查询不得读取最新草稿")
	}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// 模拟主表 latest=3、published=2；查询必须使用已发布版本快照。
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendPublishedPageSQL)).
		WithArgs(uint64(50), 21).WillReturnRows(sqlmock.NewRows([]string{
		"id", "template_code", "name", "category", "status", "version", "published_version",
		"title_template", "body_template", "variables_schema", "action_type", "action_config", "channels", "priority", "locale",
	}).AddRow(7, "maintenance", "已审核名称", "system", "published", 2, 2,
		"已审核标题", "已审核正文", []byte(`{}`), "none", []byte(`{}`), []byte(`["in_app"]`), 2, "zh-CN"))
	items, next, err := notification.HGNewTemplateRepository(db).HGPublishedPage(context.Background(), 50)
	if err != nil || next != "" || len(items) != 1 || items[0].Version != 2 ||
		items[0].TitleTemplate != "已审核标题" || items[0].Name != "已审核名称" {
		t.Fatalf("必须返回发布版不可变快照: %+v %q %v", items, next, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGMineListRejectsUntrustedIdentityAndInvalidCursor(t *testing.T) {
	handler := HGNewHandler(nil) // 非法输入必须在任何数据库访问之前拒绝。
	for _, target := range []string{"/send-tasks", "/send-tasks?cursor=0", "/send-tasks?cursor=invalid"} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		response := httptest.NewRecorder()
		handler.HGMineList(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("未认证或非法游标必须拒绝: %s %d", target, response.Code)
		}
	}
}

func TestHGScheduledTaskRequiresApprovalAndAudit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	when := time.Now().UTC().Add(time.Hour).Truncate(time.Millisecond)
	cmd := HGCommand{
		TemplateID: "7", TemplateVersion: 2, UserIDs: []string{"user_1"},
		Variables:   map[string]string{"name": "用户A", "when": "02:00"},
		ScheduledAt: when.Format(time.RFC3339Nano),
	}
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendByRequestSQL)).WithArgs("schedule-1").WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendTemplateSQL)).WithArgs(uint32(2), uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"category", "title", "body", "schema", "action", "config", "channels"}).
			AddRow("system", "维护 {{name}}", "您好 {{name}}，维护将在 {{when}} 开始.",
				[]byte(hgVersion().Schema), "none", []byte(`{}`), []byte(`["in_app"]`)))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendUsersPrefixSQL)).WithArgs("user_1").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("user_1"))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendAdminLockSQL)).WithArgs("ADMIN").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendPendingLimitSQL)).WithArgs("ADMIN").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendScheduledInsertSQL)).
		WithArgs("schedule-1", sqlmock.AnyArg(), uint64(7), uint32(2),
			[]byte(`["user_1"]`), sqlmock.AnyArg(), "ADMIN", when, when).
		WillReturnResult(sqlmock.NewResult(55, 1))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendAuditSQL)).
		WithArgs(int64(55), "created", "ADMIN", uint32(2), "", "").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if _, err := HGNewRepository(db).HGCreate(context.Background(), "schedule-1", "ADMIN", cmd); err != nil {
		t.Fatalf("定时任务应等待审批: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGApproveRejectsSelfAndKeepsTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendApproveLockSQL)).WithArgs(uint64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"creator", "template", "version", "scheduled"}).
			AddRow("AUTHOR", 7, 2, nil))
	mock.ExpectRollback()
	if err := HGNewRepository(db).HGApprove(context.Background(), 12, "AUTHOR"); !errors.Is(err, HGErrConflict) {
		t.Fatalf("不得自审: %v", err)
	}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendApproveLockSQL)).WithArgs(uint64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"creator", "template", "version", "scheduled"}).
			AddRow("AUTHOR", 7, 2, nil))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendBoundVersionSQL)).WithArgs(uint32(2), uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"category", "title", "body", "schema", "action", "config", "channels"}).
			AddRow("system", "标题", "正文", []byte(`{}`), "none", []byte(`{}`), []byte(`["in_app"]`)))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendApproveSQL)).
		WithArgs("REVIEWER", uint64(12)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendAuditSQL)).
		WithArgs(uint64(12), "approved", "REVIEWER", uint32(2), "", "").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := HGNewRepository(db).HGApprove(context.Background(), 12, "REVIEWER"); err != nil {
		t.Fatalf("他人审批应原子提交: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGApproveAuditFailureRollsBack(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendApproveLockSQL)).WithArgs(uint64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"creator", "template", "version", "scheduled"}).
			AddRow("AUTHOR", 7, 2, nil))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendBoundVersionSQL)).WithArgs(uint32(2), uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"category", "title", "body", "schema", "action", "config", "channels"}).
			AddRow("system", "标题", "正文", []byte(`{}`), "none", []byte(`{}`), []byte(`["in_app"]`)))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendApproveSQL)).
		WithArgs("REVIEWER", uint64(12)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendAuditSQL)).
		WithArgs(uint64(12), "approved", "REVIEWER", uint32(2), "", "").
		WillReturnError(errors.New("审计不可用"))
	mock.ExpectRollback()
	if err := HGNewRepository(db).HGApprove(context.Background(), 12, "REVIEWER"); err == nil {
		t.Fatal("审批审计失败不得放行任务")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGCancelWritesAuditAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendCancelLockSQL)).
		WithArgs(uint64(12), "AUTHOR").
		WillReturnRows(sqlmock.NewRows([]string{"version", "status"}).AddRow(2, "pending_approval"))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendCancelSQL)).
		WithArgs(uint64(12), "AUTHOR").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendAuditSQL)).
		WithArgs(uint64(12), "cancelled", "AUTHOR", uint32(2), "", "").
		WillReturnError(errors.New("审计写入失败"))
	mock.ExpectRollback()
	if err := HGNewRepository(db).HGCancel(context.Background(), 12, "AUTHOR"); err == nil {
		t.Fatal("审计失败必须回滚取消")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGReviewDetailRendersBoundVariablesWithoutRecipientIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendReviewDetailSQL)).
		WithArgs(uint64(12), "REVIEWER").
		WillReturnRows(sqlmock.NewRows([]string{"id", "template_id", "version", "status", "total", "created_at", "category",
			"title", "body", "schema", "action", "config", "channels", "variables", "scheduled"}).
			AddRow(12, 7, 2, "pending_approval", 2, time.Now().UTC(), "system",
				"维护 {{name}}", "您好 {{name}}，维护将在 {{when}} 开始。",
				[]byte(hgVersion().Schema), "none", []byte(`{}`), []byte(`["in_app"]`),
				[]byte(`{"name":"用户A","when":"02:00"}`), nil))
	review, err := HGNewRepository(db).HGReviewDetail(context.Background(), 12, "REVIEWER")
	if err != nil || review.Title != "维护 用户A" || review.Total != 2 ||
		!strings.Contains(review.Body, "02:00") {
		t.Fatalf("必须使用权威版本与变量渲染: %+v %v", review, err)
	}
	payload, err := json.Marshal(review)
	if err != nil || strings.Contains(string(payload), "user_") || strings.Contains(string(payload), "variables") {
		t.Fatalf("不得公开接收用户或原始变量: %s %v", payload, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHGMarketingImmediateMustBeReviewed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cmd := HGCommand{TemplateID: "7", TemplateVersion: 2, UserIDs: []string{"user_1"},
		Variables: map[string]string{"name": "用户A", "when": "02:00"}}
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendByRequestSQL)).
		WithArgs("marketing-1").WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendTemplateSQL)).
		WithArgs(uint32(2), uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"category", "title", "body", "schema", "action", "config", "channels"}).
			AddRow("marketing", "维护 {{name}}", "您好 {{name}}，维护将在 {{when}} 开始。",
				[]byte(hgVersion().Schema), "none", []byte(`{}`), []byte(`["in_app"]`)))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendUsersPrefixSQL)).
		WithArgs("user_1").WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow("user_1"))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendAdminLockSQL)).
		WithArgs("ADMIN").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(queries.HGNotificationSendPendingLimitSQL)).
		WithArgs("ADMIN").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendScheduledInsertSQL)).
		WithArgs("marketing-1", sqlmock.AnyArg(), uint64(7), uint32(2),
			[]byte(`["user_1"]`), sqlmock.AnyArg(), "ADMIN", nil, nil).
		WillReturnResult(sqlmock.NewResult(19, 1))
	mock.ExpectExec(regexp.QuoteMeta(queries.HGNotificationSendAuditSQL)).
		WithArgs(int64(19), "created", "ADMIN", uint32(2), "", "").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if _, err := HGNewRepository(db).HGCreate(context.Background(), "marketing-1", "ADMIN", cmd); err != nil {
		t.Fatalf("营销通知必须先持久化为待审: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

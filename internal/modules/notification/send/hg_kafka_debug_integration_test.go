//go:build notification_acceptance

package send_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	hgkafkaruntime "MLC_GO/internal/infrastructure/kafka"
	notification "MLC_GO/internal/modules/notification"
	send "MLC_GO/internal/modules/notification/send"
	config "MLC_GO/internal/pkg/config"
	hgkafka "MLC_GO/internal/pkg/kafka"
	queries "MLC_GO/internal/pkg/mysql/queries"

	"github.com/go-sql-driver/mysql"
	"github.com/twmb/franz-go/pkg/kgo"
)

type hgAcceptanceProducer struct {
	client *kgo.Client // 专用生产客户端，只投递本次事务性 Outbox 事件。
}

func (producer hgAcceptanceProducer) Send(ctx context.Context, topic, key string, payload []byte) error {
	return producer.client.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: []byte(key), Value: payload}).FirstErr()
}

// TestHGDebugKafkaSend 仅在显式开关和本机 debug 库运行；使用随机隔离身份，不启动应用其他 worker。
func TestHGDebugKafkaSend(t *testing.T) {
	if os.Getenv("MLC_NOTIFICATION_KAFKA_INTEGRATION") != "1" {
		t.Skip("未启用本机 debug 专用 Kafka 通知验收")
	}
	if os.Getenv("MLC_CONFIG_DIR") == "" {
		root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "config"))
		if err != nil {
			t.Fatal("无法定位本机 debug 配置目录")
		}
		t.Setenv("MLC_CONFIG_DIR", root)
	}
	if err := config.LoadConfig("debug"); err != nil {
		t.Fatal("无法读取 debug 配置")
	}
	dbCfg, err := config.GetMySQLConfig()
	if err != nil {
		t.Fatal("无法读取 debug MySQL 配置")
	}
	kafkaCfg, _, err := config.GetKafkaConfig()
	if err != nil {
		t.Fatal("无法读取 debug Kafka 配置")
	}
	if !hgAcceptanceLoopback(dbCfg.Host) || len(kafkaCfg.Business.Brokers) != 3 {
		t.Fatal("隔离验收只允许本机 MySQL 和三节点 Kafka")
	}
	for _, broker := range kafkaCfg.Business.Brokers {
		host, _, splitErr := net.SplitHostPort(broker)
		if splitErr != nil || !hgAcceptanceLoopback(host) {
			t.Fatal("隔离验收 Kafka broker 不是本机")
		}
	}
	dsn := (&mysql.Config{User: dbCfg.User, Passwd: dbCfg.Password, Net: "tcp",
		Addr: net.JoinHostPort(dbCfg.Host, dbCfg.Port), DBName: dbCfg.Database,
		ParseTime: true, Loc: time.UTC}).FormatDSN()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("无法打开 debug MySQL")
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	var version uint64
	var dirty bool
	if err := db.QueryRowContext(ctx, queries.HGGetCrawlerMigrationVersionSQL).Scan(&version, &dirty); err != nil || dirty || version < 86 {
		t.Fatal("debug 通知 Kafka 迁移尚未就绪")
	}
	var other uint64
	if err := db.QueryRowContext(ctx, queries.HGNotificationAcceptancePendingSQL).Scan(&other); err != sql.ErrNoRows {
		t.Fatal("存在其他 Kafka 待处理任务或检查失败，拒绝抢占")
	}
	var random [6]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal("无法创建隔离身份")
	}
	suffix := hex.EncodeToString(random[:])
	receiver := "hg_ntf_accept_" + suffix
	author := "hg_ntf_author_" + suffix
	reviewer := "hg_ntf_reviewer_" + suffix
	templateCode := "hg_ntf_accept." + suffix
	taskRequest := "hg-ntf-accept-" + suffix
	var templateID, taskID, outboxID uint64
	var runtime *hgkafkaruntime.HGRuntime
	// 即使断言失败也先停消费，再按本次新建身份与主键清理；清理失败保留现场并报告。
	t.Cleanup(func() {
		if runtime != nil {
			runtime.Close()
		}
		hgAcceptanceCleanup(t, db, receiver, author, reviewer, templateID, taskID, outboxID, templateCode)
	})
	t.Logf("隔离消费组（需在测试结束后删除）：mlc-go-debug-ntf-accept-%s", suffix)
	for _, user := range []string{receiver, author, reviewer} {
		_, err := db.ExecContext(ctx, queries.HGNotificationAcceptanceInsertUserSQL,
			user, user, "disabled-acceptance-login", suffix)
		if err != nil {
			t.Fatal("专用用户创建失败")
		}
	}
	for _, admin := range []string{author, reviewer} {
		_, err := db.ExecContext(ctx, queries.HGNotificationAcceptanceInsertAdminSQL,
			admin, "隔离验收", admin, "no-login-"+admin, "disabled-acceptance-login")
		if err != nil {
			t.Fatal("专用管理员创建失败")
		}
	}
	template := notification.HGTemplateForm{
		TemplateCode: templateCode, Name: "隔离通知", Category: "system",
		TitleTemplate: "隔离验收", BodyTemplate: "仅用于本机 Kafka 通知验收",
	}
	if err := notification.HGValidateTemplateForm(&template, true); err != nil {
		t.Fatal("测试模板格式无效")
	}
	templateID, err = notification.HGNewTemplateRepository(db).HGCreate(ctx, template, author)
	if err != nil {
		t.Fatal("测试模板创建失败")
	}
	if err := send.HGNewRepository(db).HGPublish(ctx, templateID, 1, reviewer); err != nil {
		t.Fatal("测试模板双人发布失败")
	}
	taskText, err := send.HGNewKafkaRepository(db).HGCreate(ctx, taskRequest, author, send.HGCommand{
		TemplateID: strconv.FormatUint(templateID, 10), TemplateVersion: 1,
		UserIDs: []string{receiver}, Variables: map[string]string{},
	})
	if err != nil {
		t.Fatal("专用 Kafka 任务创建失败")
	}
	taskID, err = strconv.ParseUint(taskText, 10, 64)
	if err != nil {
		t.Fatal("任务 ID 无效")
	}
	if queued, err := send.HGNewRepository(db).HGQueueKafkaOnce(ctx); err != nil || !queued {
		t.Fatalf("任务与 Outbox 未原子排队: %v", err)
	}
	var status string
	var next int
	var generation uint64
	if err := db.QueryRowContext(ctx, queries.HGNotificationAcceptanceTaskStateSQL, taskID, author).
		Scan(&status, &next, &generation, &outboxID); err != nil || status != "queued" || next != 0 || generation != 1 || outboxID == 0 {
		t.Fatal("Kafka 排队代次与游标异常")
	}
	kafkaCfg.Business.Consumers = hgkafka.HGConsumerGroupConfigs{
		NotificationSend: hgkafka.HGConsumerConfig{
			Enabled: true, GroupID: "mlc-go-debug-ntf-accept-" + suffix,
			ClientID: "mlc-go-debug-ntf-accept", Topics: []string{"mlc.notification.send.ready.v1"},
		},
	}
	worker := send.HGNewWorker(send.HGNewRepository(db),
		notification.HGNewService(notification.HGNewRepository(db)))
	runtime, err = hgkafkaruntime.NewRuntime(ctx, kafkaCfg.Business, hgkafkaruntime.RuntimeDependencies{
		NotificationSendWorker: worker,
	})
	if err != nil {
		t.Fatal("专用 Kafka 消费组初始化失败")
	}
	runtime.Start()
	opts, err := hgkafka.HGNewBusinessProducerOpts(kafkaCfg.Business)
	if err != nil {
		t.Fatal("Kafka 生产配置非法")
	}
	producer, err := kgo.NewClient(opts...)
	if err != nil {
		t.Fatal("Kafka 生产客户端不可用")
	}
	defer producer.Close()
	// 仅发布本次主键对应的 Outbox：通用 dispatcher 会抢占全库其他业务事件，不可在验收时运行。
	var payload []byte
	if err := db.QueryRowContext(ctx, queries.HGNotificationAcceptanceOutboxPayloadSQL, outboxID).Scan(&payload); err != nil {
		t.Fatal("本次 Outbox 事件不存在")
	}
	if err := (hgAcceptanceProducer{client: producer}).Send(ctx,
		"mlc.notification.send.ready.v1", taskText, payload); err != nil {
		t.Fatal("专用 Kafka 消息发布失败")
	}
	result, err := db.ExecContext(ctx, queries.HGNotificationAcceptanceOutboxPublishSQL, outboxID)
	if err != nil {
		t.Fatal("本次 Outbox 发布状态未写入")
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		t.Fatal("Outbox 发布状态与隔离主键不一致")
	}
	// 消费组进度可能在启动后分配；只按本次主键观察结果，超时即失败，绝不扩大投递人群。
	var processed bool
	for ctx.Err() == nil {
		if err := db.QueryRowContext(ctx, queries.HGNotificationAcceptanceTaskStateSQL, taskID, author).
			Scan(&status, &next, &generation, &outboxID); err != nil {
			t.Fatal("无法读取专用任务状态")
		}
		if status == "done" && next == 1 {
			processed = true
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !processed {
		t.Fatal("Kafka 消费未在时限内确认专用任务")
	}
	eventID := hgAcceptanceNoticeID(taskID, receiver)
	var inboxID uint64
	var read bool
	if err := db.QueryRowContext(ctx, queries.HGNotificationAcceptanceInboxSQL, receiver, eventID).Scan(&inboxID, &read); err != nil {
		t.Fatal("专用接收人未收到站内通知")
	}
	if err := db.QueryRowContext(ctx, queries.HGNotificationAcceptanceOutboxStatusSQL, outboxID).Scan(&status); err != nil || status != "published" {
		t.Fatal("Outbox 未确认发布")
	}
	t.Log("本机 debug：隔离身份经 Outbox → 专用 Kafka 消费组 → MySQL 收件箱投递成功")
}

func hgAcceptanceLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func hgAcceptanceNoticeID(taskID uint64, receiver string) string {
	hash := sha256.Sum256([]byte(receiver))
	return fmt.Sprintf("ntf-%d-%x", taskID, hash[:8])
}

// hgAcceptanceCleanup 严格按本次身份和主键清理，不通过全表前缀删除历史数据。
func hgAcceptanceCleanup(t *testing.T, db *sql.DB, receiver, author, reviewer string,
	templateID, taskID, outboxID uint64, templateCode string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Error("隔离数据清理事务未启动，需人工按测试身份核对")
		return
	}
	defer tx.Rollback()
	remove := func(query string, args ...any) bool {
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			t.Error("隔离数据清理失败，事务已回滚；需人工核对测试身份")
			return false
		}
		return true
	}
	if taskID != 0 {
		eventID := hgAcceptanceNoticeID(taskID, receiver)
		var inboxID uint64
		var read bool
		err = tx.QueryRowContext(ctx, queries.HGNotificationAcceptanceInboxSQL, receiver, eventID).Scan(&inboxID, &read)
		if err != nil && err != sql.ErrNoRows {
			t.Error("无法读取隔离通知，清理已中止")
			return
		}
		if err == nil {
			var counterReady bool
			if err := tx.QueryRowContext(ctx, queries.HGNotificationCounterReadySQL).Scan(&counterReady); err != nil {
				t.Error("通知计数状态不可用，清理已中止")
				return
			}
			if counterReady && !read {
				result, err := tx.ExecContext(ctx, queries.HGNotificationCounterDecreaseSQL,
					1, receiver, "system", inboxID%16, 1)
				if err != nil {
					t.Error("隔离通知计数回退失败，清理已中止")
					return
				}
				if affected, _ := result.RowsAffected(); affected != 1 {
					t.Error("隔离通知计数不一致，清理已中止")
					return
				}
			}
			if !remove(queries.HGNotificationAcceptanceDeleteInboxSQL, inboxID, receiver, eventID) ||
				!remove(queries.HGNotificationAcceptanceDeleteCounterSQL, receiver) {
				return
			}
		}
		if !remove(queries.HGNotificationAcceptanceDeleteLedgerSQL, receiver, eventID) ||
			!remove(queries.HGNotificationAcceptanceDeleteAuditSQL, taskID) ||
			!remove(queries.HGNotificationAcceptanceDeleteTaskSQL, taskID, author) {
			return
		}
	}
	if outboxID != 0 && !remove(queries.HGNotificationAcceptanceDeleteOutboxSQL, outboxID) {
		return
	}
	if templateID != 0 {
		if !remove(queries.HGNotificationAcceptanceDeleteTemplateAuditSQL, templateID) ||
			!remove(queries.HGNotificationAcceptanceDeleteVersionSQL, templateID) ||
			!remove(queries.HGNotificationAcceptanceDeleteTemplateSQL, templateID, templateCode) {
			return
		}
	}
	for _, admin := range []string{author, reviewer} {
		if !remove(queries.HGNotificationAcceptanceDeleteAdminSQL, admin, "no-login-"+admin) {
			return
		}
	}
	for _, user := range []string{receiver, author, reviewer} {
		if !remove(queries.HGNotificationAcceptanceDeleteUserSQL, user, user) {
			return
		}
	}
	if err := tx.Commit(); err != nil {
		t.Error("隔离数据清理提交失败，需人工核对")
	}
}

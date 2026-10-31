package im

import (
	"MLC_GO/internal/consumer"
	"MLC_GO/internal/events"
	IMEvents "MLC_GO/internal/events/im"
	"MLC_GO/internal/modules/im/transport"
	SQL "MLC_GO/internal/pkg/mysql/queries"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync/atomic"
	"time"
)

// HGRealtime 指针投递服务：Inbox 与任务同事务，网关发布在事务外，设备显式 ACK 才结束消息重试。
type HGRealtime struct {
	db              *sql.DB                 // 复用应用连接池。
	gateway         *transport.HGCentrifugo // 已验证配置的网关适配器。
	topic           string                  // 唯一 IM 专用 topic，禁止回退到业务 topic。
	publishFailures atomic.Uint64           // 网关发布失败计数，不含用户/消息标签。
	exhausted       atomic.Uint64           // 达到尝试上限的任务计数，不代表消息丢失或已读。
}

func HGNewRealtime(db *sql.DB, gateway *transport.HGCentrifugo, topic string) *HGRealtime {
	return &HGRealtime{db: db, gateway: gateway, topic: topic}
}

// HGRealtimeHint 是 v1 网关指针协议；不含私信正文，收到后经 JWT 资源授权补拉，按 seq/version 去重排序。
type HGRealtimeHint struct {
	EventID        string `json:"eventId"`             // 稳定 Outbox ID。
	EventType      string `json:"eventType"`           // message.committed 或 call.changed。
	EventVersion   int    `json:"eventVersion"`        // 当前固定为一。
	ConversationID string `json:"conversationId"`      // 会话主键十进制字符串。
	MessageID      string `json:"messageId,omitempty"` // 消息主键，字符串避免 JS 精度丢失。
	Seq            uint64 `json:"seq,omitempty"`       // 会话内权威顺序。
	CallID         string `json:"callId,omitempty"`    // 通话业务 ID。
	Version        uint64 `json:"version,omitempty"`   // 通话状态版本，旧事件不能回退客户端状态。
}

type hgTarget struct {
	device []byte // 16 字节设备主键。
	user   string // 权威设备归属。
}

// HGIngest 校验 Kafka 坐标及消息权威指针；仅在任务提交后允许框架提交 offset。
func (s *HGRealtime) HGIngest(ctx context.Context, e events.EventEnvelope) error {
	if s == nil || s.db == nil || s.gateway == nil {
		return fmt.Errorf("IM 实时服务未配置")
	}
	delivery, ok := consumer.DeliveryFromContext(ctx)
	if !ok || delivery.Topic != s.topic || delivery.Partition < 0 || delivery.Offset < 0 || e.Version != events.EventVersionV1 || len(e.EventID) < 1 || len(e.EventID) > 64 || !hgIMASCII(e.EventID) {
		return HGErrInvalid
	}
	if e.EventName != IMEvents.HGMessageCommittedEventName && e.EventName != IMEvents.HGCallChangedEventName {
		return fmt.Errorf("IM 专用 topic 包含未知事件")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	hash := sha256.Sum256(append([]byte(e.EventName+"\x00"+e.EventKey+"\x00"), e.Payload...))
	if _, err = tx.ExecContext(ctx, SQL.HGIMInboxInsertSQL, e.EventID, hash[:], delivery.Topic, delivery.Partition, delivery.Offset); err != nil {
		return err
	}
	var stored []byte
	if err = tx.QueryRowContext(ctx, SQL.HGIMInboxHashSQL, e.EventID).Scan(&stored); err != nil {
		return err
	}
	if !bytes.Equal(stored, hash[:]) {
		return HGErrConflict
	}
	hint := HGRealtimeHint{EventID: e.EventID, EventVersion: 1}
	var targets []hgTarget
	var messageID uint64
	if e.EventName == IMEvents.HGMessageCommittedEventName {
		var event IMEvents.HGMessageCommittedEvent
		if json.Unmarshal(e.Payload, &event) != nil || event.MessageID == 0 || event.ConversationID == 0 || event.Seq == 0 || event.EventKey() != e.EventKey {
			return HGErrInvalid
		}
		var conversation, seq uint64
		if err = tx.QueryRowContext(ctx, SQL.HGIMEventMessageSQL, event.MessageID).Scan(&conversation, &seq); err != nil {
			return err
		}
		if conversation != event.ConversationID || seq != event.Seq {
			return HGErrConflict
		}
		messageID = event.MessageID
		hint.EventType = "message.committed"
		hint.ConversationID = e.EventKey
		hint.MessageID = strconv.FormatUint(messageID, 10)
		hint.Seq = seq
		rows, err := tx.QueryContext(ctx, SQL.HGIMEventDevicesSQL, messageID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var target hgTarget
			if err = rows.Scan(&target.device, &target.user); err != nil {
				break
			}
			targets = append(targets, target)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return err
		}
	} else {
		var event IMEvents.HGCallChangedEvent
		if json.Unmarshal(e.Payload, &event) != nil || event.EventKey() != e.EventKey || event.VersionID == 0 {
			return HGErrInvalid
		}
		id, err := hgRandomID(event.CallID)
		if err != nil {
			return err
		}
		var conversation, version uint64
		var caller, callee string
		if err = tx.QueryRowContext(ctx, SQL.HGIMCallEventSQL, id).Scan(&conversation, &caller, &callee, &version); err != nil {
			return err
		}
		if conversation != event.ConversationID || event.VersionID > version {
			return HGErrConflict
		}
		hint.EventType = "call.changed"
		hint.CallID = event.CallID
		hint.ConversationID = e.EventKey
		hint.Version = event.VersionID
		for _, user := range []string{caller, callee} {
			rows, err := tx.QueryContext(ctx, SQL.HGIMActiveDevicesSQL, user)
			if err != nil {
				return err
			}
			for rows.Next() {
				var device []byte
				if err = rows.Scan(&device); err != nil {
					break
				}
				targets = append(targets, hgTarget{device: device, user: user})
			}
			if err == nil {
				err = rows.Err()
			}
			rows.Close()
			if err != nil {
				return err
			}
		}
	}
	if len(targets) > 10 {
		return fmt.Errorf("IM 活跃设备超过单聊上限")
	}
	payload, err := json.Marshal(hint)
	if err != nil {
		return err
	}
	for _, target := range targets {
		if _, err = tx.ExecContext(ctx, SQL.HGIMJobInsertSQL, e.EventID, target.device, target.user, messageID, payload); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type hgRealtimeJob struct {
	id       uint64          // 队列主键。
	device   []byte          // 目标设备。
	user     string          // 目标归属。
	message  uint64          // 消息 ID，通话为零。
	payload  json.RawMessage // 安全指针，不含正文。
	attempts int             // 已发生的尝试数。
}

// HGDispatchOnce 每次最多四条，每条发布最多三秒；30 秒租约在进程故障后允许其他节点接管。
func (s *HGRealtime) HGDispatchOnce(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var jobs []hgRealtimeJob
	// 优先接管崩溃租约，随后新任务；每轮仍只处理四条。
	for _, state := range []int{1, 0} {
		rows, err := tx.QueryContext(ctx, SQL.HGIMJobClaimSQL, state)
		if err != nil {
			return err
		}
		for rows.Next() {
			var job hgRealtimeJob
			if err = rows.Scan(&job.id, &job.device, &job.user, &job.message, &job.payload, &job.attempts); err != nil {
				break
			}
			jobs = append(jobs, job)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return err
		}
		if len(jobs) > 0 {
			break
		}
	}
	var lease [16]byte
	if _, err = rand.Read(lease[:]); err != nil {
		return err
	}
	for _, job := range jobs {
		if _, err = tx.ExecContext(ctx, SQL.HGIMJobLeaseSQL, lease[:], job.id); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	var firstPublishError error
	for _, job := range jobs {
		state := 0
		var found int
		err = s.db.QueryRowContext(ctx, SQL.HGIMJobDeviceActiveSQL, job.device, job.user).Scan(&found)
		if errors.Is(err, sql.ErrNoRows) {
			state = 2
		} else if err != nil {
			return err
		}
		if state == 0 && job.message != 0 {
			err = s.db.QueryRowContext(ctx, SQL.HGIMJobMessagePendingSQL, job.device, job.message, job.user).Scan(&found)
			if errors.Is(err, sql.ErrNoRows) {
				state = 2
			} else if err != nil {
				return err
			}
		}
		if state == 0 {
			if job.attempts >= 12 {
				state = 3
			} else {
				publishCtx, stop := context.WithTimeout(ctx, 3*time.Second)
				publishErr := s.gateway.HGPublish(publishCtx, hex.EncodeToString(job.device), job.payload)
				stop()
				if publishErr != nil {
					s.publishFailures.Add(1)
					if firstPublishError == nil {
						firstPublishError = publishErr
					}
				}
				if publishErr == nil && job.message == 0 {
					state = 2
				} // 通话仅确认信令发布，不称为用户送达。
				if state == 0 && job.attempts+1 >= 12 {
					state = 3
				}
			}
		}
		delay := 1 << min(job.attempts+1, 8)
		result, finishErr := s.db.ExecContext(ctx, SQL.HGIMJobFinishSQL, state, delay, job.id, lease[:])
		if finishErr != nil {
			return finishErr
		}
		updated, finishErr := result.RowsAffected()
		if finishErr != nil {
			return finishErr
		}
		if updated != 1 {
			return fmt.Errorf("IM 投递任务租约已变化，等待新持有者处理")
		}
		if state == 3 {
			s.exhausted.Add(1)
		}
	}
	return firstPublishError
}

// HGWriteMetrics 仅输出进程级计数；持久任务积压须另按有界索引采样观察。
func (s *HGRealtime) HGWriteMetrics(w io.Writer) {
	if s == nil {
		return
	}
	fmt.Fprintf(w, "# TYPE mlc_im_publish_failures_total counter\nmlc_im_publish_failures_total %d\n# TYPE mlc_im_delivery_exhausted_total counter\nmlc_im_delivery_exhausted_total %d\n", s.publishFailures.Load(), s.exhausted.Load())
}

func hgRandomID(raw string) ([]byte, error) {
	id, err := hex.DecodeString(raw)
	if err != nil || len(id) != 16 || raw != hex.EncodeToString(id) {
		return nil, HGErrInvalid
	}
	return id, nil
}

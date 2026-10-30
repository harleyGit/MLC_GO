package im

import (
	"MLC_GO/internal/events"
	IMEventsPackage "MLC_GO/internal/events/im"
	"MLC_GO/internal/outbox"
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"
)

var (
	HGErrInvalid   = errors.New("私信参数不合法")
	HGErrForbidden = errors.New("非会话成员")
	HGErrConflict  = errors.New("私信幂等键与原内容冲突")
)

// HGService 是单聊权威存储，不复用站内信或弹幕。调用方必须从已验证 JWT 注入 user ID。
type HGService struct {
	db    *sql.DB           // 由应用管理生命周期的 MySQL 连接池。
	topic string            // 独立 IM Kafka topic；未配置时不可发送。
	media HGMediaAuthorizer // 私有媒体归属和审核状态校验器，nil 时媒体发送失败关闭。
}

// HGNewService 创建独立私信业务服务；发送前必须部署 im_message_events_v1 topic 与表迁移。
func HGNewService(db *sql.DB, topic string, media HGMediaAuthorizer) *HGService {
	return &HGService{db: db, topic: topic, media: media}
}

// HGCreateConversation 根据两名用户的稳定排序唯一创建单聊；先检查对方存在，再在短事务中写会话与成员。
func (s *HGService) HGCreateConversation(ctx context.Context, user, peer string) (string, error) {
	if s == nil || s.db == nil || len(user) == 0 || len(peer) == 0 || len(user) > 128 || len(peer) > 128 || user == peer {
		return "", HGErrInvalid
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, SQLQueriesPackage.HGIMUserExistsSQL, peer).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", HGErrInvalid
		}
		return "", err
	}
	low, high := user, peer
	if low > high {
		low, high = high, low
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.HGIMCreateConversationSQL, low, high)
	if err != nil {
		return "", err
	}
	id, err := result.LastInsertId()
	if err != nil || id < 1 {
		return "", fmt.Errorf("获取会话 ID 失败: %w", err)
	}
	for _, member := range []string{low, high} {
		if _, err := tx.ExecContext(ctx, SQLQueriesPackage.HGIMAddMemberSQL, id, member); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return strconv.FormatInt(id, 10), nil
}

// HGSend 在会话行锁下先检查幂等键、验证成员并分配 seq；消息、设备投递与 Outbox 同事务提交。
// 返回 ACK 仅表示服务端持久化，绝不把 WebSocket 写成功解释为设备已送达。
func (s *HGService) HGSend(ctx context.Context, sender string, cmd HGSendCommand) (HGSendReceipt, error) {
	if s == nil || s.db == nil || s.topic == "" || len(sender) == 0 || len(sender) > 128 || len(cmd.ClientMsgID) < 1 || len(cmd.ClientMsgID) > 64 || !hgIMASCII(cmd.ClientMsgID) || len(cmd.Content) == 0 || len(cmd.Content) > 4096 {
		return HGSendReceipt{}, HGErrInvalid
	}
	id, err := strconv.ParseUint(cmd.ConversationID, 10, 63)
	if err != nil || id == 0 {
		return HGSendReceipt{}, HGErrInvalid
	}
	content, mediaID, err := hgIMContent(cmd)
	if err != nil {
		return HGSendReceipt{}, err
	}
	if mediaID != "" {
		if s.media == nil {
			return HGSendReceipt{}, HGErrForbidden
		}
		// 私有对象存储/审核 I/O 必须在事务外完成；提交前由媒体权威状态再次校验，审核撤销需阻断下载。
		if err := s.media.HGAuthorizeSend(ctx, sender, mediaID, cmd.MessageType); err != nil {
			return HGSendReceipt{}, err
		}
	}
	hash := sha256.Sum256(append([]byte(string(cmd.MessageType)+"\x00"), content...))
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return HGSendReceipt{}, err
	}
	defer tx.Rollback()
	var low, high string
	var previous uint64
	if err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGIMLockConversationSQL, id).Scan(&low, &high, &previous); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGSendReceipt{}, HGErrForbidden
		}
		return HGSendReceipt{}, err
	}
	if sender != low && sender != high {
		return HGSendReceipt{}, HGErrForbidden
	}
	var member int
	if err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGIMConversationMemberSQL, id, sender).Scan(&member); err != nil {
		return HGSendReceipt{}, HGErrForbidden
	}
	var existingID, existingConversation, existingSeq uint64
	var existingType string
	var existingHash []byte
	err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGIMExistingMessageSQL, sender, cmd.ClientMsgID).Scan(&existingID, &existingConversation, &existingSeq, &existingType, &existingHash)
	if err == nil {
		if existingConversation != id || existingType != string(cmd.MessageType) || !bytes.Equal(existingHash, hash[:]) {
			return HGSendReceipt{}, HGErrConflict
		}
		return HGSendReceipt{MessageID: strconv.FormatUint(existingID, 10), Seq: existingSeq}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return HGSendReceipt{}, err
	}
	if previous >= uint64(^uint64(0)>>1) {
		return HGSendReceipt{}, HGErrInvalid
	}
	seq := previous + 1
	result, err := tx.ExecContext(ctx, SQLQueriesPackage.HGIMAdvanceSeqSQL, seq, id, previous)
	if err != nil {
		return HGSendReceipt{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return HGSendReceipt{}, fmt.Errorf("分配会话序号失败: %w", err)
	}
	result, err = tx.ExecContext(ctx, SQLQueriesPackage.HGIMInsertMessageSQL, id, seq, sender, cmd.ClientMsgID, cmd.MessageType, content, hash[:])
	if err != nil {
		return HGSendReceipt{}, err
	}
	messageID, err := result.LastInsertId()
	if err != nil || messageID < 1 {
		return HGSendReceipt{}, fmt.Errorf("获取消息 ID 失败: %w", err)
	}
	for _, user := range []string{low, high} {
		rows, err := tx.QueryContext(ctx, SQLQueriesPackage.HGIMActiveDevicesSQL, user)
		if err != nil {
			return HGSendReceipt{}, err
		}
		var devices [][]byte
		for rows.Next() {
			var device []byte
			if err = rows.Scan(&device); err != nil {
				break
			}
			devices = append(devices, device)
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return HGSendReceipt{}, err
		}
		if len(devices) > 5 {
			return HGSendReceipt{}, fmt.Errorf("设备数量超过上限")
		}
		for _, device := range devices {
			if _, err := tx.ExecContext(ctx, SQLQueriesPackage.HGIMInsertDeliverySQL, device, messageID, user); err != nil {
				return HGSendReceipt{}, err
			}
		}
	}
	if err := outbox.NewRepository(s.db, s.topic).SaveTx(ctx, tx, IMEventsPackage.HGMessageCommittedEvent{
		EventMeta: events.NewEventMeta(ctx), ConversationID: id, MessageID: uint64(messageID), Seq: seq,
	}); err != nil {
		return HGSendReceipt{}, err
	}
	if err := tx.Commit(); err != nil {
		return HGSendReceipt{}, err
	}
	return HGSendReceipt{MessageID: strconv.FormatInt(messageID, 10), Seq: seq}, nil
}

func hgIMASCII(value string) bool {
	for _, b := range []byte(value) {
		if b < 33 || b > 126 {
			return false
		}
	}
	return true
}

func hgIMContent(cmd HGSendCommand) ([]byte, string, error) {
	decoder := json.NewDecoder(bytes.NewReader(cmd.Content))
	decoder.DisallowUnknownFields()
	var mediaID string
	switch cmd.MessageType {
	case HGText:
		var value struct {
			Text string `json:"text"`
		}
		if err := decoder.Decode(&value); err != nil || utf8.RuneCountInString(value.Text) < 1 || utf8.RuneCountInString(value.Text) > 1000 {
			return nil, "", HGErrInvalid
		}
	case HGImage, HGVideo:
		var value HGMediaContent
		if err := decoder.Decode(&value); err != nil || len(value.MediaID) < 1 || len(value.MediaID) > 64 || value.Width <= 0 || value.Height <= 0 || value.Width > 16384 || value.Height > 16384 || value.DurationMS < 0 || (cmd.MessageType == HGImage && value.DurationMS != 0) {
			return nil, "", HGErrInvalid
		}
		mediaID = value.MediaID
	default:
		return nil, "", HGErrInvalid
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, "", HGErrInvalid
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, cmd.Content); err != nil {
		return nil, "", HGErrInvalid
	}
	return compact.Bytes(), mediaID, nil
}

package im

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// HGDeviceCredential 只在注册响应中返回一次；secret 是设备凭据，不得出现在 URL、日志或 Kafka 中。
type HGDeviceCredential struct {
	DeviceID string `json:"deviceId"` // 服务端生成的 128 位随机设备 ID，绑定当前 JWT 用户。
	Secret   string `json:"secret"`   // 256 位随机凭据，数据库仅保留摘要。
}

// HGDeviceDelivery 是单台设备尚未 ACK 的消息指针，消息正文仍需按会话成员授权读取。
type HGDeviceDelivery struct {
	MessageID      string `json:"messageId"`      // MySQL 消息主键，作为有界补拉游标。
	ConversationID string `json:"conversationId"` // 单聊会话主键。
	Seq            uint64 `json:"seq"`            // 会话顺序号。
}

// HGRegisterDevice 串行检查用户活跃设备上限；30 天未活动设备不参与投递，历史由会话补拉。
func (s *HGService) HGRegisterDevice(ctx context.Context, user string) (HGDeviceCredential, error) {
	if s == nil || s.db == nil || user == "" || len(user) > 128 {
		return HGDeviceCredential{}, HGErrInvalid
	}
	var id [16]byte
	var secret [32]byte
	if _, err := rand.Read(id[:]); err != nil {
		return HGDeviceCredential{}, err
	}
	if _, err := rand.Read(secret[:]); err != nil {
		return HGDeviceCredential{}, err
	}
	digest := sha256.Sum256(secret[:])
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return HGDeviceCredential{}, err
	}
	defer tx.Rollback()
	var found int
	if err = tx.QueryRowContext(ctx, SQLQueriesPackage.HGIMLockDeviceOwnerSQL, user).Scan(&found); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HGDeviceCredential{}, HGErrForbidden
		}
		return HGDeviceCredential{}, err
	}
	rows, err := tx.QueryContext(ctx, SQLQueriesPackage.HGIMActiveDevicesSQL, user)
	if err != nil {
		return HGDeviceCredential{}, err
	}
	count := 0
	for rows.Next() {
		count++
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return HGDeviceCredential{}, err
	}
	rows.Close()
	if count >= 5 {
		return HGDeviceCredential{}, fmt.Errorf("最多保留五台活跃设备")
	}
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.HGIMCreateDeviceSQL, id[:], user, digest[:]); err != nil {
		return HGDeviceCredential{}, err
	}
	if err := tx.Commit(); err != nil {
		return HGDeviceCredential{}, err
	}
	return HGDeviceCredential{DeviceID: hex.EncodeToString(id[:]), Secret: hex.EncodeToString(secret[:])}, nil
}

func (s *HGService) hgDevice(ctx context.Context, user string, credential HGDeviceCredential) ([]byte, error) {
	if s == nil || s.db == nil || user == "" {
		return nil, HGErrForbidden
	}
	id, idErr := hex.DecodeString(credential.DeviceID)
	secret, secretErr := hex.DecodeString(credential.Secret)
	if idErr != nil || secretErr != nil || len(id) != 16 || len(secret) != 32 {
		return nil, HGErrForbidden
	}
	var stored []byte
	if err := s.db.QueryRowContext(ctx, SQLQueriesPackage.HGIMDeviceCredentialSQL, id, user).Scan(&stored); err != nil {
		return nil, HGErrForbidden
	}
	hash := sha256.Sum256(secret)
	if subtle.ConstantTimeCompare(hash[:], stored) != 1 {
		return nil, HGErrForbidden
	}
	return id, nil
}

// HGDevicePending 仅返回当前认证设备未确认的有界消息指针；重连从消息游标补拉并逐条 ACK。
func (s *HGService) HGDevicePending(ctx context.Context, user string, credential HGDeviceCredential, after uint64, limit int) ([]HGDeviceDelivery, error) {
	if limit < 1 || limit > 100 || after > uint64(^uint64(0)>>1) {
		return nil, HGErrInvalid
	}
	id, err := s.hgDevice(ctx, user, credential)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, SQLQueriesPackage.HGIMPendingDeviceSQL, id, user, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]HGDeviceDelivery, 0, limit)
	for rows.Next() {
		var messageID, conversationID uint64
		var seq uint64
		if err := rows.Scan(&messageID, &conversationID, &seq); err != nil {
			return nil, err
		}
		items = append(items, HGDeviceDelivery{MessageID: strconv.FormatUint(messageID, 10), ConversationID: strconv.FormatUint(conversationID, 10), Seq: seq})
	}
	return items, rows.Err()
}

// HGAckDeviceMessage 仅在客户端显式确认后更新本设备的投递状态；重复确认幂等。
func (s *HGService) HGAckDeviceMessage(ctx context.Context, user string, credential HGDeviceCredential, messageID uint64) error {
	if messageID == 0 || messageID > uint64(^uint64(0)>>1) {
		return HGErrInvalid
	}
	id, err := s.hgDevice(ctx, user, credential)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, SQLQueriesPackage.HGIMAcknowledgeDeviceSQL, id, user, messageID)
	return err
}

// HGRevokeDevice 撤销设备凭据；已撤销设备的未确认记录保留给有界离线清理，不再参与重试。
func (s *HGService) HGRevokeDevice(ctx context.Context, user string, credential HGDeviceCredential) error {
	id, err := s.hgDevice(ctx, user, credential)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, SQLQueriesPackage.HGIMRevokeDeviceSQL, id, user)
	return err
}

// HGHeartbeatDevice 只有成功验证设备凭据后才续租；服务端以 30 天无活动作为失活边界。
func (s *HGService) HGHeartbeatDevice(ctx context.Context, user string, credential HGDeviceCredential) error {
	id, err := s.hgDevice(ctx, user, credential)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err = s.db.ExecContext(ctx, SQLQueriesPackage.HGIMTouchDeviceSQL, id, user)
	return err
}

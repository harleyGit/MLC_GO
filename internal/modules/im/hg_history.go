package im

import (
	SQLQueriesPackage "MLC_GO/internal/pkg/mysql/queries"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

// HGHistoryMessage 是经本人会话成员鉴权后返回的消息快照；媒体内容仅含私有 mediaId，不含公开对象地址。
type HGHistoryMessage struct {
	MessageID string          `json:"messageId"`   // 服务端持久化消息 ID。
	Seq       uint64          `json:"seq"`         // 同会话服务端递增游标。
	SenderID  string          `json:"senderId"`    // 消息发送人 ID。
	Type      HGMessageType   `json:"messageType"` // 受支持的内容类型。
	Content   json.RawMessage `json:"content"`     // 文本或经审核的私有媒体引用。
	CreatedAt time.Time       `json:"createdAt"`   // MySQL 消息创建时间。
}

// HGConversation 是本人单聊列表的有界快照；未读数不能用 last_seq-last_read_seq 推断（自己发送不计未读）。
type HGConversation struct {
	ConversationID string    `json:"conversationId"` // 单聊主键。
	LastSeq        uint64    `json:"lastSeq"`        // 服务器会话最新消息序号。
	LastReadSeq    uint64    `json:"lastReadSeq"`    // 本人已读位置。
	UpdatedAt      time.Time `json:"updatedAt"`      // 会话最后活动时间。
}

// HGHistory 每次最多读取 100 条，以 seq 递增补洞；会话成员校验在服务端进行。
func (s *HGService) HGHistory(ctx context.Context, user string, conversationID, after uint64, limit int) ([]HGHistoryMessage, error) {
	if s == nil || s.db == nil || user == "" || conversationID == 0 || limit < 1 || limit > 100 || after > uint64(^uint64(0)>>1) {
		return nil, HGErrInvalid
	}
	var member int
	if err := s.db.QueryRowContext(ctx, SQLQueriesPackage.HGIMConversationMemberSQL, conversationID, user).Scan(&member); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, HGErrForbidden
		}
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, SQLQueriesPackage.HGIMHistorySQL, conversationID, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]HGHistoryMessage, 0, limit)
	for rows.Next() {
		var item HGHistoryMessage
		var id uint64
		if err = rows.Scan(&id, &item.Seq, &item.SenderID, &item.Type, &item.Content, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.MessageID = strconv.FormatUint(id, 10)
		items = append(items, item)
	}
	return items, rows.Err()
}

// HGConversations 按会话主键倒序、有界读取本人会话；没有深 OFFSET。
func (s *HGService) HGConversations(ctx context.Context, user string, before uint64, limit int) ([]HGConversation, error) {
	if s == nil || s.db == nil || user == "" || before == 0 || limit < 1 || limit > 50 {
		return nil, HGErrInvalid
	}
	rows, err := s.db.QueryContext(ctx, SQLQueriesPackage.HGIMConversationPageSQL, user, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]HGConversation, 0, limit)
	for rows.Next() {
		var item HGConversation
		var id uint64
		if err := rows.Scan(&id, &item.LastSeq, &item.LastReadSeq, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.ConversationID = strconv.FormatUint(id, 10)
		items = append(items, item)
	}
	return items, rows.Err()
}

// HGMarkRead 检查本人会话归属和 seq 上界；客户端不能将已读位置推进到不存在的消息之后。
func (s *HGService) HGMarkRead(ctx context.Context, user string, conversationID, seq uint64) error {
	if s == nil || s.db == nil || user == "" || conversationID == 0 || seq > uint64(^uint64(0)>>1) {
		return HGErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var low, high string
	var last uint64
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.HGIMLockConversationSQL, conversationID).Scan(&low, &high, &last); err != nil {
		return HGErrForbidden
	}
	if user != low && user != high {
		return HGErrForbidden
	}
	if seq > last {
		return HGErrInvalid
	}
	var member int
	if err := tx.QueryRowContext(ctx, SQLQueriesPackage.HGIMConversationMemberSQL, conversationID, user).Scan(&member); err != nil {
		return HGErrForbidden
	}
	if _, err := tx.ExecContext(ctx, SQLQueriesPackage.HGIMReadSQL, seq, conversationID, user); err != nil {
		return err
	}
	return tx.Commit()
}
